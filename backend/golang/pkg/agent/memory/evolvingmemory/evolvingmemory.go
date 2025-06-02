package evolvingmemory

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const (
	ClassName         = "TextDocument"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	tagsProperty      = "tags"
	metadataProperty  = "metadataJson"
	openAIEmbedModel  = "text-embedding-3-small"
	openAIChatModel   = "gpt-4o-mini"

	// Tool Names (matching function names in tools.go).
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
	SpeakerID  string
	Timestamp  time.Time
	DateString string // Pre-formatted
}

// Processing pipeline types.
type ExtractedFact struct {
	Content   string
	SpeakerID string
	Source    PreparedDocument
}

// Memory actions.
type MemoryAction string

const (
	// Using existing constants from above.
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
	Metadata  map[string]string // Contains speakerID if present
}

// Validation rules.
type ValidationRule struct {
	CurrentSpeakerID string // Who's processing this fact
	IsDocumentLevel  bool   // Is current context document-level?
	TargetMemoryID   string // For UPDATE/DELETE
	TargetSpeakerID  string // Speaker of target memory
	Action           MemoryAction
}

// Interfaces for IO boundaries.
type FactExtractor interface {
	ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error)
}

type MemoryOperations interface {
	SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error)
	DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error)
	UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error
	DeleteMemory(ctx context.Context, memoryID string) error
}

type StorageOperations interface {
	StoreBatch(ctx context.Context, objects []*models.Object) error
}

// --- Structs for Tool Call Arguments ---

// AddToolArguments is currently empty as per tools.go definition
// type AddToolArguments struct {}

// UpdateToolArguments matches the parameters defined in updateMemoryTool in tools.go.
type UpdateToolArguments struct {
	MemoryID      string `json:"id"`
	UpdatedMemory string `json:"updated_content"`
	Reason        string `json:"reason,omitempty"`
}

// DeleteToolArguments matches the parameters defined in deleteMemoryTool in tools.go.
type DeleteToolArguments struct {
	MemoryID string `json:"id"`
	Reason   string `json:"reason,omitempty"`
}

// NoneToolArguments matches the parameters defined in noneMemoryTool in tools.go.
type NoneToolArguments struct {
	Reason string `json:"reason"`
}

// ExtractFactsToolArguments matches the parameters defined in extractFactsTool in tools.go.
type ExtractFactsToolArguments struct {
	Facts []string `json:"facts"`
}

// WeaviateStorage implements the memory.Storage interface using Weaviate.
type WeaviateStorage struct {
	client             *weaviate.Client
	logger             *log.Logger
	completionsService *ai.Service
	embeddingsService  *ai.Service
}

// New creates a new WeaviateStorage instance.
// weaviateHost should be like "localhost:8081".
// weaviateScheme is "http" or "https".
// The logger is used for logging messages.
// The aiService is used for generating embeddings.
func New(logger *log.Logger, client *weaviate.Client, completionsService *ai.Service, embeddingsService *ai.Service) (*WeaviateStorage, error) {
	storage := &WeaviateStorage{
		client:             client,
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
	}

	return storage, nil
}

// firstNChars is a helper to get the first N characters of a string for logging.
func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// StoreConversations is an alias for Store to maintain backward compatibility.
func (s *WeaviateStorage) StoreConversations(ctx context.Context, documents []memory.ConversationDocument, progressChan chan<- memory.ProgressUpdate) error {
	var callback memory.ProgressCallback
	if progressChan != nil {
		callback = func(processed, total int) {
			select {
			case progressChan <- memory.ProgressUpdate{Processed: processed, Total: total}:
			default:
				s.logger.Warnf("Progress update dropped for StoreConversations: processed %d, total %d (channel busy or closed)", processed, total)
			}
		}
	}

	// Convert ConversationDocuments to unified Document interface
	unifiedDocs := memory.ConversationDocumentsToDocuments(documents)
	return s.Store(ctx, unifiedDocs, callback)
}
