//go:build test
// +build test

package personalities

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
)

// MockMemoryStorage provides a complete mock implementation for testing.
type MockMemoryStorage struct {
	facts map[string][]string
}

func NewMockMemoryStorage() *MockMemoryStorage {
	return &MockMemoryStorage{
		facts: make(map[string][]string),
	}
}

func (m *MockMemoryStorage) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	// Mock implementation - store document summaries
	for i, doc := range documents {
		key := doc.ID()
		m.facts[key] = []string{doc.Content()}
		if progressCallback != nil {
			progressCallback(i+1, len(documents))
		}
	}
	return nil
}

func (m *MockMemoryStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	// Mock implementation - return personality-relevant facts
	result := memory.QueryResult{
		Facts: make([]memory.MemoryFact, 0),
	}

	// Simulate different responses based on query content
	if queryText != "" {
		// Add some mock personality facts
		mockFacts := []string{
			"User has strong interest in technology and innovation",
			"User prefers data-driven decision making",
			"User values creative expression and artistic quality",
			"User actively follows industry trends and developments",
		}

		for i, factContent := range mockFacts {
			result.Facts = append(result.Facts, memory.MemoryFact{
				ID:      fmt.Sprintf("mock-fact-%d", i),
				Content: factContent,
			})
		}
	}

	return result, nil
}

func (m *MockMemoryStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	return []*storage.DocumentReference{}, nil
}

// MockHolonRepository provides a complete mock implementation for testing.
type MockHolonRepository struct{}

func NewMockHolonRepository() *MockHolonRepository {
	return &MockHolonRepository{}
}

func (m *MockHolonRepository) UpdateThreadWithEvaluation(ctx context.Context, threadID, state string, reason *string, confidence *float64, evaluatedBy *string) error {
	return nil
}

func (m *MockHolonRepository) GetThreadsByState(ctx context.Context, state string) ([]*model.Thread, error) {
	return []*model.Thread{}, nil
}
