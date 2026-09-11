package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelCircuitBreakerOption(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{name: "channel IDs", key: ChannelCircuitBreakerSettingPrefix + "channel_ids", value: `[18,28]`},
		{name: "duplicate channel ID", key: ChannelCircuitBreakerSettingPrefix + "channel_ids", value: `[18,18]`, wantErr: true},
		{name: "invalid channel ID", key: ChannelCircuitBreakerSettingPrefix + "channel_ids", value: `[0]`, wantErr: true},
		{name: "status range", key: ChannelCircuitBreakerSettingPrefix + "failure_status_codes", value: "502-504,524"},
		{name: "empty statuses", key: ChannelCircuitBreakerSettingPrefix + "failure_status_codes", value: "", wantErr: true},
		{name: "threshold", key: ChannelCircuitBreakerSettingPrefix + "failure_threshold", value: "2"},
		{name: "threshold too low", key: ChannelCircuitBreakerSettingPrefix + "failure_threshold", value: "1", wantErr: true},
		{name: "window", key: ChannelCircuitBreakerSettingPrefix + "window_seconds", value: "60"},
		{name: "open duration", key: ChannelCircuitBreakerSettingPrefix + "open_seconds", value: "600"},
		{name: "boolean", key: ChannelCircuitBreakerSettingPrefix + "emergency_failover", value: "true"},
		{name: "unknown field", key: ChannelCircuitBreakerSettingPrefix + "unknown", value: "1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChannelCircuitBreakerOption(tt.key, tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestChannelCircuitBreakerChannelAndStatusMatching(t *testing.T) {
	setting := GetChannelCircuitBreakerSetting()
	original := *setting
	original.ChannelIDs = append([]int(nil), setting.ChannelIDs...)
	t.Cleanup(func() {
		*setting = original
	})

	setting.Enabled = true
	setting.ChannelIDs = []int{18}
	setting.FailureStatusCodes = "502-504,524"

	assert.True(t, IsChannelCircuitBreakerEnabledForChannel(18))
	assert.False(t, IsChannelCircuitBreakerEnabledForChannel(28))
	assert.True(t, ShouldCountChannelCircuitStatus(503))
	assert.True(t, ShouldCountChannelCircuitStatus(524))
	assert.False(t, ShouldCountChannelCircuitStatus(429))
}

func TestValidateChannelCircuitBreakerOptionRequiresChannelsWhenEnabled(t *testing.T) {
	setting := GetChannelCircuitBreakerSetting()
	original := *setting
	original.ChannelIDs = append([]int(nil), setting.ChannelIDs...)
	t.Cleanup(func() {
		*setting = original
	})

	setting.Enabled = false
	setting.ChannelIDs = nil
	require.Error(t, ValidateChannelCircuitBreakerOption(ChannelCircuitBreakerSettingPrefix+"enabled", "true"))

	setting.ChannelIDs = []int{18}
	require.NoError(t, ValidateChannelCircuitBreakerOption(ChannelCircuitBreakerSettingPrefix+"enabled", "true"))

	setting.Enabled = true
	require.Error(t, ValidateChannelCircuitBreakerOption(ChannelCircuitBreakerSettingPrefix+"channel_ids", "[]"))
}
