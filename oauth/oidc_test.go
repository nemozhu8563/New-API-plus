package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCProvider_GetName(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalDisplayName := settings.DisplayName
	defer func() { settings.DisplayName = originalDisplayName }()

	p := &OIDCProvider{}

	settings.DisplayName = ""
	assert.Equal(t, "OIDC", p.GetName())

	settings.DisplayName = "  Acme SSO  "
	assert.Equal(t, "Acme SSO", p.GetName())
}

func TestOIDCProvider_ExchangeTokenUsesAPICallback(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalSettings := *settings
	originalServerAddress := system_setting.ServerAddress
	defer func() {
		*settings = originalSettings
		system_setting.ServerAddress = originalServerAddress
	}()

	redirectURI := make(chan string, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		redirectURI <- r.Form.Get("redirect_uri")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer"}`))
	}))
	defer tokenServer.Close()

	settings.ClientId = "test-client-id"
	settings.ClientSecret = "test-client-secret"
	settings.TokenEndpoint = tokenServer.URL
	system_setting.ServerAddress = " https://api.tryvalo.com/ "

	token, err := (&OIDCProvider{}).ExchangeToken(context.Background(), "test-code", nil)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "test-token", token.AccessToken)
	assert.Equal(t, "https://api.tryvalo.com/api/oauth/oidc", <-redirectURI)
}
