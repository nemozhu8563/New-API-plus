package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	errorMonitorDefaultIntervalSeconds = 60
	errorMonitorDefaultWindowSeconds   = 300
	errorMonitorDefaultThreshold       = 5
	errorMonitorDefaultCooldownSeconds = 1800
	errorMonitorDefaultMaxLogs         = 500
)

type ErrorMonitorSetting struct {
	Enabled          bool
	IntervalSeconds  int
	WindowSeconds    int
	Threshold        int
	CooldownSeconds  int
	MaxLogs          int
	StatusCodes      map[int]struct{}
	ContentKeywords  []string
	FeishuWebhookURL string
	FeishuSecret     string
}

type ErrorMonitorRunSummary struct {
	Scanned          int                        `json:"scanned"`
	Matched          int                        `json:"matched"`
	Alerted          int                        `json:"alerted"`
	Suppressed       int                        `json:"suppressed"`
	WindowSeconds    int                        `json:"window_seconds"`
	Threshold        int                        `json:"threshold"`
	CooldownSeconds  int                        `json:"cooldown_seconds"`
	Groups           []ErrorMonitorGroupSummary `json:"groups,omitempty"`
	DisabledReason   string                     `json:"disabled_reason,omitempty"`
	NotificationFail string                     `json:"notification_fail,omitempty"`
}

type ErrorMonitorGroupSummary struct {
	Key          string      `json:"key"`
	Count        int         `json:"count"`
	ChannelId    int         `json:"channel_id"`
	ChannelName  string      `json:"channel_name,omitempty"`
	ModelName    string      `json:"model_name,omitempty"`
	StatusCode   int         `json:"status_code"`
	StatusCounts map[int]int `json:"status_counts,omitempty"`
	ErrorCode    string      `json:"error_code,omitempty"`
	Sent         bool        `json:"sent"`
	Suppressed   bool        `json:"suppressed"`
}

type errorMonitorGroup struct {
	ErrorMonitorGroupSummary
	FirstAt        int64
	LastAt         int64
	ErrorType      string
	ContentSample  string
	RequestIds     []string
	StatusMatched  bool
	KeywordMatched bool
}

type feishuTextContent struct {
	Text string `json:"text"`
}

type feishuWebhookMessage struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Sign      string            `json:"sign,omitempty"`
	MsgType   string            `json:"msg_type"`
	Content   feishuTextContent `json:"content"`
}

var errorMonitorCooldown = struct {
	sync.Mutex
	lastSentAt map[string]int64
}{
	lastSentAt: map[string]int64{},
}

func LoadErrorMonitorSetting() ErrorMonitorSetting {
	setting := ErrorMonitorSetting{
		Enabled:          common.GetEnvOrDefaultBool("ERROR_MONITOR_ENABLED", false),
		IntervalSeconds:  common.GetEnvOrDefault("ERROR_MONITOR_INTERVAL_SECONDS", errorMonitorDefaultIntervalSeconds),
		WindowSeconds:    common.GetEnvOrDefault("ERROR_MONITOR_WINDOW_SECONDS", errorMonitorDefaultWindowSeconds),
		Threshold:        common.GetEnvOrDefault("ERROR_MONITOR_THRESHOLD", errorMonitorDefaultThreshold),
		CooldownSeconds:  common.GetEnvOrDefault("ERROR_MONITOR_COOLDOWN_SECONDS", errorMonitorDefaultCooldownSeconds),
		MaxLogs:          common.GetEnvOrDefault("ERROR_MONITOR_MAX_LOGS", errorMonitorDefaultMaxLogs),
		StatusCodes:      parseStatusCodeSet(common.GetEnvOrDefaultString("ERROR_MONITOR_STATUS_CODES", "502,503,504")),
		ContentKeywords:  parseCSVList(common.GetEnvOrDefaultString("ERROR_MONITOR_CONTENT_KEYWORDS", "Upstream request failed")),
		FeishuWebhookURL: strings.TrimSpace(common.GetEnvOrDefaultString("ERROR_MONITOR_FEISHU_WEBHOOK_URL", "")),
		FeishuSecret:     strings.TrimSpace(common.GetEnvOrDefaultString("ERROR_MONITOR_FEISHU_SECRET", "")),
	}
	if setting.IntervalSeconds < 15 {
		setting.IntervalSeconds = errorMonitorDefaultIntervalSeconds
	}
	if setting.WindowSeconds < 1 {
		setting.WindowSeconds = errorMonitorDefaultWindowSeconds
	}
	if setting.Threshold < 1 {
		setting.Threshold = errorMonitorDefaultThreshold
	}
	if setting.CooldownSeconds < 0 {
		setting.CooldownSeconds = errorMonitorDefaultCooldownSeconds
	}
	if setting.MaxLogs < setting.Threshold {
		setting.MaxLogs = setting.Threshold
	}
	return setting
}

func IsErrorMonitorEnabled() bool {
	setting := LoadErrorMonitorSetting()
	return setting.Enabled && setting.FeishuWebhookURL != ""
}

func ErrorMonitorInterval() time.Duration {
	return time.Duration(LoadErrorMonitorSetting().IntervalSeconds) * time.Second
}

func RunErrorMonitorOnce(ctx context.Context) (ErrorMonitorRunSummary, error) {
	return runErrorMonitorOnce(ctx, LoadErrorMonitorSetting(), SendFeishuWebhookNotify, common.GetTimestamp())
}

func runErrorMonitorOnce(ctx context.Context, setting ErrorMonitorSetting, send func(string, string, string) error, now int64) (ErrorMonitorRunSummary, error) {
	summary := ErrorMonitorRunSummary{
		WindowSeconds:   setting.WindowSeconds,
		Threshold:       setting.Threshold,
		CooldownSeconds: setting.CooldownSeconds,
	}
	if !setting.Enabled {
		summary.DisabledReason = "disabled"
		return summary, nil
	}
	if setting.FeishuWebhookURL == "" {
		summary.DisabledReason = "missing_feishu_webhook_url"
		return summary, nil
	}

	logs, err := model.GetRecentErrorLogsSince(now-int64(setting.WindowSeconds), setting.MaxLogs)
	if err != nil {
		return summary, err
	}
	summary.Scanned = len(logs)

	groups := collectErrorMonitorGroups(logs, setting)
	for _, group := range groups {
		summary.Matched += group.Count
		if group.Count < setting.Threshold {
			continue
		}

		item := group.ErrorMonitorGroupSummary
		if withinErrorMonitorCooldown(group.Key, now, int64(setting.CooldownSeconds)) {
			item.Suppressed = true
			summary.Suppressed++
			summary.Groups = append(summary.Groups, item)
			continue
		}

		if err := send(setting.FeishuWebhookURL, setting.FeishuSecret, formatErrorMonitorAlert(group, setting)); err != nil {
			summary.NotificationFail = err.Error()
			return summary, err
		}
		markErrorMonitorSent(group.Key, now)
		item.Sent = true
		summary.Alerted++
		summary.Groups = append(summary.Groups, item)

		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}
	}

	return summary, nil
}

func collectErrorMonitorGroups(logs []*model.Log, setting ErrorMonitorSetting) map[string]*errorMonitorGroup {
	groups := map[string]*errorMonitorGroup{}
	for _, log := range logs {
		other, _ := common.StrToMap(log.Other)
		statusCode, _ := mapIntValue(other, "status_code")
		errorCode := mapStringValue(other, "error_code")
		errorType := mapStringValue(other, "error_type")
		_, statusMatched := setting.StatusCodes[statusCode]
		keywordMatched := len(setting.ContentKeywords) > 0 && matchesErrorMonitorKeywords(log.Content, errorCode, errorType, setting.ContentKeywords)

		channelId := log.ChannelId
		if channelId == 0 {
			channelId, _ = mapIntValue(other, "channel_id")
		}
		channelName := mapStringValue(other, "channel_name")
		key := fmt.Sprintf("%d|%s|%s|%s", channelId, log.ModelName, errorType, errorCode)
		group := groups[key]
		if group == nil {
			group = &errorMonitorGroup{
				ErrorMonitorGroupSummary: ErrorMonitorGroupSummary{
					Key:          key,
					ChannelId:    channelId,
					ChannelName:  channelName,
					ModelName:    log.ModelName,
					StatusCode:   statusCode,
					StatusCounts: map[int]int{},
					ErrorCode:    errorCode,
				},
				FirstAt:       log.CreatedAt,
				LastAt:        log.CreatedAt,
				ErrorType:     errorType,
				ContentSample: common.LocalLogPreview(log.Content),
			}
			groups[key] = group
		}
		group.Count++
		group.StatusCounts[statusCode]++
		group.StatusMatched = group.StatusMatched || statusMatched
		group.KeywordMatched = group.KeywordMatched || keywordMatched
		if log.CreatedAt < group.FirstAt {
			group.FirstAt = log.CreatedAt
		}
		if log.CreatedAt > group.LastAt {
			group.LastAt = log.CreatedAt
		}
		if log.RequestId != "" && len(group.RequestIds) < 3 {
			group.RequestIds = append(group.RequestIds, log.RequestId)
		}
	}
	return groups
}

func formatErrorMonitorAlert(group *errorMonitorGroup, setting ErrorMonitorSetting) string {
	statusCodes := make([]int, 0, len(group.StatusCounts))
	for statusCode := range group.StatusCounts {
		statusCodes = append(statusCodes, statusCode)
	}
	sort.Ints(statusCodes)
	statusParts := make([]string, 0, len(statusCodes))
	for _, statusCode := range statusCodes {
		statusParts = append(statusParts, fmt.Sprintf("%d×%d", statusCode, group.StatusCounts[statusCode]))
	}

	matchedBy := make([]string, 0, 2)
	if group.StatusMatched {
		matchedBy = append(matchedBy, "状态码")
	}
	if group.KeywordMatched {
		matchedBy = append(matchedBy, "内容关键词")
	}
	matchDescription := "重复异常兜底"
	if len(matchedBy) > 0 {
		matchDescription = strings.Join(matchedBy, "、")
	}
	lines := []string{
		"new-api 上游错误监控告警",
		fmt.Sprintf("最近 %d 秒内同一组错误出现 %d 次，已达到阈值 %d。", setting.WindowSeconds, group.Count, setting.Threshold),
		"匹配来源: " + matchDescription,
		fmt.Sprintf("渠道: #%d %s", group.ChannelId, group.ChannelName),
		fmt.Sprintf("模型: %s", group.ModelName),
		"状态码: " + strings.Join(statusParts, ", "),
		fmt.Sprintf("错误码: %s", group.ErrorCode),
		fmt.Sprintf("错误类型: %s", group.ErrorType),
		fmt.Sprintf("时间范围: %s - %s", formatAlertTime(group.FirstAt), formatAlertTime(group.LastAt)),
	}
	if len(group.RequestIds) > 0 {
		lines = append(lines, "样例 request_id: "+strings.Join(group.RequestIds, ", "))
	}
	if group.ContentSample != "" {
		lines = append(lines, "样例错误: "+group.ContentSample)
	}
	lines = append(lines, "建议: 先检查该渠道上游状态、供应商限流/额度、网络连接；如果持续触发，临时降权或禁用该渠道。")
	return strings.Join(lines, "\n")
}

func SendFeishuWebhookNotify(webhookURL string, secret string, text string) error {
	payload := feishuWebhookMessage{
		MsgType: "text",
		Content: feishuTextContent{Text: text},
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload.Timestamp = timestamp
		payload.Sign = generateFeishuSign(timestamp, secret)
	}

	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal feishu payload: %v", err)
	}

	if system_setting.EnableWorker() {
		workerReq := &WorkerRequest{
			URL:    webhookURL,
			Key:    system_setting.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: payloadBytes,
		}
		resp, err := DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send feishu request through worker: %v", err)
		}
		defer resp.Body.Close()
		return checkFeishuWebhookResponse(resp)
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(webhookURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("request reject: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create feishu request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send feishu request: %v", err)
	}
	defer resp.Body.Close()
	return checkFeishuWebhookResponse(resp)
}

func checkFeishuWebhookResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu request failed with status code: %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := common.DecodeJson(io.LimitReader(resp.Body, 4096), &data); err != nil {
		return fmt.Errorf("failed to decode feishu response: %v", err)
	}
	if code, ok := mapIntValue(data, "code"); ok && code != 0 {
		return fmt.Errorf("feishu request failed with code %d: %s", code, feishuResponseMessage(data))
	}
	if code, ok := mapIntValue(data, "StatusCode"); ok && code != 0 {
		return fmt.Errorf("feishu request failed with status code %d: %s", code, feishuResponseMessage(data))
	}
	if code, ok := mapIntValue(data, "status_code"); ok && code != 0 {
		return fmt.Errorf("feishu request failed with status code %d: %s", code, feishuResponseMessage(data))
	}
	return nil
}

func generateFeishuSign(timestamp string, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func parseStatusCodeSet(raw string) map[int]struct{} {
	values := parseCSVList(raw)
	if len(values) == 0 {
		return nil
	}
	set := map[int]struct{}{}
	for _, value := range values {
		code, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		set[code] = struct{}{}
	}
	return set
}

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func matchesErrorMonitorKeywords(content string, errorCode string, errorType string, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}
	target := strings.ToLower(strings.Join([]string{content, errorCode, errorType}, " "))
	for _, keyword := range keywords {
		if strings.Contains(target, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func mapStringValue(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func mapIntValue(m map[string]interface{}, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(v)
		return n, err == nil
	default:
		return 0, false
	}
}

func feishuResponseMessage(m map[string]interface{}) string {
	for _, key := range []string{"msg", "message", "StatusMessage", "status_message"} {
		if value := mapStringValue(m, key); value != "" {
			return value
		}
	}
	return "unknown"
}

func withinErrorMonitorCooldown(key string, now int64, cooldownSeconds int64) bool {
	if cooldownSeconds <= 0 {
		return false
	}
	errorMonitorCooldown.Lock()
	defer errorMonitorCooldown.Unlock()
	lastSentAt := errorMonitorCooldown.lastSentAt[key]
	return lastSentAt > 0 && now-lastSentAt < cooldownSeconds
}

func markErrorMonitorSent(key string, now int64) {
	errorMonitorCooldown.Lock()
	defer errorMonitorCooldown.Unlock()
	errorMonitorCooldown.lastSentAt[key] = now
}

func formatAlertTime(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}
