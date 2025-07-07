package storage

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// EmbeddingWrapper wraps the AI service to provide float32 embeddings for Weaviate.
// It handles the conversion from float64 to float32 and stores the model configuration.
type EmbeddingWrapper struct {
	service ai.Embeddings
	model   string
}

// NewEmbeddingWrapper creates a new embedding wrapper with the specified model.
func NewEmbeddingWrapper(service ai.Embeddings, model string) (*EmbeddingWrapper, error) {
	if service == nil {
		return nil, fmt.Errorf("ai service cannot be nil")
	}
	if model == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	return &EmbeddingWrapper{
		service: service,
		model:   model,
	}, nil
}

// Embedding returns a single embedding as float32 slice using the configured model.
func (w *EmbeddingWrapper) Embedding(ctx context.Context, input string) ([]float32, error) {
	embedding, err := w.service.Embedding(ctx, input, w.model)
	if err != nil {
		return nil, err
	}
	return w.convertToFloat32(embedding), nil
}

// Embeddings returns multiple embeddings as float32 slices using the configured model.
func (w *EmbeddingWrapper) Embeddings(ctx context.Context, inputs []string) ([][]float32, error) {
	embeddings, err := w.service.Embeddings(ctx, inputs, w.model)
	if err != nil {
		return nil, err
	}

	result := make([][]float32, len(embeddings))
	for i, embedding := range embeddings {
		result[i] = w.convertToFloat32(embedding)
	}
	return result, nil
}

// convertToFloat32 converts []float64 to []float32.
func (w *EmbeddingWrapper) convertToFloat32(vector []float64) []float32 {
	if len(vector) == 0 {
		return nil
	}

	result := make([]float32, len(vector))
	for i, v := range vector {
		result[i] = float32(v)
	}
	return result
}
