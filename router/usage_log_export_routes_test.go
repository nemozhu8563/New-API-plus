package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUsageLogExportRouterTest(t *testing.T) (*gin.Engine, string, string) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousCriticalRateLimitEnabled := common.CriticalRateLimitEnable
	previousGlobalAPIRateLimitEnabled := common.GlobalApiRateLimitEnable
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.CriticalRateLimitEnable = false
	common.GlobalApiRateLimitEnable = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.Channel{},
		&model.Midjourney{},
		&model.Task{},
	))

	userToken := "usage-export-user-token"
	adminToken := "usage-export-admin-token"
	require.NoError(t, db.Create(&model.User{
		Username:    "export-user",
		Password:    "unused-password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "export-user",
		AccessToken: &userToken,
		AuthVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Username:    "export-admin",
		Password:    "unused-password",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "export-admin",
		AccessToken: &adminToken,
		AuthVersion: 1,
	}).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.CriticalRateLimitEnable = previousCriticalRateLimitEnabled
		common.GlobalApiRateLimitEnable = previousGlobalAPIRateLimitEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return engine, userToken, adminToken
}

func requestUsageLogExport(engine *gin.Engine, target string, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestUsageLogExportRoutesEnforceAdminAndSelfScopes(t *testing.T) {
	engine, userToken, adminToken := setupUsageLogExportRouterTest(t)
	routePairs := [][2]string{
		{"/api/log/export", "/api/log/self/export"},
		{"/api/mj/export", "/api/mj/self/export"},
		{"/api/task/export", "/api/task/self/export"},
	}

	for _, routes := range routePairs {
		adminWithUser := requestUsageLogExport(engine, routes[0], userToken)
		assert.Equal(t, http.StatusForbidden, adminWithUser.Code, routes[0])

		adminExport := requestUsageLogExport(engine, routes[0], adminToken)
		assert.Equal(t, http.StatusOK, adminExport.Code, routes[0])
		assert.Equal(t, "text/csv; charset=utf-8", adminExport.Header().Get("Content-Type"), routes[0])

		selfExport := requestUsageLogExport(engine, routes[1], userToken)
		assert.Equal(t, http.StatusOK, selfExport.Code, routes[1])
		assert.Equal(t, "text/csv; charset=utf-8", selfExport.Header().Get("Content-Type"), routes[1])
	}
}
