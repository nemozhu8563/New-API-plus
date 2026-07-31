package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAffiliateRoutesRejectUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for _, target := range []string{
		"/api/user/affiliate/summary",
		"/api/user/affiliate/invitees",
		"/api/affiliate/agents",
		"/api/affiliate/withdrawals",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, target, nil)
		engine.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code, target)
		assert.Contains(t, recorder.Body.String(), `"success":false`, target)
	}
}
