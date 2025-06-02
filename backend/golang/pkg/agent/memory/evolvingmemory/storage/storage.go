package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

const (
	ClassName         = "TextDocument"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	metadataProperty  = "metadataJson"
)

// Interface defines the storage operations needed by evolvingmemory.
type Interface interface {
	GetByID(ctx context.Context, id string) (*memory.TextDocument, error)
	Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error
	Delete(ctx context.Context, id string) error
	StoreBatch(ctx context.Context, objects []*models.Object) error
}

// WeaviateStorage implements the storage interface using Weaviate.
type WeaviateStorage struct {
	client *weaviate.Client
	logger *log.Logger
}

// New creates a new WeaviateStorage instance.
func New(client *weaviate.Client, logger *log.Logger) Interface {
	return &WeaviateStorage{
		client: client,
		logger: logger,
	}
}

// GetByID retrieves a specific memory document from Weaviate by its ID.
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	result, err := s.client.Data().ObjectsGetter().
		WithID(id).
		WithClassName(ClassName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting object by ID: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no object found with ID %s", id)
	}

	obj := result[0]
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid properties type for object %s", id)
	}

	// Parse content
	content, ok := props[contentProperty].(string)
	if !ok {
		return nil, fmt.Errorf("invalid content type for object %s", id)
	}

	// Parse timestamp
	var timestamp *time.Time
	if ts, ok := props[timestampProperty].(string); ok {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			timestamp = &parsed
		}
	}

	// Parse metadata
	metadata := make(map[string]string)
	if metaStr, ok := props[metadataProperty].(string); ok {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			s.logger.Warnf("Failed to unmarshal metadata for object %s: %v", id, err)
		}
	}

	doc := &memory.TextDocument{
		FieldID:        string(obj.ID),
		FieldContent:   content,
		FieldTimestamp: timestamp,
		FieldMetadata:  metadata,
		FieldTags:      []string{}, // Tags not currently stored
	}

	return doc, nil
}

// Update updates an existing memory document in Weaviate.
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	// Prepare metadata JSON
	metadataJSON, err := json.Marshal(doc.Metadata())
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Prepare properties
	properties := map[string]interface{}{
		contentProperty:  doc.Content(),
		metadataProperty: string(metadataJSON),
	}

	// Add timestamp if provided
	if doc.Timestamp() != nil {
		properties[timestampProperty] = doc.Timestamp().Format(time.RFC3339)
	}

	// Perform the update
	err = s.client.Data().Updater().
		WithID(id).
		WithClassName(ClassName).
		WithProperties(properties).
		WithVector(vector).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("updating object: %w", err)
	}

	s.logger.Infof("Successfully updated memory with ID %s", id)
	return nil
}

// Delete removes a memory document from Weaviate by ID.
func (s *WeaviateStorage) Delete(ctx context.Context, id string) error {
	err := s.client.Data().Deleter().
		WithID(id).
		WithClassName(ClassName).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("deleting object: %w", err)
	}

	s.logger.Infof("Successfully deleted memory with ID %s", id)
	return nil
}

// StoreBatch implements the StorageOperations interface for batch storing objects.
func (s *WeaviateStorage) StoreBatch(ctx context.Context, objects []*models.Object) error {
	if len(objects) == 0 {
		return nil
	}

	// Use the client's batch API
	batcher := s.client.Batch().ObjectsBatcher()
	for _, obj := range objects {
		batcher = batcher.WithObjects(obj)
	}

	result, err := batcher.Do(ctx)
	if err != nil {
		return fmt.Errorf("batch storing objects: %w", err)
	}

	// Check for individual errors and fail on first error
	for _, obj := range result {
		if obj.Result.Errors != nil && len(obj.Result.Errors.Error) > 0 {
			return fmt.Errorf("object error: %s", obj.Result.Errors.Error[0].Message)
		}
	}

	s.logger.Infof("Successfully batch stored %d objects", len(objects))
	return nil
}
