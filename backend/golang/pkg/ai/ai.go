package ai

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type Completion interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string) (openai.ChatCompletionMessage, error)
}

type Embedding interface {
	Embedding(ctx context.Context, input string, model string) ([]float64, error)
	Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error)
}
