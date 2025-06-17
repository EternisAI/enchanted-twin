package ai

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/openai/openai-go"
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

// UnmarshalToolCall unmarshals a tool call's arguments into a struct
func UnmarshalToolCall(toolCall openai.ChatCompletionMessageToolCall, v interface{}) error {
	return json.Unmarshal([]byte(toolCall.Function.Arguments), v)
}
