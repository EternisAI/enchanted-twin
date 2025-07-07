package localmodel

import "context"

type EmbeddingsModel interface {
	Embedding(ctx context.Context, input string) ([]float32, error)
}
