package controller

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUsageLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.Channel{},
		&model.Midjourney{},
		&model.Task{},
		&model.User{},
	))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func decodeCSVResponse(t *testing.T, recorder *httptest.ResponseRecorder) [][]string {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/csv; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.True(t, bytes.HasPrefix(recorder.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF}))

	reader := csv.NewReader(bytes.NewReader(recorder.Body.Bytes()[3:]))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	return records
}

func TestExportUserLogsCSVUsesSelfScopeAndSanitizesSensitiveFields(t *testing.T) {
	db := setupUsageLogExportTestDB(t)
	matching := &model.Log{
		UserId:    7,
		CreatedAt: 120,
		Type:      model.LogTypeConsume,
		Username:  "alice",
		TokenName: "primary",
		ModelName: "gpt-5",
		ChannelId: 12,
		RequestId: "req-match",
		Content:   "=HYPERLINK(\"https://example.invalid\",\"matching\")",
		Other: common.MapToJsonStr(map[string]interface{}{
			"model_price": 0.004,
			"admin_info":  map[string]interface{}{"channel_key": "secret"},
			"audit_info":  map[string]interface{}{"route": "/api/test"},
			"stream_status": map[string]interface{}{
				"status": "completed",
			},
		}),
	}
	require.NoError(t, db.Create(matching).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    8,
		CreatedAt: 120,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-5",
		Content:   "other-user",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 7)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/log/self/export?type=2&model_name=gpt-5&start_timestamp=100&end_timestamp=200",
		nil,
	)

	ExportUserLogsCSV(ctx)

	records := decodeCSVResponse(t, recorder)
	require.Len(t, records, 2)
	assert.Equal(t, "id", records[0][0])
	assert.NotContains(t, records[0], "user_id")
	assert.NotContains(t, records[0], "username")
	assert.NotContains(t, records[0], "channel_id")
	assert.NotContains(t, records[0], "channel_name")
	assert.Equal(t, fmt.Sprint(matching.Id), records[1][0])
	assert.NotContains(t, strings.Join(records[1], ","), "admin_info")
	assert.NotContains(t, strings.Join(records[1], ","), "audit_info")
	assert.Contains(t, strings.Join(records[1], ","), "stream_status")
	assert.Contains(t, strings.Join(records[1], ","), "model_price")
	assert.Contains(t, records[1], "'=HYPERLINK(\"https://example.invalid\",\"matching\")")
}

func TestExportAllLogsCSVUsesStableColumnsAndLoadsChannelNamesFromDatabase(t *testing.T) {
	db := setupUsageLogExportTestDB(t)
	require.NoError(t, db.Create(&model.Channel{Id: 12, Name: "primary-channel"}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    7,
		CreatedAt: 120,
		Type:      model.LogTypeConsume,
		Username:  "alice",
		TokenName: "primary",
		ModelName: "gpt-5",
		ChannelId: 12,
		TokenId:   34,
		Group:     "default",
		RequestId: "req-match",
		Content:   "matching",
	}).Error)
	common.MemoryCacheEnabled = true

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/export?type=2", nil)

	ExportAllLogsCSV(ctx)

	records := decodeCSVResponse(t, recorder)
	require.Len(t, records, 2)
	require.Len(t, records[1], len(records[0]))
	channelNameIndex := slices.Index(records[0], "channel_name")
	tokenIDIndex := slices.Index(records[0], "token_id")
	groupIndex := slices.Index(records[0], "group")
	require.NotEqual(t, -1, channelNameIndex)
	require.NotEqual(t, -1, tokenIDIndex)
	require.NotEqual(t, -1, groupIndex)
	assert.Equal(t, "primary-channel", records[1][channelNameIndex])
	assert.Equal(t, "34", records[1][tokenIDIndex])
	assert.Equal(t, "default", records[1][groupIndex])
}

func TestExportAllTaskCSVNeverIncludesPrivateData(t *testing.T) {
	db := setupUsageLogExportTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "alice"}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task-match",
		Platform:   constant.TaskPlatform("kling"),
		UserId:     7,
		ChannelId:  3,
		Action:     "GENERATE",
		Status:     model.TaskStatusSuccess,
		SubmitTime: 1500,
		PrivateData: model.TaskPrivateData{
			Key:            "must-not-export",
			UpstreamTaskID: "upstream-secret",
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/task/export?platform=kling&status=SUCCESS",
		nil,
	)

	ExportAllTaskCSV(ctx)

	records := decodeCSVResponse(t, recorder)
	require.Len(t, records, 2)
	assert.NotContains(t, records[0], "private_data")
	assert.NotContains(t, recorder.Body.String(), "must-not-export")
	assert.NotContains(t, recorder.Body.String(), "upstream-secret")
	assert.Contains(t, records[1], "task-match")
	assert.Contains(t, records[1], "alice")
}
