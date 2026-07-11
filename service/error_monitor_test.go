package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetErrorMonitorCooldownForTest() {
	errorMonitorCooldown.Lock()
	defer errorMonitorCooldown.Unlock()
	errorMonitorCooldown.lastSentAt = map[string]int64{}
}

func seedErrorMonitorLog(t *testing.T, now int64, channelId int, modelName string, statusCode any, content string) {
	t.Helper()
	other := map[string]interface{}{
		"status_code":  statusCode,
		"error_code":   "bad_response_status_code",
		"error_type":   "upstream_error",
		"channel_id":   channelId,
		"channel_name": "primary-upstream",
	}
	log := &model.Log{
		UserId:    1,
		CreatedAt: now,
		Type:      model.LogTypeError,
		Content:   content,
		ModelName: modelName,
		ChannelId: channelId,
		RequestId: "req-test",
		Other:     common.MapToJsonStr(other),
	}
	require.NoError(t, model.LOG_DB.Create(log).Error)
}

func TestRunErrorMonitorOnceAlertsAndSuppressesWithinCooldown(t *testing.T) {
	truncate(t)
	resetErrorMonitorCooldownForTest()

	now := int64(1800000000)
	for i := 0; i < 3; i++ {
		seedErrorMonitorLog(t, now-int64(i), 8, "gpt-test", "502", "status_code=502, Upstream request failed")
	}

	setting := ErrorMonitorSetting{
		Enabled:          true,
		WindowSeconds:    300,
		Threshold:        3,
		CooldownSeconds:  600,
		MaxLogs:          20,
		StatusCodes:      map[int]struct{}{502: {}},
		ContentKeywords:  []string{"Upstream request failed"},
		FeishuWebhookURL: "https://open.feishu.cn/test-webhook",
	}
	var sent []string
	send := func(_ string, _ string, text string) error {
		sent = append(sent, text)
		return nil
	}

	summary, err := runErrorMonitorOnce(context.Background(), setting, send, now)
	require.NoError(t, err)
	assert.Equal(t, 3, summary.Scanned)
	assert.Equal(t, 3, summary.Matched)
	assert.Equal(t, 1, summary.Alerted)
	assert.Len(t, sent, 1)
	assert.Contains(t, sent[0], "最近 300 秒内同一组错误出现 3 次")
	assert.Contains(t, sent[0], "渠道: #8 primary-upstream")

	summary, err = runErrorMonitorOnce(context.Background(), setting, send, now+60)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.Alerted)
	assert.Equal(t, 1, summary.Suppressed)
	assert.Len(t, sent, 1)
}

func TestRunErrorMonitorOnceMatchesStatusOrKeyword(t *testing.T) {
	truncate(t)
	resetErrorMonitorCooldownForTest()

	now := int64(1800000300)
	seedErrorMonitorLog(t, now, 8, "status-match", 502, "status_code=502, other failure")
	seedErrorMonitorLog(t, now, 9, "keyword-match", 400, "status_code=400, Upstream request failed")

	setting := ErrorMonitorSetting{
		Enabled:          true,
		WindowSeconds:    300,
		Threshold:        1,
		CooldownSeconds:  0,
		MaxLogs:          20,
		StatusCodes:      map[int]struct{}{502: {}},
		ContentKeywords:  []string{"Upstream request failed"},
		FeishuWebhookURL: "https://open.feishu.cn/test-webhook",
	}
	var sent []string
	summary, err := runErrorMonitorOnce(context.Background(), setting, func(_ string, _ string, text string) error {
		sent = append(sent, text)
		return nil
	}, now)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.Scanned)
	assert.Equal(t, 2, summary.Matched)
	assert.Equal(t, 2, summary.Alerted)
	require.Len(t, sent, 2)
	alerts := strings.Join(sent, "\n")
	assert.Contains(t, alerts, "状态码: 502")
	assert.Contains(t, alerts, "状态码: 400")
}

func TestRunErrorMonitorOnceAlertsForRepeatedUnconfiguredError(t *testing.T) {
	truncate(t)
	resetErrorMonitorCooldownForTest()

	now := int64(1800000450)
	seedErrorMonitorLog(t, now, 10, "fallback-match", 429, "status_code=429, rate limit exceeded")
	seedErrorMonitorLog(t, now-1, 10, "fallback-match", 429, "status_code=429, rate limit exceeded")

	setting := ErrorMonitorSetting{
		Enabled:          true,
		WindowSeconds:    300,
		Threshold:        2,
		CooldownSeconds:  0,
		MaxLogs:          20,
		StatusCodes:      map[int]struct{}{502: {}},
		ContentKeywords:  []string{"Upstream request failed"},
		FeishuWebhookURL: "https://open.feishu.cn/test-webhook",
	}
	var sent []string
	summary, err := runErrorMonitorOnce(context.Background(), setting, func(_ string, _ string, text string) error {
		sent = append(sent, text)
		return nil
	}, now)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.Scanned)
	assert.Equal(t, 2, summary.Matched)
	assert.Equal(t, 1, summary.Alerted)
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0], "状态码: 429")
}

func TestRunErrorMonitorOnceGroupsStatusVariantsOfSameError(t *testing.T) {
	truncate(t)
	resetErrorMonitorCooldownForTest()

	now := int64(1800000500)
	seedErrorMonitorLog(t, now, 11, "gateway-variants", 521, "status_code=521, upstream unavailable")
	seedErrorMonitorLog(t, now-1, 11, "gateway-variants", 521, "status_code=521, upstream unavailable")
	seedErrorMonitorLog(t, now-2, 11, "gateway-variants", 524, "status_code=524, upstream timeout")
	seedErrorMonitorLog(t, now-3, 11, "gateway-variants", 524, "status_code=524, upstream timeout")

	setting := ErrorMonitorSetting{
		Enabled:          true,
		WindowSeconds:    300,
		Threshold:        4,
		CooldownSeconds:  0,
		MaxLogs:          20,
		StatusCodes:      map[int]struct{}{502: {}},
		ContentKeywords:  []string{"Upstream request failed"},
		FeishuWebhookURL: "https://open.feishu.cn/test-webhook",
	}
	var sent []string
	summary, err := runErrorMonitorOnce(context.Background(), setting, func(_ string, _ string, text string) error {
		sent = append(sent, text)
		return nil
	}, now)
	require.NoError(t, err)
	assert.Equal(t, 4, summary.Matched)
	assert.Equal(t, 1, summary.Alerted)
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0], "状态码: 521×2, 524×2")
}

func TestRunErrorMonitorOnceReturnsNotificationError(t *testing.T) {
	truncate(t)
	resetErrorMonitorCooldownForTest()

	now := int64(1800000600)
	seedErrorMonitorLog(t, now, 8, "gpt-test", 502, "status_code=502, Upstream request failed")
	sendErr := errors.New("send failed")
	setting := ErrorMonitorSetting{
		Enabled:          true,
		WindowSeconds:    300,
		Threshold:        1,
		CooldownSeconds:  0,
		MaxLogs:          20,
		StatusCodes:      map[int]struct{}{502: {}},
		ContentKeywords:  []string{"Upstream request failed"},
		FeishuWebhookURL: "https://open.feishu.cn/test-webhook",
	}

	summary, err := runErrorMonitorOnce(context.Background(), setting, func(_ string, _ string, _ string) error {
		return sendErr
	}, now)
	require.ErrorIs(t, err, sendErr)
	assert.True(t, strings.Contains(summary.NotificationFail, "send failed"))
}

func TestCheckFeishuWebhookResponseDetectsBodyError(t *testing.T) {
	err := checkFeishuWebhookResponse(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"StatusCode":9499,"StatusMessage":"bad sign"}`)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad sign")
}

func TestCheckFeishuWebhookResponseAcceptsSuccessBody(t *testing.T) {
	err := checkFeishuWebhookResponse(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"StatusCode":0,"StatusMessage":"success"}`)),
	})
	require.NoError(t, err)
}
