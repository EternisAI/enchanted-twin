package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// MemoryEngine defines the pure business logic interface for memory operations.
// This interface contains no orchestration concerns (channels, workers, progress reporting).
type MemoryEngine interface {
	// Core business operations
	ExtractFacts(ctx context.Context, doc PreparedDocument) ([]ExtractedFact, error)
	ProcessFact(ctx context.Context, fact ExtractedFact) (FactResult, error)
	ExecuteDecision(ctx context.Context, fact ExtractedFact, decision MemoryDecision) (FactResult, error)

	// Memory operations
	SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error)
	DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error)
	UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error
	DeleteMemory(ctx context.Context, memoryID string) error
	CreateMemoryObject(ctx context.Context, fact ExtractedFact, decision MemoryDecision) (*models.Object, error)

	// Storage operations
	StoreBatch(ctx context.Context, objects []*models.Object) error

	GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error)
}

// memoryEngine implements MemoryEngine with pure business logic.
type memoryEngine struct {
	completionsService *ai.Service
	embeddingsService  *ai.Service
	storage            storage.Interface
}

// NewMemoryEngine creates a new MemoryEngine instance.
func NewMemoryEngine(completionsService *ai.Service, embeddingsService *ai.Service, storage storage.Interface) (MemoryEngine, error) {
	if completionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if embeddingsService == nil {
		return nil, fmt.Errorf("embeddings service cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	return &memoryEngine{
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
		storage:            storage,
	}, nil
}

// convertEmbedding converts a slice of float64 to float32 for vector operations.
func convertEmbedding(embedding []float64) []float32 {
	result := make([]float32, len(embedding))
	for i, v := range embedding {
		result[i] = float32(v)
	}
	return result
}

// ExtractFacts extracts facts from a document using pure business logic.
func (e *memoryEngine) ExtractFacts(ctx context.Context, doc PreparedDocument) ([]ExtractedFact, error) {
	return ExtractFactsFromDocument(ctx, doc, e.completionsService)
}

// ProcessFact processes a single fact through the complete memory pipeline.
func (e *memoryEngine) ProcessFact(ctx context.Context, fact ExtractedFact) (FactResult, error) {
	// Search for similar memories
	similar, err := e.SearchSimilar(ctx, fact.Content, fact.SpeakerID)
	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("search failed: %w", err)}, nil
	}

	// Decide what action to take
	decision, err := e.DecideAction(ctx, fact.Content, similar)
	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("decision failed: %w", err)}, nil
	}

	// Execute the decision
	return e.ExecuteDecision(ctx, fact, decision)
}

// ExecuteDecision executes a memory decision (UPDATE, DELETE, ADD, NONE).
func (e *memoryEngine) ExecuteDecision(ctx context.Context, fact ExtractedFact, decision MemoryDecision) (FactResult, error) {
	// Validation for UPDATE/DELETE operations
	if decision.Action == UPDATE || decision.Action == DELETE {
		similar, err := e.SearchSimilar(ctx, fact.Content, fact.SpeakerID)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("validation search failed: %w", err)}, nil
		}

		targetMemory := findMemoryByID(similar, decision.TargetID)
		if targetMemory == nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("target memory %s not found", decision.TargetID)}, nil
		}

		rule := ValidationRule{
			CurrentSpeakerID: fact.SpeakerID,
			IsDocumentLevel:  fact.SpeakerID == "",
			TargetMemoryID:   decision.TargetID,
			TargetSpeakerID:  targetMemory.Metadata["speakerID"],
			Action:           decision.Action,
		}

		if err := ValidateMemoryOperation(rule); err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: err}, nil
		}
	}

	// Execute based on action
	switch decision.Action {
	case UPDATE:
		embedding, err := e.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("embedding failed: %w", err)}, nil
		}

		if err := e.UpdateMemory(ctx, decision.TargetID, fact.Content, convertEmbedding(embedding)); err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("update failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Decision: decision}, nil

	case DELETE:
		if err := e.DeleteMemory(ctx, decision.TargetID); err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("delete failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Decision: decision}, nil

	case ADD:
		obj, err := e.CreateMemoryObject(ctx, fact, decision)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("object creation failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Decision: decision, Object: obj}, nil

	case NONE:
		return FactResult{Fact: fact, Decision: decision}, nil

	default:
		return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("unknown action: %s", decision.Action)}, nil
	}
}

// SearchSimilar searches for similar memories.
func (e *memoryEngine) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
	return SearchSimilarMemories(ctx, fact, speakerID, e.storage)
}

// DecideAction decides what action to take with a fact given similar memories.
func (e *memoryEngine) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
	// Build separate system and user messages to prevent prompt injection
	systemPrompt, userPrompt := BuildSeparateMemoryDecisionPrompts(fact, similar)

	decisionMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	// Use existing tools from tools.go
	memoryDecisionToolsList := []openai.ChatCompletionToolParam{
		addMemoryTool,
		updateMemoryTool,
		deleteMemoryTool,
		noneMemoryTool,
	}

	response, err := e.completionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, openAIChatModel)
	if err != nil {
		return MemoryDecision{}, fmt.Errorf("LLM decision failed: %w", err)
	}

	return ParseMemoryDecisionResponse(response)
}

// UpdateMemory updates an existing memory.
func (e *memoryEngine) UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error {
	// Get the existing memory document
	existingDoc, err := e.storage.GetByID(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("getting existing memory: %w", err)
	}

	// Update the content
	updatedDoc := *existingDoc
	updatedDoc.FieldContent = newContent

	return e.storage.Update(ctx, memoryID, updatedDoc, embedding)
}

// DeleteMemory deletes an existing memory.
func (e *memoryEngine) DeleteMemory(ctx context.Context, memoryID string) error {
	return e.storage.Delete(ctx, memoryID)
}

// CreateMemoryObject creates a memory object for storage with separate document storage.
func (e *memoryEngine) CreateMemoryObject(ctx context.Context, fact ExtractedFact, decision MemoryDecision) (*models.Object, error) {
	documentID, err := e.storage.StoreDocument(
		ctx,
		fact.Source.Original.Content(),
		string(fact.Source.Type),
		fact.Source.Original.ID(),
		fact.Source.Original.Metadata(),
	)
	if err != nil {
		return nil, fmt.Errorf("storing document: %w", err)
	}

	obj := CreateMemoryObjectWithDocumentReferences(fact, decision, []string{documentID})

	embedding, err := e.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	obj.Vector = convertEmbedding(embedding)
	return obj, nil
}

// StoreBatch stores a batch of objects.
func (e *memoryEngine) StoreBatch(ctx context.Context, objects []*models.Object) error {
	return e.storage.StoreBatch(ctx, objects)
}

// GetDocumentReferences retrieves all document references for a memory.
func (e *memoryEngine) GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error) {
	storageRefs, err := e.storage.GetDocumentReferences(ctx, memoryID)
	if err != nil {
		return nil, err
	}

	refs := make([]*DocumentReference, len(storageRefs))
	for i, storageRef := range storageRefs {
		refs[i] = &DocumentReference{
			ID:      storageRef.ID,
			Content: storageRef.Content,
			Type:    storageRef.Type,
		}
	}

	return refs, nil
}
