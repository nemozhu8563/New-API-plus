package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type telegramTestTransport struct{ target *url.URL }

func (transport telegramTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != "oauth.telegram.org" {
		return nil, fmt.Errorf("unexpected OAuth host: %s", request.URL.Host)
	}
	clone := request.Clone(request.Context())
	clone.URL.Scheme, clone.URL.Host = transport.target.Scheme, transport.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

type telegramTestGrant struct {
	challenge string
	claims    jwt.MapClaims
	key       *rsa.PrivateKey
}

type telegramOAuthFixture struct {
	user        *model.User
	identity    service.AuthIdentity
	client      *http.Client
	key         *rsa.PrivateKey
	mutex       sync.Mutex
	grants      map[string]telegramTestGrant
	tokenStatus int
	jwksStatus  int
}

func setupTelegramOAuthTest(t *testing.T) *telegramOAuthFixture {
	t.Helper()
	user, identity := setupSecurityEnrollmentTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ExternalIdentityClaim{}, &model.Option{}))
	previousEnabled := common.TelegramOAuthEnabled
	previousSettings := *system_setting.GetTelegramSettings()
	previousAddress := system_setting.ServerAddress
	previousProvider := oauth.GetProvider("telegram")
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	maps.Copy(common.OptionMap, previousOptions)
	common.OptionMapRWMutex.Unlock()
	common.TelegramOAuthEnabled = true
	*system_setting.GetTelegramSettings() = system_setting.TelegramSettings{ClientID: "12345", ClientSecret: "telegram-client-secret"}
	system_setting.ServerAddress = "https://example.com"
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		common.TelegramOAuthEnabled = previousEnabled
		*system_setting.GetTelegramSettings() = previousSettings
		system_setting.ServerAddress = previousAddress
		oauth.Register("telegram", previousProvider)
		oauth.UnregisterCustomProvider("telegram")
	})
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	fixture := &telegramOAuthFixture{user: user, identity: identity, key: key, grants: map[string]telegramTestGrant{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		fixture.mutex.Lock()
		defer fixture.mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/.well-known/jwks.json":
			if fixture.jwksStatus != 0 {
				w.WriteHeader(fixture.jwksStatus)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, telegramID)
		})
	}
}

func signedTelegramAuthorization(token string, authDate time.Time) url.Values {
	params := url.Values{
		"auth_date":  {strconv.FormatInt(authDate.Unix(), 10)},
		"first_name": {"Test"},
		"id":         {"123456"},
	}
	signTelegramAuthorization(token, params)
	return params
}

func signTelegramAuthorization(token string, params url.Values) {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key == "hash" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dataCheck := make([]string, 0, len(keys))
	for _, key := range keys {
		dataCheck = append(dataCheck, key+"="+params.Get(key))
	}
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(strings.Join(dataCheck, "\n")))
	params.Set("hash", hex.EncodeToString(mac.Sum(nil)))
}

func createTelegramBindTestFlow(t *testing.T, db *gorm.DB, name string, status int, now time.Time) (*model.User, string) {
	t.Helper()
	user := &model.User{
		Username: name, Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: status, Group: "default", AuthVersion: 1, AffCode: name,
	}
	require.NoError(t, db.Create(user).Error)
	session := &model.UserSession{
		SID: name + "-session", UserID: user.Id, Version: 1, UserAuthVersion: user.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: name + "-refresh-hash", LoginMethod: "password",
		CreatedAt: now.Unix(), LastActiveAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, model.CreateUserSession(session))
	flowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeTelegramBind, UserId: user.Id, SessionId: session.SID,
		ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	return user, flowToken
}

func assertTelegramBindRedirect(t *testing.T, response *httptest.ResponseRecorder, flowToken, errorCode string) {
	t.Helper()
	require.Equal(t, http.StatusFound, response.Code)
	location, err := url.Parse(response.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/oauth/telegram", location.Path)
	assert.Equal(t, "error", location.Query().Get("telegram_bind"))
	assert.Equal(t, flowToken, location.Query().Get("flow_token"))
	assert.Equal(t, errorCode, location.Query().Get("error_code"))
	assert.Empty(t, location.Query().Get("error_description"))
	assert.Empty(t, location.Query().Get("message"))
}

func TestTelegramBindFailureResponseContract(t *testing.T) {
	failures := []struct {
		name      string
		errorCode string
	}{
		{name: "disabled", errorCode: telegramBindErrorDisabled},
		{name: "invalid request", errorCode: telegramBindErrorInvalidRequest},
		{name: "invalid flow", errorCode: telegramBindErrorFlowInvalid},
		{name: "invalid session", errorCode: telegramBindErrorSessionInvalid},
		{name: "already bound", errorCode: telegramBindErrorAlreadyBound},
		{name: "deleted user", errorCode: telegramBindErrorUserDeleted},
		{name: "disabled user", errorCode: telegramBindErrorUserDisabled},
		{name: "internal error", errorCode: telegramBindErrorInternal},
	}

	for _, failure := range failures {
		t.Run(failure.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Params = gin.Params{{Key: "flow_token", Value: "flow token"}}
			context.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/flow-token", nil)

			telegramBindFailure(context, failure.errorCode)

			assertTelegramBindRedirect(t, response, "flow token", failure.errorCode)
		})
	}
}

func TestTelegramBindCommitsFlowAssertionAndBindingAtomically(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousEnabled := common.TelegramOAuthEnabled
	previousToken := common.TelegramBotToken
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserSession{},
		&model.AuthFlow{},
		&model.ExternalIdentityClaim{},
		&model.UserOAuthBinding{},
	))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.TelegramOAuthEnabled = true
	common.TelegramBotToken = "telegram-bind-test-token"
	common.SessionSecret = "telegram-bind-session-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.TelegramOAuthEnabled = previousEnabled
		common.TelegramBotToken = previousToken
		common.SessionSecret = previousSecret
	})

	user := &model.User{
		Username: "telegram-bind-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "telegram-bind-user",
	}
	require.NoError(t, db.Create(user).Error)
	now := time.Now()
	session := &model.UserSession{
		SID: "telegram-bind-session", UserID: user.Id, Version: 1, UserAuthVersion: user.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: "refresh-hash", LoginMethod: "password",
		CreatedAt: now.Unix(), LastActiveAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, model.CreateUserSession(session))
	flowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeTelegramBind, UserId: user.Id, SessionId: session.SID,
		ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	params := signedTelegramAuthorization(common.TelegramBotToken, now)
	router := gin.New()
	router.GET("/api/oauth/telegram/bind/:flow_token", TelegramBind)

	common.TelegramOAuthEnabled = false
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/disabled-flow", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, "disabled-flow", telegramBindErrorDisabled)
	common.TelegramOAuthEnabled = true

	request = httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/invalid-request", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, "invalid-request", telegramBindErrorInvalidRequest)

	request = httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/missing-flow?"+params.Encode(), nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, "missing-flow", telegramBindErrorFlowInvalid)

	invalidSessionFlowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeTelegramBind, UserId: user.Id, SessionId: "missing-session",
		ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	request = httptest.NewRequest(
		http.MethodGet,
		"/api/oauth/telegram/bind/"+invalidSessionFlowToken+"?"+params.Encode(),
		nil,
	)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, invalidSessionFlowToken, telegramBindErrorSessionInvalid)
	invalidSessionFlow, err := model.GetAuthFlow(invalidSessionFlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	require.NoError(t, err)
	assert.Nil(t, invalidSessionFlow.ConsumedAt)

	request = httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/"+flowToken+"?"+params.Encode(), nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/oauth/telegram?telegram_bind=success&flow_token="+url.QueryEscape(flowToken), response.Header().Get("Location"))
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	assert.Equal(t, "123456", storedUser.TelegramId)
	var identityClaim model.ExternalIdentityClaim
	require.NoError(t, db.Where("provider = ? AND subject = ?", model.ExternalIdentityProviderTelegram, "123456").
		First(&identityClaim).Error)
	assert.Equal(t, user.Id, identityClaim.UserId)
	_, err = model.GetAuthFlow(flowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)

	replayFlowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeTelegramBind, UserId: user.Id, SessionId: session.SID,
		ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/bind/"+replayFlowToken+"?"+params.Encode(), nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, replayFlowToken, telegramBindErrorInvalidRequest)
	replayFlow, err := model.GetAuthFlow(replayFlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	require.NoError(t, err)
	assert.Nil(t, replayFlow.ConsumedAt)

	competingUser := &model.User{
		Username: "telegram-bind-competing-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "telegram-bind-competing-user",
	}
	require.NoError(t, db.Create(competingUser).Error)
	competingSession := &model.UserSession{
		SID: "telegram-bind-competing-session", UserID: competingUser.Id, Version: 1,
		UserAuthVersion: competingUser.AuthVersion, Status: model.UserSessionStatusActive,
		RefreshHash: "competing-refresh-hash", LoginMethod: "password",
		CreatedAt: now.Unix(), LastActiveAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, model.CreateUserSession(competingSession))
	competingFlowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeTelegramBind, UserId: competingUser.Id, SessionId: competingSession.SID,
		ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	competingParams := signedTelegramAuthorization(common.TelegramBotToken, now)
	competingParams.Set("first_name", "Competing")
	signTelegramAuthorization(common.TelegramBotToken, competingParams)
	request = httptest.NewRequest(
		http.MethodGet,
		"/api/oauth/telegram/bind/"+competingFlowToken+"?"+competingParams.Encode(),
		nil,
	)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, competingFlowToken, telegramBindErrorAlreadyBound)

	require.NoError(t, db.First(competingUser, competingUser.Id).Error)
	assert.Empty(t, competingUser.TelegramId)
	competingFlow, err := model.GetAuthFlow(competingFlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	require.NoError(t, err)
	assert.Nil(t, competingFlow.ConsumedAt)
	competingAssertion, competingAssertionExpiry, err := telegramAuthorizationClaim(competingParams, time.Now())
	require.NoError(t, err)
	require.NoError(t, model.ClaimExternalAuthAssertion(
		model.AuthFlowPurposeTelegramAssertion,
		competingAssertion,
		competingAssertionExpiry,
	))

	disabledUser, disabledFlowToken := createTelegramBindTestFlow(
		t, db, "telegram-bind-disabled-user", common.UserStatusDisabled, now,
	)
	disabledParams := signedTelegramAuthorization(common.TelegramBotToken, now)
	disabledParams.Set("id", "disabled-telegram-id")
	disabledParams.Set("first_name", "Disabled")
	signTelegramAuthorization(common.TelegramBotToken, disabledParams)
	request = httptest.NewRequest(
		http.MethodGet,
		"/api/oauth/telegram/bind/"+disabledFlowToken+"?"+disabledParams.Encode(),
		nil,
	)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, disabledFlowToken, telegramBindErrorUserDisabled)
	var storedDisabledUser model.User
	require.NoError(t, db.First(&storedDisabledUser, disabledUser.Id).Error)
	assert.Empty(t, storedDisabledUser.TelegramId)
	disabledFlow, err := model.GetAuthFlow(disabledFlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	require.NoError(t, err)
	assert.Nil(t, disabledFlow.ConsumedAt)
	disabledAssertion, disabledAssertionExpiry, err := telegramAuthorizationClaim(disabledParams, time.Now())
	require.NoError(t, err)
	require.NoError(t, model.ClaimExternalAuthAssertion(
		model.AuthFlowPurposeTelegramAssertion,
		disabledAssertion,
		disabledAssertionExpiry,
	))

	deletedUser, deletedFlowToken := createTelegramBindTestFlow(
		t, db, "telegram-bind-deleted-user", common.UserStatusEnabled, now,
	)
	require.NoError(t, db.Delete(deletedUser).Error)
	deletedParams := signedTelegramAuthorization(common.TelegramBotToken, now)
	deletedParams.Set("id", "deleted-telegram-id")
	deletedParams.Set("first_name", "Deleted")
	signTelegramAuthorization(common.TelegramBotToken, deletedParams)
	request = httptest.NewRequest(
		http.MethodGet,
		"/api/oauth/telegram/bind/"+deletedFlowToken+"?"+deletedParams.Encode(),
		nil,
	)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertTelegramBindRedirect(t, response, deletedFlowToken, telegramBindErrorUserDeleted)
	deletedFlow, err := model.GetAuthFlow(deletedFlowToken, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeTelegramBind})
	require.NoError(t, err)
	assert.Nil(t, deletedFlow.ConsumedAt)
	deletedAssertion, deletedAssertionExpiry, err := telegramAuthorizationClaim(deletedParams, time.Now())
	require.NoError(t, err)
	require.NoError(t, model.ClaimExternalAuthAssertion(
		model.AuthFlowPurposeTelegramAssertion,
		deletedAssertion,
		deletedAssertionExpiry,
	))

	_, internalFlowToken := createTelegramBindTestFlow(
		t, db, "telegram-bind-internal-error", common.UserStatusEnabled, now,
	)
	internalParams := signedTelegramAuthorization(common.TelegramBotToken, now)
	internalParams.Set("id", "internal-error-telegram-id")
	internalParams.Set("first_name", "Internal")
	signTelegramAuthorization(common.TelegramBotToken, internalParams)
	forcedError := errors.New("forced telegram session query failure")
	const callbackName = "test:telegram-bind-session-query-failure"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "user_sessions" {
			return
		}
		if _, inTransaction := tx.Statement.ConnPool.(gorm.TxCommitter); inTransaction {
			tx.AddError(forcedError)
		}
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	fixture.client = &http.Client{Transport: telegramTestTransport{target: target}, Timeout: 5 * time.Second}
	oauth.Register("telegram", oauth.NewTelegramProvider(fixture.client))
	return fixture
}

func (fixture *telegramOAuthFixture) authorization(t *testing.T, intent string, identity service.AuthIdentity, scope string, claims jwt.MapClaims) (string, string) {
	t.Helper()
	request, err := common.Marshal(oauthStateRequest{Provider: "telegram", Intent: intent, Scope: scope})
	require.NoError(t, err)
	proof := ""
	if intent == "bind" {
		proof = issueSecurityEnrollmentProof(t, identity, service.VerificationOperation{Scope: service.VerificationScopeAccountBind, Context: []byte(`{"provider":"telegram"}`)}, service.VerificationMethodPassword)
	}
	response := securityEnrollmentRequest("POST", "/api/oauth/state", string(request), proof, identity, GenerateOAuthCode)
	var body securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, response.Body.String())
	var data struct {
		State string `json:"flow_token"`
		URL   string `json:"authorization_url"`
	}
	require.NoError(t, common.Unmarshal(body.Data, &data))
	authorizationURL, err := url.Parse(data.URL)
	require.NoError(t, err)
	assert.Equal(t, "https://oauth.telegram.org/auth", authorizationURL.Scheme+"://"+authorizationURL.Host+authorizationURL.Path)
	assert.Equal(t, "openid profile", authorizationURL.Query().Get("scope"))
	assert.Equal(t, "S256", authorizationURL.Query().Get("code_challenge_method"))
	assert.Equal(t, data.State, authorizationURL.Query().Get("state"))
	assert.Empty(t, authorizationURL.Query().Get("code_verifier"))
	assert.NotContains(t, response.Body.String(), "telegram-client-secret")
	code := "code-" + data.State
	fixture.mutex.Lock()
	fixture.grants[code] = telegramTestGrant{challenge: authorizationURL.Query().Get("code_challenge"), claims: claims, key: fixture.key}
	fixture.mutex.Unlock()
	return data.State, code
}

func telegramIdentityClaims(id any) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": oauth.TelegramIssuer, "aud": "12345", "sub": "different-oidc-subject",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		"id": id, "name": "Telegram User", "preferred_username": "telegram-user",
	}
}

func telegramOAuthCallback(state, code string, identity service.AuthIdentity) *httptest.ResponseRecorder {
	path := "/api/oauth/telegram?" + url.Values{"state": {state}, "code": {code}}.Encode()
	return securityEnrollmentRequest("GET", path, "", "", identity, func(c *gin.Context) {
		c.Params = gin.Params{{Key: "provider", Value: "telegram"}}
		HandleOAuth(c)
	})
}

func TestTelegramOAuthPreservesExistingAccountAndBinding(t *testing.T) {
	fixture := setupTelegramOAuthTest(t)
	const telegramID = "1234567890123456"
	require.NoError(t, model.DB.Model(fixture.user).Update("telegram_id", telegramID).Error)
	require.NoError(t, model.InitializeExternalIdentityClaims())
	require.NoError(t, model.InitializeExternalIdentityClaims())
	state, code := fixture.authorization(t, "login", service.AuthIdentity{}, "", telegramIdentityClaims(json.Number(telegramID)))
	response := telegramOAuthCallback(state, code, service.AuthIdentity{})
	var body securityEnrollmentResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success, response.Body.String())
	var login struct {
		User struct {
			ID int `json:"id"`
		} `json:"user"`
	}
	require.NoError(t, common.Unmarshal(body.Data, &login))
	assert.Equal(t, fixture.user.Id, login.User.ID)
	stored, err := model.GetUserByTelegramID(telegramID)
	require.NoError(t, err)
	assert.Equal(t, telegramID, stored.TelegramId)
	var claim model.ExternalIdentityClaim
	require.NoError(t, model.DB.Where("provider = ? AND subject = ?", "telegram", telegramID).First(&claim).Error)
	assert.Equal(t, stored.Id, claim.UserId)
	replay := telegramOAuthCallback(state, code, service.AuthIdentity{})
	assert.Equal(t, http.StatusForbidden, replay.Code)
	state, code = fixture.authorization(t, "login", service.AuthIdentity{}, "", telegramIdentityClaims(999))
	response = telegramOAuthCallback(state, code, service.AuthIdentity{})
	assert.Contains(t, response.Body.String(), "TELEGRAM_ACCOUNT_NOT_BOUND")
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestTelegramOAuthRejectsInvalidTokens(t *testing.T) {
	fixture := setupTelegramOAuthTest(t)
	require.NoError(t, model.DB.Model(fixture.user).Update("telegram_id", "42").Error)
	for _, test := range []struct {
		name   string
		change func(jwt.MapClaims)
	}{
		{"issuer", func(c jwt.MapClaims) { c["iss"] = "https://other.example" }},
		{"audience", func(c jwt.MapClaims) { c["aud"] = "other-client" }},
		{"expired", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }},
		{"missing id", func(c jwt.MapClaims) { delete(c, "id") }},
		{"negative id", func(c jwt.MapClaims) { c["id"] = -1 }},
		{"fractional id", func(c jwt.MapClaims) { c["id"] = 1.5 }},
		{"overflow id", func(c jwt.MapClaims) { c["id"] = "18446744073709551616" }},
		{"missing subject", func(c jwt.MapClaims) { delete(c, "sub") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := telegramIdentityClaims(42)
			test.change(claims)
			state, code := fixture.authorization(t, "login", service.AuthIdentity{}, "", claims)
			response := telegramOAuthCallback(state, code, service.AuthIdentity{})
			assert.Contains(t, response.Body.String(), "TELEGRAM_OAUTH_FAILED")
			assert.NotContains(t, response.Body.String(), "access_token")
		})
	}
	t.Run("signature", func(t *testing.T) {
		state, code := fixture.authorization(t, "login", service.AuthIdentity{}, "", telegramIdentityClaims(42))
		otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		grant := fixture.grants[code]
		grant.key = otherKey
		fixture.grants[code] = grant
		assert.Contains(t, telegramOAuthCallback(state, code, service.AuthIdentity{}).Body.String(), "TELEGRAM_OAUTH_FAILED")
	})
	t.Run("PKCE", func(t *testing.T) {
		state, code := fixture.authorization(t, "login", service.AuthIdentity{}, "", telegramIdentityClaims(42))
		grant := fixture.grants[code]
		grant.challenge = "wrong-challenge"
		fixture.grants[code] = grant
		assert.Contains(t, telegramOAuthCallback(state, code, service.AuthIdentity{}).Body.String(), "TELEGRAM_OAUTH_FAILED")
	})
	for _, endpoint := range []string{"token", "jwks"} {
		t.Run(endpoint+" unavailable", func(t *testing.T) {
			state, code := fixture.authorization(t, "login", service.AuthIdentity{}, "", telegramIdentityClaims(42))
			oauth.Register("telegram", oauth.NewTelegramProvider(fixture.client))
			if endpoint == "token" {
				fixture.tokenStatus = 503
			} else {
				fixture.jwksStatus = 503
			}
			response := telegramOAuthCallback(state, code, service.AuthIdentity{})
			assert.Contains(t, response.Body.String(), "TELEGRAM_OAUTH_FAILED")
			fixture.tokenStatus, fixture.jwksStatus = 0, 0
		})
	}
}

func TestTelegramOAuthBindingIsAtomicAndSessionBound(t *testing.T) {
	fixture := setupTelegramOAuthTest(t)
	// Older rows may predate the Telegram column and have a NULL binding.
	require.NoError(t, model.DB.Model(fixture.user).Update("telegram_id", nil).Error)
	state, code := fixture.authorization(t, "bind", fixture.identity, "", telegramIdentityClaims(42))
	otherBundle, err := service.CreateLoginSession(fixture.user.Id, "password", "127.0.0.1", "other")
	require.NoError(t, err)
	otherIdentity, err := service.ParseAccessToken(otherBundle.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, telegramOAuthCallback(state, code, otherIdentity).Code)
	failure := errors.New("private Telegram bind storage failure")
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("telegram_bind_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(failure)
		}
	}))
	response := telegramOAuthCallback(state, code, fixture.identity)
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), "private")
	require.NoError(t, model.DB.Callback().Update().Remove("telegram_bind_failure"))
	_, err = model.GetAuthFlow(state, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeOAuth, Provider: "telegram"})
	require.NoError(t, err)
	var claimCount int64
	require.NoError(t, model.DB.Model(&model.ExternalIdentityClaim{}).Count(&claimCount).Error)
	assert.Zero(t, claimCount)
	state, code = fixture.authorization(t, "bind", fixture.identity, "", telegramIdentityClaims(42))
	response = telegramOAuthCallback(state, code, fixture.identity)
	assert.Contains(t, response.Body.String(), `"success":true`)
	stored, err := model.GetUserById(fixture.user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "42", stored.TelegramId)
	assert.Equal(t, fixture.user.Role, stored.Role)
	assert.Equal(t, fixture.user.Status, stored.Status)
	assert.Equal(t, fixture.user.AuthVersion, stored.AuthVersion)
	assert.Equal(t, http.StatusForbidden, telegramOAuthCallback(state, code, fixture.identity).Code)
}

func TestTelegramOAuthBindingRejectsChangedAccountsAndDuplicateOwnership(t *testing.T) {
	for _, change := range []string{"revoked", "disabled", "deleted", "already bound", "owned by another user"} {
		t.Run(change, func(t *testing.T) {
			fixture := setupTelegramOAuthTest(t)
			state, code := fixture.authorization(t, "bind", fixture.identity, "", telegramIdentityClaims(42))
			switch change {
			case "revoked":
				_, err := model.RevokeAllUserSessions(fixture.user.Id, "test")
				require.NoError(t, err)
			case "disabled":
				require.NoError(t, model.DB.Model(fixture.user).Update("status", common.UserStatusDisabled).Error)
			case "deleted":
				require.NoError(t, model.DB.Delete(fixture.user).Error)
			case "already bound":
				require.NoError(t, model.DB.Model(fixture.user).Update("telegram_id", "99").Error)
				require.NoError(t, model.InitializeExternalIdentityClaims())
			case "owned by another user":
				owner := &model.User{Username: "owner", AffCode: "owner", Status: common.UserStatusEnabled, AuthVersion: 1, TelegramId: "42"}
				require.NoError(t, model.DB.Create(owner).Error)
				require.NoError(t, model.InitializeExternalIdentityClaims())
			}
			response := telegramOAuthCallback(state, code, fixture.identity)
			assert.Contains(t, response.Body.String(), `"success":false`)
			var claims int64
			require.NoError(t, model.DB.Model(&model.ExternalIdentityClaim{}).Where("user_id = ? AND subject = ?", fixture.user.Id, "42").Count(&claims).Error)
			assert.Zero(t, claims)
		})
	}
}

func TestTelegramOAuthConfigurationAndLegacyEndpoints(t *testing.T) {
	fixture := setupTelegramOAuthTest(t)
	previousBotToken := common.TelegramBotToken
	t.Cleanup(func() { common.TelegramBotToken = previousBotToken })
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"telegram.client_id": "12345", "telegram.client_secret": "telegram-client-secret",
		"TelegramBotToken": "stored-legacy-token", "TelegramOAuthEnabled": "true",
	}))
	for range 2 {
		*system_setting.GetTelegramSettings() = system_setting.TelegramSettings{}
		common.TelegramOAuthEnabled = false
		model.InitOptionMap()
		assert.True(t, common.TelegramOAuthEnabled)
		assert.True(t, system_setting.GetTelegramSettings().IsConfigured())
		assert.Equal(t, "telegram-client-secret", system_setting.GetTelegramSettings().ClientSecret)
		assert.Equal(t, "stored-legacy-token", common.TelegramBotToken)
	}
	var stored model.Option
	require.NoError(t, model.DB.Where(&model.Option{Key: "telegram.client_id"}).First(&stored).Error)
	assert.Equal(t, "12345", stored.Value)
	status := securityEnrollmentRequest("GET", "/api/status", "", "", fixture.identity, GetStatus)
	assert.Contains(t, status.Body.String(), `"telegram_oauth_configured":true`)
	assert.NotContains(t, status.Body.String(), "telegram-client-secret")
	options := securityEnrollmentRequest("GET", "/api/option/", "", "", fixture.identity, GetOptions)
	assert.NotContains(t, options.Body.String(), "telegram-client-secret")
	assert.NotContains(t, options.Body.String(), "stored-legacy-token")
	require.NoError(t, model.DB.Model(fixture.user).Updates(map[string]any{"password": "", "telegram_id": "42"}).Error)
	for _, unavailable := range []string{"missing secret", "disabled", "conflict"} {
		t.Run(unavailable, func(t *testing.T) {
			switch unavailable {
			case "missing secret":
				system_setting.GetTelegramSettings().ClientSecret = ""
			case "disabled":
				common.TelegramOAuthEnabled = false
			case "conflict":
				require.Error(t, oauth.RegisterCustom("telegram", oauth.NewGenericOAuthProvider(&model.CustomOAuthProvider{Slug: "telegram"})))
			}
			response := securityEnrollmentRequest("POST", "/api/oauth/state", `{"provider":"telegram","intent":"login"}`, "", service.AuthIdentity{}, GenerateOAuthCode)
			assert.Contains(t, response.Body.String(), `"success":false`)
			assert.NotContains(t, response.Body.String(), "flow_token")
			for _, scope := range []string{"2fa.setup", "passkey.register"} {
				requirements, err := service.GetVerificationRequirements(fixture.identity, scope)
				require.NoError(t, err)
				require.Len(t, requirements.Methods, 1)
				assert.False(t, requirements.Methods[0].Available)
				assert.Contains(t, requirements.Methods[0].Reason, "administrator")
				_, err = service.VerifySecurityInput(fixture.identity, service.VerificationInput{Scope: scope, Method: "session"})
				assert.Error(t, err)
				_, err = service.StartOAuthVerification(fixture.identity, service.VerificationOperation{Scope: scope}, "telegram")
				assert.Error(t, err)
			}
			for _, handler := range []gin.HandlerFunc{Setup2FA, PasskeyRegisterBegin} {
				response := securityEnrollmentRequest("POST", "/setup", `{}`, "", fixture.identity, handler)
				assert.Contains(t, response.Body.String(), `"success":false`)
				assert.NotContains(t, response.Body.String(), "flow_token")
			}
			common.TelegramOAuthEnabled = true
			system_setting.GetTelegramSettings().ClientSecret = "telegram-client-secret"
			oauth.UnregisterCustomProvider("telegram")
		})
	}
	for _, endpoint := range []struct{ method, path string }{
		{"GET", "/api/oauth/telegram/login"},
		{"POST", "/api/oauth/telegram/bind/start"},
		{"GET", "/api/oauth/telegram/bind/old-flow"},
	} {
		response := securityEnrollmentRequest(endpoint.method, endpoint.path, "", "", fixture.identity, TelegramLegacyAuth)
		assert.Equal(t, http.StatusGone, response.Code)
		assert.Contains(t, response.Body.String(), "TELEGRAM_LEGACY_AUTH_REMOVED")
	}
}

func TestTelegramOAuthConcurrentBindingHasSingleOwner(t *testing.T) {
	fixture := setupTelegramOAuthTest(t)
	other := &model.User{Username: "other-owner", AffCode: "other-owner", Password: fixture.user.Password, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, model.DB.Create(other).Error)
	bundle, err := service.CreateLoginSession(other.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	otherIdentity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	state, code := fixture.authorization(t, "bind", fixture.identity, "", telegramIdentityClaims(42))
	otherState, otherCode := fixture.authorization(t, "bind", otherIdentity, "", telegramIdentityClaims(42))
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	go func() { <-start; responses <- telegramOAuthCallback(state, code, fixture.identity) }()
	go func() { <-start; responses <- telegramOAuthCallback(otherState, otherCode, otherIdentity) }()
	close(start)
	successes := 0
	for range 2 {
		response := <-responses
		var body securityEnrollmentResponse
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
		if body.Success {
			successes++
		}
	}
	assert.Equal(t, 1, successes)
	var owners []model.User
	require.NoError(t, model.DB.Where("telegram_id = ?", "42").Find(&owners).Error)
	require.Len(t, owners, 1)
	var claims []model.ExternalIdentityClaim
	require.NoError(t, model.DB.Where("provider = ? AND subject = ?", "telegram", "42").Find(&claims).Error)
	require.Len(t, claims, 1)
	assert.Equal(t, owners[0].Id, claims[0].UserId)
}
