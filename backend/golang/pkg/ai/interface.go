package ai

import (
	"context"

	"github.com/openai/openai-go"
)

type Completions interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error)
}

type Embeddings interface {
	Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error)
	Embedding(ctx context.Context, input string, model string) ([]float64, error)
}
