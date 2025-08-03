package storage

import (
	"context"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// DocumentReference holds the original document information.
type DocumentReference struct {
	ID      string
	Content string
	Type    string
}

// StoredDocument represents a document in the separate document table.
type StoredDocument struct {
	ID          string
	Content     string
	Type        string
	OriginalID  string
	ContentHash string
	Metadata    map[string]string
	CreatedAt   time.Time
}

// DocumentChunk represents a chunk of a document stored separately from facts.
type DocumentChunk struct {
	ID                 string
	Content            string
	ChunkIndex         int
	OriginalDocumentID string
	Source             string
	FilePath           string
	Tags               []string
	Metadata           map[string]string
	CreatedAt          time.Time
}

// Interface defines the storage operations needed by evolvingmemory.
type Interface interface {
	GetByID(ctx context.Context, id string) (*memory.MemoryFact, error)
	Update(ctx context.Context, id string, fact *memory.MemoryFact, vector []float32) error
	Delete(ctx context.Context, id string) error
	StoreBatch(ctx context.Context, objects []*models.Object) error
	DeleteAll(ctx context.Context) error
	Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error)
	EnsureSchemaExists(ctx context.Context) error

	// Document reference operations - now supports multiple references
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error)

	// Document storage operations
	UpsertDocument(ctx context.Context, doc memory.Document) (string, error)
	GetStoredDocument(ctx context.Context, documentID string) (*StoredDocument, error)
	GetStoredDocumentsBatch(ctx context.Context, documentIDs []string) ([]*StoredDocument, error)

	// Document chunk operations
	StoreDocumentChunksBatch(ctx context.Context, chunks []*DocumentChunk) error
	QueryDocumentChunks(ctx context.Context, queryText string, filter *memory.Filter) ([]*DocumentChunk, error)

	// Batch fact retrieval for intelligent querying
	GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error)
}
