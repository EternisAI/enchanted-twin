package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-openapi/strfmt"
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
	StorageTimeout        time.Duration

	// Features
	EnableRichContext      bool
	ParallelFactExtraction bool
	StreamingProgress      bool
}

// ExtractMemoryFactsToolArguments is used for the LLM fact extraction tool.
type ExtractMemoryFactsToolArguments struct {
	Facts []memory.MemoryFact `json:"facts"`
}

// Simplified processing result - only contains what we actually use.
type FactResult struct {
	Fact   *memory.MemoryFact
	Source memory.Document // Source document for the fact
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

type MemoryStorage interface {
	memory.Storage // Inherit the base storage interface

	// Document reference operations - now supports multiple references per memory
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error)

	// Direct fact storage - bypasses extraction for testing and consolidation
	StoreFactsDirectly(ctx context.Context, facts []*memory.MemoryFact, progressCallback memory.ProgressCallback) error
}

// Dependencies holds all the required dependencies for creating a MemoryStorage instance.
type Dependencies struct {
	Logger             *log.Logger
	Storage            storage.Interface
	CompletionsService *ai.Service
	CompletionsModel   string
	EmbeddingsWrapper  *storage.EmbeddingWrapper
}

// MemoryEngine implements pure business logic for memory operations.
// This contains no orchestration concerns (channels, workers, progress reporting).
type MemoryEngine struct {
	CompletionsService *ai.Service
	EmbeddingsWrapper  *storage.EmbeddingWrapper
	storage            storage.Interface
	CompletionsModel   string
}

// NewMemoryEngine creates a new MemoryEngine instance.
func NewMemoryEngine(completionsService *ai.Service, embeddingsWrapper *storage.EmbeddingWrapper, storage storage.Interface, completionsModel string) (*MemoryEngine, error) {
	if completionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if embeddingsWrapper == nil {
		return nil, fmt.Errorf("embeddings wrapper cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if completionsModel == "" {
		return nil, fmt.Errorf("completions model cannot be empty")
	}

	return &MemoryEngine{
		CompletionsService: completionsService,
		EmbeddingsWrapper:  embeddingsWrapper,
		storage:            storage,
		CompletionsModel:   completionsModel,
	}, nil
}

// StorageImpl is the main implementation of the MemoryStorage interface.
// It orchestrates memory operations using a clean 3-layer architecture:
// StorageImpl (public API) -> MemoryOrchestrator (coordination) -> MemoryEngine (business logic).
type StorageImpl struct {
	logger       *log.Logger
	orchestrator *MemoryOrchestrator
	storage      storage.Interface
	engine       *MemoryEngine
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
	if deps.EmbeddingsWrapper == nil {
		return nil, fmt.Errorf("embeddings wrapper cannot be nil")
	}

	// Create the memory engine with business logic
	engine, err := NewMemoryEngine(deps.CompletionsService, deps.EmbeddingsWrapper, deps.Storage, deps.CompletionsModel)
	if err != nil {
		return nil, fmt.Errorf("creating memory engine: %w", err)
	}

	// Create the orchestrator with coordination logic
	orchestrator, err := NewMemoryOrchestrator(engine, deps.Storage, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating memory orchestrator: %w", err)
	}

	return &StorageImpl{
		logger:       deps.Logger,
		orchestrator: orchestrator,
		storage:      deps.Storage,
		engine:       engine,
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
	return s.storage.Query(ctx, queryText, filter)
}

// GetDocumentReferences retrieves all document references for a memory.
func (s *StorageImpl) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	return s.storage.GetDocumentReferences(ctx, memoryID)
}

// StoreFactsDirectly stores memory facts directly without going through the extraction pipeline.
// This is useful for:
// - Test suite: storing pre-extracted facts from JSON
// - Consolidation: storing consolidated facts
// - Production: final step of the pipeline (called by orchestrator).
func (s *StorageImpl) StoreFactsDirectly(ctx context.Context, facts []*memory.MemoryFact, progressCallback memory.ProgressCallback) error {
	if len(facts) == 0 {
		s.logger.Info("No facts to store")
		return nil
	}

	config := DefaultConfig()

	// Convert facts to storage objects with embeddings
	var objects []*models.Object
	processed := 0

	// Batch generate embeddings for efficiency
	var factContents []string
	for _, fact := range facts {
		factContents = append(factContents, fact.GenerateContent())
	}

	// Generate ALL embeddings in batch - MUCH FASTER! 🚀
	s.logger.Info("Generating embeddings in batch", "count", len(factContents))
	embeddings, err := s.engine.EmbeddingsWrapper.Embeddings(ctx, factContents)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	// Create Weaviate objects with pre-generated embeddings
	for i, fact := range facts {
		// Build tags
		tags := fact.Tags
		if len(tags) == 0 {
			tags = []string{fact.Category} // Fallback to category
		}

		// Build metadata JSON
		metadata := fact.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["fact_id"] = fact.ID
		if fact.TemporalContext != nil {
			metadata["temporal_context"] = *fact.TemporalContext
		}
		metadataJSON, _ := json.Marshal(metadata)

		// Create Weaviate object with structured fields
		properties := map[string]interface{}{
			"content":            fact.GenerateContent(),
			"timestamp":          fact.Timestamp.Format(time.RFC3339),
			"source":             fact.Source,
			"tags":               tags,
			"documentReferences": fact.DocumentReferences,
			"metadataJson":       string(metadataJSON),
			// Structured fact fields for precise filtering
			"factCategory":    fact.Category,
			"factSubject":     fact.Subject,
			"factAttribute":   fact.Attribute,
			"factValue":       fact.Value,
			"factSensitivity": fact.Sensitivity,
			"factImportance":  fact.Importance,
		}

		// Add temporal context if present
		if fact.TemporalContext != nil {
			properties["factTemporalContext"] = *fact.TemporalContext
		}

		obj := &models.Object{
			Class:      "MemoryFact",
			Properties: properties,
			Vector:     embeddings[i], // Use pre-generated embedding
		}

		// Only set ID if it's a valid UUID format
		if fact.ID != "" {
			obj.ID = strfmt.UUID(fact.ID)
		}

		objects = append(objects, obj)

		// Batch storage when we reach the batch size
		if len(objects) >= config.BatchSize {
			if err := s.storage.StoreBatch(ctx, objects); err != nil {
				return fmt.Errorf("batch storage failed: %w", err)
			}

			processed += len(objects)
			objects = objects[:0] // Reset slice

			// Report progress
			if progressCallback != nil {
				progressCallback(processed, len(facts))
			}

			s.logger.Infof("Stored batch of %d memory objects", config.BatchSize)
		}
	}

	// Store remaining objects
	if len(objects) > 0 {
		if err := s.storage.StoreBatch(ctx, objects); err != nil {
			return fmt.Errorf("final batch storage failed: %w", err)
		}
		processed += len(objects)

		// Final progress callback
		if progressCallback != nil {
			progressCallback(processed, len(facts))
		}

		s.logger.Infof("Stored final batch of %d memory objects", len(objects))
	}

	s.logger.Infof("Successfully stored %d memory objects", processed)
	return nil
}
