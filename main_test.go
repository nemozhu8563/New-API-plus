package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedRootPageFlowsThroughSiteRuntimeRenderer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	siteRuntime, err := router.NewSiteRuntimeConfig(
		"https://tryvalo.com",
		"https://tryvalo.com",
		"G-TEST123",
		"abc123",
	)
	require.NoError(t, err)

	router.SetWebRouter(engine, router.WebAssets{
		BuildFS:     buildFS,
		IndexPage:   indexPage,
		SiteRuntime: siteRuntime,
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "https://tryvalo.com/", nil)
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, response.Body.String(), "window.__SITE_RUNTIME__")
	assert.Contains(t, response.Body.String(), "G-TEST123")
	assert.NotContains(t, response.Body.String(), "<!--site-runtime-->")
}
