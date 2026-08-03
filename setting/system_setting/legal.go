package system_setting

import (
	_ "embed"

	"github.com/QuantumNous/new-api/setting/config"
)

//go:embed default_user_agreement.md
var defaultUserAgreement string

//go:embed default_privacy_policy.md
var defaultPrivacyPolicy string

type LegalSettings struct {
	UserAgreement string `json:"user_agreement"`
	PrivacyPolicy string `json:"privacy_policy"`
}

var defaultLegalSettings = LegalSettings{
	UserAgreement: defaultUserAgreement,
	PrivacyPolicy: defaultPrivacyPolicy,
}

func init() {
	config.GlobalConfig.Register("legal", &defaultLegalSettings)
}

func GetLegalSettings() *LegalSettings {
	return &defaultLegalSettings
}
