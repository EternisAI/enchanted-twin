package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// DistributeWork splits documents evenly among workers.
func DistributeWork(docs []memory.Document, workers int) [][]memory.Document {
	if workers <= 0 {
		workers = 1
	}

	chunks := make([][]memory.Document, workers)
	for i, doc := range docs {
		workerIdx := i % workers
		chunks[workerIdx] = append(chunks[workerIdx], doc)
	}

	return chunks
}

// CreateMemoryObject builds the Weaviate object for ADD operations.
func CreateMemoryObject(fact StructuredFact, source memory.Document, decision MemoryDecision) *models.Object {
	// Get tags from the source document
	tags := source.Tags()

	// Get timestamp from source document
	timestamp := time.Now()
	if ts := source.Timestamp(); ts != nil && !ts.IsZero() {
		timestamp = *ts
	}

	// Prepare properties with new direct fields
	properties := map[string]interface{}{
		"content":            fact.GenerateContent(),
		"metadataJson":       "{}",
		"timestamp":          timestamp.Format(time.RFC3339),
		"tags":               tags,
		"documentReferences": []string{},
		// Store structured fact fields
		"factCategory":    fact.Category,
		"factSubject":     fact.Subject,
		"factAttribute":   fact.Attribute,
		"factValue":       fact.Value,
		"factSensitivity": fact.Sensitivity,
		"factImportance":  fact.Importance,
	}

	// Store temporal context if present
	if fact.TemporalContext != nil {
		properties["factTemporalContext"] = *fact.TemporalContext
	}

	// Extract and store source as direct field
	if sourceField := source.Source(); sourceField != "" {
		properties["source"] = sourceField
	}

	return &models.Object{
		Class:      ClassName,
		Properties: properties,
	}
}

// CreateMemoryObjectWithDocumentReferences builds the Weaviate object with document references.
func CreateMemoryObjectWithDocumentReferences(fact StructuredFact, source memory.Document, decision MemoryDecision, documentIDs []string) *models.Object {
	obj := CreateMemoryObject(fact, source, decision)

	// Update with actual document references
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return obj
	}
	props["documentReferences"] = documentIDs

	return obj
}

// ExtractFactsFromDocument routes fact extraction based on document type.
// This is pure business logic extracted from the adapter.
// Returns the extracted facts. The source document is already known by the caller.
func ExtractFactsFromDocument(ctx context.Context, doc memory.Document, completionsService *ai.Service, completionsModel string, logger *log.Logger) ([]StructuredFact, error) {
	switch typedDoc := doc.(type) {
	case *memory.ConversationDocument:
		// Extract for the document-level context (no specific speaker)
		return extractFactsFromConversation(ctx, *typedDoc, completionsService, completionsModel, doc, logger)

	case *memory.TextDocument:
		return extractFactsFromTextDocument(ctx, *typedDoc, completionsService, completionsModel, doc, logger)

	default:
		return nil, fmt.Errorf("unsupported document type: %T", doc)
	}
}

// BuildSeparateMemoryDecisionPrompts constructs separate system and user prompts to prevent injection.
// This is the secure version that properly separates system instructions from user content.
func BuildSeparateMemoryDecisionPrompts(fact string, similar []ExistingMemory) (systemPrompt string, userPrompt string) {
	// System prompt contains only instructions and guidelines - no user content
	systemPrompt = MemoryUpdatePrompt

	// User prompt contains only the user data to be analyzed
	existingMemoriesContentForPrompt := []string{}
	existingMemoriesForPromptStr := "No existing relevant memories found."

	if len(similar) > 0 {
		for _, mem := range similar {
			memContext := fmt.Sprintf("ID: %s, Content: %s", mem.ID, mem.Content)
			existingMemoriesContentForPrompt = append(existingMemoriesContentForPrompt, memContext)
		}
		existingMemoriesForPromptStr = strings.Join(existingMemoriesContentForPrompt, "\n---\n")
	}

	userPrompt = fmt.Sprintf(`Context to analyze:

Existing Memories for the primary user (if any, related to the new fact):
%s

New Fact to consider for the primary user:
%s

Please analyze this context and decide what action should be taken for the NEW FACT.`, existingMemoriesForPromptStr, fact)

	return systemPrompt, userPrompt
}

// ParseMemoryDecisionResponse parses LLM tool call response into MemoryDecision.
// This is pure business logic extracted from the adapter.
func ParseMemoryDecisionResponse(llmResponse openai.ChatCompletionMessage) (MemoryDecision, error) {
	if len(llmResponse.ToolCalls) == 0 {
		return MemoryDecision{
			Action: ADD,
			Reason: "No tool call made, defaulting to ADD",
		}, nil
	}

	toolCall := llmResponse.ToolCalls[0]
	action := MemoryAction(toolCall.Function.Name)

	decision := MemoryDecision{
		Action: action,
	}

	switch action {
	case UPDATE:
		var args UpdateToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return MemoryDecision{}, fmt.Errorf("unmarshaling UPDATE arguments: %w", err)
		}
		decision.TargetID = args.MemoryID
		decision.Reason = args.Reason

	case DELETE:
		var args DeleteToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return MemoryDecision{}, fmt.Errorf("unmarshaling DELETE arguments: %w", err)
		}
		decision.TargetID = args.MemoryID
		decision.Reason = args.Reason

	case NONE:
		var args NoneToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			// Non-fatal for NONE - this is intentionally lenient
		} else {
			decision.Reason = args.Reason
		}
	}

	return decision, nil
}

// SearchSimilarMemories performs semantic search for similar memories.
// This is pure business logic extracted from the adapter.
func SearchSimilarMemories(ctx context.Context, fact string, filter *memory.Filter, storage storage.Interface, embeddingsModel string) ([]ExistingMemory, error) {
	result, err := storage.Query(ctx, fact, filter, embeddingsModel)
	if err != nil {
		return nil, fmt.Errorf("querying similar memories: %w", err)
	}

	memories := make([]ExistingMemory, 0, len(result.Facts))
	for _, memoryFact := range result.Facts {
		mem := ExistingMemory{
			ID:        memoryFact.ID,
			Content:   memoryFact.Content,
			Metadata:  memoryFact.Metadata,
			Timestamp: memoryFact.Timestamp,
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, completionsService *ai.Service, completionsModel string, sourceDoc memory.Document, logger *log.Logger) ([]StructuredFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := convDoc.Content()

	if len(convDoc.Conversation) == 0 {
		logger.Info("Skipping empty conversation", "id", convDoc.ID())
		return []StructuredFact{}, nil
	}

	logger.Debug("Normalized JSON length", "length", len(content))
	logger.Debug(" User prompt", "prompt", content[:min(2000, len(content))])
	logger.Debug(" primaryPseaker", "user", convDoc.User)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	logger.Debug("Sending conversation to LLM", "system_prompt_length", len(FactExtractionPrompt), "json_length", len(content))

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		logger.Error("LLM completion FAILED for conversation", "id", convDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for conversation %s: %w", convDoc.ID(), err)
	}

	logger.Debug("LLM Response for conversation", "id", convDoc.ID())
	logger.Debug("Response Content", "content", llmResponse.Content)
	logger.Debug("Tool Calls Count", "count", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		logger.Warn("No tool calls returned for conversation - fact extraction may have failed", "id", convDoc.ID())
	}

	var extractedFacts []StructuredFact
	for _, toolCall := range llmResponse.ToolCalls {
		logger.Debug("Tool Call", "name", toolCall.Function.Name)
		logger.Debug("Arguments", "args", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			logger.Debug("SKIPPING: Wrong tool name", "expected", ExtractFactsToolName)
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			logger.Error("FAILED to unmarshal tool arguments", "error", err)
			continue
		}

		logger.Debug("Successfully parsed structured facts from conversation", "count", len(args.Facts))

		if len(args.Facts) == 0 {
			logger.Warn("Tool call returned zero facts for conversation", "id", convDoc.ID())
		}

		for factIdx, structuredFact := range args.Facts {
			logger.Debug("Conversation Fact",
				"index", factIdx+1,
				"category", structuredFact.Category,
				"subject", structuredFact.Subject,
				"attribute", structuredFact.Attribute,
				"value", structuredFact.Value,
				"importance", structuredFact.Importance,
				"sensitivity", structuredFact.Sensitivity)

			extractedFacts = append(extractedFacts, structuredFact)
		}
	}

	logger.Debug("=== CONVERSATION FACT EXTRACTION SUMMARY ===")
	logger.Info("Conversation fact extraction completed", "id", convDoc.ID(), "facts_extracted", len(extractedFacts))
	if len(extractedFacts) == 0 {
		logger.Warn("NO FACTS EXTRACTED from conversation", "id", convDoc.ID())
	}
	logger.Debug("=== CONVERSATION FACT EXTRACTION END ===")

	return extractedFacts, nil
}

// extractFactsFromTextDocument extracts facts from text documents.
func extractFactsFromTextDocument(ctx context.Context, textDoc memory.TextDocument, completionsService *ai.Service, completionsModel string, sourceDoc memory.Document, logger *log.Logger) ([]StructuredFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := textDoc.Content()
	if content == "" {
		logger.Info("Skipping empty text document", "id", textDoc.ID())
		return []StructuredFact{}, nil
	}

	logger.Debug("=== FACT EXTRACTION START ===")
	logger.Debug("Document details",
		"id", textDoc.ID(),
		"source", textDoc.Source(),
		"tags", textDoc.Tags(),
		"metadata", textDoc.Metadata(),
		"content_length", len(content))
	logger.Debug("Full Content", "content", content)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	logger.Debug("Sending to LLM", "system_prompt_length", len(FactExtractionPrompt), "user_message_length", len(content))

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		logger.Error("LLM completion FAILED for document", "id", textDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for document %s: %w", textDoc.ID(), err)
	}

	logger.Debug("LLM Response for document", "id", textDoc.ID(), "content", llmResponse.Content, "tool_calls_count", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		logger.Warn("No tool calls returned for document - fact extraction may have failed", "id", textDoc.ID())
	}

	var extractedFacts []StructuredFact
	for i, toolCall := range llmResponse.ToolCalls {
		logger.Debug("Tool Call", "index", i+1, "name", toolCall.Function.Name)
		logger.Debug("Arguments", "args", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			logger.Debug("SKIPPING: Wrong tool name", "expected", ExtractFactsToolName)
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			logger.Error("FAILED to unmarshal tool arguments", "error", err)
			continue
		}

		logger.Debug("Successfully parsed structured facts", "count", len(args.Facts))

		if len(args.Facts) == 0 {
			logger.Warn("Tool call returned zero facts for document", "id", textDoc.ID())
		}

		for factIdx, structuredFact := range args.Facts {
			logger.Debug("Text Document Fact",
				"index", factIdx+1,
				"category", structuredFact.Category,
				"subject", structuredFact.Subject,
				"attribute", structuredFact.Attribute,
				"value", structuredFact.Value,
				"importance", structuredFact.Importance,
				"sensitivity", structuredFact.Sensitivity)

			extractedFacts = append(extractedFacts, structuredFact)
		}
	}

	if len(extractedFacts) == 0 {
		logger.Warn("NO FACTS EXTRACTED from document", "id", textDoc.ID())
	} else {
		logger.Info("Document fact extraction completed", "id", textDoc.ID(), "facts_extracted", len(extractedFacts))
	}

	return extractedFacts, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
