package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveSensitiveWords(t *testing.T) {
	t.Helper()
	originalNSFW := setting.SensitiveWordsToString()
	originalHighRisk := setting.SensitiveWordsHighRiskToString()
	originalAudit := setting.SensitiveWordsAuditToString()
	t.Cleanup(func() {
		setting.SensitiveWordsFromString(originalNSFW)
		setting.SensitiveWordsHighRiskFromString(originalHighRisk)
		setting.SensitiveWordsAuditFromString(originalAudit)
	})
}

func TestSensitiveWordContainsUsesEmbeddedDefaults(t *testing.T) {
	preserveSensitiveWords(t)

	tests := []struct {
		name     string
		text     string
		wantHit  bool
		wantWord string
	}{
		{name: "Chinese NSFW phrase", text: "这是成人色情内容", wantHit: true, wantWord: "成人色情"},
		{name: "case insensitive English NSFW", text: "This is PORN content.", wantHit: true, wantWord: "porn"},
		{name: "English next to Chinese", text: "这是porn内容", wantHit: true, wantWord: "porn"},
		{name: "ordinary text", text: "hello world", wantHit: false},
		{name: "English substring false positives", text: "class assistant analysis accumulate sextant", wantHit: false},
		{name: "Chinese short ASCII false positives", text: "apply JSON to a small payload", wantHit: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, words := SensitiveWordContains(tt.text)
			assert.Equal(t, tt.wantHit, hit)
			if tt.wantWord == "" {
				assert.Empty(t, words)
				return
			}
			assert.Contains(t, words, tt.wantWord)
		})
	}
}

func TestDefaultPolicyAuditsBroadTermsWithoutBlocking(t *testing.T) {
	preserveSensitiveWords(t)

	for _, text := range []string{
		"这段历史材料描述了当时的淫威",
		"医学讨论：所谓春药是否真实存在",
		"亚情是一个需要上下文判断的词",
	} {
		result := CheckSensitiveTextPolicy(text)
		assert.True(t, result.Matched)
		assert.Equal(t, SensitiveWordCategoryAudit, result.Category)
		assert.Equal(t, SensitiveWordActionAudit, result.Action)
	}
}

func TestSensitiveTextPolicyUsesCategoryActionsAndPriority(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("nsfw-block\nshared")
	setting.SensitiveWordsHighRiskFromString("risk-block\nshared")
	setting.SensitiveWordsAuditFromString("audit-only\nshared\nass")

	tests := []struct {
		name string
		text string
		want SensitiveWordCheckResult
	}{
		{
			name: "high risk blocks",
			text: "contains risk-block",
			want: SensitiveWordCheckResult{Matched: true, Category: SensitiveWordCategoryHighRisk, Action: SensitiveWordActionBlock, Word: "risk-block"},
		},
		{
			name: "nsfw blocks",
			text: "contains nsfw-block",
			want: SensitiveWordCheckResult{Matched: true, Category: SensitiveWordCategoryNSFW, Action: SensitiveWordActionBlock, Word: "nsfw-block"},
		},
		{
			name: "audit allows",
			text: "contains audit-only",
			want: SensitiveWordCheckResult{Matched: true, Category: SensitiveWordCategoryAudit, Action: SensitiveWordActionAudit, Word: "audit-only"},
		},
		{
			name: "block wins duplicate",
			text: "contains shared",
			want: SensitiveWordCheckResult{Matched: true, Category: SensitiveWordCategoryHighRisk, Action: SensitiveWordActionBlock, Word: "shared"},
		},
		{
			name: "bundled English boundary applies to audit list",
			text: "assistant",
			want: SensitiveWordCheckResult{},
		},
		{
			name: "standalone bundled English audit word matches",
			text: "standalone ASS",
			want: SensitiveWordCheckResult{Matched: true, Category: SensitiveWordCategoryAudit, Action: SensitiveWordActionAudit, Word: "ass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CheckSensitiveTextPolicy(tt.text))
		})
	}
}

func TestSensitiveWordContainsContinuesAfterEnglishSubstring(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("ass")

	hit, words := SensitiveWordContains("assistant first, standalone ASS second")

	assert.True(t, hit)
	assert.Equal(t, []string{"ass"}, words)
}

func TestSensitiveWordReplacePreservesUnicodeText(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("敏感词")

	hit, words, replaced := SensitiveWordReplace("前缀敏感词后缀", false)

	assert.True(t, hit)
	assert.Equal(t, []string{"敏感词"}, words)
	assert.Equal(t, "前缀**###**后缀", replaced)
}

func TestSensitiveWordReplaceUsesEnglishWordBoundaries(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("ass")

	hit, words, replaced := SensitiveWordReplace("assistant says ASS.", false)

	assert.True(t, hit)
	assert.Equal(t, []string{"ass"}, words)
	assert.Equal(t, "assistant says **###**.", replaced)
}

func TestSensitiveWordReplaceStopsAtFirstValidEnglishWord(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("ass\nbitch")

	hit, words, replaced := SensitiveWordReplace("assistant says ASS and BITCH", true)

	assert.True(t, hit)
	assert.Equal(t, []string{"ass"}, words)
	assert.Equal(t, "assistant says **###** and BITCH", replaced)
}

func TestSensitiveWordReplacePrefersLongestEnglishMatch(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("ass\nasshole")

	hit, words, replaced := SensitiveWordReplace("ASSHOLE", true)

	assert.True(t, hit)
	assert.Equal(t, []string{"asshole"}, words)
	assert.Equal(t, "**###**", replaced)
}

func TestSensitiveWordReplaceImmediatelyUsesEarliestLongestSubstring(t *testing.T) {
	preserveSensitiveWords(t)
	setting.SensitiveWordsFromString("bc\nabcd")

	hit, words, replaced := SensitiveWordReplace("abcd", true)

	assert.True(t, hit)
	assert.Equal(t, []string{"abcd"}, words)
	assert.Equal(t, "**###**", replaced)
}

func TestSensitiveWordsCustomOverrideAndClear(t *testing.T) {
	preserveSensitiveWords(t)

	setting.SensitiveWordsFromString("ass")
	hit, words := SensitiveWordContains("assistant")
	assert.False(t, hit, "bundled English terms retain word-boundary matching in an administrator override")
	assert.Empty(t, words)
	hit, words = SensitiveWordContains("ASS")
	require.True(t, hit)
	assert.Equal(t, []string{"ass"}, words)

	setting.SensitiveWordsFromString("custom-block")
	hit, words = SensitiveWordContains("prefixCUSTOM-BLOCKsuffix")
	require.True(t, hit)
	assert.Contains(t, words, "custom-block")

	hit, _ = SensitiveWordContains("无抵押贷款")
	assert.False(t, hit, "a custom list must fully replace the embedded default list")

	setting.SensitiveWordsFromString("")
	hit, words = SensitiveWordContains("custom-block")
	assert.False(t, hit)
	assert.Empty(t, words)
}

func TestSensitiveWordCacheFollowsUpdatedDictionary(t *testing.T) {
	preserveSensitiveWords(t)

	setting.SensitiveWordsFromString("alpha-block")
	hit, _ := SensitiveWordContains("alpha-block")
	require.True(t, hit)

	setting.SensitiveWordsFromString("beta-block")
	hit, _ = SensitiveWordContains("alpha-block")
	assert.False(t, hit)
	hit, words := SensitiveWordContains("BETA-BLOCK")
	assert.True(t, hit)
	assert.Contains(t, words, "beta-block")
}
