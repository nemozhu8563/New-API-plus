package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSensitiveWordsUsePinnedChineseAndEnglishSnapshots(t *testing.T) {
	rawChineseWords := parseSensitiveWords(defaultChineseSensitiveWordsText)
	rawEnglishWords := parseSensitiveWords(defaultEnglishSensitiveWordsText)
	englishWords, englishWordSet := parseDefaultEnglishSensitiveWords(defaultEnglishSensitiveWordsText)
	legacyWords := parseDefaultSensitiveWords(defaultChineseSensitiveWordsText, englishWords)
	snapshot := GetSensitiveWordListsSnapshot()

	require.Len(t, rawChineseWords, 1153)
	require.Len(t, rawEnglishWords, 962)
	require.Len(t, englishWords, 948)
	require.Len(t, englishWordSet, 948)
	require.Len(t, legacyWords, 2094)
	assert.NotContains(t, legacyWords, "test_sensitive")
	assert.Contains(t, snapshot.NSFW, "成人色情")
	assert.Contains(t, snapshot.NSFW, "porn")
	assert.Contains(t, snapshot.HighRisk, "炸弹制作")
	assert.Contains(t, snapshot.HighRisk, "date rape")
	assert.Contains(t, snapshot.Audit, "无抵押贷款")
	assert.Contains(t, snapshot.Audit, "bitch")
	assert.Contains(t, snapshot.Audit, "春药")
	assert.Contains(t, snapshot.Audit, "亚情")
	assert.Contains(t, snapshot.Audit, "淫威")
	assert.Contains(t, snapshot.Audit, "习近平")
	assert.Contains(t, snapshot.Audit, "🖕")
	for _, ambiguous := range []string{"QQ", "3P", "LY", "JS", "BT", "SM"} {
		assert.Contains(t, rawChineseWords, ambiguous)
		assert.NotContains(t, legacyWords, ambiguous)
	}
	for _, ambiguous := range []string{"ho", "xx"} {
		assert.Contains(t, rawEnglishWords, ambiguous)
		assert.NotContains(t, englishWords, ambiguous)
	}
	assert.True(t, IsDefaultEnglishSensitiveWord("ASS"))
	assert.False(t, IsDefaultEnglishSensitiveWord("ho"))

	legacySet := make(map[string]struct{}, len(legacyWords))
	for _, word := range legacyWords {
		legacySet[strings.ToLower(word)] = struct{}{}
	}

	classified := make(map[string]string, len(legacyWords))
	for category, words := range map[string][]string{
		"nsfw":      snapshot.NSFW,
		"high_risk": snapshot.HighRisk,
		"audit":     snapshot.Audit,
	} {
		for _, word := range words {
			assert.False(t, isAmbiguousShortASCIIWord(word), "short alphanumeric entries are too broad")
			normalized := strings.ToLower(word)
			_, exists := legacySet[normalized]
			assert.True(t, exists, "classified word %q must come from a pinned source snapshot", word)
			previousCategory, duplicated := classified[normalized]
			assert.False(t, duplicated, "word %q appears in both %s and %s", word, previousCategory, category)
			classified[normalized] = category
		}
	}
	assert.Len(t, classified, len(legacyWords), "every active legacy word must have exactly one category")
}

func TestSensitiveWordsFromStringOverridesAndClears(t *testing.T) {
	originalNSFW := SensitiveWordsToString()
	originalHighRisk := SensitiveWordsHighRiskToString()
	originalAudit := SensitiveWordsAuditToString()
	t.Cleanup(func() {
		SensitiveWordsFromString(originalNSFW)
		SensitiveWordsHighRiskFromString(originalHighRisk)
		SensitiveWordsAuditFromString(originalAudit)
	})

	SensitiveWordsFromString(" Foo \n\nSM\nBAR \n")
	assert.Equal(t, []string{"Foo", "SM", "BAR"}, GetSensitiveWords(), "explicit administrator overrides are not filtered")

	SensitiveWordsFromString("")
	assert.Empty(t, GetSensitiveWords())
}

func TestSensitiveWordListsUpdateIndependentlyWithOneVersion(t *testing.T) {
	originalNSFW := SensitiveWordsToString()
	originalHighRisk := SensitiveWordsHighRiskToString()
	originalAudit := SensitiveWordsAuditToString()
	t.Cleanup(func() {
		SensitiveWordsFromString(originalNSFW)
		SensitiveWordsHighRiskFromString(originalHighRisk)
		SensitiveWordsAuditFromString(originalAudit)
	})

	before := GetSensitiveWordListsSnapshot()
	SensitiveWordsFromString("nsfw-only")
	afterNSFW := GetSensitiveWordListsSnapshot()
	assert.Equal(t, []string{"nsfw-only"}, afterNSFW.NSFW)
	assert.Equal(t, before.HighRisk, afterNSFW.HighRisk)
	assert.Equal(t, before.Audit, afterNSFW.Audit)
	assert.Greater(t, afterNSFW.Version, before.Version)

	SensitiveWordsHighRiskFromString("risk-only")
	afterHighRisk := GetSensitiveWordListsSnapshot()
	assert.Equal(t, []string{"nsfw-only"}, afterHighRisk.NSFW)
	assert.Equal(t, []string{"risk-only"}, afterHighRisk.HighRisk)
	assert.Equal(t, before.Audit, afterHighRisk.Audit)
	assert.Greater(t, afterHighRisk.Version, afterNSFW.Version)

	SensitiveWordsAuditFromString("")
	afterAudit := GetSensitiveWordListsSnapshot()
	assert.Equal(t, []string{"nsfw-only"}, afterAudit.NSFW)
	assert.Equal(t, []string{"risk-only"}, afterAudit.HighRisk)
	assert.Empty(t, afterAudit.Audit)
	assert.Greater(t, afterAudit.Version, afterHighRisk.Version)
}
