package operation_setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ChannelCircuitBreakerSettingPrefix = "channel_circuit_breaker_setting."

type ChannelCircuitBreakerSetting struct {
	Enabled            bool   `json:"enabled"`
	ChannelIDs         []int  `json:"channel_ids"`
	FailureStatusCodes string `json:"failure_status_codes"`
	FailureThreshold   int    `json:"failure_threshold"`
	WindowSeconds      int    `json:"window_seconds"`
	OpenSeconds        int    `json:"open_seconds"`
	EmergencyFailover  bool   `json:"emergency_failover"`
}

var channelCircuitBreakerSetting = ChannelCircuitBreakerSetting{
	Enabled:            false,
	ChannelIDs:         []int{},
	FailureStatusCodes: "524",
	FailureThreshold:   2,
	WindowSeconds:      60,
	OpenSeconds:        600,
	EmergencyFailover:  true,
}

func init() {
	config.GlobalConfig.Register("channel_circuit_breaker_setting", &channelCircuitBreakerSetting)
}

func GetChannelCircuitBreakerSetting() *ChannelCircuitBreakerSetting {
	return &channelCircuitBreakerSetting
}

func IsChannelCircuitBreakerEnabledForChannel(channelID int) bool {
	setting := GetChannelCircuitBreakerSetting()
	if setting == nil || !setting.Enabled || channelID <= 0 {
		return false
	}
	for _, configuredID := range setting.ChannelIDs {
		if configuredID == channelID {
			return true
		}
	}
	return false
}

func ShouldCountChannelCircuitStatus(statusCode int) bool {
	setting := GetChannelCircuitBreakerSetting()
	if setting == nil {
		return false
	}
	ranges, err := ParseHTTPStatusCodeRanges(setting.FailureStatusCodes)
	if err != nil {
		common.SysError("invalid channel circuit breaker status codes: " + err.Error())
		return false
	}
	return shouldMatchStatusCodeRanges(ranges, statusCode)
}

func ValidateChannelCircuitBreakerOption(key string, value string) error {
	if !strings.HasPrefix(key, ChannelCircuitBreakerSettingPrefix) {
		return nil
	}
	field := strings.TrimPrefix(key, ChannelCircuitBreakerSettingPrefix)
	switch field {
	case "enabled":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s must be true or false", key)
		}
		if enabled && len(GetChannelCircuitBreakerSetting().ChannelIDs) == 0 {
			return fmt.Errorf("%s requires at least one configured channel ID", key)
		}
	case "emergency_failover":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be true or false", key)
		}
	case "channel_ids":
		var channelIDs []int
		if err := common.UnmarshalJsonStr(value, &channelIDs); err != nil {
			return fmt.Errorf("%s must be a JSON array of channel IDs", key)
		}
		seen := make(map[int]struct{}, len(channelIDs))
		for _, channelID := range channelIDs {
			if channelID <= 0 {
				return fmt.Errorf("%s only accepts positive channel IDs", key)
			}
			if _, exists := seen[channelID]; exists {
				return fmt.Errorf("%s contains duplicate channel ID %d", key, channelID)
			}
			seen[channelID] = struct{}{}
		}
		if len(channelIDs) == 0 && GetChannelCircuitBreakerSetting().Enabled {
			return fmt.Errorf("%s cannot be empty while the circuit breaker is enabled", key)
		}
	case "failure_status_codes":
		ranges, err := ParseHTTPStatusCodeRanges(value)
		if err != nil {
			return err
		}
		if len(ranges) == 0 {
			return fmt.Errorf("%s cannot be empty", key)
		}
	case "failure_threshold":
		return validateChannelCircuitInteger(key, value, 2, 100)
	case "window_seconds":
		return validateChannelCircuitInteger(key, value, 1, 3600)
	case "open_seconds":
		return validateChannelCircuitInteger(key, value, 1, 86400)
	default:
		return fmt.Errorf("unknown channel circuit breaker option: %s", key)
	}
	return nil
}

func validateChannelCircuitInteger(key string, value string, minValue int, maxValue int) error {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minValue || parsed > maxValue {
		return fmt.Errorf("%s must be an integer between %d and %d", key, minValue, maxValue)
	}
	return nil
}
