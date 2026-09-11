---
title: Validate TryValo gpt-image-2 image sizes before changing new-api
date: 2026-05-14
category: best-practices
module: relay/image-upstream-validation
problem_type: best_practice
component: development_workflow
severity: medium
applies_when:
  - Validating a new-api-compatible upstream for OpenAI image generation or image edit endpoints
  - Testing gpt-image-2 support against /v1/images/generations and /v1/images/edits
  - Building a concrete size compatibility matrix for image providers
related_components:
  - relay
  - image-generation
  - image-edits
  - upstream-provider
tags: [tryvalo, gpt-image-2, image-generation, image-edits, size-matrix, upstream-validation, http-timeout]
---

# Validate TryValo gpt-image-2 image sizes before changing new-api

## Context
The upstream target was an OpenAI-compatible image API at `https://api.tryvalo.com`, intended to be connected through `new-api` and resold downstream.

The integration question was whether `gpt-image-2` needed special size mapping inside `new-api`, or whether downstream clients could send concrete `size` strings such as `3840x2160` and let `new-api` pass them through as-is.

Live verification on 2026-05-14 showed that concrete size strings work for both image generation and image edits. No code change is needed solely for 1K/2K/4K size support when the upstream is already exposed through an OpenAI-compatible image route.

Session history was checked during documentation. After excluding the current conversation and its compound subagents, no directly reusable prior session material was available.

## Guidance
Before changing relay code, test the upstream endpoint directly with the exact model and size strings downstream clients will use.

Use this validation shape:

1. Confirm the model exists via `GET /v1/models`.
2. Test `POST /v1/images/generations`.
3. Test `POST /v1/images/edits` with `multipart/form-data`.
4. Save every returned image.
5. Verify actual PNG dimensions, not just HTTP success.
6. Use a long client timeout for large edit requests.

The validated request sizes were:

| Tier | Size | Ratio |
|---|---:|---|
| 1K | `1024x1024` | 1:1 |
| 2K | `1536x1024` | 3:2 |
| 2K | `1024x1536` | 2:3 |
| 2K | `1792x1024` | wide |
| 2K | `1024x1792` | tall |
| 2K | `2048x2048` | 1:1 |
| 2K | `2048x1152` | 16:9 |
| 2K | `1152x2048` | 9:16 |
| 4K | `3840x2160` | 16:9 |
| 4K | `2160x3840` | 9:16 |

Both endpoints succeeded for the full matrix:

- `POST /v1/images/generations`
- `POST /v1/images/edits`

Do not treat aliases like `2k` or `4k` as verified request values for this upstream path. The verified compatibility contract is concrete `widthxheight` strings.

## Why This Matters
For OpenAI-compatible upstreams, integration bugs often come from assuming the gateway must translate, normalize, or restrict fields before proving what the upstream actually accepts.

Here, direct testing showed that `gpt-image-2` accepts all 10 concrete size strings on both generation and edit endpoints. Preserving pass-through behavior is the safer default unless another concrete incompatibility appears.

This also prevents a misleading conclusion from local tooling limits. The first edit run falsely failed on 8 larger sizes because the local PowerShell script used .NET `HttpClient`'s default 100 second timeout. Retrying with a 15 minute timeout succeeded. Those were client-side timeouts, not upstream size rejections.

## When to Apply
- Adding or validating a new OpenAI-compatible upstream image provider.
- A provider claims support for a model like `gpt-image-2`, but accepted sizes are unclear.
- Deciding whether `new-api` needs provider-specific size mapping.
- Testing large image edit requests that may exceed default HTTP client timeouts.
- Verifying upstream behavior before changing relay, DTO, or billing logic.

## Examples
Model check:

```powershell
$base = "https://api.tryvalo.com"
$headers = @{
  Authorization = "Bearer $env:TRYVALO_API_KEY"
}

Invoke-RestMethod `
  -Method Get `
  -Uri "$base/v1/models" `
  -Headers $headers
```

Generation request shape:

```bash
curl https://api.tryvalo.com/v1/images/generations \
  -H "Authorization: Bearer REDACTED" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "a simple integration test image",
    "size": "2048x1152",
    "response_format": "b64_json"
  }'
```

Edit request shape:

```bash
curl https://api.tryvalo.com/v1/images/edits \
  -H "Authorization: Bearer REDACTED" \
  -F model="gpt-image-2" \
  -F prompt="make a small visible edit for integration testing" \
  -F size="2048x1152" \
  -F response_format="b64_json" \
  -F image="@input.png"
```

PowerShell timeout guard for large edits:

```powershell
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromMinutes(15)
```

Dimension verification:

```powershell
Add-Type -AssemblyName System.Drawing

$image = [System.Drawing.Image]::FromFile("output.png")
"$($image.Width)x$($image.Height)"
$image.Dispose()
```

## Related
- `docs/solutions/best-practices/tryvalo-responses-image-generation-best-practice-2026-04-24.md` — adjacent TryValo image-generation note for `/v1/responses`; keep it distinct from `/v1/images/generations` and `/v1/images/edits`.
- `docs/solutions/integration-issues/channel-test-503-no-available-accounts-2026-04-20.md` — useful when separating upstream account/channel failures from local integration problems.
- Test artifacts from this verification run: `C:\learning\project\new-api\tmp\tryvalo-full-test-20260514-155658`.
