package system_setting

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

//go:embed default_user_agreement.md
var defaultUserAgreement string

//go:embed default_privacy_policy.md
var defaultPrivacyPolicy string

//go:embed legal/*.md
var localizedLegalDocumentFS embed.FS

var defaultUserAgreements = map[string]string{
	"en":    mustReadLocalizedLegalDocument("legal/user-agreement.en.md"),
	"fr":    mustReadLocalizedLegalDocument("legal/user-agreement.fr.md"),
	"ja":    mustReadLocalizedLegalDocument("legal/user-agreement.ja.md"),
	"ru":    mustReadLocalizedLegalDocument("legal/user-agreement.ru.md"),
	"vi":    mustReadLocalizedLegalDocument("legal/user-agreement.vi.md"),
	"zh":    defaultUserAgreement,
	"zh-TW": mustReadLocalizedLegalDocument("legal/user-agreement.zh-TW.md"),
}

var defaultPrivacyPolicies = map[string]string{
	"en":    mustReadLocalizedLegalDocument("legal/privacy-policy.en.md"),
	"fr":    mustReadLocalizedLegalDocument("legal/privacy-policy.fr.md"),
	"ja":    mustReadLocalizedLegalDocument("legal/privacy-policy.ja.md"),
	"ru":    mustReadLocalizedLegalDocument("legal/privacy-policy.ru.md"),
	"vi":    mustReadLocalizedLegalDocument("legal/privacy-policy.vi.md"),
	"zh":    defaultPrivacyPolicy,
	"zh-TW": mustReadLocalizedLegalDocument("legal/privacy-policy.zh-TW.md"),
}

var legacyDefaultUserAgreementDigests = map[string]struct{}{
	"6cd4e072628991f39520397bff18a33d5c9eff7a8bcbdc21debd72b5f2bfa2d2": {},
}

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

func GetLocalizedUserAgreement(locale string) string {
	return localizedLegalDocument(
		defaultLegalSettings.UserAgreement,
		defaultUserAgreement,
		defaultUserAgreements,
		legacyDefaultUserAgreementDigests,
		locale,
	)
}

func GetLocalizedPrivacyPolicy(locale string) string {
	return localizedLegalDocument(
		defaultLegalSettings.PrivacyPolicy,
		defaultPrivacyPolicy,
		defaultPrivacyPolicies,
		nil,
		locale,
	)
}

func mustReadLocalizedLegalDocument(path string) string {
	content, err := localizedLegalDocumentFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(content)
}

func localizedLegalDocument(
	configuredDocument string,
	currentDefault string,
	localizedDefaults map[string]string,
	legacyDefaultDigests map[string]struct{},
	locale string,
) string {
	if strings.TrimSpace(locale) == "" || configuredDocument == "" {
		return configuredDocument
	}
	if configuredDocument != currentDefault {
		digest := sha256.Sum256([]byte(configuredDocument))
		if _, ok := legacyDefaultDigests[hex.EncodeToString(digest[:])]; !ok {
			return configuredDocument
		}
	}

	document, ok := localizedDefaults[canonicalLegalLocale(locale)]
	if !ok {
		return configuredDocument
	}
	return document
}

func canonicalLegalLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	switch normalized {
	case "en", "fr", "ja", "ru", "vi":
		return normalized
	case "zh", "zh-cn", "zhcn", "zh-hans":
		return "zh"
	case "zh-tw", "zhtw", "zh-hant":
		return "zh-TW"
	default:
		return ""
	}
}
