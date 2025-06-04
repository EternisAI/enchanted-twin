package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	DocumentClassName = "SourceDocument" // New separate class for documents
	contentProperty   = "content"
	timestampProperty = "timestamp"
	metadataProperty  = "metadataJson"
	tagsProperty      = "tags"
	// Updated properties for document references - now stores multiple reference IDs.
	documentReferencesProperty = "documentReferences" // Array of document IDs
	// Document table properties.
	documentContentHashProperty = "contentHash"
	documentTypeProperty        = "documentType"
	documentOriginalIDProperty  = "originalId"
	documentMetadataProperty    = "metadata"
	documentCreatedAtProperty   = "createdAt"
	openAIEmbedModel            = "text-embedding-3-small"
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

// Interface defines the storage operations needed by evolvingmemory.
type Interface interface {
	GetByID(ctx context.Context, id string) (*memory.TextDocument, error)
	Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error
	Delete(ctx context.Context, id string) error
	StoreBatch(ctx context.Context, objects []*models.Object) error
	DeleteAll(ctx context.Context) error
	Query(ctx context.Context, queryText string) (memory.QueryResult, error)
	QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error)
	EnsureSchemaExists(ctx context.Context) error

	// Document reference operations - now supports multiple references
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error)

	// Document storage operations
	StoreDocument(ctx context.Context, content, docType, originalID string, metadata map[string]string) (string, error)
	GetStoredDocument(ctx context.Context, documentID string) (*StoredDocument, error)
	GetStoredDocumentsBatch(ctx context.Context, documentIDs []string) ([]*StoredDocument, error)
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

	content, ok := props[contentProperty].(string)
	if !ok {
		return nil, fmt.Errorf("invalid content type for object %s", id)
	}

	var timestamp *time.Time
	if ts, ok := props[timestampProperty].(string); ok {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			timestamp = &parsed
		}
	}

	metadata := make(map[string]string)
	if metaStr, ok := props[metadataProperty].(string); ok {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			s.logger.Warnf("Failed to unmarshal metadata for object %s: %v", id, err)
		}
	}

	var tags []string
	if tagsInterface, ok := props[tagsProperty].([]interface{}); ok {
		for _, tagInterface := range tagsInterface {
			if tagStr, ok := tagInterface.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	if docRefsInterface, ok := props[documentReferencesProperty].([]interface{}); ok {
		var docRefs []string
		for _, docRefInterface := range docRefsInterface {
			if docRefStr, ok := docRefInterface.(string); ok {
				docRefs = append(docRefs, docRefStr)
			}
		}
		if len(docRefs) > 0 {
			docRefsJSON, err := json.Marshal(docRefs)
			if err == nil {
				metadata[documentReferencesProperty] = string(docRefsJSON)
			}
		}
	}

	doc := &memory.TextDocument{
		FieldID:        string(obj.ID),
		FieldContent:   content,
		FieldTimestamp: timestamp,
		FieldMetadata:  metadata,
		FieldTags:      tags,
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

	properties := map[string]interface{}{
		contentProperty:  doc.Content(),
		metadataProperty: string(metadataJSON),
	}

	if doc.Timestamp() != nil {
		properties[timestampProperty] = doc.Timestamp().Format(time.RFC3339)
	}

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

	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence before delete all for '%s': %w", ClassName, err)
	}
	if !exists {
		s.logger.Info("Class does not exist, no need to delete.", "class", ClassName)
		return nil
	}

	err = s.client.Schema().ClassDeleter().WithClassName(ClassName).Do(ctx)
	if err != nil {
		existsAfterAttempt, checkErr := s.client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
		if checkErr == nil && !existsAfterAttempt {
			s.logger.Info("Class was deleted, possibly concurrently.", "class", ClassName)
			return nil
		}
		return fmt.Errorf("failed to delete class '%s': %w", ClassName, err)
	}

	s.logger.Info("Successfully deleted all memories by removing class", "class", ClassName)

	return s.EnsureSchemaExists(ctx)
}

// EnsureSchemaExists ensures the Weaviate schema exists for both Memory and Document classes.
func (s *WeaviateStorage) EnsureSchemaExists(ctx context.Context) error {
	if err := s.ensureMemoryClassExists(ctx); err != nil {
		return err
	}

	if err := s.ensureDocumentClassExists(ctx); err != nil {
		return err
	}

	return nil
}

// ensureMemoryClassExists ensures the memory class schema exists.
func (s *WeaviateStorage) ensureMemoryClassExists(ctx context.Context) error {
	schema, err := s.client.Schema().Getter().Do(ctx)
	if err != nil {
		return fmt.Errorf("getting schema: %w", err)
	}

	for _, class := range schema.Classes {
		if class.Class == ClassName {
			s.logger.Infof("Schema for class %s already exists, validating...", ClassName)

			expectedProps := map[string]string{
				contentProperty:            "text",
				timestampProperty:          "date",
				metadataProperty:           "text",
				tagsProperty:               "text[]",
				documentReferencesProperty: "text[]",
			}

			existingProps := make(map[string]string)
			for _, prop := range class.Properties {
				for _, dt := range prop.DataType {
					existingProps[prop.Name] = dt
				}
			}

			for propName, propType := range expectedProps {
				if existingType, exists := existingProps[propName]; exists {
					if existingType != propType {
						return fmt.Errorf("property %s has type %s, expected %s", propName, existingType, propType)
					}
				} else {
					s.logger.Infof("Property %s is missing from schema but will be added automatically", propName)
				}
			}

			s.logger.Info("Memory schema validation successful")
			return nil
		}
	}

	s.logger.Infof("Creating schema for class %s", ClassName)

	indexFilterable := true
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
			{
				Name:        tagsProperty,
				DataType:    []string{"text[]"},
				Description: "Tags associated with the memory",
			},
			{
				Name:            documentReferencesProperty,
				DataType:        []string{"text[]"},
				Description:     "Array of document IDs that generated this memory",
				IndexFilterable: &indexFilterable,
			},
		},
		Vectorizer: "none",
	}

	err = s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		return fmt.Errorf("creating memory schema: %w", err)
	}

	s.logger.Info("Memory schema created successfully")
	return nil
}

// ensureDocumentClassExists ensures the document class schema exists.
func (s *WeaviateStorage) ensureDocumentClassExists(ctx context.Context) error {
	schema, err := s.client.Schema().Getter().Do(ctx)
	if err != nil {
		return fmt.Errorf("getting schema: %w", err)
	}

	for _, class := range schema.Classes {
		if class.Class == DocumentClassName {
			s.logger.Infof("Schema for class %s already exists, validating...", DocumentClassName)

			expectedProps := map[string]string{
				contentProperty:             "text",
				documentContentHashProperty: "text",
				documentTypeProperty:        "text",
				documentOriginalIDProperty:  "text",
				documentMetadataProperty:    "text",
				documentCreatedAtProperty:   "date",
			}

			existingProps := make(map[string]string)
			for _, prop := range class.Properties {
				for _, dt := range prop.DataType {
					existingProps[prop.Name] = dt
				}
			}

			for propName, propType := range expectedProps {
				if existingType, exists := existingProps[propName]; exists {
					if existingType != propType {
						return fmt.Errorf("property %s has type %s, expected %s", propName, existingType, propType)
					}
				} else {
					return fmt.Errorf("property %s is missing from document schema", propName)
				}
			}

			s.logger.Info("Document schema validation successful")
			return nil
		}
	}

	// Schema doesn't exist, create it
	s.logger.Infof("Creating schema for class %s", DocumentClassName)

	indexFilterable := true
	classObj := &models.Class{
		Class:       DocumentClassName,
		Description: "A document in the separate document storage system",
		Properties: []*models.Property{
			{
				Name:        contentProperty,
				DataType:    []string{"text"},
				Description: "The full content of the document",
			},
			{
				Name:            documentContentHashProperty,
				DataType:        []string{"text"},
				Description:     "SHA256 hash of the document content for deduplication",
				IndexFilterable: &indexFilterable,
			},
			{
				Name:            documentTypeProperty,
				DataType:        []string{"text"},
				Description:     "Type of the document (conversation, text, etc.)",
				IndexFilterable: &indexFilterable,
			},
			{
				Name:            documentOriginalIDProperty,
				DataType:        []string{"text"},
				Description:     "Original ID of the document from source",
				IndexFilterable: &indexFilterable,
			},
			{
				Name:        documentMetadataProperty,
				DataType:    []string{"text"},
				Description: "JSON-encoded metadata for the document",
			},
			{
				Name:            documentCreatedAtProperty,
				DataType:        []string{"date"},
				Description:     "When the document was stored",
				IndexFilterable: &indexFilterable,
			},
		},
		Vectorizer: "none",
	}

	err = s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		return fmt.Errorf("creating document schema: %w", err)
	}

	s.logger.Info("Document schema created successfully")
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

	if len(metadataFilters) > 0 {
		filterMap := metadataFilters[0]
		for key, value := range filterMap {
			if key == "type" {
				escapedKey := strings.ReplaceAll(key, `"`, `\"`)
				escapedKey = strings.ReplaceAll(escapedKey, `*`, `\*`)
				escapedValue := strings.ReplaceAll(value, `"`, `\"`)
				escapedValue = strings.ReplaceAll(escapedValue, `*`, `\*`)
				escapedValue = strings.ReplaceAll(escapedValue, `\`, `\\`)

				queryBuilder = queryBuilder.WithWhere(filters.Where().
					WithPath([]string{metadataProperty}).
					WithOperator(filters.Like).
					WithValueText(fmt.Sprintf(`*"%s":"%s"*`, escapedKey, escapedValue)))

				s.logger.Debug("Added WHERE filter", "key", key, "value", value, "pattern", fmt.Sprintf(`*"%s":"%s"*`, key, escapedValue))
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

// GetStoredDocumentsBatch retrieves multiple stored documents in a single query.
func (s *WeaviateStorage) GetStoredDocumentsBatch(ctx context.Context, documentIDs []string) ([]*StoredDocument, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}

	// Build GraphQL query with WHERE filter for multiple IDs
	fields := []weaviateGraphql.Field{
		{Name: contentProperty},
		{Name: documentContentHashProperty},
		{Name: documentTypeProperty},
		{Name: documentOriginalIDProperty},
		{Name: documentMetadataProperty},
		{Name: documentCreatedAtProperty},
		{Name: "_additional", Fields: []weaviateGraphql.Field{{Name: "id"}}},
	}

	whereFilter := filters.Where().
		WithPath([]string{"id"}).
		WithOperator(filters.ContainsAny).
		WithValueText(documentIDs...)

	result, err := s.client.GraphQL().Get().
		WithClassName(DocumentClassName).
		WithFields(fields...).
		WithWhere(whereFilter).
		WithLimit(len(documentIDs)).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("batch query for documents: %w", err)
	}

	if len(result.Errors) > 0 {
		var errorMsgs []string
		for _, err := range result.Errors {
			errorMsgs = append(errorMsgs, err.Message)
		}
		return nil, fmt.Errorf("GraphQL query errors: %s", strings.Join(errorMsgs, "; "))
	}

	var allDocuments []*StoredDocument
	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return allDocuments, nil
	}

	classData, ok := data[DocumentClassName].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", DocumentClassName)
		return allDocuments, nil
	}

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		additional, _ := obj["_additional"].(map[string]interface{})
		id, _ := additional["id"].(string)

		content, _ := obj[contentProperty].(string)
		contentHash, _ := obj[documentContentHashProperty].(string)
		docType, _ := obj[documentTypeProperty].(string)
		originalID, _ := obj[documentOriginalIDProperty].(string)
		metadataJSON, _ := obj[documentMetadataProperty].(string)
		createdAt, _ := obj[documentCreatedAtProperty].(string)

		var metadata map[string]string
		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
				s.logger.Warnf("Failed to unmarshal metadata for document %s: %v", id, err)
				metadata = make(map[string]string)
			}
		} else {
			metadata = make(map[string]string)
		}

		parsedCreatedAt, _ := time.Parse(time.RFC3339, createdAt)

		doc := &StoredDocument{
			ID:          id,
			Content:     content,
			Type:        docType,
			OriginalID:  originalID,
			ContentHash: contentHash,
			Metadata:    metadata,
			CreatedAt:   parsedCreatedAt,
		}

		allDocuments = append(allDocuments, doc)
	}

	// Log if some documents were not found
	if len(allDocuments) < len(documentIDs) {
		foundIDs := make(map[string]bool)
		for _, doc := range allDocuments {
			foundIDs[doc.ID] = true
		}

		var missingIDs []string
		for _, id := range documentIDs {
			if !foundIDs[id] {
				missingIDs = append(missingIDs, id)
			}
		}
		s.logger.Warn("Some documents were not found", "missing_ids", missingIDs)
	}

	return allDocuments, nil
}

// GetDocumentReferences retrieves all document references for a memory.
func (s *WeaviateStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error) {
	s.logger.Info("Getting document references", "memory_id", memoryID)

	memory, err := s.GetByID(ctx, memoryID)
	if err != nil {
		return nil, fmt.Errorf("getting memory object: %w", err)
	}

	var documentIDs []string
	if refs, ok := memory.Metadata()[documentReferencesProperty]; ok {
		if err := json.Unmarshal([]byte(refs), &documentIDs); err != nil {
			s.logger.Warn("Failed to unmarshal document references", "error", err)
		} else {
			s.logger.Debug("Found document references in new format", "count", len(documentIDs))
		}
	}

	if len(documentIDs) == 0 {
		if oldDocID, ok := memory.Metadata()["sourceDocumentId"]; ok && oldDocID != "" {
			s.logger.Debug("Found document reference in old format", "doc_id", oldDocID)

			docType := memory.Metadata()["sourceDocumentType"]
			if docType == "" {
				docType = "unknown"
			}

			return []*DocumentReference{
				{
					ID:      oldDocID,
					Content: "",
					Type:    docType,
				},
			}, nil
		}
	}

	if len(documentIDs) == 0 {
		s.logger.Info("No document references found", "memory_id", memoryID)
		return nil, nil
	}

	s.logger.Debug("Retrieving documents from new storage system", "count", len(documentIDs))

	storedDocs, err := s.GetStoredDocumentsBatch(ctx, documentIDs)
	if err != nil {
		return nil, fmt.Errorf("getting stored documents batch: %w", err)
	}

	var refs []*DocumentReference
	for _, doc := range storedDocs {
		refs = append(refs, &DocumentReference{
			ID:      doc.ID,
			Content: doc.Content,
			Type:    doc.Type,
		})
	}

	s.logger.Info("Retrieved document references successfully",
		"memory_id", memoryID,
		"requested_count", len(documentIDs),
		"found_count", len(refs))

	return refs, nil
}

// StoreDocument stores a document in the separate document storage system.
// It returns the document ID if successful.
func (s *WeaviateStorage) StoreDocument(ctx context.Context, content, docType, originalID string, metadata map[string]string) (string, error) {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	contentHash := hex.EncodeToString(hasher.Sum(nil))

	whereFilter := filters.Where().
		WithPath([]string{documentContentHashProperty}).
		WithOperator(filters.Equal).
		WithValueText(contentHash)

	fields := []weaviateGraphql.Field{
		{Name: contentProperty},
		{Name: documentContentHashProperty},
		{Name: "_additional", Fields: []weaviateGraphql.Field{{Name: "id"}}},
	}

	result, err := s.client.GraphQL().Get().
		WithClassName(DocumentClassName).
		WithFields(fields...).
		WithWhere(whereFilter).
		Do(ctx)
	if err != nil {
		return "", fmt.Errorf("checking for existing document: %w", err)
	}

	if data, ok := result.Data["Get"].(map[string]interface{}); ok {
		if docs, ok := data[DocumentClassName].([]interface{}); ok && len(docs) > 0 {
			for _, doc := range docs {
				if docMap, ok := doc.(map[string]interface{}); ok {
					if existingContent, ok := docMap[contentProperty].(string); ok {
						if existingContent == content {
							if additional, ok := docMap["_additional"].(map[string]interface{}); ok {
								if id, ok := additional["id"].(string); ok {
									s.logger.Debug("Found existing document with same content", "id", id)
									return id, nil
								}
							}
						}
					}
				}
			}
		}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}

	properties := map[string]interface{}{
		contentProperty:             content,
		documentContentHashProperty: contentHash,
		documentTypeProperty:        docType,
		documentOriginalIDProperty:  originalID,
		documentMetadataProperty:    string(metadataJSON),
		documentCreatedAtProperty:   time.Now().Format(time.RFC3339),
	}

	batcher := s.client.Batch().ObjectsBatcher()
	obj := &models.Object{
		Class:      DocumentClassName,
		Properties: properties,
	}
	batcher = batcher.WithObjects(obj)

	batchResult, err := batcher.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("creating document: %w", err)
	}

	if len(batchResult) == 0 {
		return "", fmt.Errorf("no result returned from document creation")
	}

	if batchResult[0].Result.Errors != nil && len(batchResult[0].Result.Errors.Error) > 0 {
		return "", fmt.Errorf("error creating document: %s", batchResult[0].Result.Errors.Error[0].Message)
	}

	id := string(batchResult[0].ID)
	s.logger.Debug("Stored new document", "id", id)
	return id, nil
}

// GetStoredDocument retrieves a document from the separate document table.
func (s *WeaviateStorage) GetStoredDocument(ctx context.Context, documentID string) (*StoredDocument, error) {
	result, err := s.client.Data().ObjectsGetter().
		WithID(documentID).
		WithClassName(DocumentClassName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting object by ID: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no object found with ID %s", documentID)
	}

	obj := result[0]
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid properties type for object %s", documentID)
	}

	content, _ := props[contentProperty].(string)
	contentHash, _ := props[documentContentHashProperty].(string)
	docType, _ := props[documentTypeProperty].(string)
	originalID, _ := props[documentOriginalIDProperty].(string)
	metadataJSON, _ := props[documentMetadataProperty].(string)
	createdAt, _ := props[documentCreatedAtProperty].(string)

	var metadata map[string]string
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			s.logger.Warnf("Failed to unmarshal metadata for document %s: %v", documentID, err)
			metadata = make(map[string]string)
		}
	} else {
		metadata = make(map[string]string)
	}

	parsedCreatedAt, _ := time.Parse(time.RFC3339, createdAt)

	doc := &StoredDocument{
		ID:          documentID,
		Content:     content,
		Type:        docType,
		OriginalID:  originalID,
		ContentHash: contentHash,
		Metadata:    metadata,
		CreatedAt:   parsedCreatedAt,
	}

	return doc, nil
}
