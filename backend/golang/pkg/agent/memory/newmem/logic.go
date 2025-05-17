package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync" // Added for managing goroutines
	"time"

	"mem-zero/pkg/memory"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

// --- Prompts and Structs for LLM Interactions ---

// FactExtractionSystemPrompt defines the system role for fact extraction.
const FactExtractionSystemPrompt = `You are an expert AI tasked with extracting atomic facts from a given text. 
Each fact should be a concise, self-contained piece of information. 
For example, if the text is "User A is in San Francisco and likes ice cream", the facts are "User A is in San Francisco" and "User A likes ice cream".
If the text is a question, extract the core statement or assumption if possible, or state that it's a question about a topic.
Respond with a JSON list of strings, where each string is a fact. Example: ["fact 1", "fact 2", "fact 3"]
If there are no clear facts to extract (e.g., the text is too short, a greeting, or nonsensical), return an empty list [].`

// FactExtractionUserPromptTemplate is the template for the user message for fact extraction.
const FactExtractionUserPromptTemplate = "Extract all distinct, atomic facts from the following text:\n\n%s"

// OpenAIRequestTimeout defines the timeout for individual OpenAI API calls.
// const OpenAIRequestTimeout = 60 * time.Second // 60 seconds timeout for OpenAI calls - Replaced by llmTimeout in struct
const MaxRetries = 2               // Max retries for OpenAI calls
const RetryDelay = 1 * time.Second // Delay between retries

// FactsExtractionResponse is used to unmarshal the LLM's response for fact extraction.
type FactsExtractionResponse struct {
	Facts []string `json:"facts"`
}

// LLMDecision outlines the structure for the LLM's consolidation decision.
type LLMDecision struct {
	Action         string `json:"action"`
	IDToUpdate     string `json:"id_to_update,omitempty"`    // For UPDATE
	UpdatedContent string `json:"updated_content,omitempty"` // For UPDATE or ADD
	IDToDelete     string `json:"id_to_delete,omitempty"`    // For DELETE
}

// ConsolidationSystemPrompt defines the system role for the consolidation LLM.
const ConsolidationSystemPrompt = `You are an AI assistant helping to manage a memory system. 
Your task is to decide how a new piece of information (a "new fact") should be consolidated with existing relevant memories. 

New Fact:
{new_fact}

Existing Relevant Memories (if any):
{relevant_memories}

Based on the new fact and existing memories, choose one of the following actions:
- ADD: If the new fact is entirely new and distinct from existing memories.
- UPDATE: If the new fact provides additional details or an update to an existing memory. If you choose UPDATE, you MUST provide the ID of the memory to update and the updated content.
- DELETE: If the new fact explicitly invalidates or marks an existing memory as obsolete. If you choose DELETE, you MUST provide the ID of the memory to delete.
- NONE: If the new fact is a duplicate of an existing memory, or if it's too vague or doesn't add significant value compared to existing memories.

Respond with a JSON object containing the "action" (ADD, UPDATE, DELETE, NONE). 
If UPDATE, also include "id_to_update" and "updated_content". 
If DELETE, also include "id_to_delete".

Example for UPDATE:
{"action": "UPDATE", "id_to_update": "existing_memory_id_123", "updated_content": "Updated version of the memory with new details from the fact."}

Example for ADD:
{"action": "ADD"}

Example for NONE:
{"action": "NONE"}

Example for DELETE:
{"action": "DELETE", "id_to_delete": "existing_memory_id_456"}

Consider the timestamp and specificity of the information. A newer, more specific fact might supersede an older, vaguer one.`

// ConsolidationUserPromptTemplate is the template for the user message for the consolidation LLM.
const ConsolidationUserPromptTemplate = `New Information:
"%s"

Relevant Existing Memories (if any):
%s

Based on the new information and existing memories, what is your decision (ADD, UPDATE, DELETE, NONE) and the necessary details (id, content)?
Respond with a JSON object.`

// --- WeaviateMemoryStore Implementation ---

// WeaviateMemoryStore implements the memory.Storage interface using Weaviate and OpenAI.
type WeaviateMemoryStore struct {
	client       *weaviate.Client
	openaiClient *openai.Client
	logger       *log.Logger
	className    string
	storeTimeout time.Duration
	queryTimeout time.Duration
	llmTimeout   time.Duration
}

// NewWeaviateMemoryStore creates a new instance of WeaviateMemoryStore.
func NewWeaviateMemoryStore(
	client *weaviate.Client,
	openaiClient *openai.Client,
	className string,
	logger *log.Logger,
) memory.Storage {
	return &WeaviateMemoryStore{
		client:       client,
		openaiClient: openaiClient,
		className:    className,
		logger:       logger,
		storeTimeout: 30 * time.Minute,
		queryTimeout: 1 * time.Minute,
		llmTimeout:   60 * time.Second,
	}
}

type turnWithFacts struct {
	originalTurn    memory.TextDocument
	originalIndex   int
	extractedFacts  []string
	extractionError error
}

func (wms *WeaviateMemoryStore) Store(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
	wms.logger.Printf("Store called with %d turn-documents", len(documents))
	if progressChan != nil {
		defer close(progressChan)
	}

	if len(documents) == 0 {
		wms.logger.Printf("No documents provided to Store method.")
		return nil
	}

	wms.logger.Printf("Starting Stage 1: Parallel Fact Extraction for %d turns.", len(documents))

	var wg sync.WaitGroup
	factExtractionResultsChan := make(chan turnWithFacts, len(documents))

	for i, turnDoc := range documents {
		wg.Add(1)
		go func(td memory.TextDocument, index int) {
			defer wg.Done()
			wms.logger.Printf("Extracting facts for turn %d/%d: Content: \"%s\"", index+1, len(documents), td.Content)
			facts, err := wms.extractFactsFromTurn(ctx, td)
			if err != nil {
				wms.logger.Printf("Error extracting facts for turn %d (original content: \"%s\"): %v", index+1, td.Content, err)
			} else {
				wms.logger.Printf("Extracted %d facts for turn %d.", len(facts), index+1)
			}
			factExtractionResultsChan <- turnWithFacts{
				originalTurn:    td,
				originalIndex:   index,
				extractedFacts:  facts,
				extractionError: err,
			}
		}(turnDoc, i)
	}

	wg.Wait()
	close(factExtractionResultsChan)
	wms.logger.Printf("Finished all fact extraction goroutines.")

	processedTurns := make([]turnWithFacts, len(documents))
	for res := range factExtractionResultsChan {
		processedTurns[res.originalIndex] = res
	}
	wms.logger.Printf("Collected all fact extraction results.")

	wms.logger.Printf("Starting Stage 2: Sequential Fact Consolidation with immediate commits.")
	totalFactsProcessed := 0

	for i, processedTurn := range processedTurns {
		wms.logger.Printf("Processing facts for original turn %d/%d (Original ID: %s)", i+1, len(documents), processedTurn.originalTurn.ID)

		if processedTurn.extractionError != nil {
			wms.logger.Printf("Skipping fact consolidation for original turn %d due to fact extraction error: %v", i+1, processedTurn.extractionError)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: i + 1, Total: len(documents)}
			}
			continue
		}

		if len(processedTurn.extractedFacts) == 0 {
			wms.logger.Printf("No facts extracted for original turn %d. Nothing to consolidate.", i+1)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{Processed: i + 1, Total: len(documents)}
			}
			continue
		}

		wms.logger.Printf("Original turn %d yielded %d facts. Consolidating them now...", i+1, len(processedTurn.extractedFacts))

		for factIndex, factContent := range processedTurn.extractedFacts {
			totalFactsProcessed++
			wms.logger.Printf("  Processing fact %d/%d for turn %d: Content: \"%s\"", factIndex+1, len(processedTurn.extractedFacts), i+1, factContent)

			// Diagnostic: Add a small delay to allow for vectorization/indexing
			time.Sleep(1 * time.Second)
			wms.logger.Printf("    Waited 1s for potential indexing of previous fact.")

			relevantFactMemories, err := wms.Query(ctx, factContent)
			if err != nil {
				wms.logger.Printf("    Error querying relevant fact-memories for fact \"%s\" (turn %d): %v. Proceeding without consolidation for this fact.", factContent, i+1, err)
			} else {
				wms.logger.Printf("    Retrieved %d relevant fact-memories for fact \"%s\".", len(relevantFactMemories.Documents), factContent)
			}

			llmDecision := LLMDecision{Action: "ADD"}

			decision, err := wms.getLLMConsolidationDecision(ctx, factContent, relevantFactMemories.Documents)
			if err != nil {
				wms.logger.Printf("    Error getting LLM consolidation decision for fact \"%s\": %v. Defaulting to ADD.", factContent, err)
			} else {
				llmDecision = decision
				logMsg := fmt.Sprintf("    LLM decision for fact \"%s\": Action: %s", factContent, llmDecision.Action)
				if llmDecision.IDToUpdate != "" {
					logMsg += fmt.Sprintf(", IDToUpdate: %s", llmDecision.IDToUpdate)
				}
				if llmDecision.UpdatedContent != "" {
					logMsg += fmt.Sprintf(", UpdatedContent: \"%s\"", llmDecision.UpdatedContent)
				}
				if llmDecision.IDToDelete != "" {
					logMsg += fmt.Sprintf(", IDToDelete: %s", llmDecision.IDToDelete)
				}
				wms.logger.Printf(logMsg)
			}

			switch llmDecision.Action {
			case "ADD":
				newFactID := uuid.New().String()
				contentForStorage := llmDecision.UpdatedContent
				if contentForStorage == "" {
					contentForStorage = factContent
					wms.logger.Printf("    ADD action: LLM did not provide 'updated_content', using original fact content: \"%s\"", factContent)
				} else {
					wms.logger.Printf("    ADD action: Using 'updated_content' from LLM: \"%s\"", contentForStorage)
				}

				properties := map[string]interface{}{
					"content": contentForStorage,
				}
				if processedTurn.originalTurn.Timestamp != nil {
					properties["timestamp"] = processedTurn.originalTurn.Timestamp.Format(time.RFC3339Nano)
				} else {
					wms.logger.Printf("    ADD action: originalTurn.Timestamp is nil for new fact ID %s", newFactID)
				}

				factMetadata := make(map[string]string)
				if processedTurn.originalTurn.Metadata != nil {
					for k, v := range processedTurn.originalTurn.Metadata {
						factMetadata[k] = v
					}
				}
				factMetadata["original_turn_id"] = processedTurn.originalTurn.ID
				factMetadata["original_turn_content_preview"] = truncateString(processedTurn.originalTurn.Content, 50)
				factMetadata["fact_index_in_turn"] = fmt.Sprintf("%d", factIndex)

				metadataJSON, merr := json.Marshal(factMetadata)
				if merr != nil {
					wms.logger.Printf("    Error marshalling metadata for fact (new ID %s): %v. Metadata will not be stored.", newFactID, merr)
				} else {
					properties["metadata_map"] = string(metadataJSON)
				}

				// IMMEDIATE ADD
				_, createErr := wms.client.Data().Creator().
					WithClassName(wms.className).
					WithID(newFactID).
					WithProperties(properties).
					Do(ctx)
				if createErr != nil {
					wms.logger.Printf("    Error immediately ADDING fact (new ID %s) derived from turn %d: %v", newFactID, i+1, createErr)
				} else {
					wms.logger.Printf("    Successfully ADDED fact (new ID %s), derived from turn %d. Content: \"%s\"", newFactID, i+1, contentForStorage)
				}

			case "UPDATE":
				idToUpdate := llmDecision.IDToUpdate
				contentForUpdate := llmDecision.UpdatedContent
				if idToUpdate == "" || contentForUpdate == "" {
					wms.logger.Printf("    UPDATE action chosen for fact, but id_to_update (%s) or updated_content (%s) missing. Skipping.", idToUpdate, contentForUpdate)
					continue
				}
				wms.logger.Printf("    Attempting UPDATE for fact-memory ID %s. New Content: \"%s\"", idToUpdate, contentForUpdate)
				updateData := map[string]interface{}{ // Properties to update
					"content":   contentForUpdate,
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano), // Update timestamp always uses time.Now()
				}
				err := wms.client.Data().Updater().
					WithClassName(wms.className).
					WithID(idToUpdate).
					WithProperties(updateData).
					WithMerge(). // Use merge to only update specified fields
					Do(ctx)
				if err != nil {
					wms.logger.Printf("    Error updating fact-memory ID %s in Weaviate: %v", idToUpdate, err)
				} else {
					wms.logger.Printf("    Successfully UPDATED fact-memory ID %s.", idToUpdate)
				}

			case "DELETE":
				idToDelete := llmDecision.IDToDelete
				if idToDelete == "" {
					wms.logger.Printf("    DELETE action chosen for fact, but id_to_delete missing. Skipping.")
					continue
				}
				wms.logger.Printf("    Attempting DELETE for fact-memory ID %s.", idToDelete)
				err := wms.client.Data().Deleter().
					WithClassName(wms.className).
					WithID(idToDelete).
					Do(ctx)
				if err != nil {
					wms.logger.Printf("    Error deleting fact-memory ID %s from Weaviate: %v", idToDelete, err)
				} else {
					wms.logger.Printf("    Successfully DELETED fact-memory ID %s.", idToDelete)
				}

			case "NONE":
				wms.logger.Printf("    NONE action chosen by LLM for fact \"%s\". Skipping.", factContent)
			default:
				wms.logger.Printf("    Unknown action '%s' from LLM for fact \"%s\". Defaulting to ADD this fact.", llmDecision.Action, factContent)
				newFactID := uuid.New().String()
				properties := map[string]interface{}{ // Simplified properties for default ADD
					"content": factContent,
				}
				if processedTurn.originalTurn.Timestamp != nil {
					properties["timestamp"] = processedTurn.originalTurn.Timestamp.Format(time.RFC3339Nano)
				} else {
					wms.logger.Printf("    Default ADD action: originalTurn.Timestamp is nil for new fact ID %s", newFactID)
				}
				// IMMEDIATE ADD (fallback)
				_, createErr := wms.client.Data().Creator().
					WithClassName(wms.className).
					WithID(newFactID).
					WithProperties(properties).
					Do(ctx)
				if createErr != nil {
					wms.logger.Printf("    Error immediately ADDING fact (new ID %s) due to unknown LLM action: %v", newFactID, createErr)
				} else {
					wms.logger.Printf("    Defaulted to ADD for fact (new ID %s) due to unknown LLM action. Content: \"%s\"", newFactID, factContent)
				}
			}
		}

		if progressChan != nil {
			progressChan <- memory.ProgressUpdate{
				Processed: i + 1,
				Total:     len(documents),
			}
		}
	}

	wms.logger.Printf("Finished Stage 2: Sequential Fact Consolidation. Total facts processed: %d.", totalFactsProcessed)

	wms.logger.Printf("Store operation finished for %d initial documents.", len(documents))
	return nil
}

func (wms *WeaviateMemoryStore) extractFactsFromTurn(ctx context.Context, turnDoc memory.TextDocument) ([]string, error) {
	if strings.TrimSpace(turnDoc.Content) == "" {
		wms.logger.Println("extractFactsFromTurn: called with empty turnContent, returning no facts.")
		return []string{}, nil
	}

	prompt := fmt.Sprintf(FactExtractionUserPromptTemplate, turnDoc.Content)
	var resp openai.ChatCompletionResponse
	var err error

	for i := 0; i <= MaxRetries; i++ {
		requestCtx, cancel := context.WithTimeout(ctx, wms.llmTimeout)

		resp, err = wms.openaiClient.CreateChatCompletion(
			requestCtx,
			openai.ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: FactExtractionSystemPrompt},
					{Role: openai.ChatMessageRoleUser, Content: prompt},
				},
				ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
				Temperature:    0.1,
			},
		)
		cancel() // Release context resources as soon as possible
		if err == nil {
			break
		}
		wms.logger.Printf("extractFactsFromTurn: OpenAI call attempt %d/%d failed: %v", i+1, MaxRetries+1, err)
		if i < MaxRetries {
			time.Sleep(RetryDelay)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("fact extraction LLM call failed after %d retries: %w", MaxRetries+1, err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		wms.logger.Printf("extractFactsFromTurn: OpenAI returned no choices or empty content for turn: \"%s\"", turnDoc.Content)
		return []string{}, nil
	}

	llmOutput := resp.Choices[0].Message.Content
	var factsResponse FactsExtractionResponse
	err = json.Unmarshal([]byte(llmOutput), &factsResponse)
	if err == nil {
		return factsResponse.Facts, nil
	}

	wms.logger.Printf("Direct JSON unmarshal for fact extraction failed (err: %v), trying markdown fallback for output: %s", err, llmOutput)
	if strings.HasPrefix(llmOutput, "```json") && strings.HasSuffix(llmOutput, "```") {
		unwrappedJSON := strings.TrimPrefix(llmOutput, "```json\n")
		unwrappedJSON = strings.TrimSuffix(unwrappedJSON, "\n```")
		unwrappedJSON = strings.TrimSpace(unwrappedJSON)
		err = json.Unmarshal([]byte(unwrappedJSON), &factsResponse)
		if err == nil {
			return factsResponse.Facts, nil
		}
		wms.logger.Printf("Markdown JSON unmarshal for fact extraction also failed (err: %v)", err)
	}

	return nil, fmt.Errorf("failed to parse LLM response for fact extraction (output: %s): %w", llmOutput, err)
}

func (wms *WeaviateMemoryStore) getLLMConsolidationDecision(ctx context.Context, newFactContent string, relevantDocs []memory.TextDocument) (LLMDecision, error) {
	wms.logger.Printf("Getting LLM consolidation decision for new fact: \"%s\"", newFactContent)

	var relevantMemoriesString string
	if len(relevantDocs) > 0 {
		type PromptRelevantMemory struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		}
		promptMemories := make([]PromptRelevantMemory, 0, len(relevantDocs))
		for _, doc := range relevantDocs {
			promptMemories = append(promptMemories, PromptRelevantMemory{ID: doc.ID, Content: doc.Content})
		}
		memBytes, err := json.Marshal(promptMemories)
		if err != nil {
			wms.logger.Printf("Error marshalling relevant memories for prompt: %v", err)
			relevantMemoriesString = "[]"
		} else {
			relevantMemoriesString = string(memBytes)
		}
	} else {
		relevantMemoriesString = "No relevant existing memories found."
	}

	prompt := fmt.Sprintf(ConsolidationUserPromptTemplate, newFactContent, relevantMemoriesString)

	var resp openai.ChatCompletionResponse
	var err error
	var llmDecision LLMDecision

	for i := 0; i <= MaxRetries; i++ {
		requestCtx, cancel := context.WithTimeout(ctx, wms.llmTimeout)

		resp, err = wms.openaiClient.CreateChatCompletion(
			requestCtx,
			openai.ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: ConsolidationSystemPrompt},
					{Role: openai.ChatMessageRoleUser, Content: prompt},
				},
				ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
				Temperature:    0.1,
			},
		)
		cancel()
		if err == nil {
			break
		}
		wms.logger.Printf("getLLMConsolidationDecision: OpenAI call attempt %d/%d failed: %v", i+1, MaxRetries+1, err)
		if i < MaxRetries {
			time.Sleep(RetryDelay)
		}
	}

	if err != nil {
		return LLMDecision{}, fmt.Errorf("consolidation LLM call failed after %d retries: %w", MaxRetries+1, err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		wms.logger.Printf("getLLMConsolidationDecision: OpenAI returned no choices or empty content for fact: \"%s\"", newFactContent)
		return LLMDecision{Action: "ADD"}, nil
	}

	llmOutput := resp.Choices[0].Message.Content
	err = json.Unmarshal([]byte(llmOutput), &llmDecision)
	if err != nil {
		wms.logger.Printf("Direct JSON unmarshal for consolidation decision failed (err: %v), trying markdown fallback for output: %s", err, llmOutput)
		if strings.HasPrefix(llmOutput, "```json") && strings.HasSuffix(llmOutput, "```") {
			unwrappedJSON := strings.TrimPrefix(llmOutput, "```json\n")
			unwrappedJSON = strings.TrimSuffix(unwrappedJSON, "\n```")
			unwrappedJSON = strings.TrimSpace(unwrappedJSON)
			err = json.Unmarshal([]byte(unwrappedJSON), &llmDecision)
			if err == nil {
				wms.logger.Printf("Successfully parsed consolidation decision from markdown fallback: Action: %s", llmDecision.Action)
			} else {
				wms.logger.Printf("Markdown JSON unmarshal for consolidation decision also failed (err: %v)", err)
			}
		}
		if err != nil {
			return LLMDecision{}, fmt.Errorf("failed to parse consolidation LLM response (output: %s): %w", llmOutput, err)
		}
	}

	validActions := map[string]bool{"ADD": true, "UPDATE": true, "DELETE": true, "NONE": true}
	if !validActions[llmDecision.Action] {
		wms.logger.Printf("LLM returned invalid action '%s'. Defaulting to NONE.", llmDecision.Action)
		return LLMDecision{Action: "NONE"}, nil
	}

	logMsg := fmt.Sprintf("Consolidation LLM decision parsed: Action: %s", llmDecision.Action)
	if llmDecision.IDToUpdate != "" {
		logMsg += fmt.Sprintf(", IDToUpdate: %s", llmDecision.IDToUpdate)
	}
	if llmDecision.UpdatedContent != "" {
		logMsg += fmt.Sprintf(", UpdatedContent: \"%s\"", llmDecision.UpdatedContent)
	}
	if llmDecision.IDToDelete != "" {
		logMsg += fmt.Sprintf(", IDToDelete: %s", llmDecision.IDToDelete)
	}
	wms.logger.Printf(logMsg)
	return llmDecision, nil
}

func (wms *WeaviateMemoryStore) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, wms.queryTimeout)
	defer cancel()

	wms.logger.Printf("Querying Weaviate class '%s' with query: '%s'", wms.className, truncateString(query, 100))

	// Prepare GraphQL query
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "timestamp"},
		{Name: "tags"},
		{Name: "metadata_map"}, // This is assumed to be a JSON string
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"}, // We can include certainty if available and useful
		}},
	}

	nearText := wms.client.GraphQL().
		NearTextArgBuilder().
		WithConcepts([]string{query})

	resp, err := wms.client.GraphQL().Get().
		WithClassName(wms.className).
		WithFields(fields...).
		WithNearText(nearText).
		// WithLimit(k). // k is no longer passed, so remove or use a default from wms if available
		Do(ctx)

	if err != nil {
		return memory.QueryResult{}, fmt.Errorf("failed to execute Weaviate GraphQL query: %w", err)
	}
	if len(resp.Errors) > 0 {
		var errMsgs []string
		for _, e := range resp.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return memory.QueryResult{}, fmt.Errorf("GraphQL query returned errors: %s", strings.Join(errMsgs, "; "))
	}

	// Assuming resp.Data["Get"][wms.className] returns a list of maps or a compatible structure
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		return memory.QueryResult{}, fmt.Errorf("unexpected structure for GraphQL response data 'Get'")
	}

	classData, ok := data[wms.className].([]interface{})
	if !ok {
		// It might be that if no results, this key is missing or nil.
		wms.logger.Printf("No results found for class '%s' with query: '%s'", wms.className, query)
		return memory.QueryResult{Documents: []memory.TextDocument{}, Text: []string{}}, nil
	}

	var queryResults []memory.TextDocument
	var textResults []string

	for _, item := range classData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			wms.logger.Printf("Warning: Skipping item, not a map[string]interface{}: %T", item)
			continue
		}

		var doc memory.TextDocument
		additional, _ := itemMap["_additional"].(map[string]interface{})
		doc.ID, _ = additional["id"].(string)
		doc.Content, _ = itemMap["content"].(string)

		timestampStr, _ := itemMap["timestamp"].(string)
		if parsedTimestamp := parseTime(timestampStr); parsedTimestamp != nil {
			doc.Timestamp = parsedTimestamp
		} else if timestampStr != "" {
			wms.logger.Printf("Error parsing timestamp '%s' for doc ID %s: %v. Using nil time.", timestampStr, doc.ID, err)
		}

		if tagsInterface, ok := itemMap["tags"].([]interface{}); ok {
			for _, tagInterface := range tagsInterface {
				if tag, ok := tagInterface.(string); ok {
					doc.Tags = append(doc.Tags, tag)
				}
			}
		}

		metadataJSON, _ := itemMap["metadata_map"].(string)
		if metadata := parseMetadata(metadataJSON); metadata != nil {
			doc.Metadata = metadata
		}

		queryResults = append(queryResults, doc)
		textResults = append(textResults, doc.Content)
	}

	wms.logger.Printf("Query returned %d documents for query '%s'.", len(queryResults), query)
	return memory.QueryResult{Documents: queryResults, Text: textResults}, nil
}

// truncateString is a helper function to shorten strings for logging.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..." // Or handle this case as an error or empty string
	}
	return s[:maxLen-3] + "..."
}

// parseTime converts a timestamp string (RFC3339Nano) to a *time.Time pointer.
// Returns nil if parsing fails.
func parseTime(timeStr string) *time.Time {
	if timeStr == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		// Consider logging the error here if it's important
		// wms.logger.Printf("Error parsing timestamp string '%s': %v", timeStr, err)
		return nil
	}
	return &t
}

// parseMetadata unmarshals a JSON string into a map[string]string.
// Returns nil if unmarshalling fails or the string is empty.
func parseMetadata(metadataStr string) map[string]string {
	if metadataStr == "" {
		return nil
	}
	var metadata map[string]string
	err := json.Unmarshal([]byte(metadataStr), &metadata)
	if err != nil {
		// Consider logging the error here
		// wms.logger.Printf("Error unmarshalling metadata string '%s': %v", metadataStr, err)
		return nil
	}
	return metadata
}
