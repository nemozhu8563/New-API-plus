package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

func CheckSensitiveMessages(messages []dto.Message) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	for _, message := range messages {
		arrayContent := message.ParseContent()
		for _, m := range arrayContent {
			if m.Type == "image_url" {
				// TODO: check image url
				continue
			}
			// 检查 text 是否为空
			if m.Text == "" {
				continue
			}
			if ok, words := SensitiveWordContains(m.Text); ok {
				return words, errors.New("sensitive words detected")
			}
		}
	}
	return nil, nil
}

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

// SensitiveWordContains 是否包含敏感词，返回是否包含敏感词和敏感词列表
func SensitiveWordContains(text string) (bool, []string) {
	if len(text) == 0 {
		return false, nil
	}
	checkText := []rune(strings.ToLower(text))
	matcher := getSensitiveMatcher()
	if matcher == nil {
		return false, nil
	}
	hits := matcher.search(checkText, true)
	if len(hits) > 0 {
		return true, []string{string(hits[0].Word)}
	}
	return false, nil
}

// SensitiveWordReplace 敏感词替换，返回是否包含敏感词和替换后的文本
func SensitiveWordReplace(text string, returnImmediately bool) (bool, []string, string) {
	checkText := []rune(strings.ToLower(text))
	textRunes := []rune(text)
	matcher := getSensitiveMatcher()
	if matcher == nil {
		return false, nil, text
	}
	// Replacement needs deterministic text order even when only one word is
	// requested. Aho-Corasick's early-return mode stops at the first ending
	// position, which can choose a later-starting short word over an earlier
	// longer word.
	hits := matcher.search(checkText, false)
	if len(hits) > 0 {
		words := make([]string, 0, len(hits))
		var builder strings.Builder
		builder.Grow(len(text))
		lastPos := 0

		for _, hit := range hits {
			pos := hit.Pos
			word := string(hit.Word)
			endPos := pos + len(hit.Word)
			if pos < lastPos || pos < 0 || endPos > len(textRunes) {
				continue
			}
			builder.WriteString(string(textRunes[lastPos:pos]))
			builder.WriteString("**###**")
			lastPos = endPos
			words = append(words, word)
			if returnImmediately {
				break
			}
		}
		builder.WriteString(string(textRunes[lastPos:]))
		if len(words) == 0 {
			return false, nil, text
		}
		return true, words, builder.String()
	}
	return false, nil, text
}
