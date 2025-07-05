package ai

import (
	"context"

	"github.com/openai/openai-go"
)

// CompletionsServiceInterface defines the interface that both Service and ServiceAdapter implement
type CompletionsServiceInterface interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error)
	ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error)
	CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream
	Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error)
	Embedding(ctx context.Context, input string, model string) ([]float64, error)
}