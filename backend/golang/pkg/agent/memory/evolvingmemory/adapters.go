package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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
	if storage.completionsService == nil {
		return nil, fmt.Errorf("completions service not initialized")
	}
	return &factExtractorAdapter{storage: storage}, nil
}

// ExtractFacts routes to the appropriate extraction method based on document type.
func (f *factExtractorAdapter) ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error) {
	return ExtractFactsFromDocument(ctx, doc, f.storage.completionsService)
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
	if storage.completionsService == nil {
		return nil, fmt.Errorf("completions service not initialized")
	}
	return &memoryOperationsAdapter{storage: storage}, nil
}

func (m *memoryOperationsAdapter) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
	return SearchSimilarMemories(ctx, fact, speakerID, m.storage.storage)
}

func (m *memoryOperationsAdapter) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
	fullDecisionPrompt := BuildMemoryDecisionPrompt(fact, similar)

	decisionMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fullDecisionPrompt),
	}

	memoryDecisionToolsList := []openai.ChatCompletionToolParam{
		addMemoryTool, updateMemoryTool, deleteMemoryTool, noneMemoryTool,
	}

	llmResponse, err := m.storage.completionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, openAIChatModel)
	if err != nil {
		return MemoryDecision{}, fmt.Errorf("LLM decision for memory update: %w", err)
	}

	return ParseMemoryDecisionResponse(llmResponse)
}

// UpdateMemory updates an existing memory with new content.
func (m *memoryOperationsAdapter) UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error {
	originalDoc, err := m.storage.storage.GetByID(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("getting original document for update: %w", err)
	}

	// Create updated document preserving metadata
	docToUpdate := memory.TextDocument{
		FieldID:        memoryID,
		FieldContent:   newContent,
		FieldTimestamp: originalDoc.Timestamp(),
		FieldMetadata:  originalDoc.Metadata(),
		FieldTags:      originalDoc.Tags(),
	}

	return m.storage.storage.Update(ctx, memoryID, docToUpdate, embedding)
}

// DeleteMemory removes a memory by ID.
func (m *memoryOperationsAdapter) DeleteMemory(ctx context.Context, memoryID string) error {
	return m.storage.storage.Delete(ctx, memoryID)
}
