package ai

import (
	"context"

	"github.com/openai/openai-go"
)

type AnonymizeResult struct {
	Messages []openai.ChatCompletionMessageParamUnion
	Success  bool
	Error    error
}

type Anonymizer interface {
	Anonymize(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) AnonymizeResult
}
