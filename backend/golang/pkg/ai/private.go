package ai

import (
	"context"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
)

type Priority = microscheduler.Priority

const (
	UI         = microscheduler.UI
	LastEffort = microscheduler.LastEffort
	Background = microscheduler.Background
)

type PrivateCompletionResult struct {
	Message          openai.ChatCompletionMessage `json:"message"`
	ReplacementRules map[string]string            `json:"replacement_rules"`
}

type PrivateCompletions interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error)
	CompletionsWithContext(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error)
}

type Anonymizer interface {
	// Enhanced method with conversation context and mutable dictionary
	AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) (anonymizedMessages []openai.ChatCompletionMessageParamUnion, updatedDict map[string]string, newRules map[string]string, err error)
	DeAnonymize(anonymized string, rules map[string]string) string

	// Persistence methods
	LoadConversationDict(conversationID string) (map[string]string, error)
	SaveConversationDict(conversationID string, dict map[string]string) error
	GetMessageHash(message openai.ChatCompletionMessageParamUnion) string
	IsMessageAnonymized(conversationID, messageHash string) (bool, error)
	Shutdown()
}
