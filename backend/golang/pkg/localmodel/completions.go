package localmodel

import (
	"context"

	"github.com/openai/openai-go"
)

type Completion interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error)
}
