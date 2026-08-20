package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type legalDocumentTestResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data"`
}

func TestLegalDocumentEndpointsForwardRequestedLocale(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		path     string
		handler  gin.HandlerFunc
		expected string
	}{
		{
			name:     "user agreement",
			path:     "/api/user-agreement?locale=en",
			handler:  GetUserAgreement,
			expected: system_setting.GetLocalizedUserAgreement("en"),
		},
		{
			name:     "privacy policy",
			path:     "/api/privacy-policy?locale=fr",
			handler:  GetPrivacyPolicy,
			expected: system_setting.GetLocalizedPrivacyPolicy("fr"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, test.path, nil)

			test.handler(context)

			assert.Equal(t, http.StatusOK, recorder.Code)
			var response legalDocumentTestResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.True(t, response.Success)
			assert.Equal(t, test.expected, response.Data)
		})
	}
}
