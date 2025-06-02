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

// Document types
type DocumentType string

const (
	DocumentTypeConversation DocumentType = "conversation"
	DocumentTypeText         DocumentType = "text"
)

// Configuration for the storage system
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

// Document preparation
type PreparedDocument struct {
	Original   memory.Document
	Type       DocumentType
	SpeakerID  string
	Timestamp  time.Time
	DateString string // Pre-formatted
}

// Processing pipeline types
type ExtractedFact struct {
	Content   string
	SpeakerID string
	Source    PreparedDocument
}

// Memory actions
type MemoryAction string

const (
	// Using existing constants from above
	ADD    MemoryAction = AddMemoryToolName
	UPDATE MemoryAction = UpdateMemoryToolName
	DELETE MemoryAction = DeleteMemoryToolName
	NONE   MemoryAction = NoneMemoryToolName
)

// Memory decision from LLM
type MemoryDecision struct {
	Action     MemoryAction
	TargetID   string // For UPDATE/DELETE
	Reason     string
	Confidence float64
}

// Processing result
type FactResult struct {
	Fact     ExtractedFact
	Decision MemoryDecision
	Object   *models.Object // Only for ADD
	Error    error
}

// Document result
type DocumentResult struct {
	DocumentID string
	Facts      []FactResult
	Error      error
}

// Progress reporting
type Progress struct {
	Processed int
	Total     int
	Stage     string
	Error     error
}

// Memory search results
type ExistingMemory struct {
	ID        string
	Content   string
	Timestamp time.Time
	Score     float64
	Metadata  map[string]string // Contains speakerID if present
}

// Validation rules
type ValidationRule struct {
	CurrentSpeakerID string // Who's processing this fact
	IsDocumentLevel  bool   // Is current context document-level?
	TargetMemoryID   string // For UPDATE/DELETE
	TargetSpeakerID  string // Speaker of target memory
	Action           MemoryAction
}

// Interfaces for IO boundaries
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

func EnsureSchemaExistsInternal(client *weaviate.Client, logger *log.Logger) error {
	ctx := context.Background()
	exists, err := client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence for '%s': %w", ClassName, err)
	}
	if exists {
		logger.Debugf("Class '%s' already exists.", ClassName)
		return nil
	}

	logger.Infof("Class '%s' does not exist, creating it now.", ClassName)
	properties := []*models.Property{
		{
			Name:     contentProperty,
			DataType: []string{"text"},
		},
		{
			Name:     timestampProperty, // Added for storing the event timestamp of the memory
			DataType: []string{"date"},
		},
		{
			Name:     tagsProperty,       // For categorization or keyword tagging
			DataType: []string{"text[]"}, // Array of strings
		},
		{
			Name:     metadataProperty, // For any other structured metadata
			DataType: []string{"text"}, // Storing as JSON string
		},
	}

	classObj := &models.Class{
		Class:      ClassName,
		Properties: properties,
		Vectorizer: "none",
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	err = client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		existsAfterAttempt, checkErr := client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
		if checkErr == nil && existsAfterAttempt {
			logger.Info("Class was created concurrently. Proceeding.", "class", ClassName)
			return nil
		}
		return fmt.Errorf("creating class '%s': %w. Original error: %v", ClassName, err, err)
	}
	logger.Infof("Successfully created class '%s'", ClassName)
	return nil
}

// firstNChars is a helper to get the first N characters of a string for logging.
func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// GetByID retrieves a document by its Weaviate ID.
// speakerID (if present) will be within the Metadata map after unmarshalling metadataJson.
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	s.logger.Debugf("Attempting to get document by ID: %s", id)

	result, err := s.client.Data().ObjectsGetter().
		WithClassName(ClassName).
		WithID(id).
		// No WithAdditionalParameters needed for ObjectsGetter for standard properties
		Do(ctx)
	if err != nil {
		// Weaviate client might return a specific error for not found (e.g., status code 404 in the error details)
		// For now, returning the generic error. Could inspect err for specific handling of "not found".
		return nil, fmt.Errorf("getting document by ID '%s': %w", id, err)
	}

	if len(result) == 0 {
		s.logger.Warnf("No document found with ID: %s (empty result array)", id)
		return nil, nil // Or an error like fmt.Errorf("document with ID '%s' not found", id)
	}

	obj := result[0]
	if obj.Properties == nil {
		return nil, fmt.Errorf("document with ID '%s' has nil properties", id)
	}

	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast properties to map[string]interface{} for ID '%s'", id)
	}

	content, _ := props[contentProperty].(string)
	docTimestampStr, _ := props[timestampProperty].(string)
	tagsInterface, _ := props[tagsProperty].([]interface{})
	metadataJSON, _ := props[metadataProperty].(string) // This string contains all metadata, including speakerID

	var docTimestampP *time.Time
	if docTimestampStr != "" {
		parsedTime, pErr := time.Parse(time.RFC3339, docTimestampStr)
		if pErr != nil {
			s.logger.Warnf("Failed to parse timestamp for document ID '%s': %v. Setting to nil.", id, pErr)
		} else {
			docTimestampP = &parsedTime
		}
	}

	var tags []string
	for _, tagInterface := range tagsInterface {
		if tagStr, okT := tagInterface.(string); okT {
			tags = append(tags, tagStr)
		}
	}

	metadataMap := make(map[string]string) // This will hold all metadata, including speakerID if it was stored
	if metadataJSON != "" {
		if errJson := json.Unmarshal([]byte(metadataJSON), &metadataMap); errJson != nil {
			s.logger.Warnf("Failed to unmarshal metadataJson for document ID '%s': %v. Metadata will be empty.", id, errJson)
			// metadataMap will remain empty or partially filled if unmarshalling failed mid-way (unlikely for simple map[string]string)
		}
	}

	doc := &memory.TextDocument{
		FieldID:        obj.ID.String(), // Use the ID from Weaviate's object, converting to string
		FieldContent:   content,
		FieldTimestamp: docTimestampP,
		FieldTags:      tags,
		FieldMetadata:  metadataMap, // speakerID is now part of this map if it was stored
	}

	s.logger.Debugf("Successfully retrieved document by ID: %s. speakerID from metadata: '%s'", id, metadataMap["speakerID"])
	return doc, nil
}

// Update updates an existing document in Weaviate.
// speakerID (if present) is expected to be within doc.Metadata, which is marshaled to metadataJson.
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	s.logger.Debugf("Attempting to update document ID: %s", id)

	data := map[string]interface{}{
		contentProperty: doc.FieldContent,
	}

	if doc.FieldTimestamp != nil {
		data[timestampProperty] = doc.FieldTimestamp.Format(time.RFC3339)
	}
	if len(doc.FieldTags) > 0 {
		data[tagsProperty] = doc.FieldTags
	} else {
		data[tagsProperty] = []string{} // Explicitly clear tags if doc.Tags is empty
	}

	// All metadata, including speakerID, is expected to be in doc.Metadata.
	// This map will be marshaled into the metadataJson field.
	if len(doc.FieldMetadata) > 0 {
		metadataBytes, err := json.Marshal(doc.FieldMetadata)
		if err != nil {
			s.logger.Errorf("Failed to marshal metadata for document ID '%s': %v", id, err)
			return fmt.Errorf("marshaling metadata for update: %w", err)
		}
		data[metadataProperty] = string(metadataBytes)
		s.logger.Debugf("Updating doc %s, speakerID in marshaled metadata: '%s'", id, doc.FieldMetadata["speakerID"])
	} else {
		data[metadataProperty] = "{}" // Store an empty JSON object string if no metadata
		s.logger.Debugf("Updating doc %s with empty metadataJson", id)
	}

	updater := s.client.Data().Updater().
		WithClassName(ClassName).
		WithID(id).
		WithProperties(data)

	if len(vector) > 0 {
		updater = updater.WithVector(vector)
	}

	err := updater.Do(ctx)
	if err != nil {
		return fmt.Errorf("updating document ID '%s': %w", id, err)
	}

	s.logger.Infof("Successfully updated document ID: %s", id)
	return nil
}

// Delete removes a document from Weaviate by its ID.
func (s *WeaviateStorage) Delete(ctx context.Context, id string) error {
	err := s.client.Data().Deleter().
		WithClassName(ClassName).
		WithID(id).
		Do(ctx)
	if err != nil {
		// Check if the error is because the object was not found. Often, delete is idempotent.
		// For now, we just return the error. Specific error handling (e.g., for 404) can be added if needed.
		return fmt.Errorf("failed to delete object %s: %w", id, err)
	}

	s.logger.Info("Successfully deleted document by ID (or it was already gone)", "id", id)
	return nil
}

// DeleteAll deletes the entire Weaviate class to ensure a clean state for testing.
func (s *WeaviateStorage) DeleteAll(ctx context.Context) error {
	s.logger.Warn("Attempting to DELETE ENTIRE CLASS for testing purposes.", "class", ClassName)

	// Check if class exists before trying to delete
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
		// It's possible the class was deleted by another process between the check and here.
		// Or a genuine error occurred.
		// Check existence again to be sure.
		existsAfterAttempt, checkErr := s.client.Schema().ClassExistenceChecker().WithClassName(ClassName).Do(ctx)
		if checkErr == nil && !existsAfterAttempt {
			s.logger.Info("Class was deleted, possibly concurrently or by this attempt despite error.", "class", ClassName)
			return nil // Treat as success if it's gone
		}
		return fmt.Errorf("failed to delete class '%s': %w. Initial error: %v", ClassName, err, err)
	}
	s.logger.Info("Successfully deleted class for testing.", "class", ClassName)
	// The schema will be recreated on the next operation that requires it via ensureSchemaExistsInternal.
	return nil
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
