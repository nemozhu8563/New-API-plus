package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupChannelCircuitBreakerTest(t *testing.T) *time.Time {
	t.Helper()

	setting := operation_setting.GetChannelCircuitBreakerSetting()
	originalSetting := *setting
	originalSetting.ChannelIDs = append([]int(nil), setting.ChannelIDs...)
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	originalNow := channelCircuitNow

	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	channelCircuitNow = func() time.Time { return now }
	*setting = operation_setting.ChannelCircuitBreakerSetting{
		Enabled:            true,
		ChannelIDs:         []int{18},
		FailureStatusCodes: "524",
		FailureThreshold:   2,
		WindowSeconds:      60,
		OpenSeconds:        600,
		EmergencyFailover:  true,
	}
	common.RedisEnabled = false
	common.RDB = nil
	channelCircuitMemory.Lock()
	channelCircuitMemory.Failures = make(map[int]channelCircuitFailureWindow)
	channelCircuitMemory.Open = make(map[int]time.Time)
	channelCircuitMemory.Unlock()

	t.Cleanup(func() {
		*setting = originalSetting
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		channelCircuitNow = originalNow
		channelCircuitMemory.Lock()
		channelCircuitMemory.Failures = make(map[int]channelCircuitFailureWindow)
		channelCircuitMemory.Open = make(map[int]time.Time)
		channelCircuitMemory.Unlock()
	})
	return &now
}

func TestChannelCircuitBreakerTripsOnSecondFailureWithinWindow(t *testing.T) {
	now := setupChannelCircuitBreakerTest(t)

	first := RecordChannelCircuitFailure(18, 524)
	assert.Equal(t, 1, first.Count)
	assert.False(t, first.Tripped)
	assert.False(t, IsChannelCircuitOpen(18))

	*now = now.Add(30 * time.Second)
	second := RecordChannelCircuitFailure(18, 524)
	assert.Equal(t, 2, second.Count)
	assert.True(t, second.Tripped)
	assert.True(t, IsChannelCircuitOpen(18))

	*now = now.Add(601 * time.Second)
	assert.False(t, IsChannelCircuitOpen(18))
}

func TestChannelCircuitBreakerWindowAndSuccessResetFailures(t *testing.T) {
	now := setupChannelCircuitBreakerTest(t)

	assert.Equal(t, 1, RecordChannelCircuitFailure(18, 524).Count)
	*now = now.Add(61 * time.Second)
	afterWindow := RecordChannelCircuitFailure(18, 524)
	assert.Equal(t, 1, afterWindow.Count)
	assert.False(t, afterWindow.Tripped)

	RecordChannelCircuitSuccess(18)
	afterSuccess := RecordChannelCircuitFailure(18, 524)
	assert.Equal(t, 1, afterSuccess.Count)
	assert.False(t, afterSuccess.Tripped)
}

func TestChannelCircuitBreakerIgnoresUnconfiguredFailures(t *testing.T) {
	setupChannelCircuitBreakerTest(t)

	assert.True(t, IsChannelCircuitManagedFailure(18, 524))
	assert.False(t, IsChannelCircuitManagedFailure(18, 503))
	assert.False(t, IsChannelCircuitManagedFailure(28, 524))
	assert.Equal(t, ChannelCircuitDecision{}, RecordChannelCircuitFailure(28, 524))
	assert.Equal(t, ChannelCircuitDecision{}, RecordChannelCircuitFailure(18, 503))
	assert.False(t, IsChannelCircuitOpen(28))
}

func TestChannelCircuitBreakerUsesRedisSharedState(t *testing.T) {
	now := setupChannelCircuitBreakerTest(t)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	common.RedisEnabled = true
	common.RDB = client

	assert.False(t, RecordChannelCircuitFailure(18, 524).Tripped)
	decision := RecordChannelCircuitFailure(18, 524)
	assert.True(t, decision.Tripped)
	assert.True(t, IsChannelCircuitOpen(18))
	assert.True(t, server.Exists(channelCircuitOpenKey(18)))

	server.FastForward(601 * time.Second)
	*now = now.Add(601 * time.Second)
	assert.False(t, IsChannelCircuitOpen(18))
}

func TestEmergencyChannelCircuitFailoverOnlyOnceAndNotForSpecificChannel(t *testing.T) {
	setupChannelCircuitBreakerTest(t)
	decision := ChannelCircuitDecision{Tripped: true}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.True(t, ShouldEmergencyChannelCircuitFailover(ctx, decision))
	MarkChannelCircuitFailover(ctx)
	assert.False(t, ShouldEmergencyChannelCircuitFailover(ctx, decision))
	assert.True(t, IsChannelCircuitFailover(ctx))

	specificCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	specificCtx.Set("specific_channel_id", "18")
	assert.False(t, ShouldEmergencyChannelCircuitFailover(specificCtx, decision))
}
