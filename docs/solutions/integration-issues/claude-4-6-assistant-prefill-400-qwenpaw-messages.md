---
title: Claude 4.6 assistant-final QwenPaw messages return prefill 400
date: 2026-05-26
category: integration-issues
module: relay/claude
problem_type: integration_issue
component: assistant
symptoms:
  - "Claude 4.6 /v1/messages requests returned HTTP 400: This model does not support assistant message prefill"
  - "Production QwenPaw request bodies ended as user -> assistant with plain text assistant content"
  - "Successful requests in the same session worked once the conversation became user -> assistant -> user"
root_cause: wrong_api
resolution_type: code_fix
severity: high
related_components:
  - "billing"
  - "request-conversion"
tags: [claude-4-6, anthropic-messages, assistant-prefill, qwenpaw, billing-fallback, request-transform]
---

# Claude 4.6 assistant-final QwenPaw messages return prefill 400

## Problem
Claude 4.6 and 4.7 reject Anthropic Messages requests whose final turn is an assistant prefill. In production, QwenPaw sent a legitimate conversation history shaped as `user -> assistant`, but because that assistant turn was also the final message, upstream returned a 400 before generation started.

The fix is a narrow wire-only compatibility transform in the Claude relay path: preserve the client-supplied assistant message, append a synthetic user continuation only to the outbound upstream request, and keep billing aligned with the actual outbound request.

## Symptoms
- Repeated `/v1/messages` failures for `claude-opus-4-6` on channel 17 with `status_code=400`.
- The upstream error text was: `This model does not support assistant message prefill. The conversation must end with a user message.`
- The failing production body had two messages: the original user question and a plain text assistant continuation such as `我来帮你查一下 Grok 最新模型的信息。`
- Earlier `browser_use` output in QwenPaw was text-protocol content in the assistant stream, not native Anthropic `tool_use` blocks.
- A later request in the same session succeeded after a new user message made the history `user -> assistant -> user`.

## What Didn't Work
- Stripping the assistant message is wrong. The assistant turn has already been displayed to the user and carries real conversation state.
- Rewriting the assistant as a user turn is also wrong because it changes speaker semantics and can leak implementation details into the model's context.
- Applying the transform after adaptor conversion is too late (session history). Claude-compatible adaptors such as Vertex can wrap the request, so a helper that expects `*dto.ClaudeRequest` may no longer see the original messages.
- Only changing the outbound body without updating fallback prompt-token estimation is incomplete (session history). If upstream succeeds but omits usage, local fallback billing can undercount because the estimate was captured before the synthetic continuation existed.
- Broadly transforming native `tool_use`, `tool_result`, or `thinking` blocks would invent tool state. The observed incident was plain text, so structured assistant-final content should keep existing upstream failure semantics until it has a specific repair design.

## Solution
Add the compatibility helper before adaptor-specific wrapping in the non-passthrough Claude request body builder:

```go
if applyClaudeAssistantPrefillCompatibility(request, info) {
    tokens, err := service.EstimateRequestToken(c, request.GetTokenCountMeta(), info)
    if err != nil {
        return nil, types.NewError(err, types.ErrorCodeCountTokenFailed, types.ErrOptionWithSkipRetry())
    }
    info.SetEstimatePromptTokens(tokens)
}

convertedRequest, err := adaptor.ConvertClaudeRequest(c, info, request)
```

The helper is intentionally narrow:

```go
func applyClaudeAssistantPrefillContinuation(request *dto.ClaudeRequest) bool {
    if request == nil || !isClaudeAssistantPrefillUnsupportedModel(request.Model) || len(request.Messages) == 0 {
        return false
    }

    finalMessage := request.Messages[len(request.Messages)-1]
    if finalMessage.Role != "assistant" || !isPlainTextClaudeContent(&finalMessage) {
        return false
    }

    request.Messages = append(request.Messages, dto.ClaudeMessage{
        Role:    "user",
        Content: claudeAssistantPrefillContinuationText,
    })
    return true
}
```

The implementation records the wire-only transform as `request_conversion_meta: ["assistant_prefill_continuation"]`, making production logs explain why the upstream body differs from the client body.

Billing remains upstream-usage first. `adaptor.DoResponse` parses Claude `usage`, then `service.PostTextConsumeQuota` settles from that usage. The important guardrail is fallback: when the transform runs, `info.SetEstimatePromptTokens` is updated from the transformed request so `ResponseText2Usage(..., info.GetEstimatePromptTokens())` can still bill successful responses when upstream usage is missing.

## Why This Works
Claude's restriction is about the final message being an assistant prefill, not about having assistant messages anywhere in history. Appending a small synthetic user continuation changes only the final-turn shape seen by Claude while preserving the original assistant history.

Keeping the transform before `ConvertClaudeRequest` ensures every downstream Claude-compatible adaptor receives the already-normalized request. Keeping it outside passthrough preserves the contract that passthrough sends the raw client body.

The billing behavior stays consistent with the rest of the relay:

- Normal success path: upstream Claude `usage.input_tokens` and `usage.output_tokens` are authoritative.
- Stream fallback path: local completion estimate is paired with the transformed prompt-token estimate.
- Diagnostics path: `request_conversion_meta` records that compatibility was applied.

## Prevention
- Keep regression tests for string assistant content, text-block assistant content, older Claude models, already user-final requests, native `tool_use`, native `thinking`, nil/empty requests, and production-shaped QwenPaw input.
- Test the relay boundary, not only the helper: non-passthrough should append the synthetic user, passthrough should remain byte-for-byte unchanged.
- Assert fallback prompt-token estimation increases after the synthetic continuation so successful no-usage responses do not become free.
- Keep the conversion marker idempotent so repeated calls do not create duplicate metadata.
- When diagnosing Claude upstream errors, inspect both the client body and the final upstream body shape; the compatibility marker explains intentional differences.

## Related Issues
- [channel-test-503-no-available-accounts-2026-04-20.md](channel-test-503-no-available-accounts-2026-04-20.md) — related Claude `/v1/messages` upstream-error attribution pattern.
- [../best-practices/public-group-based-billing-split-requires-code-changes-2026-04-22.md](../best-practices/public-group-based-billing-split-requires-code-changes-2026-04-22.md) — related principle that pre-consume, retry, final settlement, logs, and pricing must share one effective billing decision.
- [../best-practices/fork-public-group-billing-vs-upstream-tiered-expr-maintenance-boundary-2026-04-24.md](../best-practices/fork-public-group-billing-vs-upstream-tiered-expr-maintenance-boundary-2026-04-24.md) — related distinction between upstream usage attribution and local fallback/maintenance boundaries.
- Anthropic Claude 4 migration guidance: <https://platform.claude.com/docs/en/docs/about-claude/models/migrating-to-claude-4>
