package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

var commonLogAdminCSVHeader = []string{
	"id",
	"created_at",
	"type",
	"user_id",
	"username",
	"token_name",
	"model_name",
	"quota",
	"prompt_tokens",
	"completion_tokens",
	"use_time",
	"is_stream",
	"channel_id",
	"channel_name",
	"token_id",
	"group",
	"request_id",
	"upstream_request_id",
	"content",
	"other",
}

var commonLogUserCSVHeader = []string{
	"id",
	"created_at",
	"type",
	"token_name",
	"model_name",
	"quota",
	"prompt_tokens",
	"completion_tokens",
	"use_time",
	"is_stream",
	"token_id",
	"group",
	"request_id",
	"upstream_request_id",
	"content",
	"other",
}

var midjourneyAdminCSVHeader = []string{
	"id",
	"user_id",
	"code",
	"action",
	"mj_id",
	"prompt",
	"prompt_en",
	"description",
	"state",
	"submit_time",
	"start_time",
	"finish_time",
	"status",
	"progress",
	"channel_id",
	"quota",
	"fail_reason",
}

var midjourneyUserCSVHeader = []string{
	"id",
	"code",
	"action",
	"mj_id",
	"prompt",
	"prompt_en",
	"description",
	"state",
	"submit_time",
	"start_time",
	"finish_time",
	"status",
	"progress",
	"quota",
	"fail_reason",
}

var taskAdminCSVHeader = []string{
	"id",
	"created_at",
	"updated_at",
	"task_id",
	"platform",
	"user_id",
	"username",
	"group",
	"channel_id",
	"quota",
	"action",
	"status",
	"fail_reason",
	"submit_time",
	"start_time",
	"finish_time",
	"progress",
	"properties",
	"data",
}

var taskUserCSVHeader = []string{
	"id",
	"created_at",
	"updated_at",
	"task_id",
	"platform",
	"quota",
	"action",
	"status",
	"fail_reason",
	"submit_time",
	"start_time",
	"finish_time",
	"progress",
	"properties",
	"data",
}

func parseLogQueryParams(c *gin.Context, includeAdminFilters bool) model.LogQueryParams {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel := 0
	username := ""
	if includeAdminFilters {
		channel, _ = strconv.Atoi(c.Query("channel"))
		username = c.Query("username")
	}
	return model.LogQueryParams{
		LogType:           logType,
		StartTimestamp:    startTimestamp,
		EndTimestamp:      endTimestamp,
		ModelName:         c.Query("model_name"),
		Username:          username,
		TokenName:         c.Query("token_name"),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
	}
}

func parseMidjourneyQueryParams(c *gin.Context, includeAdminFilters bool) model.TaskQueryParams {
	channelID := ""
	if includeAdminFilters {
		channelID = c.Query("channel_id")
	}
	return model.TaskQueryParams{
		ChannelID:      channelID,
		MjID:           c.Query("mj_id"),
		StartTimestamp: c.Query("start_timestamp"),
		EndTimestamp:   c.Query("end_timestamp"),
	}
}

func parseTaskQueryParams(c *gin.Context, includeAdminFilters bool) model.SyncTaskQueryParams {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelID := ""
	if includeAdminFilters {
		channelID = c.Query("channel_id")
	}
	return model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		ChannelID:      channelID,
		TaskID:         c.Query("task_id"),
		Action:         c.Query("action"),
		Status:         c.Query("status"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}
}

func validateCSVExportTotal(c *gin.Context, total int64) bool {
	if err := model.ValidateUsageLogExportCount(total); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return false
	}
	return true
}

func respondCSVExportError(c *gin.Context, err error) {
	logger.LogError(c, "failed to export usage logs: "+err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"message": "Failed to export usage logs",
	})
}

func streamCSVDownload(
	c *gin.Context,
	filename string,
	header []string,
	stream func(writeRow func([]string) error) error,
) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Cache-Control", "no-store")
	if _, err := c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		logger.LogError(c, "failed to write usage log CSV BOM: "+err.Error())
		return
	}

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write(header); err != nil {
		logger.LogError(c, "failed to write usage log CSV header: "+err.Error())
		return
	}
	streamErr := stream(func(row []string) error {
		safeRow := make([]string, len(row))
		for i, value := range row {
			safeRow[i] = sanitizeCSVCell(value)
		}
		return writer.Write(safeRow)
	})
	writer.Flush()
	if streamErr == nil {
		streamErr = writer.Error()
	}
	if streamErr != nil {
		logger.LogError(c, "failed while streaming usage log CSV: "+streamErr.Error())
	}
}

func sanitizeCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '@', '\t', '\r', '\n':
		return "'" + value
	case '-':
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "'" + value
		}
	}
	return value
}

func commonLogAdminCSVRow(log *model.Log) []string {
	return []string{
		strconv.Itoa(log.Id),
		strconv.FormatInt(log.CreatedAt, 10),
		strconv.Itoa(log.Type),
		strconv.Itoa(log.UserId),
		log.Username,
		log.TokenName,
		log.ModelName,
		strconv.Itoa(log.Quota),
		strconv.Itoa(log.PromptTokens),
		strconv.Itoa(log.CompletionTokens),
		strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream),
		strconv.Itoa(log.ChannelId),
		log.ChannelName,
		strconv.Itoa(log.TokenId),
		log.Group,
		log.RequestId,
		log.UpstreamRequestId,
		log.Content,
		log.Other,
	}
}

func commonLogUserCSVRow(log *model.Log) []string {
	return []string{
		strconv.Itoa(log.Id),
		strconv.FormatInt(log.CreatedAt, 10),
		strconv.Itoa(log.Type),
		log.TokenName,
		log.ModelName,
		strconv.Itoa(log.Quota),
		strconv.Itoa(log.PromptTokens),
		strconv.Itoa(log.CompletionTokens),
		strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream),
		strconv.Itoa(log.TokenId),
		log.Group,
		log.RequestId,
		log.UpstreamRequestId,
		log.Content,
		log.Other,
	}
}

func midjourneyAdminCSVRow(task *model.Midjourney) []string {
	return []string{
		strconv.Itoa(task.Id),
		strconv.Itoa(task.UserId),
		strconv.Itoa(task.Code),
		task.Action,
		task.MjId,
		task.Prompt,
		task.PromptEn,
		task.Description,
		task.State,
		strconv.FormatInt(task.SubmitTime, 10),
		strconv.FormatInt(task.StartTime, 10),
		strconv.FormatInt(task.FinishTime, 10),
		task.Status,
		task.Progress,
		strconv.Itoa(task.ChannelId),
		strconv.Itoa(task.Quota),
		task.FailReason,
	}
}

func midjourneyUserCSVRow(task *model.Midjourney) []string {
	return []string{
		strconv.Itoa(task.Id),
		strconv.Itoa(task.Code),
		task.Action,
		task.MjId,
		task.Prompt,
		task.PromptEn,
		task.Description,
		task.State,
		strconv.FormatInt(task.SubmitTime, 10),
		strconv.FormatInt(task.StartTime, 10),
		strconv.FormatInt(task.FinishTime, 10),
		task.Status,
		task.Progress,
		strconv.Itoa(task.Quota),
		task.FailReason,
	}
}

func taskCSVProperties(task *model.Task) (string, error) {
	data, err := common.Marshal(task.Properties)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func taskAdminCSVRow(task *model.Task) ([]string, error) {
	properties, err := taskCSVProperties(task)
	if err != nil {
		return nil, err
	}
	return []string{
		strconv.FormatInt(task.ID, 10),
		strconv.FormatInt(task.CreatedAt, 10),
		strconv.FormatInt(task.UpdatedAt, 10),
		task.TaskID,
		string(task.Platform),
		strconv.Itoa(task.UserId),
		task.Username,
		task.Group,
		strconv.Itoa(task.ChannelId),
		strconv.Itoa(task.Quota),
		task.Action,
		string(task.Status),
		task.FailReason,
		strconv.FormatInt(task.SubmitTime, 10),
		strconv.FormatInt(task.StartTime, 10),
		strconv.FormatInt(task.FinishTime, 10),
		task.Progress,
		properties,
		string(task.Data),
	}, nil
}

func taskUserCSVRow(task *model.Task) ([]string, error) {
	properties, err := taskCSVProperties(task)
	if err != nil {
		return nil, err
	}
	return []string{
		strconv.FormatInt(task.ID, 10),
		strconv.FormatInt(task.CreatedAt, 10),
		strconv.FormatInt(task.UpdatedAt, 10),
		task.TaskID,
		string(task.Platform),
		strconv.Itoa(task.Quota),
		task.Action,
		string(task.Status),
		task.FailReason,
		strconv.FormatInt(task.SubmitTime, 10),
		strconv.FormatInt(task.StartTime, 10),
		strconv.FormatInt(task.FinishTime, 10),
		task.Progress,
		properties,
		string(task.Data),
	}, nil
}

func ExportAllLogsCSV(c *gin.Context) {
	params := parseLogQueryParams(c, true)
	total, err := model.CountAllLogsForExport(params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-common-"+time.Now().Format("20060102")+".csv",
		commonLogAdminCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamAllLogsForExport(c.Request.Context(), params, func(log *model.Log) error {
				return writeRow(commonLogAdminCSVRow(log))
			})
		},
	)
}

func ExportUserLogsCSV(c *gin.Context) {
	userId := c.GetInt("id")
	params := parseLogQueryParams(c, false)
	total, err := model.CountUserLogsForExport(userId, params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-common-"+time.Now().Format("20060102")+".csv",
		commonLogUserCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamUserLogsForExport(c.Request.Context(), userId, params, func(log *model.Log) error {
				return writeRow(commonLogUserCSVRow(log))
			})
		},
	)
}

func ExportAllMidjourneyCSV(c *gin.Context) {
	params := parseMidjourneyQueryParams(c, true)
	total, err := model.CountAllMidjourneyForExport(params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-drawing-"+time.Now().Format("20060102")+".csv",
		midjourneyAdminCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamAllMidjourneyForExport(c.Request.Context(), params, func(task *model.Midjourney) error {
				return writeRow(midjourneyAdminCSVRow(task))
			})
		},
	)
}

func ExportUserMidjourneyCSV(c *gin.Context) {
	userId := c.GetInt("id")
	params := parseMidjourneyQueryParams(c, false)
	total, err := model.CountUserMidjourneyForExport(userId, params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-drawing-"+time.Now().Format("20060102")+".csv",
		midjourneyUserCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamUserMidjourneyForExport(c.Request.Context(), userId, params, func(task *model.Midjourney) error {
				return writeRow(midjourneyUserCSVRow(task))
			})
		},
	)
}

func ExportAllTaskCSV(c *gin.Context) {
	params := parseTaskQueryParams(c, true)
	total, err := model.CountAllTaskForExport(params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-task-"+time.Now().Format("20060102")+".csv",
		taskAdminCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamAllTaskForExport(c.Request.Context(), params, func(task *model.Task) error {
				row, err := taskAdminCSVRow(task)
				if err != nil {
					return err
				}
				return writeRow(row)
			})
		},
	)
}

func ExportUserTaskCSV(c *gin.Context) {
	userId := c.GetInt("id")
	params := parseTaskQueryParams(c, false)
	total, err := model.CountUserTaskForExport(userId, params)
	if err != nil {
		respondCSVExportError(c, err)
		return
	}
	if !validateCSVExportTotal(c, total) {
		return
	}
	streamCSVDownload(
		c,
		"usage-logs-task-"+time.Now().Format("20060102")+".csv",
		taskUserCSVHeader,
		func(writeRow func([]string) error) error {
			return model.StreamUserTaskForExport(c.Request.Context(), userId, params, func(task *model.Task) error {
				row, err := taskUserCSVRow(task)
				if err != nil {
					return err
				}
				return writeRow(row)
			})
		},
	)
}
