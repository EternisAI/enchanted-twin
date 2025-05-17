package newmem

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
)

// --- Prompts and Structs for LLM Interactions ---

// FactExtractionSystemPrompt defines the system role for fact extraction.
const FactExtractionSystemPrompt = `You are an expert AI tasked with extracting atomic facts from a given text. 
Each fact should be a concise, self-contained piece of information. 
For example, if the text is "User A is in San Francisco and likes ice cream", the facts are "User A is in San Francisco" and "User A likes ice cream".
If the text is a question, extract the core statement or assumption if possible, or state that it's a question about a topic.
Respond with a tool call object containing the "facts" (array of strings). Example: {"facts": ["fact 1", "fact 2", "fact 3"]}
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
	Reason         string `json:"reason,omitempty"`          // For ADD, UPDATE, DELETE, NONE
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

Respond with a TOol call object containing the "action" (ADD, UPDATE, DELETE, NONE). 
If UPDATE, also include "id" and "updated_content". 
If DELETE, also include "id".

Example for UPDATE:
{"action": "UPDATE", "id": "existing_memory_id_123", "updated_content": "Updated version of the memory with new details from the fact."}

Example for ADD:
{"action": "ADD"}

Example for NONE:
{"action": "NONE"}

Example for DELETE:
{"action": "DELETE", "id": "existing_memory_id_456"}

Consider the timestamp and specificity of the information. A newer, more specific fact might supersede an older, vaguer one.`

// ConsolidationUserPromptTemplate is the template for the user message for the consolidation LLM.
const ConsolidationUserPromptTemplate = `New Information:
"%s"

Relevant Existing Memories (if any):
%s

Based on the new information and existing memories, what is your decision (ADD, UPDATE, DELETE, NONE) and the necessary details (id, content)?
Respond with a tool call.`

// --- WeaviateMemoryStore Implementation ---

// WeaviateMemoryStore implements the memory.Storage interface using Weaviate and OpenAI.
type WeaviateMemoryStore struct {
	client               *weaviate.Client
	aiCompletionsService *ai.Service
	aiEmbeddingsService  *ai.Service
	logger               *log.Logger
	className            string
	storeTimeout         time.Duration
	queryTimeout         time.Duration
	llmTimeout           time.Duration
	completionsModel     string
	embeddingsModel      string
}

// NewWeaviateMemoryStore creates a new instance of WeaviateMemoryStore.
func NewWeaviateMemoryStore(
	client *weaviate.Client,
	aiCompletionsService *ai.Service,
	aiEmbeddingsService *ai.Service,
	completionsModel string,
	embeddingsModel string,
	className string,
	logger *log.Logger,
) memory.Storage {
	return &WeaviateMemoryStore{
		client:               client,
		aiCompletionsService: aiCompletionsService,
		aiEmbeddingsService:  aiEmbeddingsService,
		completionsModel:     completionsModel,
		embeddingsModel:      embeddingsModel,
		className:            className,
		logger:               logger,
		storeTimeout:         30 * time.Minute,
		queryTimeout:         1 * time.Minute,
		llmTimeout:           60 * time.Second,
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

			relevantFactMemories, err := wms.Query(ctx, factContent)
			if err != nil {
				wms.logger.Printf("    Error querying relevant fact-memories for fact \"%s\" (turn %d): %v. Proceeding without consolidation for this fact.", factContent, i+1, err)
				return errors.Wrap(err, "error querying relevant fact-memories")
			} else {
				wms.logger.Printf("    Retrieved %d relevant fact-memories for fact \"%s\".", len(relevantFactMemories.Documents), factContent)
			}

			decision, err := wms.getLLMConsolidationDecision(ctx, factContent, relevantFactMemories.Documents)
			if err != nil {
				wms.logger.Debug("Error getting LLM consolidation decision", "fact_content", factContent, "error", err)
				continue
			}

			llmDecision := decision
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

				embedding, err := wms.aiEmbeddingsService.Embedding(ctx, contentForStorage, wms.embeddingsModel)
				if err != nil {
					wms.logger.Printf("    Error generating embedding for new fact: %v", err)
					return errors.Wrap(err, "error generating embedding for new fact")
				}
				embedding32 := make([]float32, len(embedding))
				for i, v := range embedding {
					embedding32[i] = float32(v)
				}

				properties := map[string]any{
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

				metadataJSON, err := json.Marshal(factMetadata)
				if err != nil {
					wms.logger.Printf("    Error marshalling metadata for fact (new ID %s): %v. Metadata will not be stored.", newFactID, err)
					return errors.Wrap(err, "memory failed to unmarshal")
				}
				properties["metadata_map"] = string(metadataJSON)

				// IMMEDIATE ADD
				_, err = wms.client.Data().Creator().
					WithClassName(wms.className).
					WithID(newFactID).
					WithProperties(properties).
					WithVector(embedding32).
					Do(ctx)
				if err != nil {
					wms.logger.Printf("    Error immediately ADDING fact (new ID %s) derived from turn %d: %v", newFactID, i+1, err)
					return errors.Wrap(err, "error immediately ADDING fact")
				}
				wms.logger.Printf("    Successfully ADDED fact (new ID %s), derived from turn %d. Content: \"%s\"", newFactID, i+1, contentForStorage)

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
					return errors.Wrap(err, "error updating fact-memory")
				}
				wms.logger.Printf("    Successfully UPDATED fact-memory ID %s.", idToUpdate)

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
					return errors.Wrap(err, "error deleting fact-memory")
				}
				wms.logger.Printf("    Successfully DELETED fact-memory ID %s.", idToDelete)

			case "NONE":
				wms.logger.Printf("    NONE action chosen by LLM for fact \"%s\". Skipping.", factContent)
			default:
				return errors.New("unknown action from LLM")
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
		wms.logger.Debug("extractFactsFromTurn: called with empty turnContent, returning no facts.")
		return []string{}, nil
	}

	prompt := fmt.Sprintf(FactExtractionUserPromptTemplate, turnDoc.Content)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionSystemPrompt),
		openai.UserMessage(prompt),
	}

	resp, err := wms.aiCompletionsService.ParamsCompletions(
		ctx,
		openai.ChatCompletionNewParams{
			Model:       wms.completionsModel,
			Messages:    messages,
			Tools:       []openai.ChatCompletionToolParam{extractFactsTool},
			ToolChoice:  openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: param.NewOpt("required")},
			Temperature: param.NewOpt(0.1),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("fact extraction LLM call failed after %d retries: %w", MaxRetries+1, err)
	}

	wms.logger.Debug("extractFactsFromTurn", "content", turnDoc.Content, "tool_calls", resp.ToolCalls)

	if len(resp.ToolCalls) == 0 {
		return nil, errors.New("no tool call for extractFactsFromTurn")
	}

	var factsResponse FactsExtractionResponse
	for _, toolCall := range resp.ToolCalls {
		wms.logger.Debug("extractFactsFromTurn", "tool_returned", toolCall.Function.Name, "expected_name", extractFactsTool.Function.Name)
		if toolCall.Function.Name == extractFactsTool.Function.Name {
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &factsResponse)
			if err == nil {
				return factsResponse.Facts, nil
			}
		}
	}

	return nil, errors.Wrap(err, "could not find tool call for extractFactsFromTurn")
}

func (wms *WeaviateMemoryStore) getLLMConsolidationDecision(ctx context.Context, newFactContent string, relevantDocs []memory.TextDocument) (LLMDecision, error) {
	wms.logger.Debug("Getting LLM consolidation decision", "new_fact_content", newFactContent)

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
		// TODO: Why marshalling instead of just using pretty string based on the data?
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

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(ConsolidationSystemPrompt),
		openai.UserMessage(prompt),
	}
	tools := []openai.ChatCompletionToolParam{addMemoryTool, updateMemoryTool, deleteMemoryTool, noneMemoryTool}

	resp, err := wms.aiCompletionsService.ParamsCompletions(
		ctx,
		openai.ChatCompletionNewParams{
			Model:      wms.completionsModel,
			Messages:   messages,
			Tools:      tools,
			ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: param.NewOpt("required")},
		},
	)
	if err != nil {
		return LLMDecision{}, fmt.Errorf("consolidation LLM call failed: %w", err)
	}

	if len(resp.ToolCalls) == 0 {
		wms.logger.Printf("getLLMConsolidationDecision: OpenAI returned empty content for fact: \"%s\"", newFactContent)
		return LLMDecision{}, errors.New("no tool call for getLLMConsolidationDecision")
	}

	for _, toolCall := range resp.ToolCalls {
		if toolCall.Function.Name == addMemoryTool.Function.Name {
			return LLMDecision{Action: "ADD"}, nil
		} else if toolCall.Function.Name == updateMemoryTool.Function.Name {
			var updateToolRes struct {
				ID             string `json:"id"`
				UpdatedContent string `json:"updated_content"`
				Reason         string `json:"reason"`
			}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &updateToolRes)
			if err != nil {
				return LLMDecision{}, errors.Wrap(err, "could not update tool call arguments")
			}
			return LLMDecision{Action: "UPDATE", IDToUpdate: updateToolRes.ID, UpdatedContent: updateToolRes.UpdatedContent, Reason: updateToolRes.Reason}, nil
		} else if toolCall.Function.Name == deleteMemoryTool.Function.Name {
			var deleteToolRes struct {
				ID     string `json:"id"`
				Reason string `json:"reason"`
			}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &deleteToolRes)
			if err != nil {
				return LLMDecision{}, errors.Wrap(err, "could not delete tool call arguments")
			}
			return LLMDecision{Action: "DELETE", IDToDelete: deleteToolRes.ID, Reason: deleteToolRes.Reason}, nil
		} else if toolCall.Function.Name == noneMemoryTool.Function.Name {
			var noneToolRes struct {
				Reason string `json:"reason"`
			}
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &noneToolRes)
			if err != nil {
				return LLMDecision{}, errors.Wrap(err, "could not none tool call arguments")
			}
			return LLMDecision{Action: "NONE", Reason: noneToolRes.Reason}, nil
		} else {
			wms.logger.Printf("getLLMConsolidationDecision: unknown tool call: %s", toolCall.Function.Name)
		}
	}

	return LLMDecision{}, errors.New("could not find matching tool call for getLLMConsolidationDecision")
}

func (wms *WeaviateMemoryStore) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "timestamp"},
		{Name: "tags"},
		{Name: "metadata_map"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"},
		}},
	}

	embedding, err := wms.aiEmbeddingsService.Embedding(ctx, query, wms.embeddingsModel)
	if err != nil {
		return memory.QueryResult{}, errors.Wrap(err, "failed to calculate embedding")
	}

	embedding32 := make([]float32, len(embedding))
	for i, v := range embedding {
		embedding32[i] = float32(v)
	}

	nearVector := wms.client.GraphQL().
		NearVectorArgBuilder().
		WithVector(embedding32)

	resp, err := wms.client.GraphQL().Get().
		WithClassName(wms.className).
		WithFields(fields...).
		WithNearVector(nearVector).
		WithLimit(10).
		Do(ctx)

	if err != nil {
		return memory.QueryResult{}, errors.Wrap(err, "failed to execute Weaviate GraphQL query")
	}
	if len(resp.Errors) > 0 {
		var errMsgs []string
		for _, e := range resp.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return memory.QueryResult{}, fmt.Errorf("GraphQL query returned errors: %s", strings.Join(errMsgs, "; "))
	}

	// Assuming resp.Data["Get"][wms.className] returns a list of maps or a compatible structure
	data, ok := resp.Data["Get"].(map[string]any)
	if !ok {
		return memory.QueryResult{}, errors.New("unexpected structure for GraphQL response data 'Get'")
	}

	classData, ok := data[wms.className].([]any)
	if !ok {
		// It might be that if no results, this key is missing or nil.
		wms.logger.Debug("No results found for class", "class", wms.className, "query", query)
		return memory.QueryResult{Documents: []memory.TextDocument{}, Text: []string{}}, nil
	}

	var queryResults []memory.TextDocument
	var textResults []string

	for _, item := range classData {
		itemMap, ok := item.(map[string]any)
		if !ok {
			wms.logger.Debug("Warning: Skipping item, not a map[string]any", "item", item)
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
			wms.logger.Debug("Error parsing timestamp", "timestamp", timestampStr, "doc_id", doc.ID, "error", err)
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

	wms.logger.Debug("Query returned documents", "count", len(queryResults), "query", query)
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
