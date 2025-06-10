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

// Document preparation.
type PreparedDocument struct {
	Original   memory.Document
	Type       DocumentType
	Timestamp  time.Time
	DateString string // Pre-formatted
}

// Structured fact types for the new extraction system.
type StructuredFact struct {
	Category        string  `json:"category"`
	Subject         string  `json:"subject"`
	Attribute       string  `json:"attribute"`
	Value           string  `json:"value"`
	TemporalContext *string `json:"temporal_context,omitempty"`
	Sensitivity     string  `json:"sensitivity"`
	Importance      int     `json:"importance"`
}

type ExtractStructuredFactsToolArguments struct {
	Facts []StructuredFact `json:"facts"`
}

// Processing pipeline types.
type ExtractedFact struct {
	// Structured fact fields (primary data)
	Category        string  `json:"category"`
	Subject         string  `json:"subject"`
	Attribute       string  `json:"attribute"`
	Value           string  `json:"value"`
	TemporalContext *string `json:"temporal_context,omitempty"`
	Sensitivity     string  `json:"sensitivity"`
	Importance      int     `json:"importance"`

	// Legacy fields
	Content string // Derived from structured fields for backward compatibility
	Source  PreparedDocument
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
	Fact     ExtractedFact
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

// DocumentReference holds reference to the original document that generated a memory fact.
type DocumentReference struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

// MemoryStorage is the main interface for hot-swappable storage implementations.
// This interface encapsulates all memory storage operations and business logic.
type MemoryStorage interface {
	memory.Storage // Inherit the base storage interface

	// Document reference operations - now supports multiple references per memory
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error)
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

// StorageImpl implements MemoryStorage using any storage backend.
type StorageImpl struct {
	logger          *log.Logger
	orchestrator    MemoryOrchestrator
	storage         storage.Interface
	engine          MemoryEngine // Add engine reference for GetDocumentReference
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
	orchestrator, err := NewMemoryOrchestrator(engine, deps.Logger)
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

// Store implements the memory.Storage interface using channels for progress and error reporting.
func (s *StorageImpl) Store(ctx context.Context, documents []memory.Document) (<-chan memory.ProgressUpdate, <-chan error) {
	// Use default configuration
	config := DefaultConfig()

	// Get channels from orchestrator
	progressCh, errorCh := s.orchestrator.ProcessDocuments(ctx, documents, config)

	// Convert internal Progress to memory.ProgressUpdate
	convertedProgressCh := make(chan memory.ProgressUpdate)
	go func() {
		defer close(convertedProgressCh)
		for progress := range progressCh {
			convertedProgressCh <- memory.ProgressUpdate{
				Processed: progress.Processed,
				Total:     progress.Total,
				Stage:     progress.Stage,
			}
		}
	}()

	return convertedProgressCh, errorCh
}

// Query implements the memory.Storage interface by delegating to the storage interface.
func (s *StorageImpl) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	return s.storage.Query(ctx, queryText, filter, s.embeddingsModel)
}

// GetDocumentReferences retrieves all document references for a memory.
func (s *StorageImpl) GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error) {
	return s.engine.GetDocumentReferences(ctx, memoryID)
}
