package localmodel

import "context"

type CompletionModel interface {
	Completions(ctx context.Context, input string, maxTokens int) ([]string, error)
}
