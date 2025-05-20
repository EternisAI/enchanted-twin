package newmem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"weaviate-go-server/pkg/ai"
	"weaviate-go-server/pkg/memory"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	className         = "TextDocumentStoreBYOV"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	tagsProperty      = "tags"
	metadataProperty  = "metadataJson"
	openAIEmbedModel  = "text-embedding-3-small"
	openAIChatModel   = "gpt-4o-mini"
)

// --- Structs for Tool Call Arguments ---

// AddToolArguments is currently empty as per tools.go definition
// type AddToolArguments struct {}

// UpdateToolArguments matches the parameters defined in updateMemoryTool in tools.go
type UpdateToolArguments struct {
	MemoryID      string `json:"id"`
	UpdatedMemory string `json:"updated_content"`
	Reason        string `json:"reason,omitempty"`
}

// DeleteToolArguments matches the parameters defined in deleteMemoryTool in tools.go
type DeleteToolArguments struct {
	MemoryID string `json:"id"`
	Reason   string `json:"reason,omitempty"`
}

// NoneToolArguments matches the parameters defined in noneMemoryTool in tools.go
type NoneToolArguments struct {
	Reason string `json:"reason"`
}

// WeaviateStorage implements the memory.Storage interface using Weaviate.
type WeaviateStorage struct {
	client    *weaviate.Client
	logger    *log.Logger
	aiService *ai.Service
}

// New creates a new WeaviateStorage instance.
// weaviateHost should be like "localhost:8081".
// weaviateScheme is "http" or "https".
// The logger is used for logging messages.
// The aiService is used for generating embeddings.
func New(weaviateHost string, weaviateScheme string, logger *log.Logger, aiService *ai.Service) (*WeaviateStorage, error) {
	if logger == nil {
		// Default charmbracelet logger if none provided
		logger = log.NewWithOptions(os.Stderr, log.Options{
			Prefix: "[WeaviateStorageDefault] ",
			Level:  log.DebugLevel,
		})
	}
	if aiService == nil {
		return nil, fmt.Errorf("ai.Service cannot be nil")
	}

	cfg := weaviate.Config{
		Host:   weaviateHost,
		Scheme: weaviateScheme,
	}
	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create weaviate client: %w", err)
	}

	storage := &WeaviateStorage{
		client:    client,
		logger:    logger,
		aiService: aiService,
	}

	if err := storage.ensureSchemaExistsInternal(context.Background()); err != nil {
		storage.logger.Warn("Failed to ensure schema during New(), will attempt on first operation.", "error", err)
	}
	return storage, nil
}

func (s *WeaviateStorage) ensureSchemaExistsInternal(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence for '%s': %w", className, err)
	}
	if exists {
		s.logger.Debugf("Class '%s' already exists.", className)
		return nil
	}

	s.logger.Infof("Class '%s' does not exist, creating it now.", className)
	classObj := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{Name: contentProperty, DataType: []string{"text"}},
			{Name: timestampProperty, DataType: []string{"date"}},
			{Name: tagsProperty, DataType: []string{"text[]"}},
			{Name: metadataProperty, DataType: []string{"text"}},
		},
		Vectorizer: "none",
	}

	err = s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		existsAfterAttempt, checkErr := s.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
		if checkErr == nil && existsAfterAttempt {
			s.logger.Info("Class was created concurrently. Proceeding.", "class", className)
			return nil
		}
		return fmt.Errorf("creating class '%s': %w. Original error: %v", className, err, err)
	}
	s.logger.Infof("Successfully created class '%s'", className)
	return nil
}

// Store method will be significantly refactored for intelligent memory updates.
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
	defer func() {
		if progressChan != nil {
			close(progressChan)
		}
	}()

	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return fmt.Errorf("pre-store schema check failed: %w", err)
	}

	batcher := s.client.Batch().ObjectsBatcher() // Create batcher once at the function level
	var objectsAddedToBatch int                  // Counter for objects added to the main batcher

	totalDocs := len(documents)
	if totalDocs == 0 {
		s.logger.Debug("No documents provided to store.")
		return nil
	}

	// Define the tools for the LLM call
	tools := []openai.ChatCompletionToolParam{
		addMemoryTool,
		updateMemoryTool,
		deleteMemoryTool,
		noneMemoryTool,
	}
	_ = tools // Keep tools variable used for now, will be used in LLM call later

	for i, doc := range documents {
		s.logger.Info("Processing document for intelligent storage", "index", i+1, "total", totalDocs, "docID_if_any", doc.ID, "content_snippet", firstNChars(doc.Content, 50))

		// 1. Embed the "New Fact" (current document content)
		newFactEmbedding64, err := s.aiService.Embedding(ctx, doc.Content, openAIEmbedModel)
		if err != nil {
			s.logger.Error("Error generating embedding for new fact, skipping document.", "docID", doc.ID, "index", i, "error", err)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: 1, Total: totalDocs} // Mark as processed (though skipped)
			}
			continue
		}
		newFactEmbedding32 := make([]float32, len(newFactEmbedding64))
		for j, val := range newFactEmbedding64 {
			newFactEmbedding32[j] = float32(val)
		}

		// 2. Retrieve Relevant Existing Memories for Context
		// We pass newFactEmbedding32 here
		retrievedDocsRaw, existingDocsJSONForLLM, tempIDToActualIDMap, err := s.searchExistingDocumentsByVector(ctx, newFactEmbedding32, 5) // Limit 5
		if err != nil {
			s.logger.Error("Error searching existing documents by vector, skipping document.", "docID", doc.ID, "index", i, "error", err)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: 1, Total: totalDocs}
			}
			continue
		}
		s.logger.Debug("Retrieved existing documents for context", "count", len(retrievedDocsRaw), "jsonForLLM_snippet", firstNChars(existingDocsJSONForLLM, 100))
		_ = tempIDToActualIDMap // Will be used when processing LLM response

		// 3. LLM-Powered Decision Making (using Tool Calling)
		// Construct the "new facts" JSON list (currently, just the single document content)
		newFactContent := doc.Content
		escapedNewFactContent, err := json.Marshal(newFactContent)
		if err != nil {
			s.logger.Error("Error marshalling new fact content to JSON string, skipping document.", "docID", doc.ID, "index", i, "error", err)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: 1, Total: totalDocs}
			}
			continue
		}
		newFactsJSONList := fmt.Sprintf("[%s]", string(escapedNewFactContent))

		// Construct the full system prompt
		systemPromptStr := DefaultUpdateMemoryPrompt +
			"\n\nBelow is the current content of my memory which I have collected till now. Please review it carefully.\n\nExisting Memories:\n" +
			"```json\n" + existingDocsJSONForLLM + "\n```" +
			"\n\nThe new retrieved facts are mentioned below. You have to analyze these new facts in the context of the existing memories and decide whether each new fact should be added, or if an existing memory should be updated or deleted.\n\nNew Facts:\n" +
			"```json\n" + newFactsJSONList + "\n```" +
			"\n\nUse the available tools to perform the appropriate memory operation (ADD, UPDATE, DELETE, or NONE) for each new fact or necessary memory adjustment. " +
			"When calling UPDATE or DELETE, you MUST provide the 'id' of the relevant memory from the 'Existing Memories' list as the 'memory_id' argument in your tool call. " +
			"If adding a new fact, the system will assign a new ID. Always provide a concise reason for your chosen action."

		s.logger.Debug("Constructed system prompt for LLM", "length", len(systemPromptStr), "prompt_snippet", firstNChars(systemPromptStr, 250))

		// Prepare messages for the LLM
		messages := []openai.ChatCompletionMessageParamUnion{
			{ // System Message
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(systemPromptStr),
					},
				},
			},
			{ // User Message
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String("Based on the new information and existing memories, decide on the necessary memory operations using the provided tools."),
					},
				},
			},
		}

		// Call s.aiService.Completions with the prompt and tools
		llmResponse, err := s.aiService.Completions(ctx, messages, tools, openAIChatModel)
		if err != nil {
			s.logger.Error("Error calling OpenAI Completions for memory decision, skipping document.", "docID", doc.ID, "index", i, "error", err)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: 1, Total: totalDocs}
			}
			continue
		}

		s.logger.Info("LLM response received", "docID", doc.ID, "content_llm", llmResponse.Content, "toolCalls_count", len(llmResponse.ToolCalls))

		// 4. Execute LLM-Instructed Actions
		if len(llmResponse.ToolCalls) == 0 {
			s.logger.Warn("LLM did not request any tool calls. Defaulting to ADD for current document.", "docID", doc.ID)
			// Fallback: If LLM provides no tool, add the new document as a default.
			data := map[string]interface{}{
				contentProperty:   doc.Content,
				timestampProperty: time.Now().Format(time.RFC3339),
				// Potentially add tags or metadata if available in doc
				// Add doc.Source, doc.SourceType etc. if they should be part of properties directly
				// if len(doc.Tags) > 0 { data[tagsProperty] = doc.Tags }
				// if len(doc.Metadata) > 0 { metaBytes, _ := json.Marshal(doc.Metadata); data[metadataProperty] = string(metaBytes) }
			}
			// Force Weaviate to generate the ID for new documents by passing empty string for ID.
			batcher.WithObjects(&models.Object{
				Class:      className,
				ID:         "", // Let Weaviate generate the UUID
				Properties: data,
				Vector:     newFactEmbedding32,
			})
			objectsAddedToBatch++
		} else {
			for _, toolCall := range llmResponse.ToolCalls {
				s.logger.Info("Processing tool call", "toolName", toolCall.Function.Name, "toolID", toolCall.ID)
				switch toolCall.Function.Name {
				case "ADD": // Use literal string name
					s.logger.Info("LLM requested ADD action.", "original_docID_if_any", doc.ID)
					data := map[string]interface{}{
						contentProperty:   doc.Content,
						timestampProperty: time.Now().Format(time.RFC3339),
						// TODO: Add Source, SourceType, Tags from original doc if desired into properties
						// If doc.Tags or doc.Metadata are available, consider adding them here:
						// if len(doc.Tags) > 0 { data[tagsProperty] = doc.Tags }
						// if len(doc.Metadata) > 0 { metaBytes, _ := json.Marshal(doc.Metadata); data[metadataProperty] = string(metaBytes) }
					}
					// Force Weaviate to generate the ID for new documents by passing empty string for ID.
					batcher.WithObjects(&models.Object{
						Class:      className,
						ID:         "", // Let Weaviate generate the UUID
						Properties: data,
						Vector:     newFactEmbedding32,
					})
					objectsAddedToBatch++
					s.logger.Info("ADD action prepared for batching. Weaviate will generate ID.", "original_docID_if_any", doc.ID)

				case "UPDATE": // Use literal string name
					var args UpdateToolArguments
					s.logger.Debug("Raw arguments for UPDATE tool call", "raw_args", toolCall.Function.Arguments) // Log the raw arguments
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						s.logger.Error("Error unmarshalling UPDATE arguments, skipping tool call.", "args_string", toolCall.Function.Arguments, "error", err)
						continue
					}
					s.logger.Info("LLM requested UPDATE action.", "parsed_tempMemoryID", args.MemoryID, "parsed_reason", args.Reason, "parsed_updatedMemory_snippet", firstNChars(args.UpdatedMemory, 50))

					actualID, ok := tempIDToActualIDMap[args.MemoryID]
					if !ok {
						s.logger.Error("Failed to find actual Weaviate ID for temporary ID from LLM.", "tempMemoryID", args.MemoryID)
						continue
					}

					updatedEmbedding64, err := s.aiService.Embedding(ctx, args.UpdatedMemory, openAIEmbedModel)
					if err != nil {
						s.logger.Error("Error generating embedding for updated content, skipping update.", "actualID", actualID, "error", err)
						continue
					}
					updatedEmbedding32 := make([]float32, len(updatedEmbedding64))
					for j, val := range updatedEmbedding64 {
						updatedEmbedding32[j] = float32(val)
					}

					originalDocToUpdate, err := s.GetByID(ctx, actualID)
					if err != nil {
						s.logger.Error("Failed to retrieve original document for update, skipping update.", "actualID", actualID, "error", err)
						continue
					}
					if originalDocToUpdate == nil {
						s.logger.Error("Original document for update not found (nil), skipping update.", "actualID", actualID)
						continue
					}

					currentTime := time.Now()
					updatedDoc := memory.TextDocument{
						ID:      actualID,
						Content: args.UpdatedMemory,
						// Source and SourceType will be preserved if they are within Metadata
						Timestamp: &currentTime,
						Metadata:  originalDocToUpdate.Metadata, // Preserve original metadata map
						Tags:      originalDocToUpdate.Tags,     // Preserve original tags
					}

					if err := s.Update(ctx, actualID, updatedDoc, updatedEmbedding32); err != nil {
						s.logger.Error("Failed to update document in Weaviate.", "actualID", actualID, "error", err)
					} else {
						s.logger.Info("UPDATE action successfully performed.", "actualID", actualID, "updatedContentSnippet", firstNChars(args.UpdatedMemory, 50))
					}

				case "DELETE": // Use literal string name
					var args DeleteToolArguments
					s.logger.Debug("Raw arguments for DELETE tool call", "raw_args", toolCall.Function.Arguments) // Log the raw arguments
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						s.logger.Error("Error unmarshalling DELETE arguments, skipping tool call.", "args_string", toolCall.Function.Arguments, "error", err)
						continue
					}
					s.logger.Info("LLM requested DELETE action.", "parsed_tempMemoryID", args.MemoryID, "parsed_reason", args.Reason)

					actualID, ok := tempIDToActualIDMap[args.MemoryID]
					if !ok {
						s.logger.Error("Failed to find actual Weaviate ID for temporary ID from LLM for delete.", "tempMemoryID", args.MemoryID)
						continue
					}

					if err := s.Delete(ctx, actualID); err != nil {
						s.logger.Error("Failed to delete document from Weaviate.", "actualID", actualID, "error", err)
					} else {
						s.logger.Info("DELETE action successfully performed.", "actualID", actualID)
					}

				case "NONE": // Use literal string name
					var args NoneToolArguments
					s.logger.Debug("Raw arguments for NONE tool call", "raw_args", toolCall.Function.Arguments) // Log the raw arguments
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						s.logger.Error("Error unmarshalling NONE arguments, skipping tool call.", "args_string", toolCall.Function.Arguments, "error", err)
						// Log intent even if reason cannot be parsed
					}
					s.logger.Info("LLM requested NO action.", "parsed_reason", args.Reason, "docID", doc.ID)

				default:
					s.logger.Warn("LLM requested an unknown tool.", "toolName", toolCall.Function.Name, "docID", doc.ID)
				}
			}
		}

		if progressChan != nil {
			progressChan <- memory.ProgressUpdate{Processed: 1, Total: totalDocs}
		}
	}

	// Flush any remaining objects in the batch after processing all documents
	if objectsAddedToBatch > 0 {
		s.logger.Info("Flushing batched objects to Weaviate...", "count", objectsAddedToBatch)
		batchResponse, err := batcher.Do(ctx)
		if err != nil {
			// Even if flushing fails, we should report progress for docs processed *before* this batch.
			// The overall function will return an error.
			return fmt.Errorf("flushing batch to Weaviate: %w", err)
		}
		var successfulFlushes, failedFlushes int
		for _, res := range batchResponse {
			if res.Result != nil && res.Result.Status != nil {
				if *res.Result.Status == "SUCCESS" {
					successfulFlushes++
				} else {
					failedFlushes++
					s.logger.Error("Failed to add object in batch", "id", res.ID, "status", *res.Result.Status, "errors", res.Result.Errors)
				}
			} else {
				failedFlushes++
				s.logger.Error("Failed to add object in batch, nil result or status", "id", res.ID)
			}
		}
		s.logger.Info("Batch flush completed.", "successful", successfulFlushes, "failed", failedFlushes)
		if failedFlushes > 0 {
			return fmt.Errorf("some objects failed to be added during batch flush (%d failures)", failedFlushes)
		}
	} else {
		s.logger.Info("No objects to flush in batch.")
	}

	s.logger.Info("All documents processed.")
	return nil
}

// firstNChars is a helper to get the first N characters of a string for logging.
func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Query searches documents in Weaviate using a query vector.
func (s *WeaviateStorage) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return memory.QueryResult{}, fmt.Errorf("pre-query schema check failed: %w", err)
	}

	queryVectorFloat64, err := s.aiService.Embedding(ctx, query, openAIEmbedModel)
	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to generate query vector: %w", err)
	}

	queryVectorFloat32 := make([]float32, len(queryVectorFloat64))
	for i, val := range queryVectorFloat64 {
		queryVectorFloat32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().
		NearVectorArgBuilder().
		WithVector(queryVectorFloat32)

	fields := []graphql.Field{
		{Name: contentProperty},
		{Name: timestampProperty},
		{Name: tagsProperty},
		{Name: metadataProperty},
		{Name: "_additional", Fields: []graphql.Field{{Name: "id"}, {Name: "distance"}}},
	}

	resp, err := s.client.GraphQL().Get().
		WithClassName(className).
		WithFields(fields...).
		WithNearVector(nearVector).
		WithLimit(10).
		Do(ctx)

	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute GraphQL query with nearVector: %w", err)
	}

	if resp.Errors != nil && len(resp.Errors) > 0 {
		return memory.QueryResult{}, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	var queryResult memory.QueryResult

	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		return memory.QueryResult{}, fmt.Errorf("unexpected GraphQL response data structure at Get level")
	}

	classData, ok := data[className].([]interface{})
	if !ok {
		s.logger.Debug("No results found or unexpected data type.", "class", className, "dataType", fmt.Sprintf("%T", data[className]))
		return queryResult, nil
	}

	for _, item := range classData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			s.logger.Warn("Skipping item, not a map", "itemType", fmt.Sprintf("%T", item))
			continue
		}

		var doc memory.TextDocument
		if content, ok := itemMap[contentProperty].(string); ok {
			doc.Content = content
			queryResult.Text = append(queryResult.Text, content)
		}

		if additional, ok := itemMap["_additional"].(map[string]interface{}); ok {
			if id, okId := additional["id"].(string); okId {
				doc.ID = id
			}
		}

		if tsStr, ok := itemMap[timestampProperty].(string); ok {
			if parsedTime, pErr := time.Parse(time.RFC3339, tsStr); pErr == nil {
				doc.Timestamp = &parsedTime
			} else {
				s.logger.Warn("Error parsing timestamp", "docID", doc.ID, "error", pErr)
			}
		}

		if tagsList, ok := itemMap[tagsProperty].([]interface{}); ok {
			for _, tagInterface := range tagsList {
				if tagStr, okT := tagInterface.(string); okT {
					doc.Tags = append(doc.Tags, tagStr)
				}
			}
		}

		if metaJSONStr, ok := itemMap[metadataProperty].(string); ok {
			var metadata map[string]string
			if errJson := json.Unmarshal([]byte(metaJSONStr), &metadata); errJson == nil {
				doc.Metadata = metadata
			} else {
				s.logger.Warn("Error unmarshalling metadata", "docID", doc.ID, "error", errJson)
			}
		}
		queryResult.Documents = append(queryResult.Documents, doc)
	}
	s.logger.Infof("Query processed. Found %d documents.", len(queryResult.Documents))
	return queryResult, nil
}

// searchExistingDocumentsByVector is an internal helper to find documents
// in Weaviate similar to a given vector. It returns the raw documents,
// a JSON string of these documents formatted for the LLM prompt (with temporary IDs),
// and a map to translate temporary IDs back to actual Weaviate UUIDs.
func (s *WeaviateStorage) searchExistingDocumentsByVector(ctx context.Context, vector []float32, limit int) ([]memory.TextDocument, string, map[string]string, error) {
	s.logger.Debug("searchExistingDocumentsByVector called", "limit", limit)
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return nil, "", nil, fmt.Errorf("pre-searchExistingDocs schema check failed: %w", err)
	}

	nearVector := s.client.GraphQL().
		NearVectorArgBuilder().
		WithVector(vector)

	// Define fields required for the LLM context and for constructing TextDocument
	fields := []graphql.Field{
		{Name: contentProperty},
		// {Name: timestampProperty}, // May not be needed for LLM decision context, but could be useful
		// {Name: tagsProperty},      // Same as above
		// {Name: metadataProperty},  // Same as above
		{Name: "_additional", Fields: []graphql.Field{{Name: "id"}}}, // Distance might be useful for logging/debugging
	}

	resp, err := s.client.GraphQL().Get().
		WithClassName(className).
		WithFields(fields...).
		WithNearVector(nearVector).
		WithLimit(limit).
		Do(ctx)

	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to execute GraphQL query in searchExistingDocumentsByVector: %w", err)
	}

	if resp.Errors != nil && len(resp.Errors) > 0 {
		return nil, "", nil, fmt.Errorf("GraphQL query errors in searchExistingDocumentsByVector: %v", resp.Errors)
	}

	var retrievedDocs []memory.TextDocument
	var docsForPrompt []map[string]interface{} // For building the JSON string for the LLM
	tempIDToActualID := make(map[string]string)

	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		return nil, "", nil, fmt.Errorf("unexpected GraphQL response data structure at Get level (searchExisting)")
	}

	classData, ok := data[className].([]interface{})
	if !ok {
		s.logger.Debug("No results found or unexpected data type in searchExistingDocumentsByVector", "class", className, "dataType", fmt.Sprintf("%T", data[className]))
		return retrievedDocs, "[]", tempIDToActualID, nil // Return empty results, not an error
	}

	for i, item := range classData {
		itemMap, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Skipping item in searchExistingDocumentsByVector, not a map", "itemType", fmt.Sprintf("%T", item))
			continue
		}

		var doc memory.TextDocument
		actualID := ""

		if additional, okAdd := itemMap["_additional"].(map[string]interface{}); okAdd {
			if idVal, okID := additional["id"].(string); okID {
				doc.ID = idVal
				actualID = idVal
			}
		}

		if actualID == "" {
			s.logger.Warn("Skipping item in searchExistingDocumentsByVector, missing actual ID")
			continue
		}

		tempID := fmt.Sprintf("%d", i) // Simple temporary ID: "0", "1", "2", ...
		tempIDToActualID[tempID] = actualID

		docForPrompt := map[string]interface{}{
			"id":   tempID, // Use temporary ID for the prompt
			"text": "",     // Initialize text
		}

		if content, okContent := itemMap[contentProperty].(string); okContent {
			doc.Content = content
			docForPrompt["text"] = content
		}

		// We are only fetching minimal fields for the prompt (id, text) for now.
		// If more fields (timestamp, tags, metadata) were fetched, they'd be parsed here into 'doc'.

		retrievedDocs = append(retrievedDocs, doc)
		docsForPrompt = append(docsForPrompt, docForPrompt)
	}

	jsonBytes, err := json.Marshal(docsForPrompt)
	if err != nil {
		return retrievedDocs, "", tempIDToActualID, fmt.Errorf("failed to marshal docsForPrompt to JSON: %w", err)
	}

	s.logger.Debug("searchExistingDocumentsByVector completed", "retrievedCount", len(retrievedDocs), "promptJsonLength", len(jsonBytes))
	return retrievedDocs, string(jsonBytes), tempIDToActualID, nil
}

// GetByID retrieves a document from Weaviate by its ID.
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	s.logger.Debug("GetByID called", "id", id)
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return nil, fmt.Errorf("pre-getbyid schema check failed: %w", err)
	}

	obj, err := s.client.Data().ObjectsGetter().
		WithClassName(className).
		WithID(id).
		Do(ctx)

	if err != nil {
		// Weaviate client returns an error if the object is not found, which includes a 404 status code.
		// We can check for this specifically if we want to return a custom "not found" error vs. other errors.
		// For now, just return the error as is.
		return nil, fmt.Errorf("failed to get object %s: %w", id, err)
	}

	if len(obj) == 0 {
		// This case should ideally be covered by the error above from Weaviate if ID doesn't exist.
		return nil, fmt.Errorf("object %s not found (no error from client, but empty result)", id) // Or a more specific typed error
	}

	// Assuming obj[0] is our object since we queried by unique ID
	properties, ok := obj[0].Properties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast properties for object %s", id)
	}

	doc := memory.TextDocument{ID: obj[0].ID.String()} // Use the ID returned by Weaviate

	if content, ok := properties[contentProperty].(string); ok {
		doc.Content = content
	}

	if tsStr, ok := properties[timestampProperty].(string); ok {
		if parsedTime, pErr := time.Parse(time.RFC3339, tsStr); pErr == nil {
			doc.Timestamp = &parsedTime
		} else {
			s.logger.Warn("Error parsing timestamp during GetByID", "docID", doc.ID, "error", pErr)
		}
	}

	if tagsList, ok := properties[tagsProperty].([]interface{}); ok {
		for _, tagInterface := range tagsList {
			if tagStr, okT := tagInterface.(string); okT {
				doc.Tags = append(doc.Tags, tagStr)
			}
		}
	}

	if metaJSONStr, ok := properties[metadataProperty].(string); ok {
		var metadata map[string]string
		if errJson := json.Unmarshal([]byte(metaJSONStr), &metadata); errJson == nil {
			doc.Metadata = metadata
		} else {
			s.logger.Warn("Error unmarshalling metadata during GetByID", "docID", doc.ID, "error", errJson)
		}
	}

	s.logger.Info("Successfully retrieved document by ID", "id", id)
	return &doc, nil
}

// Update modifies an existing document in Weaviate.
// It will replace the document with the given ID with the new doc content, metadata, and vector.
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	s.logger.Debug("Update called", "id", id)
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return fmt.Errorf("pre-update schema check failed: %w", err)
	}

	properties := map[string]interface{}{
		contentProperty: doc.Content,
	}
	if doc.Timestamp != nil {
		properties[timestampProperty] = doc.Timestamp.Format(time.RFC3339)
	}
	if len(doc.Tags) > 0 {
		properties[tagsProperty] = doc.Tags
	} else { // Ensure tags are explicitly set to empty if cleared, to overwrite previous tags
		properties[tagsProperty] = []string{}
	}

	if len(doc.Metadata) > 0 {
		metaJSON, errJson := json.Marshal(doc.Metadata)
		if errJson != nil {
			// Decide if this is a fatal error for the update or just a warning.
			// For now, let's make it a warning and proceed without metadata if marshalling fails.
			s.logger.Warn("Error marshalling metadata for update, proceeding without it.", "docID", id, "error", errJson)
		} else {
			properties[metadataProperty] = string(metaJSON)
		}
	} else { // Ensure metadata is explicitly set if cleared
		// Weaviate might treat null differently from an empty JSON string for text fields.
		// Sending an empty map marshalled might be appropriate, or explicitly nullifying.
		// For simplicity, if metadata is empty, we can marshal an empty map.
		emptyMetaJson, _ := json.Marshal(map[string]string{})
		properties[metadataProperty] = string(emptyMetaJson)
	}

	updater := s.client.Data().Updater().
		WithClassName(className).
		WithID(id).
		WithProperties(properties).
		WithVector(vector) // Set the new vector
	// WithMerge(false) is the default and means it's a PUT (replace)

	err := updater.Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to update object %s: %w", id, err)
	}

	s.logger.Info("Successfully updated document by ID", "id", id)
	return nil
}

// Delete removes a document from Weaviate by its ID.
func (s *WeaviateStorage) Delete(ctx context.Context, id string) error {
	s.logger.Debug("Delete called", "id", id)
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return fmt.Errorf("pre-delete schema check failed: %w", err)
	}

	err := s.client.Data().Deleter().
		WithClassName(className).
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
	s.logger.Warn("Attempting to DELETE ENTIRE CLASS for testing purposes.", "class", className)

	// Check if class exists before trying to delete
	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking class existence before delete all for '%s': %w", className, err)
	}
	if !exists {
		s.logger.Info("Class does not exist, no need to delete.", "class", className)
		return nil
	}

	err = s.client.Schema().ClassDeleter().WithClassName(className).Do(ctx)
	if err != nil {
		// It's possible the class was deleted by another process between the check and here.
		// Or a genuine error occurred.
		// Check existence again to be sure.
		existsAfterAttempt, checkErr := s.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
		if checkErr == nil && !existsAfterAttempt {
			s.logger.Info("Class was deleted, possibly concurrently or by this attempt despite error.", "class", className)
			return nil // Treat as success if it's gone
		}
		return fmt.Errorf("failed to delete class '%s': %w. Initial error: %v", className, err, err)
	}
	s.logger.Info("Successfully deleted class for testing.", "class", className)
	// The schema will be recreated on the next operation that requires it via ensureSchemaExistsInternal.
	return nil
}
