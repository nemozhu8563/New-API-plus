package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUsageLogExportCountRejectsMoreThanLimit(t *testing.T) {
	require.NoError(t, ValidateUsageLogExportCount(UsageLogExportLimit))

	err := ValidateUsageLogExportCount(UsageLogExportLimit + 1)

	require.ErrorIs(t, err, ErrUsageLogExportLimitExceeded)
}

func TestStreamUserLogsForExportAppliesFiltersAndSanitizesSensitiveData(t *testing.T) {
	truncateTables(t)

	matching := &Log{
		UserId:    7,
		CreatedAt: 120,
		Type:      LogTypeConsume,
		Username:  "alice",
		TokenName: "primary",
		ModelName: "gpt-5",
		ChannelId: 12,
		RequestId: "req-match",
		Content:   "matching",
		Other: common.MapToJsonStr(map[string]interface{}{
			"model_price": 0.004,
			"admin_info":  map[string]interface{}{"channel_key": "secret"},
			"audit_info":  map[string]interface{}{"route": "/api/test"},
			"stream_status": map[string]interface{}{
				"status": "completed",
			},
		}),
	}
	require.NoError(t, LOG_DB.Create(matching).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    7,
		CreatedAt: 130,
		Type:      LogTypeConsume,
		ModelName: "other-model",
		RequestId: "req-other-model",
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    8,
		CreatedAt: 125,
		Type:      LogTypeConsume,
		ModelName: "gpt-5",
		RequestId: "req-other-user",
	}).Error)

	params := LogQueryParams{
		LogType:        LogTypeConsume,
		StartTimestamp: 100,
		EndTimestamp:   200,
		ModelName:      "gpt-5",
	}
	total, err := CountUserLogsForExport(7, params)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	var exported []*Log
	err = StreamUserLogsForExport(
		context.Background(),
		7,
		params,
		func(log *Log) error {
			exported = append(exported, log)
			return nil
		},
	)
	require.NoError(t, err)
	require.Len(t, exported, 1)
	assert.Equal(t, matching.Id, exported[0].Id, "CSV export must keep the database ID")
	assert.Empty(t, exported[0].ChannelName)

	other, err := common.StrToMap(exported[0].Other)
	require.NoError(t, err)
	assert.Contains(t, other, "model_price")
	assert.NotContains(t, other, "admin_info")
	assert.NotContains(t, other, "audit_info")
	assert.Contains(t, other, "stream_status")
}

func TestMidjourneyExportUsesTheSameScopeAndFiltersAsTheList(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Midjourney{}))
	require.NoError(t, DB.Exec("DELETE FROM midjourneys").Error)
	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM midjourneys").Error
	})

	require.NoError(t, DB.Create(&Midjourney{
		UserId:     7,
		ChannelId:  3,
		MjId:       "mj-match",
		SubmitTime: 1500,
		Status:     "SUCCESS",
	}).Error)
	require.NoError(t, DB.Create(&Midjourney{
		UserId:     8,
		ChannelId:  3,
		MjId:       "mj-match",
		SubmitTime: 1500,
		Status:     "SUCCESS",
	}).Error)
	require.NoError(t, DB.Create(&Midjourney{
		UserId:     7,
		ChannelId:  4,
		MjId:       "mj-outside-range",
		SubmitTime: 3000,
		Status:     "SUCCESS",
	}).Error)

	params := TaskQueryParams{
		ChannelID:      "3",
		MjID:           "mj-match",
		StartTimestamp: "1000",
		EndTimestamp:   "2000",
	}
	adminTotal, err := CountAllMidjourneyForExport(params)
	require.NoError(t, err)
	require.EqualValues(t, 2, adminTotal)

	var userItems []*Midjourney
	userTotal, err := CountUserMidjourneyForExport(7, params)
	require.NoError(t, err)
	require.EqualValues(t, 1, userTotal)
	require.NoError(t, StreamUserMidjourneyForExport(
		context.Background(),
		7,
		params,
		func(task *Midjourney) error {
			userItems = append(userItems, task)
			return nil
		},
	))
	require.Len(t, userItems, 1)
	assert.Equal(t, 7, userItems[0].UserId)
	assert.Equal(t, "mj-match", userItems[0].MjId)
}

func TestTaskExportAppliesFiltersAndNeverLoadsPrivateData(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Task{
		TaskID:     "task-match",
		Platform:   constant.TaskPlatform("kling"),
		UserId:     7,
		ChannelId:  3,
		Action:     "GENERATE",
		Status:     TaskStatusSuccess,
		SubmitTime: 1500,
		PrivateData: TaskPrivateData{
			Key:            "must-not-export",
			UpstreamTaskID: "upstream-secret",
		},
	}).Error)
	require.NoError(t, DB.Create(&Task{
		TaskID:     "task-other-user",
		Platform:   constant.TaskPlatform("kling"),
		UserId:     8,
		ChannelId:  3,
		Action:     "GENERATE",
		Status:     TaskStatusSuccess,
		SubmitTime: 1500,
	}).Error)
	require.NoError(t, DB.Create(&Task{
		TaskID:     "task-other-status",
		Platform:   constant.TaskPlatform("kling"),
		UserId:     7,
		ChannelId:  3,
		Action:     "GENERATE",
		Status:     TaskStatusFailure,
		SubmitTime: 1500,
	}).Error)

	params := SyncTaskQueryParams{
		Platform:       constant.TaskPlatform("kling"),
		ChannelID:      "3",
		Action:         "GENERATE",
		Status:         string(TaskStatusSuccess),
		StartTimestamp: 1000,
		EndTimestamp:   2000,
	}
	adminTotal, err := CountAllTaskForExport(params)
	require.NoError(t, err)
	require.EqualValues(t, 2, adminTotal)

	var userItems []*Task
	userTotal, err := CountUserTaskForExport(7, params)
	require.NoError(t, err)
	require.EqualValues(t, 1, userTotal)
	require.NoError(t, StreamUserTaskForExport(
		context.Background(),
		7,
		params,
		func(task *Task) error {
			userItems = append(userItems, task)
			return nil
		},
	))
	require.Len(t, userItems, 1)
	assert.Equal(t, "task-match", userItems[0].TaskID)
	assert.Equal(t, TaskPrivateData{}, userItems[0].PrivateData)
}
