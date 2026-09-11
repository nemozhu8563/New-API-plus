package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate option table: %v", err)
	}
	model.InitOptionMap()

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
	})

	return db
}

func newOptionUpdateContext(t *testing.T, payload string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewBufferString(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestUpdateOptionAcceptsPublicGroupTagRatio(t *testing.T) {
	setupOptionControllerTestDB(t)

	original := ratio_setting.PublicGroupTagRatio2JSONString()
	t.Cleanup(func() {
		_ = ratio_setting.UpdatePublicGroupTagRatioByJSONString(original)
	})

	ctx, recorder := newOptionUpdateContext(t, `{"key":"group_ratio_setting.public_group_tag_ratio","value":"{\"ask-public\":{\"GPT\":1.6}}"}`)
	UpdateOption(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http 200, got %d", recorder.Code)
	}
	ratio, ok := ratio_setting.GetPublicGroupTagRatio("ask-public", "GPT")
	if !ok {
		t.Fatalf("expected persisted GPT ratio")
	}
	if ratio != 1.6 {
		t.Fatalf("expected ratio 1.6, got %v", ratio)
	}
}

func TestUpdateOptionRejectsNegativePublicGroupTagRatio(t *testing.T) {
	setupOptionControllerTestDB(t)

	ctx, recorder := newOptionUpdateContext(t, `{"key":"group_ratio_setting.public_group_tag_ratio","value":"{\"ask-public\":{\"GPT\":-1}}"}`)
	UpdateOption(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"success":false`) {
		t.Fatalf("expected validation failure, got %s", recorder.Body.String())
	}
}

func TestUpdateOptionRequiresCompleteGitHubOAuthConfiguration(t *testing.T) {
	originalClientID := common.GitHubClientId
	originalClientSecret := common.GitHubClientSecret
	originalEnabled := common.GitHubOAuthEnabled
	t.Cleanup(func() {
		common.GitHubClientId = originalClientID
		common.GitHubClientSecret = originalClientSecret
		common.GitHubOAuthEnabled = originalEnabled
	})

	testCases := []struct {
		name         string
		clientID     string
		clientSecret string
		wantSuccess  bool
	}{
		{name: "missing client ID", clientID: "", clientSecret: "secret"},
		{name: "blank client ID", clientID: "  ", clientSecret: "secret"},
		{name: "missing client secret", clientID: "client", clientSecret: ""},
		{name: "blank client secret", clientID: "client", clientSecret: "  "},
		{name: "complete", clientID: "client", clientSecret: "secret", wantSuccess: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupOptionControllerTestDB(t)
			common.GitHubClientId = testCase.clientID
			common.GitHubClientSecret = testCase.clientSecret
			common.GitHubOAuthEnabled = false

			ctx, recorder := newOptionUpdateContext(t, `{"key":"GitHubOAuthEnabled","value":true}`)
			UpdateOption(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, testCase.wantSuccess, response.Success)

			var count int64
			require.NoError(t, db.Model(&model.Option{}).
				Where("key = ? AND value = ?", "GitHubOAuthEnabled", "true").Count(&count).Error)
			if testCase.wantSuccess {
				assert.Equal(t, int64(1), count)
			} else {
				assert.Zero(t, count)
			}
		})
	}
}

func TestUpdateOptionRequiresCompleteOIDCConfiguration(t *testing.T) {
	original := *system_setting.GetOIDCSettings()
	t.Cleanup(func() {
		*system_setting.GetOIDCSettings() = original
	})

	complete := system_setting.OIDCSettings{
		ClientId:              "client",
		ClientSecret:          "secret",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		UserInfoEndpoint:      "https://accounts.example.com/userinfo",
	}
	testCases := []struct {
		name         string
		missingField string
		blank        bool
		wantSuccess  bool
	}{
		{name: "missing client ID", missingField: "client_id"},
		{name: "missing client secret", missingField: "client_secret"},
		{name: "missing authorization endpoint", missingField: "authorization_endpoint"},
		{name: "missing token endpoint", missingField: "token_endpoint"},
		{name: "missing user info endpoint", missingField: "user_info_endpoint"},
		{name: "blank required value", missingField: "token_endpoint", blank: true},
		{name: "complete", wantSuccess: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupOptionControllerTestDB(t)
			settings := complete
			missingValue := ""
			if testCase.blank {
				missingValue = "  "
			}
			switch testCase.missingField {
			case "client_id":
				settings.ClientId = missingValue
			case "client_secret":
				settings.ClientSecret = missingValue
			case "authorization_endpoint":
				settings.AuthorizationEndpoint = missingValue
			case "token_endpoint":
				settings.TokenEndpoint = missingValue
			case "user_info_endpoint":
				settings.UserInfoEndpoint = missingValue
			}
			*system_setting.GetOIDCSettings() = settings

			ctx, recorder := newOptionUpdateContext(t, `{"key":"oidc.enabled","value":true}`)
			UpdateOption(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, testCase.wantSuccess, response.Success)

			var count int64
			require.NoError(t, db.Model(&model.Option{}).
				Where("key = ? AND value = ?", "oidc.enabled", "true").Count(&count).Error)
			if testCase.wantSuccess {
				assert.Equal(t, int64(1), count)
			} else {
				assert.Zero(t, count)
			}
		})
	}
}
