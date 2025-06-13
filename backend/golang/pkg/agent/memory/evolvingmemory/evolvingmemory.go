package evolvingmemory

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const (
	ClassName         = "MemoryFact"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	tagsProperty      = "tags"
	metadataProperty  = "metadataJson"

	AddMemoryToolName    = "ADD"
	UpdateMemoryToolName = "UPDATE"
	DeleteMemoryToolName = "DELETE"
	NoneMemoryToolName   = "NONE"
	ExtractFactsToolName = "EXTRACT_FACTS"
)

// Document types.
type DocumentType string

const (
	DocumentTypeConversation DocumentType = "conversation"
	DocumentTypeText         DocumentType = "text"
)

// Configuration for the storage system.
type Config struct {
	// Parallelism
	Workers        int
	FactsPerWorker int

	// Batching
	BatchSize     int
	FlushInterval time.Duration

	// Timeouts
	FactExtractionTimeout time.Duration
	MemoryDecisionTimeout time.Duration
	StorageTimeout        time.Duration

	// Features
	EnableRichContext      bool
	ParallelFactExtraction bool
	StreamingProgress      bool
}

// Memory actions.
type MemoryAction string

const (
	ADD    MemoryAction = AddMemoryToolName
	UPDATE MemoryAction = UpdateMemoryToolName
	DELETE MemoryAction = DeleteMemoryToolName
	NONE   MemoryAction = NoneMemoryToolName
)

// Memory decision from LLM.
type MemoryDecision struct {
	Action     MemoryAction
	TargetID   string // For UPDATE/DELETE
	Reason     string
	Confidence float64
}

// Processing result.
type FactResult struct {
	Fact     *memory.MemoryFact
	Source   memory.Document // Source document for the fact
	Decision MemoryDecision
	Object   *models.Object // Only for ADD
	Error    error
}

// Document result.
type DocumentResult struct {
	DocumentID string
	Facts      []FactResult
	Error      error
}

// Progress reporting.
type Progress struct {
	Processed int
	Total     int
	Stage     string
	Error     error
}

// Memory search results.
type ExistingMemory struct {
	ID        string
	Content   string
	Timestamp time.Time
	Score     float64
	Metadata  map[string]string
}

type UpdateToolArguments struct {
	MemoryID      string `json:"id"`
	UpdatedMemory string `json:"updated_content"`
	Reason        string `json:"reason,omitempty"`
}

type DeleteToolArguments struct {
	MemoryID string `json:"id"`
	Reason   string `json:"reason,omitempty"`
}

type NoneToolArguments struct {
	Reason string `json:"reason"`
}

type MemoryStorage interface {
	memory.Storage // Inherit the base storage interface

	// Document reference operations - now supports multiple references per memory
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error)
}

// Dependencies holds all the required dependencies for creating a MemoryStorage instance.
type Dependencies struct {
	Logger             *log.Logger
	Storage            storage.Interface
	CompletionsService *ai.Service
	EmbeddingsService  *ai.Service
	CompletionsModel   string
	EmbeddingsModel    string
}

// StorageImpl is the main implementation of the MemoryStorage interface.
// It orchestrates memory operations using a clean 3-layer architecture:
// StorageImpl (public API) -> MemoryOrchestrator (coordination) -> MemoryEngine (business logic).
type StorageImpl struct {
	logger          *log.Logger
	orchestrator    *MemoryOrchestrator
	storage         storage.Interface
	engine          *MemoryEngine
	embeddingsModel string
}

// New creates a new StorageImpl instance that can work with any storage backend.
func New(deps Dependencies) (MemoryStorage, error) {
	if deps.Storage == nil {
		return nil, fmt.Errorf("storage interface cannot be nil")
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if deps.CompletionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if deps.EmbeddingsService == nil {
		return nil, fmt.Errorf("embeddings service cannot be nil")
	}

	// Create the memory engine with business logic
	engine, err := NewMemoryEngine(deps.CompletionsService, deps.EmbeddingsService, deps.Storage, deps.CompletionsModel, deps.EmbeddingsModel)
	if err != nil {
		return nil, fmt.Errorf("creating memory engine: %w", err)
	}

	// Create the orchestrator with coordination logic
	orchestrator, err := NewMemoryOrchestrator(engine, deps.Storage, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating memory orchestrator: %w", err)
	}

	return &StorageImpl{
		logger:          deps.Logger,
		orchestrator:    orchestrator,
		storage:         deps.Storage,
		engine:          engine,
		embeddingsModel: deps.EmbeddingsModel,
	}, nil
}

// Store implements the memory.Storage interface.
func (s *StorageImpl) Store(ctx context.Context, documents []memory.Document, callback memory.ProgressCallback) error {
	// Use default configuration
	config := DefaultConfig()

	// Get the channels directly from orchestrator
	progressCh, errorCh := s.orchestrator.ProcessDocuments(ctx, documents, config)

	// Convert internal Progress to external ProgressUpdate and handle callback
	total := len(documents)
	processed := 0

	// Collect all errors
	var errors []error

	// Process results from both channels
	for progressCh != nil || errorCh != nil {
		select {
		case progress, ok := <-progressCh:
			if !ok {
				progressCh = nil
				continue
			}
			processed = progress.Processed
			if callback != nil {
				callback(processed, total)
			}

		case err, ok := <-errorCh:
			if !ok {
				errorCh = nil
				continue
			}
			errors = append(errors, err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// If any errors occurred, return meaningful error information
	if len(errors) > 0 {
		if len(errors) == 1 {
			s.logger.Errorf("Store encountered error: %v", errors[0])
			return errors[0]
		}
		// Multiple errors - provide comprehensive information
		s.logger.Errorf("Store encountered %d errors, returning first with details of others: %v", len(errors), errors[0])
		return fmt.Errorf("multiple errors occurred (%d total): %w, and %d more",
			len(errors), errors[0], len(errors)-1)
	}

	return nil
}

// Query implements the memory.Storage interface by delegating to the storage interface.
func (s *StorageImpl) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	return s.storage.Query(ctx, queryText, filter, s.embeddingsModel)
}

// GetDocumentReferences retrieves all document references for a memory.
func (s *StorageImpl) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	return s.storage.GetDocumentReferences(ctx, memoryID)
}
