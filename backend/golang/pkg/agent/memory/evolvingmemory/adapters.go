package evolvingmemory

import (
	"context"
	"fmt"
)

// factExtractorAdapter wraps StorageImpl to implement FactExtractor interface.
type factExtractorAdapter struct {
	storage *StorageImpl
}

// NewFactExtractor creates a new FactExtractor implementation.
func NewFactExtractor(storage *StorageImpl) (FactExtractor, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if storage.GetEngine() == nil {
		return nil, fmt.Errorf("memory engine not initialized")
	}
	return &factExtractorAdapter{storage: storage}, nil
}

// ExtractFacts routes to the appropriate extraction method based on document type.
func (f *factExtractorAdapter) ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error) {
	return f.storage.GetEngine().ExtractFacts(ctx, doc)
}

// memoryOperationsAdapter wraps StorageImpl to implement MemoryOperations interface.
type memoryOperationsAdapter struct {
	storage *StorageImpl
}

// NewMemoryOperations creates a new MemoryOperations implementation.
func NewMemoryOperations(storage *StorageImpl) (MemoryOperations, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if storage.storage == nil {
		return nil, fmt.Errorf("storage interface not initialized")
	}
	if storage.GetEngine() == nil {
		return nil, fmt.Errorf("memory engine not initialized")
	}
	return &memoryOperationsAdapter{storage: storage}, nil
}

func (m *memoryOperationsAdapter) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
	return m.storage.GetEngine().SearchSimilar(ctx, fact, speakerID)
}

func (m *memoryOperationsAdapter) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
	return m.storage.GetEngine().DecideAction(ctx, fact, similar)
}

// UpdateMemory updates an existing memory with new content.
func (m *memoryOperationsAdapter) UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error {
	return m.storage.GetEngine().UpdateMemory(ctx, memoryID, newContent, embedding)
}

// DeleteMemory removes a memory by ID.
func (m *memoryOperationsAdapter) DeleteMemory(ctx context.Context, memoryID string) error {
	return m.storage.GetEngine().DeleteMemory(ctx, memoryID)
}
