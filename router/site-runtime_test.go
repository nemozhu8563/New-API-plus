package router

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiteRuntimeConfigOnlyInjectsPublicIDsForMatchingOrigin(t *testing.T) {
	config, err := NewSiteRuntimeConfig(
		"https://tryvalo.com",
		"https://tryvalo.com",
		"G-TEST123",
		"abc123",
	)
	require.NoError(t, err)
	page := []byte("<head><!--site-runtime-->\n</head>")

	matchingRequest := httptest.NewRequest("GET", "https://tryvalo.com/pricing/", nil)
	matchingPage := string(config.RenderIndexPage(page, matchingRequest))
	assert.Contains(t, matchingPage, "G-TEST123")
	assert.Contains(t, matchingPage, "abc123")
	assert.Contains(t, matchingPage, "analytics_storage:'denied'")
	assert.Contains(t, matchingPage, "<link rel=\"canonical\" href=\"https://tryvalo.com/pricing/\">")
	assert.Contains(t, matchingPage, "window.__SITE_TELEMETRY_GA_CONSENT_DEFAULTED__=true")

	previewRequest := httptest.NewRequest("GET", "https://test.tryvalo.com/pricing/", nil)
	previewPage := string(config.RenderIndexPage(page, previewRequest))
	assert.NotContains(t, previewPage, "G-TEST123")
	assert.NotContains(t, previewPage, "abc123")
	assert.NotContains(t, previewPage, "__SITE_RUNTIME__")
}

func TestSiteRuntimeConfigRejectsUnsafeTelemetryConfiguration(t *testing.T) {
	_, err := NewSiteRuntimeConfig("", "http://tryvalo.com", "G-TEST123", "abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SITE_TELEMETRY_ORIGIN")

	_, err = NewSiteRuntimeConfig("", "https://tryvalo.com", "not-a-measurement-id", "abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_ANALYTICS_ID")
}

func TestSiteRuntimeConfigDoesNotLeakQueryOrPathIntoOrigin(t *testing.T) {
	_, err := NewSiteRuntimeConfig(
		"https://tryvalo.com/path",
		"",
		"",
		"",
	)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "SITE_CANONICAL_ORIGIN"))
}
