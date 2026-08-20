package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInLegalDocumentsResolveSupportedLocales(t *testing.T) {
	tests := []struct {
		locale        string
		agreementText string
		privacyText   string
	}{
		{locale: "en", agreementText: "Effective date", privacyText: "Effective Date"},
		{locale: "zh", agreementText: "生效日期", privacyText: "生效日期"},
		{locale: "zh-CN", agreementText: "生效日期", privacyText: "生效日期"},
		{locale: "zhCN", agreementText: "生效日期", privacyText: "生效日期"},
		{locale: "zh-TW", agreementText: "生效日期", privacyText: "生效日期"},
		{locale: "zhTW", agreementText: "生效日期", privacyText: "生效日期"},
		{locale: "fr", agreementText: "Date d’entrée en vigueur", privacyText: "Date d’entrée en vigueur"},
		{locale: "ru", agreementText: "Дата вступления в силу", privacyText: "Дата вступления в силу"},
		{locale: "ja", agreementText: "発効日", privacyText: "発効日"},
		{locale: "vi", agreementText: "Ngày có hiệu lực", privacyText: "Ngày có hiệu lực"},
	}

	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			agreement := GetLocalizedUserAgreement(test.locale)
			privacy := GetLocalizedPrivacyPolicy(test.locale)

			require.NotEmpty(t, agreement)
			require.NotEmpty(t, privacy)
			assert.Contains(t, agreement, test.agreementText)
			assert.Contains(t, privacy, test.privacyText)
			assert.Contains(t, agreement, "contract@tryvalo.com")
			assert.Contains(t, privacy, "contract@tryvalo.com")
		})
	}
}

func TestLocalizedLegalDocumentsPreserveConfigurationOverrides(t *testing.T) {
	original := *GetLegalSettings()
	t.Cleanup(func() {
		*GetLegalSettings() = original
	})

	GetLegalSettings().UserAgreement = "https://example.com/custom-terms"
	GetLegalSettings().PrivacyPolicy = ""

	assert.Equal(t, "https://example.com/custom-terms", GetLocalizedUserAgreement("en"))
	assert.Empty(t, GetLocalizedPrivacyPolicy("en"))
}

func TestLocalizedLegalDocumentsKeepLegacyBehaviorWithoutLocale(t *testing.T) {
	settings := GetLegalSettings()

	assert.Equal(t, settings.UserAgreement, GetLocalizedUserAgreement(""))
	assert.Equal(t, settings.PrivacyPolicy, GetLocalizedPrivacyPolicy(""))
}
