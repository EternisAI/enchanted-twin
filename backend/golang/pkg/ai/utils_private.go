package ai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
)

// HighlightReplacements creates a human-readable string showing what was anonymized
func HighlightReplacements(result PrivateCompletionResult) string {
	if len(result.ReplacementRules) == 0 {
		return "No anonymization performed"
	}
	
	var highlights []string
	for token, original := range result.ReplacementRules {
		highlights = append(highlights, fmt.Sprintf("%s → %s", original, token))
	}
	
	return fmt.Sprintf("Anonymized %d items: %v", len(result.ReplacementRules), highlights)
}

// GetAnonymizedTokens returns all the anonymized tokens used in the completion
func GetAnonymizedTokens(result PrivateCompletionResult) []string {
	var tokens []string
	for token := range result.ReplacementRules {
		tokens = append(tokens, token)
	}
	return tokens
}

// GetOriginalValues returns all the original values that were anonymized
func GetOriginalValues(result PrivateCompletionResult) []string {
	var values []string
	for _, original := range result.ReplacementRules {
		values = append(values, original)
	}
	return values
}

// RestoreFromRules manually applies de-anonymization rules to any text
func RestoreFromRules(text string, rules map[string]string) string {
	result := text
	for token, original := range rules {
		// Simple string replacement - in production you might want more sophisticated replacement
		result = replaceAll(result, token, original)
	}
	return result
}

// replaceAll is a helper function for string replacement
func replaceAll(text, old, new string) string {
	// This is a simplified implementation
	// In production, you might want to use regex or more sophisticated replacement
	return text // placeholder - would implement actual replacement logic
}

// PrivateCompletionsHelper provides convenience methods for working with private completions
type PrivateCompletionsHelper struct {
	service *Service
}

// NewPrivateCompletionsHelper creates a helper for working with private completions
func NewPrivateCompletionsHelper(service *Service) *PrivateCompletionsHelper {
	return &PrivateCompletionsHelper{service: service}
}

// CompleteWithDebugInfo performs completion and returns both the result and debug information
func (h *PrivateCompletionsHelper) CompleteWithDebugInfo(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, string, error) {
	result, err := h.service.Completions(ctx, messages, tools, model)
	if err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	
	debugInfo := HighlightReplacements(result)
	return result.Message, debugInfo, nil
}

// CompleteWithHighPriority performs completion with UI priority and returns debug info
func (h *PrivateCompletionsHelper) CompleteWithHighPriority(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, string, error) {
	result, err := h.service.CompletionsWithPriority(ctx, messages, tools, model, UI)
	if err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	
	debugInfo := HighlightReplacements(result)
	return result.Message, debugInfo, nil
}

// IsPrivacyEnabled checks if private completions are enabled on the service
func (h *PrivateCompletionsHelper) IsPrivacyEnabled() bool {
	return h.service.privateCompletions != nil
}