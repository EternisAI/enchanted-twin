package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	weaviateGraphql "github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const (
	ClassName         = "TextDocument"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	metadataProperty  = "metadataJson"
	tagsProperty      = "tags"
	openAIEmbedModel  = "text-embedding-3-small"
)

// Interface defines the storage operations needed by evolvingmemory.
type Interface interface {
	GetByID(ctx context.Context, id string) (*memory.TextDocument, error)
	Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error
	Delete(ctx context.Context, id string) error
	StoreBatch(ctx context.Context, objects []*models.Object) error
	DeleteAll(ctx context.Context) error
	Query(ctx context.Context, queryText string) (memory.QueryResult, error)
	QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error)
}

// WeaviateStorage implements the storage interface using Weaviate.
type WeaviateStorage struct {
	client            *weaviate.Client
	logger            *log.Logger
	embeddingsService *ai.Service
}

// New creates a new WeaviateStorage instance.
func New(client *weaviate.Client, logger *log.Logger, embeddingsService *ai.Service) Interface {
	return &WeaviateStorage{
		client:            client,
		logger:            logger,
		embeddingsService: embeddingsService,
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
	return s.ensureSchemaExists(ctx)
}

// ensureSchemaExists ensures the Weaviate schema exists for the Memory class.
func (s *WeaviateStorage) ensureSchemaExists(ctx context.Context) error {
	// First, check if schema already exists
	schema, err := s.client.Schema().Getter().Do(ctx)
	if err != nil {
		return fmt.Errorf("getting schema: %w", err)
	}

	// Check if our class already exists
	for _, class := range schema.Classes {
		if class.Class == ClassName {
			// Schema exists, validate it matches our expectations
			s.logger.Infof("Schema for class %s already exists, validating...", ClassName)

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

			s.logger.Info("Schema validation successful")
			return nil
		}
	}

	// Schema doesn't exist, create it
	s.logger.Infof("Creating schema for class %s", ClassName)

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

	err = s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	s.logger.Info("Schema created successfully")
	return nil
}

// Query retrieves memories relevant to the query text.
func (s *WeaviateStorage) Query(ctx context.Context, queryText string) (memory.QueryResult, error) {
	s.logger.Info("Query method called", "query_text", queryText)

	vector, err := s.embeddingsService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	contentField := weaviateGraphql.Field{Name: contentProperty}
	timestampField := weaviateGraphql.Field{Name: timestampProperty}
	metaField := weaviateGraphql.Field{Name: metadataProperty}
	tagsField := weaviateGraphql.Field{Name: tagsProperty}
	additionalFields := weaviateGraphql.Field{
		Name: "_additional",
		Fields: []weaviateGraphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(ClassName).
		WithNearVector(nearVector).
		WithLimit(10).
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		return memory.QueryResult{}, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	finalResults := []memory.TextDocument{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
	}

	classData, ok := data[ClassName].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", ClassName)
		return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate (pre-filtering)", "count", len(classData))

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)
		metadataJSON, _ := obj[metadataProperty].(string)

		var parsedTimestamp *time.Time
		if tsStr, tsOk := obj[timestampProperty].(string); tsOk {
			t, pErr := time.Parse(time.RFC3339, tsStr)
			if pErr == nil {
				parsedTimestamp = &t
			} else {
				s.logger.Warn("Failed to parse timestamp from Weaviate", "timestamp_str", tsStr, "error", pErr)
			}
		}

		additional, _ := obj["_additional"].(map[string]interface{})
		id, _ := additional["id"].(string)

		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.TextDocument{
			FieldID:        id,
			FieldContent:   content,
			FieldTimestamp: parsedTimestamp,
			FieldMetadata:  metaMap,
			FieldTags:      tags,
		})
	}
	s.logger.Info("Query processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return memory.QueryResult{Facts: []memory.MemoryFact{}, Documents: finalResults}, nil
}

// QueryWithDistance retrieves memories relevant to the query text with similarity distances, with optional metadata filtering.
func (s *WeaviateStorage) QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error) {
	s.logger.Info("QueryWithDistance method called", "query_text", queryText, "filters", metadataFilters)

	vector, err := s.embeddingsService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	contentField := weaviateGraphql.Field{Name: contentProperty}
	timestampField := weaviateGraphql.Field{Name: timestampProperty}
	metaField := weaviateGraphql.Field{Name: metadataProperty}
	tagsField := weaviateGraphql.Field{Name: tagsProperty}
	additionalFields := weaviateGraphql.Field{
		Name: "_additional",
		Fields: []weaviateGraphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(ClassName).
		WithNearVector(nearVector).
		WithLimit(10).
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	// Add WHERE filtering if metadata filters are provided
	if len(metadataFilters) > 0 {
		filterMap := metadataFilters[0] // Use first filter map if provided
		for key, value := range filterMap {
			if key == "type" {
				// Create a filter that looks for the type in the metadata JSON
				// Since metadata is stored as JSON string, we need to use a Like operator
				// to match the pattern: "type":"friend"
				whereFilter := filters.Where().
					WithPath([]string{metadataProperty}).
					WithOperator(filters.Like).
					WithValueText(fmt.Sprintf(`*"%s":"%s"*`, key, value))

				queryBuilder = queryBuilder.WithWhere(whereFilter)
				s.logger.Debug("Added WHERE filter", "key", key, "value", value, "pattern", fmt.Sprintf(`*"%s":"%s"*`, key, value))
			}
		}
	}

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		return memory.QueryWithDistanceResult{}, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	finalResults := []memory.DocumentWithDistance{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return memory.QueryWithDistanceResult{Documents: finalResults}, nil
	}

	classData, ok := data[ClassName].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", ClassName)
		return memory.QueryWithDistanceResult{Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate (pre-filtering)", "count", len(classData))

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)
		metadataJSON, _ := obj[metadataProperty].(string)

		var parsedTimestamp *time.Time
		if tsStr, tsOk := obj[timestampProperty].(string); tsOk {
			t, pErr := time.Parse(time.RFC3339, tsStr)
			if pErr == nil {
				parsedTimestamp = &t
			} else {
				s.logger.Warn("Failed to parse timestamp from Weaviate", "timestamp_str", tsStr, "error", pErr)
			}
		}

		additional, _ := obj["_additional"].(map[string]interface{})
		id, _ := additional["id"].(string)
		distance, _ := additional["distance"].(float64)

		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.DocumentWithDistance{
			Document: memory.TextDocument{
				FieldID:        id,
				FieldContent:   content,
				FieldTimestamp: parsedTimestamp,
				FieldMetadata:  metaMap,
				FieldTags:      tags,
			},
			Distance: float32(distance),
		})
	}
	s.logger.Info("QueryWithDistance processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return memory.QueryWithDistanceResult{Documents: finalResults}, nil
}
