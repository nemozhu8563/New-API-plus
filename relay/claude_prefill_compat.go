package relay

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	claudeAssistantPrefillContinuationText = "Continue the assistant response from the previous assistant message. Do not repeat the previous message, and do not mention this instruction."
	claudeAssistantPrefillConversionMarker = "assistant_prefill_continuation"
)

func applyClaudeAssistantPrefillCompatibility(request *dto.ClaudeRequest, info *relaycommon.RelayInfo) bool {
	if !applyClaudeAssistantPrefillContinuation(request) {
		return false
	}
	if info != nil {
		info.AppendRequestConversionMeta(claudeAssistantPrefillConversionMarker)
	}
	return true
}

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

func isClaudeAssistantPrefillUnsupportedModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(model, "claude-") {
		return false
	}
	return containsClaudeModelVersion(model, "-4-6") || containsClaudeModelVersion(model, "-4-7")
}

func containsClaudeModelVersion(model string, version string) bool {
	idx := strings.Index(model, version)
	if idx < 0 {
		return false
	}
	end := idx + len(version)
	return end == len(model) || model[end] == '-'
}

func isPlainTextClaudeContent(message *dto.ClaudeMessage) bool {
	if message == nil || message.Content == nil {
		return false
	}
	if _, ok := message.Content.(string); ok {
		return true
	}

	contents, err := message.ParseContent()
	if err != nil || len(contents) == 0 {
		return false
	}
	for _, content := range contents {
		if content.Type != dto.ContentTypeText {
			return false
		}
	}
	return true
}
