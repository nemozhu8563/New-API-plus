package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	channelCircuitFailureKeyPrefix = "new-api:channel_circuit_breaker:v1:failures:"
	channelCircuitOpenKeyPrefix    = "new-api:channel_circuit_breaker:v1:open:"
	ginKeyChannelCircuitFailover   = "channel_circuit_breaker_failover"
	ginKeyChannelCircuitBypass     = "channel_circuit_breaker_bypass"
)

var recordChannelCircuitFailureScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
if count >= tonumber(ARGV[2]) then
  redis.call("SET", KEYS[2], ARGV[4], "EX", ARGV[3])
  redis.call("DEL", KEYS[1])
  return {count, 1, tonumber(ARGV[4])}
end
return {count, 0, 0}
`)

type ChannelCircuitDecision struct {
	Count     int
	Tripped   bool
	OpenUntil time.Time
}

type channelCircuitFailureWindow struct {
	Count     int
	ExpiresAt time.Time
}

var channelCircuitMemory = struct {
	sync.Mutex
	Failures map[int]channelCircuitFailureWindow
	Open     map[int]time.Time
}{
	Failures: make(map[int]channelCircuitFailureWindow),
	Open:     make(map[int]time.Time),
}

var channelCircuitNow = time.Now

func RecordChannelCircuitFailure(channelID int, statusCode int) ChannelCircuitDecision {
	if !IsChannelCircuitManagedFailure(channelID, statusCode) {
		return ChannelCircuitDecision{}
	}

	setting := operation_setting.GetChannelCircuitBreakerSetting()
	now := channelCircuitNow()
	memoryDecision := recordChannelCircuitFailureInMemory(channelID, now, setting.FailureThreshold, setting.WindowSeconds, setting.OpenSeconds)
	if !common.RedisEnabled || common.RDB == nil {
		return memoryDecision
	}

	result, err := recordChannelCircuitFailureScript.Run(
		context.Background(),
		common.RDB,
		[]string{channelCircuitFailureKey(channelID), channelCircuitOpenKey(channelID)},
		setting.WindowSeconds,
		setting.FailureThreshold,
		setting.OpenSeconds,
		now.Add(time.Duration(setting.OpenSeconds)*time.Second).Unix(),
	).Int64Slice()
	if err != nil || len(result) != 3 {
		common.SysError(fmt.Sprintf("channel circuit breaker Redis record failed: channel_id=%d, err=%v", channelID, err))
		return memoryDecision
	}

	decision := ChannelCircuitDecision{Count: int(result[0]), Tripped: result[1] == 1}
	if result[2] > 0 {
		decision.OpenUntil = time.Unix(result[2], 0)
	}
	return decision
}

func IsChannelCircuitManagedFailure(channelID int, statusCode int) bool {
	return operation_setting.IsChannelCircuitBreakerEnabledForChannel(channelID) &&
		operation_setting.ShouldCountChannelCircuitStatus(statusCode)
}

func RecordChannelCircuitSuccess(channelID int) {
	if !operation_setting.IsChannelCircuitBreakerEnabledForChannel(channelID) {
		return
	}

	channelCircuitMemory.Lock()
	delete(channelCircuitMemory.Failures, channelID)
	channelCircuitMemory.Unlock()

	if common.RedisEnabled && common.RDB != nil {
		if err := common.RDB.Del(context.Background(), channelCircuitFailureKey(channelID)).Err(); err != nil {
			common.SysError(fmt.Sprintf("channel circuit breaker Redis success reset failed: channel_id=%d, err=%v", channelID, err))
		}
	}
}

func IsChannelCircuitOpen(channelID int) bool {
	if !operation_setting.IsChannelCircuitBreakerEnabledForChannel(channelID) {
		return false
	}

	if common.RedisEnabled && common.RDB != nil {
		_, err := common.RDB.Get(context.Background(), channelCircuitOpenKey(channelID)).Result()
		if err == nil {
			return true
		} else if err != redis.Nil {
			common.SysError(fmt.Sprintf("channel circuit breaker Redis read failed: channel_id=%d, err=%v", channelID, err))
		}
	}

	return isChannelCircuitOpenInMemory(channelID, channelCircuitNow())
}

func GetOpenChannelCircuitIDs() map[int]struct{} {
	openChannels := make(map[int]struct{})
	setting := operation_setting.GetChannelCircuitBreakerSetting()
	if setting == nil || !setting.Enabled {
		return openChannels
	}
	for _, channelID := range setting.ChannelIDs {
		if IsChannelCircuitOpen(channelID) {
			openChannels[channelID] = struct{}{}
		}
	}
	return openChannels
}

func ShouldEmergencyChannelCircuitFailover(c *gin.Context, decision ChannelCircuitDecision) bool {
	setting := operation_setting.GetChannelCircuitBreakerSetting()
	if c == nil || setting == nil || !setting.EmergencyFailover || !decision.Tripped || IsChannelCircuitFailover(c) {
		return false
	}
	_, specificChannel := c.Get("specific_channel_id")
	return !specificChannel
}

func MarkChannelCircuitFailover(c *gin.Context) {
	if c != nil {
		c.Set(ginKeyChannelCircuitFailover, true)
	}
}

func IsChannelCircuitFailover(c *gin.Context) bool {
	return c != nil && c.GetBool(ginKeyChannelCircuitFailover)
}

func MarkChannelCircuitBypass(c *gin.Context) {
	if c != nil {
		c.Set(ginKeyChannelCircuitBypass, true)
	}
}

func IsChannelCircuitBypass(c *gin.Context) bool {
	return c != nil && c.GetBool(ginKeyChannelCircuitBypass)
}

func recordChannelCircuitFailureInMemory(channelID int, now time.Time, threshold int, windowSeconds int, openSeconds int) ChannelCircuitDecision {
	channelCircuitMemory.Lock()
	defer channelCircuitMemory.Unlock()

	if openUntil, exists := channelCircuitMemory.Open[channelID]; exists && !openUntil.After(now) {
		delete(channelCircuitMemory.Open, channelID)
	}
	failure := channelCircuitMemory.Failures[channelID]
	if failure.ExpiresAt.IsZero() || !failure.ExpiresAt.After(now) {
		failure = channelCircuitFailureWindow{ExpiresAt: now.Add(time.Duration(windowSeconds) * time.Second)}
	}
	failure.Count++
	if failure.Count < threshold {
		channelCircuitMemory.Failures[channelID] = failure
		return ChannelCircuitDecision{Count: failure.Count}
	}

	openUntil := now.Add(time.Duration(openSeconds) * time.Second)
	delete(channelCircuitMemory.Failures, channelID)
	channelCircuitMemory.Open[channelID] = openUntil
	return ChannelCircuitDecision{Count: failure.Count, Tripped: true, OpenUntil: openUntil}
}

func isChannelCircuitOpenInMemory(channelID int, now time.Time) bool {
	channelCircuitMemory.Lock()
	defer channelCircuitMemory.Unlock()
	openUntil, exists := channelCircuitMemory.Open[channelID]
	if !exists {
		return false
	}
	if !openUntil.After(now) {
		delete(channelCircuitMemory.Open, channelID)
		return false
	}
	return true
}

func channelCircuitFailureKey(channelID int) string {
	return channelCircuitFailureKeyPrefix + strconv.Itoa(channelID)
}

func channelCircuitOpenKey(channelID int) string {
	return channelCircuitOpenKeyPrefix + strconv.Itoa(channelID)
}
