package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEORoutesServePlainRobotsAndXMLSitemapBeforeSPAFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	config, err := NewSiteRuntimeConfig("https://tryvalo.com", "", "", "")
	require.NoError(t, err)
	SetSEORouter(router, config)
	router.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html>"))
	})

	robotsResponse := httptest.NewRecorder()
	router.ServeHTTP(robotsResponse, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	assert.Equal(t, http.StatusOK, robotsResponse.Code)
	assert.Contains(t, robotsResponse.Header().Get("Content-Type"), "text/plain")
	assert.Contains(t, robotsResponse.Body.String(), "Sitemap: https://tryvalo.com/sitemap.xml")
	assert.NotContains(t, robotsResponse.Body.String(), "<!doctype html>")

	sitemapResponse := httptest.NewRecorder()
	router.ServeHTTP(sitemapResponse, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	assert.Equal(t, http.StatusOK, sitemapResponse.Code)
	assert.Contains(t, sitemapResponse.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, sitemapResponse.Body.String(), "<loc>https://tryvalo.com/about/</loc>")
	assert.NotContains(t, sitemapResponse.Body.String(), "<!doctype html>")

	spaResponse := httptest.NewRecorder()
	router.ServeHTTP(spaResponse, httptest.NewRequest(http.MethodGet, "/unknown-route", nil))
	assert.Equal(t, http.StatusOK, spaResponse.Code)
	assert.Contains(t, spaResponse.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "<!doctype html>", spaResponse.Body.String())
}

func TestSEORoutesFailClosedWithoutCanonicalOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetSEORouter(router, SiteRuntimeConfig{})

	robotsResponse := httptest.NewRecorder()
	router.ServeHTTP(robotsResponse, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	assert.Equal(t, http.StatusOK, robotsResponse.Code)
	assert.Equal(t, "User-agent: *\nDisallow: /\n", robotsResponse.Body.String())

	sitemapResponse := httptest.NewRecorder()
	router.ServeHTTP(sitemapResponse, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	assert.Equal(t, http.StatusServiceUnavailable, sitemapResponse.Code)
	assert.NotContains(t, sitemapResponse.Body.String(), "<!doctype html>")
}
