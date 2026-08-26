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
	words := parseDefaultSensitiveWords(defaultChineseSensitiveWordsText, englishWords)

	require.Len(t, rawChineseWords, 1153)
	require.Len(t, rawEnglishWords, 962)
	require.Len(t, englishWords, 948)
	require.Len(t, englishWordSet, 948)
	require.Len(t, words, 2094)
	assert.NotContains(t, words, "test_sensitive")
	assert.Contains(t, words, "无抵押贷款")
	assert.Contains(t, words, "fuck")
	assert.Contains(t, words, "bitch")
	assert.Contains(t, words, "🖕")
	for _, ambiguous := range []string{"QQ", "3P", "LY", "JS", "BT", "SM"} {
		assert.Contains(t, rawChineseWords, ambiguous)
		assert.NotContains(t, words, ambiguous)
	}
	for _, ambiguous := range []string{"ho", "xx"} {
		assert.Contains(t, rawEnglishWords, ambiguous)
		assert.NotContains(t, englishWords, ambiguous)
	}
	assert.True(t, IsDefaultEnglishSensitiveWord("ASS"))
	assert.False(t, IsDefaultEnglishSensitiveWord("ho"))

	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		assert.False(t, isAmbiguousShortASCIIWord(word), "short alphanumeric entries are too broad")
		normalized := strings.ToLower(word)
		_, duplicated := seen[normalized]
		assert.False(t, duplicated, "duplicate embedded word %q", word)
		seen[normalized] = struct{}{}
	}
}

func TestSensitiveWordsFromStringOverridesAndClears(t *testing.T) {
	original := SensitiveWordsToString()
	t.Cleanup(func() {
		SensitiveWordsFromString(original)
	})

	SensitiveWordsFromString(" Foo \n\nSM\nBAR \n")
	assert.Equal(t, []string{"Foo", "SM", "BAR"}, GetSensitiveWords(), "explicit administrator overrides are not filtered")

	SensitiveWordsFromString("")
	assert.Empty(t, GetSensitiveWords())
}
