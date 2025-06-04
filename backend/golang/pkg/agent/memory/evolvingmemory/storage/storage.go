package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	sourceProperty    = "source"
	speakerProperty   = "speakerID"
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
	Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error)
	QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error)
	EnsureSchemaExists(ctx context.Context) error
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

	// Parse direct fields
	source, _ := props[sourceProperty].(string)
	speakerID, _ := props[speakerProperty].(string)

	// Parse metadata
	metadata := make(map[string]string)
	if metaStr, ok := props[metadataProperty].(string); ok {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			s.logger.Warnf("Failed to unmarshal metadata for object %s: %v", id, err)
		}
	}

	// Merge direct fields into metadata (they take precedence)
	if source != "" {
		metadata["source"] = source
	}
	if speakerID != "" {
		metadata["speakerID"] = speakerID
	}

	doc := &memory.TextDocument{
		FieldID:        string(obj.ID),
		FieldContent:   content,
		FieldTimestamp: timestamp,
		FieldMetadata:  metadata,
		FieldSource:    source,     // Populate the direct source field
		FieldTags:      []string{}, // Tags not currently stored
	}

	return doc, nil
}

// Update updates an existing memory document in Weaviate.
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	// Prepare metadata JSON (for backward compatibility)
	metadataJSON, err := json.Marshal(doc.Metadata())
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Prepare properties
	properties := map[string]interface{}{
		contentProperty:  doc.Content(),
		metadataProperty: string(metadataJSON), // Keep for backward compatibility
	}

	// Extract and store source as direct field
	if source := doc.Source(); source != "" {
		properties[sourceProperty] = source
	} else if source, exists := doc.Metadata()["source"]; exists {
		properties[sourceProperty] = source
	}

	// Extract and store speakerID as direct field
	if speakerID, exists := doc.Metadata()["speakerID"]; exists {
		properties[speakerProperty] = speakerID
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
	return s.EnsureSchemaExists(ctx)
}

// EnsureSchemaExists ensures the Weaviate schema exists for the Memory class.
func (s *WeaviateStorage) EnsureSchemaExists(ctx context.Context) error {
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
			requiredProps := map[string]string{
				contentProperty:   "text",
				timestampProperty: "date",
				metadataProperty:  "text",
				tagsProperty:      "text[]",
			}

			existingProps := make(map[string]string)
			for _, prop := range class.Properties {
				for _, dt := range prop.DataType {
					existingProps[prop.Name] = dt
				}
			}

			// Check if all expected properties exist - allow missing new fields for migration
			for propName, propType := range requiredProps {
				if existingType, exists := existingProps[propName]; !exists {
					return fmt.Errorf("missing required property %s in existing schema", propName)
				} else if existingType != propType {
					return fmt.Errorf("required property %s has type %s, expected %s", propName, existingType, propType)
				}
			}

			// Check if new fields exist, if not we need to add them
			needsUpdate := false
			if _, exists := existingProps[sourceProperty]; !exists {
				needsUpdate = true
			}
			if _, exists := existingProps[speakerProperty]; !exists {
				needsUpdate = true
			}

			if needsUpdate {
				s.logger.Info("Schema needs update to add new metadata fields")
				return s.addMetadataFields(ctx)
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
				Description: "JSON-encoded metadata for the memory (legacy)",
			},
			{
				Name:        sourceProperty,
				DataType:    []string{"text"},
				Description: "Source of the memory document",
			},
			{
				Name:        speakerProperty,
				DataType:    []string{"text"},
				Description: "Speaker/contact ID for the memory",
			},
			{
				Name:        tagsProperty,
				DataType:    []string{"text[]"},
				Description: "Tags associated with the memory",
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
func (s *WeaviateStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	s.logger.Info("Query method called", "query_text", queryText, "filter", filter)

	vector, err := s.embeddingsService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	// Set distance filter if provided
	if filter != nil && filter.Distance > 0 {
		nearVector = nearVector.WithDistance(filter.Distance)
		s.logger.Debug("Added distance filter", "distance", filter.Distance)
	}

	contentField := weaviateGraphql.Field{Name: contentProperty}
	timestampField := weaviateGraphql.Field{Name: timestampProperty}
	metaField := weaviateGraphql.Field{Name: metadataProperty}
	sourceField := weaviateGraphql.Field{Name: sourceProperty}
	speakerField := weaviateGraphql.Field{Name: speakerProperty}
	tagsField := weaviateGraphql.Field{Name: tagsProperty}
	additionalFields := weaviateGraphql.Field{
		Name: "_additional",
		Fields: []weaviateGraphql.Field{
			{Name: "id"},
			{Name: "distance"},
		},
	}

	// Set limit from filter or use default
	limit := 10
	if filter != nil && filter.Limit != nil {
		limit = *filter.Limit
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(ClassName).
		WithNearVector(nearVector).
		WithLimit(limit).
		WithFields(contentField, timestampField, metaField, sourceField, speakerField, tagsField, additionalFields)

	// Add WHERE filtering if filter is provided
	if filter != nil {
		var whereFilters []*filters.WhereBuilder

		if filter.Source != nil {
			// Use direct field filtering instead of JSON pattern matching
			sourceFilter := filters.Where().
				WithPath([]string{sourceProperty}).
				WithOperator(filters.Equal).
				WithValueText(*filter.Source)
			whereFilters = append(whereFilters, sourceFilter)
			s.logger.Debug("Added source filter", "source", *filter.Source)
		}

		if filter.ContactName != nil {
			// Use direct field filtering for speaker ID
			contactFilter := filters.Where().
				WithPath([]string{speakerProperty}).
				WithOperator(filters.Equal).
				WithValueText(*filter.ContactName)
			whereFilters = append(whereFilters, contactFilter)
			s.logger.Debug("Added contact name filter", "contactName", *filter.ContactName)
		}

		if filter.Tags != nil && !filter.Tags.IsEmpty() {
			// Use the new TagsFilter structure
			tagsFilter, err := s.buildTagsFilter(filter.Tags)
			if err != nil {
				return memory.QueryResult{}, fmt.Errorf("failed to build tags filter: %w", err)
			}
			if tagsFilter != nil {
				whereFilters = append(whereFilters, tagsFilter)
				s.logger.Debug("Added tags filter", "tagsFilter", filter.Tags)
			}
		}

		if len(whereFilters) > 0 {
			combinedFilter := filters.Where().
				WithOperator(filters.And).
				WithOperands(whereFilters)
			queryBuilder = queryBuilder.WithWhere(combinedFilter)
			s.logger.Debug("Applied combined WHERE filters", "filter_count", len(whereFilters))
		}
	}

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		var errorMsgs []string
		for _, err := range resp.Errors {
			errorMsgs = append(errorMsgs, err.Message)
		}
		return memory.QueryResult{}, fmt.Errorf("GraphQL query errors: %s", strings.Join(errorMsgs, "; "))
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
		source, _ := obj[sourceProperty].(string)
		speakerID, _ := obj[speakerProperty].(string)

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

		// Start with metadata from JSON (for backward compatibility)
		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		// Merge direct fields into metadata (they take precedence)
		if source != "" {
			metaMap["source"] = source
		}
		if speakerID != "" {
			metaMap["speakerID"] = speakerID
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
			FieldSource:    source, // Populate the direct source field
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
		var errorMsgs []string
		for _, err := range resp.Errors {
			errorMsgs = append(errorMsgs, err.Message)
		}
		return memory.QueryWithDistanceResult{}, fmt.Errorf("GraphQL query errors: %s", strings.Join(errorMsgs, "; "))
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
		source, _ := obj[sourceProperty].(string)
		speakerID, _ := obj[speakerProperty].(string)

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

		// Start with metadata from JSON (for backward compatibility)
		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		// Merge direct fields into metadata (they take precedence)
		if source != "" {
			metaMap["source"] = source
		}
		if speakerID != "" {
			metaMap["speakerID"] = speakerID
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
				FieldSource:    source, // Populate the direct source field
				FieldTags:      tags,
			},
			Distance: float32(distance),
		})
	}
	s.logger.Info("QueryWithDistance processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return memory.QueryWithDistanceResult{Documents: finalResults}, nil
}

// addMetadataFields adds the new metadata fields to the existing schema.
func (s *WeaviateStorage) addMetadataFields(ctx context.Context) error {
	// Add source property
	if err := s.client.Schema().PropertyCreator().
		WithClassName(ClassName).
		WithProperty(&models.Property{
			Name:        sourceProperty,
			DataType:    []string{"text"},
			Description: "Source of the memory document",
		}).Do(ctx); err != nil {
		return fmt.Errorf("failed to add source property: %w", err)
	}

	// Add speakerID property
	if err := s.client.Schema().PropertyCreator().
		WithClassName(ClassName).
		WithProperty(&models.Property{
			Name:        speakerProperty,
			DataType:    []string{"text"},
			Description: "Speaker/contact ID for the memory",
		}).Do(ctx); err != nil {
		return fmt.Errorf("failed to add speakerID property: %w", err)
	}

	s.logger.Info("Successfully added new metadata fields to schema")
	return nil
}

// buildTagsFilter creates a Weaviate filter from a TagsFilter structure.
// It supports simple ALL/ANY logic as well as complex boolean expressions.
func (s *WeaviateStorage) buildTagsFilter(tagsFilter *memory.TagsFilter) (*filters.WhereBuilder, error) {
	if tagsFilter == nil || tagsFilter.IsEmpty() {
		return nil, nil
	}

	// Handle simple ALL case (backward compatible)
	if len(tagsFilter.All) > 0 {
		s.logger.Debug("Building tags filter with ALL logic", "tags", tagsFilter.All)
		return filters.Where().
			WithPath([]string{tagsProperty}).
			WithOperator(filters.ContainsAll).
			WithValueText(tagsFilter.All...), nil
	}

	// Handle simple ANY case
	if len(tagsFilter.Any) > 0 {
		s.logger.Debug("Building tags filter with ANY logic", "tags", tagsFilter.Any)
		// For ANY logic, we need to create OR conditions for each tag
		var orFilters []*filters.WhereBuilder
		for _, tag := range tagsFilter.Any {
			tagFilter := filters.Where().
				WithPath([]string{tagsProperty}).
				WithOperator(filters.ContainsAny).
				WithValueText(tag)
			orFilters = append(orFilters, tagFilter)
		}

		if len(orFilters) == 1 {
			return orFilters[0], nil
		}

		return filters.Where().
			WithOperator(filters.Or).
			WithOperands(orFilters), nil
	}

	// Handle complex boolean expression
	if tagsFilter.Expression != nil {
		s.logger.Debug("Building tags filter with complex expression")
		return s.buildBooleanExpressionFilter(tagsFilter.Expression)
	}

	return nil, nil
}

// buildBooleanExpressionFilter recursively builds a Weaviate filter from a BooleanExpression.
func (s *WeaviateStorage) buildBooleanExpressionFilter(expr *memory.BooleanExpression) (*filters.WhereBuilder, error) {
	if expr == nil {
		return nil, fmt.Errorf("boolean expression cannot be nil")
	}

	// Handle leaf nodes (tags)
	if expr.IsLeaf() {
		s.logger.Debug("Building leaf expression filter", "operator", expr.Operator, "tags", expr.Tags)

		switch expr.Operator {
		case memory.AND:
			// For AND with multiple tags, use ContainsAll
			return filters.Where().
				WithPath([]string{tagsProperty}).
				WithOperator(filters.ContainsAll).
				WithValueText(expr.Tags...), nil
		case memory.OR:
			// For OR with multiple tags, create OR conditions
			var orFilters []*filters.WhereBuilder
			for _, tag := range expr.Tags {
				tagFilter := filters.Where().
					WithPath([]string{tagsProperty}).
					WithOperator(filters.ContainsAny).
					WithValueText(tag)
				orFilters = append(orFilters, tagFilter)
			}

			if len(orFilters) == 1 {
				return orFilters[0], nil
			}

			return filters.Where().
				WithOperator(filters.Or).
				WithOperands(orFilters), nil
		default:
			return nil, fmt.Errorf("unsupported boolean operator in leaf expression: %v", expr.Operator)
		}
	}

	// Handle branch nodes (left and right operands)
	if expr.IsBranch() {
		s.logger.Debug("Building branch expression filter", "operator", expr.Operator)

		leftFilter, err := s.buildBooleanExpressionFilter(expr.Left)
		if err != nil {
			return nil, fmt.Errorf("building left operand: %w", err)
		}

		rightFilter, err := s.buildBooleanExpressionFilter(expr.Right)
		if err != nil {
			return nil, fmt.Errorf("building right operand: %w", err)
		}

		if leftFilter == nil || rightFilter == nil {
			return nil, fmt.Errorf("both left and right operands must be non-nil for branch expression")
		}

		switch expr.Operator {
		case memory.AND:
			return filters.Where().
				WithOperator(filters.And).
				WithOperands([]*filters.WhereBuilder{leftFilter, rightFilter}), nil
		case memory.OR:
			return filters.Where().
				WithOperator(filters.Or).
				WithOperands([]*filters.WhereBuilder{leftFilter, rightFilter}), nil
		default:
			return nil, fmt.Errorf("unsupported boolean operator in branch expression: %v", expr.Operator)
		}
	}

	return nil, fmt.Errorf("boolean expression must be either a leaf node (with tags) or a branch node (with left/right operands)")
}
