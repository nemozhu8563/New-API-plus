package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
