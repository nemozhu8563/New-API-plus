package router

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const siteRuntimePlaceholder = "<!--site-runtime-->\n"

var (
	gaMeasurementIDPattern  = regexp.MustCompile(`^G-[A-Z0-9]+$`)
	clarityProjectIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)
)

// SiteRuntimeConfig keeps public website metadata out of the frontend bundle
// until a request has matched its exact public origin.
type SiteRuntimeConfig struct {
	CanonicalOrigin   string
	TelemetryOrigin   string
	GoogleAnalyticsID string
	ClarityProjectID  string
}

type siteRuntimePayload struct {
	CanonicalOrigin   string `json:"canonicalOrigin,omitempty"`
	TelemetryOrigin   string `json:"telemetryOrigin,omitempty"`
	GoogleAnalyticsID string `json:"googleAnalyticsId,omitempty"`
	ClarityProjectID  string `json:"clarityProjectId,omitempty"`
}

func LoadSiteRuntimeConfig() (SiteRuntimeConfig, error) {
	return NewSiteRuntimeConfig(
		os.Getenv("SITE_CANONICAL_ORIGIN"),
		os.Getenv("SITE_TELEMETRY_ORIGIN"),
		os.Getenv("GOOGLE_ANALYTICS_ID"),
		os.Getenv("CLARITY_PROJECT_ID"),
	)
}

func NewSiteRuntimeConfig(
	canonicalOrigin string,
	telemetryOrigin string,
	googleAnalyticsID string,
	clarityProjectID string,
) (SiteRuntimeConfig, error) {
	config := SiteRuntimeConfig{}

	if strings.TrimSpace(canonicalOrigin) != "" {
		normalizedOrigin, err := normalizeHTTPSOrigin(canonicalOrigin)
		if err != nil {
			return SiteRuntimeConfig{}, fmt.Errorf("SITE_CANONICAL_ORIGIN: %w", err)
		}
		config.CanonicalOrigin = normalizedOrigin
	}

	googleAnalyticsID = strings.TrimSpace(googleAnalyticsID)
	clarityProjectID = strings.TrimSpace(clarityProjectID)
	if googleAnalyticsID == "" && clarityProjectID == "" {
		return config, nil
	}

	normalizedOrigin, err := normalizeHTTPSOrigin(telemetryOrigin)
	if err != nil {
		return SiteRuntimeConfig{}, fmt.Errorf("SITE_TELEMETRY_ORIGIN: %w", err)
	}
	if googleAnalyticsID != "" && !gaMeasurementIDPattern.MatchString(googleAnalyticsID) {
		return SiteRuntimeConfig{}, fmt.Errorf("GOOGLE_ANALYTICS_ID must be a GA4 measurement ID")
	}
	if clarityProjectID != "" && !clarityProjectIDPattern.MatchString(clarityProjectID) {
		return SiteRuntimeConfig{}, fmt.Errorf("CLARITY_PROJECT_ID must be a lowercase alphanumeric project ID")
	}

	config.TelemetryOrigin = normalizedOrigin
	config.GoogleAnalyticsID = googleAnalyticsID
	config.ClarityProjectID = clarityProjectID
	return config, nil
}

func normalizeHTTPSOrigin(raw string) (string, error) {
	origin, err := common.NormalizeOrigin(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(origin, "https://") {
		return "", fmt.Errorf("origin must use https")
	}
	return origin, nil
}

func (config SiteRuntimeConfig) HasTelemetry() bool {
	return config.TelemetryOrigin != "" &&
		(config.GoogleAnalyticsID != "" || config.ClarityProjectID != "")
}

func (config SiteRuntimeConfig) RenderIndexPage(indexPage []byte, request *http.Request) []byte {
	payload := siteRuntimePayload{}
	if config.matchesRequestOrigin(config.CanonicalOrigin, request) {
		payload.CanonicalOrigin = config.CanonicalOrigin
	}
	if config.HasTelemetry() && config.matchesRequestOrigin(config.TelemetryOrigin, request) {
		payload.TelemetryOrigin = config.TelemetryOrigin
		payload.GoogleAnalyticsID = config.GoogleAnalyticsID
		payload.ClarityProjectID = config.ClarityProjectID
	}
	if payload == (siteRuntimePayload{}) {
		return bytes.ReplaceAll(indexPage, []byte(siteRuntimePlaceholder), []byte("<!--site runtime disabled-->\n"))
	}

	serializedPayload, err := common.Marshal(payload)
	if err != nil {
		return bytes.ReplaceAll(indexPage, []byte(siteRuntimePlaceholder), []byte("<!--site runtime unavailable-->\n"))
	}

	bootstrap := ""
	if payload.CanonicalOrigin != "" {
		canonicalPath := "/"
		if request != nil && request.URL != nil {
			if requestPath := request.URL.EscapedPath(); requestPath != "" {
				canonicalPath = requestPath
			}
		}
		bootstrap += "<link rel=\"canonical\" href=\"" + payload.CanonicalOrigin + canonicalPath + "\">\n"
	}
	bootstrap += "<script>window.__SITE_RUNTIME__=" + string(serializedPayload) + ";"
	if payload.GoogleAnalyticsID != "" {
		bootstrap += "window.dataLayer=window.dataLayer||[];window.gtag=window.gtag||function(){window.dataLayer.push(arguments)};window.gtag('consent','default',{analytics_storage:'denied',ad_storage:'denied',ad_user_data:'denied',ad_personalization:'denied'});window.__SITE_TELEMETRY_GA_CONSENT_DEFAULTED__=true;"
	}
	bootstrap += "</script><!--site runtime-->\n"
	return bytes.ReplaceAll(indexPage, []byte(siteRuntimePlaceholder), []byte(bootstrap))
}

func (config SiteRuntimeConfig) matchesRequestOrigin(expectedOrigin string, request *http.Request) bool {
	if expectedOrigin == "" || request == nil {
		return false
	}

	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	} else if forwardedProto := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	}
	origin, err := common.NormalizeOrigin(scheme + "://" + request.Host)
	return err == nil && origin == expectedOrigin
}
