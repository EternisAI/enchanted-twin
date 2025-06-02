package evolvingmemory

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

// EnsureSchemaExistsInternal ensures the Weaviate schema exists for the Memory class.
func EnsureSchemaExistsInternal(client *weaviate.Client, logger *log.Logger) error {
	// First, check if schema already exists
	schema, err := client.Schema().Getter().Do(context.Background())
	if err != nil {
		return fmt.Errorf("getting schema: %w", err)
	}

	// Check if our class already exists
	for _, class := range schema.Classes {
		if class.Class == ClassName {
			// Schema exists, validate it matches our expectations
			logger.Infof("Schema for class %s already exists, validating...", ClassName)

			// Basic validation - check properties exist
			expectedProps := map[string]string{
				contentProperty:   "text",
				timestampProperty: "date",
				metadataProperty:  "text",
			}

			existingProps := make(map[string]string)
			for _, prop := range class.Properties {
				for _, dt := range prop.DataType {
					existingProps[prop.Name] = dt
				}
			}

			// Check if all expected properties exist
			for propName, propType := range expectedProps {
				if existingType, exists := existingProps[propName]; !exists {
					return fmt.Errorf("missing property %s in existing schema", propName)
				} else if existingType != propType {
					return fmt.Errorf("property %s has type %s, expected %s", propName, existingType, propType)
				}
			}

			logger.Info("Schema validation successful")
			return nil
		}
	}

	// Schema doesn't exist, create it
	logger.Infof("Creating schema for class %s", ClassName)

	classObj := &models.Class{
		Class:       ClassName,
		Description: "A memory entry in the evolving memory system",
		Properties: []*models.Property{
			{
				Name:        contentProperty,
				DataType:    []string{"text"},
				Description: "The content of the memory",
			},
			{
				Name:        timestampProperty,
				DataType:    []string{"date"},
				Description: "When this memory was created or last updated",
			},
			{
				Name:        metadataProperty,
				DataType:    []string{"text"},
				Description: "JSON-encoded metadata for the memory",
			},
		},
		Vectorizer: "none", // We provide our own vectors
	}

	err = client.Schema().ClassCreator().WithClass(classObj).Do(context.Background())
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	logger.Info("Schema created successfully")
	return nil
}

// GetByID retrieves a specific memory document from Weaviate by its ID.
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	if s.client == nil || s.client.Data() == nil {
		return nil, fmt.Errorf("weaviate client not initialized")
	}

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
	if s.client == nil || s.client.Data() == nil {
		return fmt.Errorf("weaviate client not initialized")
	}

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

// DeleteAll removes all memory documents from Weaviate.
func (s *WeaviateStorage) DeleteAll(ctx context.Context) error {
	// Note: For v5 client, we need to delete the class and recreate it
	// as batch delete with filters has changed significantly

	// Check if class exists before trying to delete
	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence before delete all for '%s': %w", ClassName, err)
	}
	if !exists {
		s.logger.Info("Class does not exist, no need to delete.", "class", ClassName)
		return nil
	}

	// Delete the entire class
	err = s.client.Schema().ClassDeleter().WithClassName(ClassName).Do(ctx)
	if err != nil {
		// Check if it was deleted concurrently
		existsAfterAttempt, checkErr := s.client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
		if checkErr == nil && !existsAfterAttempt {
			s.logger.Info("Class was deleted, possibly concurrently.", "class", ClassName)
			return nil
		}
		return fmt.Errorf("failed to delete class '%s': %w", ClassName, err)
	}

	s.logger.Info("Successfully deleted all memories by removing class", "class", ClassName)

	// Recreate the schema
	return EnsureSchemaExistsInternal(s.client, s.logger)
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

	// Check for individual errors
	var errors []error
	for _, obj := range result {
		if obj.Result.Errors != nil && len(obj.Result.Errors.Error) > 0 {
			for _, e := range obj.Result.Errors.Error {
				errors = append(errors, fmt.Errorf("object error: %s", e.Message))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch store had %d errors: %v", len(errors), errors[0])
	}

	s.logger.Infof("Successfully batch stored %d objects", len(objects))
	return nil
}
