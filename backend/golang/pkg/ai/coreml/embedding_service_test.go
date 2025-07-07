package coreml

import (
	"context"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestEmbeddingServiceInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewEmbeddingServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	inputs := []string{"hello world", "test input"}
	embeddings, err := service.Embeddings(ctx, inputs, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got: %d", len(embeddings))
	}

	expectedEmbedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	for i, embedding := range embeddings {
		if len(embedding) != len(expectedEmbedding) {
			t.Errorf("Embedding %d: expected length %d, got %d", i, len(expectedEmbedding), len(embedding))
		}
		for j, val := range embedding {
			if val != expectedEmbedding[j] {
				t.Errorf("Embedding %d[%d]: expected %f, got %f", i, j, expectedEmbedding[j], val)
			}
		}
	}
}

func TestEmbeddingServiceSingle(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewEmbeddingServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	embedding, err := service.Embedding(ctx, "hello world", "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedEmbedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	if len(embedding) != len(expectedEmbedding) {
		t.Errorf("Expected length %d, got %d", len(expectedEmbedding), len(embedding))
	}

	for i, val := range embedding {
		if val != expectedEmbedding[i] {
			t.Errorf("Embedding[%d]: expected %f, got %f", i, expectedEmbedding[i], val)
		}
	}
}

func TestEmbeddingServiceNonInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewEmbeddingServiceWithProcess("./mock-binary", "./mock-model", false, mockProcess)

	inputs := []string{"hello world", "test input"}
	embeddings, err := service.Embeddings(ctx, inputs, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got: %d", len(embeddings))
	}
}

func TestEmbeddingServiceFailure(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockFailingProcess("embedding model unavailable")
	service := NewEmbeddingServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	_, err := service.Embeddings(ctx, []string{"test"}, "test-model")
	if err == nil {
		t.Fatal("Expected error, got none")
	}

	if err.Error() != "embedding failed: embedding model unavailable" {
		t.Errorf("Expected 'embedding failed: embedding model unavailable', got: %s", err.Error())
	}
}

func TestEmbeddingServiceImplementsInterface(t *testing.T) {
	mockProcess := NewMockSuccessfulProcess()
	service := NewEmbeddingServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	var _ ai.Embeddings = service
}
