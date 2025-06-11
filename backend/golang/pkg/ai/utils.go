package ai

import (
	"regexp"
	"strings"
)

// StripThinkingTags removes thinking tags and their content from AI responses
// This handles various formats like <think>...</think>, <thinking>...</thinking>, etc.
func StripThinkingTags(content string) string {
	thinkRegex := regexp.MustCompile(`(?is)<think>.*?</think>`)
	content = thinkRegex.ReplaceAllString(content, "")

	thinkingRegex := regexp.MustCompile(`(?is)<thinking>.*?</thinking>`)
	content = thinkingRegex.ReplaceAllString(content, "")

	reasoningRegex := regexp.MustCompile(`(?is)<reasoning>.*?</reasoning>`)
	content = reasoningRegex.ReplaceAllString(content, "")

	content = strings.TrimSpace(content)

	multiNewlineRegex := regexp.MustCompile(`\n{3,}`)
	content = multiNewlineRegex.ReplaceAllString(content, "\n\n")

	return content
}
