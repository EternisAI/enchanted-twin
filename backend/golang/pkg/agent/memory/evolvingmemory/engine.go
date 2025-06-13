package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// MemoryEngine implements pure business logic for memory operations.
// This contains no orchestration concerns (channels, workers, progress reporting).
type MemoryEngine struct {
	CompletionsService *ai.Service
	EmbeddingsService  *ai.Service
	storage            storage.Interface
	CompletionsModel   string
	EmbeddingsModel    string
}

// NewMemoryEngine creates a new MemoryEngine instance.
func NewMemoryEngine(completionsService *ai.Service, embeddingsService *ai.Service, storage storage.Interface, completionsModel, embeddingsModel string) (*MemoryEngine, error) {
	if completionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if embeddingsService == nil {
		return nil, fmt.Errorf("embeddings service cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if completionsModel == "" {
		return nil, fmt.Errorf("completions model cannot be empty")
	}
	if embeddingsModel == "" {
		return nil, fmt.Errorf("embeddings model cannot be empty")
	}

	return &MemoryEngine{
		CompletionsService: completionsService,
		EmbeddingsService:  embeddingsService,
		storage:            storage,
		CompletionsModel:   completionsModel,
		EmbeddingsModel:    embeddingsModel,
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

// ProcessFact processes a single fact through the complete memory pipeline.
func (e *MemoryEngine) ProcessFact(ctx context.Context, fact StructuredFact, source memory.Document) (FactResult, error) {
	// Generate content for search and decision making
	content := fact.GenerateContent()

	// Search for similar memories filtered by the same subject
	// This ensures we only update memories about the same person/entity
	filter := &memory.Filter{Subject: &fact.Subject}
	similar, err := SearchSimilarMemories(ctx, content, filter, e.storage, e.EmbeddingsModel)
	if err != nil {
		return FactResult{Fact: fact, Source: source, Error: fmt.Errorf("search failed: %w", err)}, nil
	}

	// Decide what action to take
	decision, err := e.DecideAction(ctx, content, similar)
	if err != nil {
		return FactResult{Fact: fact, Source: source, Error: fmt.Errorf("decision failed: %w", err)}, nil
	}

	// Execute the decision
	return e.ExecuteDecision(ctx, fact, source, decision)
}

// ExecuteDecision executes a memory decision (UPDATE, DELETE, ADD, NONE).
func (e *MemoryEngine) ExecuteDecision(ctx context.Context, fact StructuredFact, source memory.Document, decision MemoryDecision) (FactResult, error) {
	// Execute based on action
	switch decision.Action {
	case UPDATE:
		content := fact.GenerateContent()
		embedding, err := e.EmbeddingsService.Embedding(ctx, content, e.EmbeddingsModel)
		if err != nil {
			return FactResult{Fact: fact, Source: source, Decision: decision, Error: fmt.Errorf("embedding failed: %w", err)}, nil
		}

		if err := e.UpdateMemory(ctx, decision.TargetID, content, convertEmbedding(embedding)); err != nil {
			return FactResult{Fact: fact, Source: source, Decision: decision, Error: fmt.Errorf("update failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Source: source, Decision: decision}, nil

	case DELETE:
		if err := e.storage.Delete(ctx, decision.TargetID); err != nil {
			return FactResult{Fact: fact, Source: source, Decision: decision, Error: fmt.Errorf("delete failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Source: source, Decision: decision}, nil

	case ADD:
		obj, err := e.CreateMemoryObject(ctx, fact, source, decision)
		if err != nil {
			return FactResult{Fact: fact, Source: source, Decision: decision, Error: fmt.Errorf("object creation failed: %w", err)}, nil
		}

		return FactResult{Fact: fact, Source: source, Decision: decision, Object: obj}, nil

	case NONE:
		return FactResult{Fact: fact, Source: source, Decision: decision}, nil

	default:
		return FactResult{Fact: fact, Source: source, Decision: decision, Error: fmt.Errorf("unknown action: %s", decision.Action)}, nil
	}
}

// DecideAction decides what action to take with a fact given similar memories.
func (e *MemoryEngine) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
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

	response, err := e.CompletionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, e.CompletionsModel)
	if err != nil {
		return MemoryDecision{}, fmt.Errorf("LLM decision failed: %w", err)
	}

	return ParseMemoryDecisionResponse(response)
}

// UpdateMemory updates an existing memory.
func (e *MemoryEngine) UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error {
	// Get the existing memory fact
	existingFact, err := e.storage.GetByID(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("getting existing memory: %w", err)
	}

	// Update the content
	updatedFact := *existingFact
	updatedFact.Content = newContent

	return e.storage.Update(ctx, memoryID, &updatedFact, embedding)
}

// CreateMemoryObject creates a memory object for storage with separate document storage.
func (e *MemoryEngine) CreateMemoryObject(ctx context.Context, fact StructuredFact, source memory.Document, decision MemoryDecision) (*models.Object, error) {
	// Determine document type
	var docType string
	switch source.(type) {
	case *memory.ConversationDocument:
		docType = string(DocumentTypeConversation)
	case *memory.TextDocument:
		docType = string(DocumentTypeText)
	default:
		docType = "unknown"
	}

	documentID, err := e.storage.StoreDocument(
		ctx,
		source.Content(),
		docType,
		source.ID(),
		source.Metadata(),
	)
	if err != nil {
		return nil, fmt.Errorf("storing document: %w", err)
	}

	obj := CreateMemoryObjectWithDocumentReferences(fact, source, decision, []string{documentID})

	embedding, err := e.EmbeddingsService.Embedding(ctx, fact.GenerateContent(), e.EmbeddingsModel)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	obj.Vector = convertEmbedding(embedding)
	return obj, nil
}
