package localmodel

import "context"

type EmbeddingModel interface {
	Embedding(ctx context.Context, input string) ([]float32, error)
	Embeddings(ctx context.Context, inputs []string) ([][]float32, error)
}
