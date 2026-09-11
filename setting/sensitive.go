package setting

import (
	_ "embed"
	"strings"
	"sync"
)

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// defaultChineseSensitiveWordsText is sourced from fwwdn/sensitive-stop-words at
// revision a7d06bb1c321e669943b6841570d9da6dad8ce2b.
//
//go:embed data/fwwdn_sensitive_words.txt
var defaultChineseSensitiveWordsText string

// defaultEnglishSensitiveWordsText is sourced from
// coffee-and-fun/google-profanity-words at revision
// 0ae3460863120bc671361b9403cc65d5f2075b89.
//
//go:embed data/google_profanity_en.txt
var defaultEnglishSensitiveWordsText string

var defaultEnglishSensitiveWords, defaultEnglishSensitiveWordSet = parseDefaultEnglishSensitiveWords(defaultEnglishSensitiveWordsText)

// The pinned source snapshots are classified into three explicit, disjoint
// policy lists. Keeping the classification as data makes every default term
// reviewable without relying on runtime heuristics.
//
//go:embed data/sensitive_words_nsfw.txt
var defaultSensitiveWordsText string

//go:embed data/sensitive_words_high_risk.txt
var defaultSensitiveWordsHighRiskText string

//go:embed data/sensitive_words_audit.txt
var defaultSensitiveWordsAuditText string

// SensitiveWords is the NSFW block list. The name remains unchanged for
// compatibility with existing administrator options and integrations.
var SensitiveWords = parseSensitiveWords(defaultSensitiveWordsText)

// SensitiveWordsHighRisk blocks high-risk requests such as sexual violence,
// child sexual exploitation, self-harm instructions, weapons, and explosives.
var SensitiveWordsHighRisk = parseSensitiveWords(defaultSensitiveWordsHighRiskText)

// SensitiveWordsAudit records a policy hit but does not block the request.
var SensitiveWordsAudit = parseSensitiveWords(defaultSensitiveWordsAuditText)

var sensitiveWordsMutex sync.RWMutex
var sensitiveWordsVersion uint64 = 1

type SensitiveWordListsSnapshot struct {
	NSFW     []string
	HighRisk []string
	Audit    []string
	Version  uint64
}

func SensitiveWordsToString() string {
	sensitiveWordsMutex.RLock()
	defer sensitiveWordsMutex.RUnlock()
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsHighRiskToString() string {
	sensitiveWordsMutex.RLock()
	defer sensitiveWordsMutex.RUnlock()
	return strings.Join(SensitiveWordsHighRisk, "\n")
}

func SensitiveWordsAuditToString() string {
	sensitiveWordsMutex.RLock()
	defer sensitiveWordsMutex.RUnlock()
	return strings.Join(SensitiveWordsAudit, "\n")
}

func SensitiveWordsFromString(s string) {
	updateSensitiveWords(&SensitiveWords, s)
}

func SensitiveWordsHighRiskFromString(s string) {
	updateSensitiveWords(&SensitiveWordsHighRisk, s)
}

func SensitiveWordsAuditFromString(s string) {
	updateSensitiveWords(&SensitiveWordsAudit, s)
}

func updateSensitiveWords(target *[]string, text string) {
	words := parseSensitiveWords(text)
	sensitiveWordsMutex.Lock()
	*target = words
	sensitiveWordsVersion++
	sensitiveWordsMutex.Unlock()
}

// GetSensitiveWords returns the current immutable word slice. Updates replace
// the whole slice, so callers may safely retain the returned value for a check.
func GetSensitiveWords() []string {
	words, _ := GetSensitiveWordsSnapshot()
	return words
}

func GetSensitiveWordsHighRisk() []string {
	return GetSensitiveWordListsSnapshot().HighRisk
}

func GetSensitiveWordsAudit() []string {
	return GetSensitiveWordListsSnapshot().Audit
}

// GetSensitiveWordsSnapshot returns the immutable current word slice together
// with a version that changes whenever an administrator replaces the list.
func GetSensitiveWordsSnapshot() ([]string, uint64) {
	snapshot := GetSensitiveWordListsSnapshot()
	return snapshot.NSFW, snapshot.Version
}

// GetSensitiveWordListsSnapshot returns one consistent view of all policy
// lists. Updates replace complete slices, so callers may safely retain them.
func GetSensitiveWordListsSnapshot() SensitiveWordListsSnapshot {
	sensitiveWordsMutex.RLock()
	snapshot := SensitiveWordListsSnapshot{
		NSFW:     SensitiveWords,
		HighRisk: SensitiveWordsHighRisk,
		Audit:    SensitiveWordsAudit,
		Version:  sensitiveWordsVersion,
	}
	sensitiveWordsMutex.RUnlock()
	return snapshot
}

// IsDefaultEnglishSensitiveWord reports whether a word came from the bundled
// English dictionary. Those entries use ASCII word boundaries in the matcher
// to avoid substring false positives such as "ass" in "assistant".
func IsDefaultEnglishSensitiveWord(word string) bool {
	_, ok := defaultEnglishSensitiveWordSet[strings.ToLower(word)]
	return ok
}

func parseDefaultSensitiveWords(chineseText string, englishWords []string) []string {
	chineseWords := parseSensitiveWords(chineseText)
	words := make([]string, 0, len(chineseWords)+len(englishWords))
	seen := make(map[string]struct{}, cap(words))
	for _, word := range chineseWords {
		// The upstream list contains two-character ASCII terms such as JS and
		// LY. With substring matching they block ordinary words like JSON and
		// apply, so keep them in the source snapshot but not in active defaults.
		if isAmbiguousShortASCIIWord(word) {
			continue
		}
		normalized := strings.ToLower(word)
		if _, duplicated := seen[normalized]; duplicated {
			continue
		}
		seen[normalized] = struct{}{}
		words = append(words, word)
	}
	for _, word := range englishWords {
		normalized := strings.ToLower(word)
		if _, duplicated := seen[normalized]; duplicated {
			continue
		}
		seen[normalized] = struct{}{}
		words = append(words, word)
	}
	return words
}

func parseDefaultEnglishSensitiveWords(s string) ([]string, map[string]struct{}) {
	parsed := parseSensitiveWords(s)
	words := make([]string, 0, len(parsed))
	wordSet := make(map[string]struct{}, len(parsed))
	for _, word := range parsed {
		// Two-character ASCII entries such as "ho" and "xx" are too broad
		// even with boundary matching, so retain them in the source snapshot
		// but exclude them from the active defaults.
		if isAmbiguousShortASCIIWord(word) {
			continue
		}
		normalized := strings.ToLower(word)
		if _, duplicated := wordSet[normalized]; duplicated {
			continue
		}
		wordSet[normalized] = struct{}{}
		words = append(words, word)
	}
	return words, wordSet
}

func parseSensitiveWords(s string) []string {
	words := make([]string, 0)
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

func isAmbiguousShortASCIIWord(word string) bool {
	if len(word) == 0 || len(word) > 2 {
		return false
	}
	for i := 0; i < len(word); i++ {
		char := word[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
