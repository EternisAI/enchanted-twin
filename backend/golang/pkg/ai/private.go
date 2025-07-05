package ai

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
	"github.com/openai/openai-go"
)

// Priority reuses the microscheduler priority system
type Priority = microscheduler.Priority

const (
	// UI priority for user-facing requests that need immediate response
	UI = microscheduler.UI
	// LastEffort for critical requests that can't wait in background
	LastEffort = microscheduler.LastEffort
	// Background for non-urgent batch processing
	Background = microscheduler.Background
)

// PrivateResult contains the completion result along with anonymization mapping
type PrivateResult struct {
	Message          openai.ChatCompletionMessage `json:"message"`
	ReplacementRules map[string]string            `json:"replacement_rules"`
}

// PrivateCompletions provides completions with privacy protection through anonymization
type PrivateCompletions interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateResult, error)
}

// Anonymizer handles the anonymization and de-anonymization of content
type Anonymizer interface {
	// Anonymize takes original content and returns anonymized content plus replacement rules
	Anonymize(ctx context.Context, content string) (anonymized string, rules map[string]string, err error)
	
	// DeAnonymize takes anonymized content and replacement rules and restores original content
	DeAnonymize(anonymized string, rules map[string]string) string
}