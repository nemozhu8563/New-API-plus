package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlansArePublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/subscription/plans", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestSubscriptionPurchasesOnlyExposeStripe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost {
			routes[route.Path] = true
		}
	}

	require.True(t, routes["/api/subscription/stripe/pay"])
	for _, route := range []string{
		"/api/subscription/balance/pay",
		"/api/subscription/epay/pay",
		"/api/subscription/creem/pay",
		"/api/subscription/waffo-pancake/pay",
	} {
		require.False(t, routes[route], route)
	}
}
