package service

import (
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/setting"
	goahocorasick "github.com/anknown/ahocorasick"
)

type sensitiveMatcher struct {
	substringMachine *goahocorasick.Machine
	boundaryRoot     *sensitiveBoundaryNode
}

type sensitiveBoundaryNode struct {
	children map[rune]*sensitiveBoundaryNode
	word     []rune
}

var sensitiveMatcherCache struct {
	sync.Mutex
	version uint64
	matcher *sensitiveMatcher
}

func getSensitiveMatcher() *sensitiveMatcher {
	words, version := setting.GetSensitiveWordsSnapshot()
	if len(words) == 0 {
		return nil
	}

	sensitiveMatcherCache.Lock()
	defer sensitiveMatcherCache.Unlock()
	if sensitiveMatcherCache.matcher != nil && sensitiveMatcherCache.version == version {
		return sensitiveMatcherCache.matcher
	}

	substringWords := make([]string, 0, len(words))
	boundaryRoot := &sensitiveBoundaryNode{children: make(map[rune]*sensitiveBoundaryNode)}
	for _, word := range words {
		normalized := strings.ToLower(strings.TrimSpace(word))
		if normalized == "" {
			continue
		}
		if setting.IsDefaultEnglishSensitiveWord(normalized) {
			boundaryRoot.add([]rune(normalized))
			continue
		}
		substringWords = append(substringWords, normalized)
	}

	matcher := &sensitiveMatcher{boundaryRoot: boundaryRoot}
	if len(substringWords) > 0 {
		matcher.substringMachine = InitAc(substringWords)
	}
	if len(boundaryRoot.children) == 0 {
		matcher.boundaryRoot = nil
	}
	sensitiveMatcherCache.version = version
	sensitiveMatcherCache.matcher = matcher
	return matcher
}

func (m *sensitiveMatcher) search(text []rune, returnImmediately bool) []*goahocorasick.Term {
	var substringHits []*goahocorasick.Term
	if m.substringMachine != nil {
		substringHits = m.substringMachine.MultiPatternSearch(text, returnImmediately)
	}
	var boundaryHits []*goahocorasick.Term
	if m.boundaryRoot != nil {
		boundaryHits = m.boundaryRoot.search(text, returnImmediately)
	}

	if returnImmediately {
		if len(substringHits) == 0 {
			return boundaryHits
		}
		if len(boundaryHits) == 0 {
			return substringHits
		}
		if boundaryHits[0].Pos < substringHits[0].Pos ||
			boundaryHits[0].Pos == substringHits[0].Pos && len(boundaryHits[0].Word) > len(substringHits[0].Word) {
			return boundaryHits
		}
		return substringHits
	}
	hits := append(substringHits, boundaryHits...)
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Pos == hits[j].Pos {
			return len(hits[i].Word) > len(hits[j].Word)
		}
		return hits[i].Pos < hits[j].Pos
	})
	return hits
}

func (n *sensitiveBoundaryNode) add(word []rune) {
	current := n
	for _, char := range word {
		if current.children[char] == nil {
			current.children[char] = &sensitiveBoundaryNode{children: make(map[rune]*sensitiveBoundaryNode)}
		}
		current = current.children[char]
	}
	current.word = word
}

func (n *sensitiveBoundaryNode) search(text []rune, returnImmediately bool) []*goahocorasick.Term {
	hits := make([]*goahocorasick.Term, 0)
	for start := 0; start < len(text); start++ {
		if isASCIIWordRune(text[start]) && start > 0 && isASCIIWordRune(text[start-1]) {
			continue
		}

		current := n
		var firstAtStart *goahocorasick.Term
		for end := start; end < len(text); end++ {
			current = current.children[text[end]]
			if current == nil {
				break
			}
			if current.word == nil {
				continue
			}
			if isASCIIWordRune(current.word[len(current.word)-1]) && end+1 < len(text) && isASCIIWordRune(text[end+1]) {
				continue
			}

			hit := &goahocorasick.Term{Pos: start, Word: current.word}
			if returnImmediately {
				firstAtStart = hit
				continue
			}
			hits = append(hits, hit)
		}
		if firstAtStart != nil {
			return []*goahocorasick.Term{firstAtStart}
		}
	}
	return hits
}

func isASCIIWordRune(char rune) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
}
