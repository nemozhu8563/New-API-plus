package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveControllerSensitiveWords(t *testing.T) {
	t.Helper()
	originalNSFW := setting.SensitiveWordsToString()
	originalHighRisk := setting.SensitiveWordsHighRiskToString()
	originalAudit := setting.SensitiveWordsAuditToString()
	t.Cleanup(func() {
		setting.SensitiveWordsFromString(originalNSFW)
		setting.SensitiveWordsHighRiskFromString(originalHighRisk)
		setting.SensitiveWordsAuditFromString(originalAudit)
	})
}

func TestContentPolicyViolationErrorContract(t *testing.T) {
	apiErr := newContentPolicyViolationError()

	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeContentPolicyViolation, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.Equal(t, "request blocked by content policy", apiErr.Error())

	openAIError := apiErr.ToOpenAIError()
	assert.Equal(t, types.ErrorCodeContentPolicyViolation, openAIError.Code)
	assert.Equal(t, "request blocked by content policy", openAIError.Message)
}

func TestPromptSensitivePolicyBlocksNSFWAndHighRiskButAllowsAudit(t *testing.T) {
	preserveControllerSensitiveWords(t)
	setting.SensitiveWordsFromString("nsfw-block")
	setting.SensitiveWordsHighRiskFromString("risk-block")
	setting.SensitiveWordsAuditFromString("audit-only")

	tests := []struct {
		name      string
		text      string
		wantBlock bool
	}{
		{name: "NSFW is blocked", text: "contains nsfw-block", wantBlock: true},
		{name: "high risk is blocked", text: "contains risk-block", wantBlock: true},
		{name: "audit is allowed", text: "contains audit-only", wantBlock: false},
		{name: "ordinary text is allowed", text: "ordinary request", wantBlock: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			apiErr := checkPromptSensitivePolicy(context, tt.text)
			if !tt.wantBlock {
				assert.Nil(t, apiErr)
				return
			}

			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
			assert.Equal(t, types.ErrorCodeContentPolicyViolation, apiErr.GetErrorCode())
			assert.True(t, types.IsSkipRetryError(apiErr))
		})
	}
}
