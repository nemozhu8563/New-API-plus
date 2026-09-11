package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const oauthCallbackContractProviderName = "oauth-callback-contract"

type oauthCallbackContractProvider struct {
	providerUserID string
}

func (*oauthCallbackContractProvider) GetName() string { return "GitHub" }
func (*oauthCallbackContractProvider) IsEnabled() bool { return true }
func (*oauthCallbackContractProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{}, nil
}
func (provider *oauthCallbackContractProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{ProviderUserID: provider.providerUserID}, nil
}
func (*oauthCallbackContractProvider) IsUserIDTaken(providerUserID string) bool {
	var count int64
	if err := model.DB.Unscoped().Model(&model.User{}).Where("github_id = ?", providerUserID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}
func (*oauthCallbackContractProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	return model.DB.Unscoped().Where("github_id = ?", providerUserID).First(user).Error
}
func (*oauthCallbackContractProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}
func (*oauthCallbackContractProvider) GetProviderPrefix() string    { return "github_" }
func (*oauthCallbackContractProvider) ProviderUserIDColumn() string { return "github_id" }

func setupOAuthCallbackContractTest(t *testing.T) (*gorm.DB, *oauthCallbackContractProvider) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousRegisterEnabled := common.RegisterEnabled
	previousSessionSecret := common.SessionSecret
	previousActiveLimit := common.UserSessionActiveLimit
	previousIssuanceLimit := common.UserSessionIssuanceLimit
	previousIssuanceWindow := common.UserSessionIssuanceWindowSeconds

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserSession{},
		&model.AuthFlow{},
		&model.ExternalIdentityClaim{},
		&model.Log{},
	))

	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.SessionSecret = "oauth-callback-contract-test-secret"
	common.UserSessionActiveLimit = common.DefaultUserSessionActiveLimit
	common.UserSessionIssuanceLimit = common.DefaultUserSessionIssuanceLimit
	common.UserSessionIssuanceWindowSeconds = int64(common.DefaultUserSessionIssuanceWindowSeconds)

	provider := &oauthCallbackContractProvider{providerUserID: "github-external-user"}
	oauth.Register(oauthCallbackContractProviderName, provider)
	t.Cleanup(func() {
		oauth.Unregister(oauthCallbackContractProviderName)
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.RegisterEnabled = previousRegisterEnabled
		common.SessionSecret = previousSessionSecret
		common.UserSessionActiveLimit = previousActiveLimit
		common.UserSessionIssuanceLimit = previousIssuanceLimit
		common.UserSessionIssuanceWindowSeconds = previousIssuanceWindow
		_ = sqlDB.Close()
	})
	return db, provider
}

func createOAuthCallbackContractFlow(t *testing.T, intent string, userID int, sessionID string) string {
	t.Helper()
	flowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeOAuth,
		Provider:  oauthCallbackContractProviderName,
		Intent:    intent,
		UserId:    userID,
		SessionId: sessionID,
		Payload:   `{}`,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	return flowToken
}

func TestOAuthLoginReusesExistingExternalIdentity(t *testing.T) {
	db, provider := setupOAuthCallbackContractTest(t)
	common.RegisterEnabled = false
	existingUser := &model.User{
		Username:    "existing-oauth-user",
		Password:    "unused",
		GitHubId:    provider.providerUserID,
		AffCode:     "existing-oauth-user-aff",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(existingUser).Error)

	router := gin.New()
	router.GET("/api/oauth/:provider", HandleOAuth)
	for range 2 {
		flowToken := createOAuthCallbackContractFlow(t, model.AuthFlowIntentLogin, 0, "")
		request := httptest.NewRequest(
			http.MethodGet,
			"/api/oauth/"+oauthCallbackContractProviderName+"?state="+flowToken+"&code=test",
			nil,
		)
		request.Header.Set("Accept-Language", "en")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code)
		var body struct {
			Success bool `json:"success"`
			Data    struct {
				AccessToken string `json:"access_token"`
				User        struct {
					ID int `json:"id"`
				} `json:"user"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
		require.True(t, body.Success)
		assert.NotEmpty(t, body.Data.AccessToken)
		assert.Equal(t, existingUser.Id, body.Data.User.ID)
	}

	var userCount int64
	require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
	assert.Equal(t, int64(1), userCount)
	var sessionCount int64
	require.NoError(t, db.Model(&model.UserSession{}).Count(&sessionCount).Error)
	assert.Equal(t, int64(2), sessionCount)
}

func TestOAuthBindRejectsIdentityOwnedByAnotherUserWithoutSwitchingLogin(t *testing.T) {
	db, provider := setupOAuthCallbackContractTest(t)
	owner := &model.User{
		Username: "oauth-owner", Password: "unused", GitHubId: provider.providerUserID,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
		AffCode: "oauth-owner-aff",
	}
	binder := &model.User{
		Username: "oauth-binder", Password: "unused",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
		AffCode: "oauth-binder-aff",
	}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(binder).Error)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.UserSession{
		SID: "binder-session", UserID: binder.Id, Version: 1, UserAuthVersion: binder.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: "existing-refresh-hash", LoginMethod: "password",
		CreatedAt: now, LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)
	flowToken := createOAuthCallbackContractFlow(t, model.AuthFlowIntentBind, binder.Id, "binder-session")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", binder.Id)
		c.Set("session_id", "binder-session")
		c.Set("auth_version", binder.AuthVersion)
		c.Set("session_version", int64(1))
		c.Next()
	})
	router.GET("/api/oauth/:provider", HandleOAuth)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/oauth/"+oauthCallbackContractProviderName+"?state="+flowToken+"&code=test",
		nil,
	)
	request.Header.Set("Accept-Language", "en")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, i18n.MsgOAuthAlreadyBound, body.Message)
	assert.Empty(t, body.Data)
	assert.Empty(t, response.Header().Values("Set-Cookie"))

	var reloadedOwner model.User
	require.NoError(t, db.First(&reloadedOwner, owner.Id).Error)
	assert.Equal(t, provider.providerUserID, reloadedOwner.GitHubId)
	var reloadedBinder model.User
	require.NoError(t, db.First(&reloadedBinder, binder.Id).Error)
	assert.Empty(t, reloadedBinder.GitHubId)
	var sessionCount int64
	require.NoError(t, db.Model(&model.UserSession{}).Count(&sessionCount).Error)
	assert.Equal(t, int64(1), sessionCount)
}

var _ oauth.Provider = (*oauthCallbackContractProvider)(nil)
