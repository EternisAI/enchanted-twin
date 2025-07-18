package agent

import (
	"context"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// AIService interface defines the methods needed by the Agent.
type AIService interface {
	CompletionsStreamWithPrivacy(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, onDelta func(ai.StreamDelta)) (ai.PrivateCompletionResult, error)
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority ai.Priority) (ai.PrivateCompletionResult, error)
}

// Ensure ai.Service implements AIService.
var _ AIService = (*ai.Service)(nil)
