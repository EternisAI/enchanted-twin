package ai

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
	"github.com/openai/openai-go"
)

type Priority = microscheduler.Priority

const (
	UI = microscheduler.UI
	LastEffort = microscheduler.LastEffort
	Background = microscheduler.Background
)

type PrivateCompletionResult struct {
	Message          openai.ChatCompletionMessage `json:"message"`
	ReplacementRules map[string]string            `json:"replacement_rules"`
}

type PrivateCompletions interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error)
}

type Anonymizer interface {
	AnonymizeMessages(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, interruptChan <-chan struct{}) (anonymizedMessages []openai.ChatCompletionMessageParamUnion, rules map[string]string, err error)
	DeAnonymize(anonymized string, rules map[string]string) string
}