package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// DistributeWork splits documents evenly among workers.
func DistributeWork(docs []PreparedDocument, workers int) [][]PreparedDocument {
	if workers <= 0 {
		workers = 1
	}

	chunks := make([][]PreparedDocument, workers)
	for i, doc := range docs {
		workerIdx := i % workers
		chunks[workerIdx] = append(chunks[workerIdx], doc)
	}

	return chunks
}

// CreateMemoryObject builds the Weaviate object for ADD operations.
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object {
	metadata := make(map[string]string)

	// Only keep document references for tracking lineage
	metadata["sourceDocumentId"] = fact.Source.Original.ID()
	metadata["sourceDocumentType"] = string(fact.Source.Type)

	// Get tags from the source document
	tags := fact.Source.Original.Tags()

	// Prepare properties with new direct fields
	properties := map[string]interface{}{
		"content":            fact.Content,
		"metadataJson":       marshalMetadata(metadata),
		"timestamp":          fact.Source.Timestamp.Format(time.RFC3339),
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
	if source := fact.Source.Original.Source(); source != "" {
		properties["source"] = source
	}

	return &models.Object{
		Class:      ClassName,
		Properties: properties,
	}
}

// CreateMemoryObjectWithDocumentReferences builds the Weaviate object with document references.
func CreateMemoryObjectWithDocumentReferences(fact ExtractedFact, decision MemoryDecision, documentIDs []string) *models.Object {
	obj := CreateMemoryObject(fact, decision)

	// Update with actual document references
	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return obj
	}
	props["documentReferences"] = documentIDs

	return obj
}

// marshalMetadata converts a metadata map to JSON string for storage.
func marshalMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return "{}"
	}

	// Use proper JSON marshaling instead of manual construction
	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		// Fallback to empty JSON object if marshaling fails
		return "{}"
	}
	return string(jsonBytes)
}

// aggregateErrors combines multiple errors into a single error with context about all failures.
func aggregateErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	var messages []string
	for i, err := range errors {
		messages = append(messages, fmt.Sprintf("error %d: %v", i+1, err))
	}

	return fmt.Errorf("multiple errors occurred (%d total): %s", len(errors), strings.Join(messages, "; "))
}

// ExtractFactsFromDocument routes fact extraction based on document type.
// This is pure business logic extracted from the adapter.
func ExtractFactsFromDocument(ctx context.Context, doc PreparedDocument, completionsService *ai.Service, completionsModel string) ([]ExtractedFact, error) {
	currentSystemDate := doc.Timestamp.Format("2006-01-02")
	docEventDateStr := doc.DateString

	switch doc.Type {
	case DocumentTypeConversation:
		convDoc, ok := doc.Original.(*memory.ConversationDocument)
		if !ok {
			return nil, fmt.Errorf("expected ConversationDocument but got %T", doc.Original)
		}

		// Extract for the document-level context (no specific speaker)
		return extractFactsFromConversation(ctx, *convDoc, currentSystemDate, docEventDateStr, completionsService, completionsModel)

	case DocumentTypeText:
		textDoc, ok := doc.Original.(*memory.TextDocument)
		if !ok {
			return nil, fmt.Errorf("expected TextDocument but got %T", doc.Original)
		}

		return extractFactsFromTextDocument(ctx, *textDoc, currentSystemDate, docEventDateStr, completionsService, completionsModel)

	default:
		return nil, fmt.Errorf("unsupported document type: %s", doc.Type)
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
func SearchSimilarMemories(ctx context.Context, fact string, storage storage.Interface, embeddingsModel string) ([]ExistingMemory, error) {
	result, err := storage.Query(ctx, fact, nil, embeddingsModel)
	if err != nil {
		return nil, fmt.Errorf("querying similar memories: %w", err)
	}

	memories := make([]ExistingMemory, 0, len(result.Facts))
	for _, fact := range result.Facts {
		mem := ExistingMemory{
			ID:        fact.ID,
			Content:   fact.Content,
			Metadata:  fact.Metadata,
			Timestamp: fact.Timestamp,
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// formatConversationForLLM formats a conversation document into a clean, readable format
// optimized for LLM fact extraction instead of using the raw JSON format.
func formatConversationForLLM(convDoc memory.ConversationDocument) string {
	if len(convDoc.Conversation) == 0 {
		return ""
	}

	var formatted strings.Builder

	for _, msg := range convDoc.Conversation {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		formatted.WriteString(msg.Speaker)
		formatted.WriteString(": ")
		formatted.WriteString(content)
		formatted.WriteString("\n")
	}

	return strings.TrimSpace(formatted.String())
}

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, currentSystemDate string, docEventDateStr string, completionsService *ai.Service, completionsModel string) ([]ExtractedFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := formatConversationForLLM(convDoc)

	if len(convDoc.Conversation) == 0 {
		log.Printf("Skipping empty conversation: ID=%s", convDoc.ID())
		return []ExtractedFact{}, nil
	}

	log.Printf("Formatted conversation length: %d", len(content))
	log.Printf("User prompt %s", content[:min(800, len(content))])

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	log.Printf("Sending conversation to LLM - System prompt length: %d, Formatted length: %d", len(FactExtractionPrompt), len(content))

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		log.Printf("❌LLM completion FAILED for conversation %s: %v", convDoc.ID(), err)
		return nil, fmt.Errorf("LLM completion error for conversation %s: %w", convDoc.ID(), err)
	}

	log.Printf("LLM Response for conversation %s:", convDoc.ID())
	log.Printf("  Response Content: %s", llmResponse.Content)
	log.Printf("  Tool Calls Count: %d", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		log.Printf("❌ ERROR: No tool calls returned for conversation %s - fact extraction may have failed", convDoc.ID())
		log.Printf("❌ ERROR: LLM response: %s", llmResponse.Content)
	}

	var extractedFacts []ExtractedFact
	for _, toolCall := range llmResponse.ToolCalls {
		log.Printf("  Tool Call: Name=%s", toolCall.Function.Name)
		log.Printf("  Arguments: %s", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			log.Printf("    SKIPPING: Wrong tool name (expected %s)", ExtractFactsToolName)
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("    FAILED to unmarshal tool arguments: %v", err)
			continue
		}

		log.Printf("    Successfully parsed %d structured facts from conversation", len(args.Facts))

		if len(args.Facts) == 0 {
			log.Printf("    WARNING: Tool call returned zero facts for conversation %s", convDoc.ID())
		}

		for factIdx, structuredFact := range args.Facts {
			log.Printf("    Conversation Fact %d:", factIdx+1)
			log.Printf("      Category: %s", structuredFact.Category)
			log.Printf("      Subject: %s", structuredFact.Subject)
			log.Printf("      Attribute: %s", structuredFact.Attribute)
			log.Printf("      Value: %s", structuredFact.Value)
			log.Printf("      Importance: %d", structuredFact.Importance)
			log.Printf("      Sensitivity: %s", structuredFact.Sensitivity)

			// TODO: reconsider `content` field
			extractedFacts = append(extractedFacts, ExtractedFact{
				Content:         fmt.Sprintf("%s - %s", structuredFact.Subject, structuredFact.Value), // Combined for embeddings
				Category:        structuredFact.Category,
				Subject:         structuredFact.Subject,
				Attribute:       structuredFact.Attribute,
				Value:           structuredFact.Value,
				Sensitivity:     structuredFact.Sensitivity,
				Importance:      structuredFact.Importance,
				TemporalContext: structuredFact.TemporalContext,
			})
		}
	}

	log.Printf("=== CONVERSATION FACT EXTRACTION SUMMARY ===")
	log.Printf("Conversation %s: Extracted %d total facts", convDoc.ID(), len(extractedFacts))
	if len(extractedFacts) == 0 {
		log.Printf("NO FACTS EXTRACTED from conversation")
	}
	log.Printf("=== CONVERSATION FACT EXTRACTION END ===")

	return extractedFacts, nil
}

// extractFactsFromTextDocument extracts facts from text documents.
func extractFactsFromTextDocument(ctx context.Context, textDoc memory.TextDocument, currentSystemDate string, docEventDateStr string, completionsService *ai.Service, completionsModel string) ([]ExtractedFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := textDoc.Content()
	if content == "" {
		log.Printf("Skipping empty text document: ID=%s", textDoc.ID())
		return []ExtractedFact{}, nil
	}

	log.Printf("=== FACT EXTRACTION START ===")
	log.Printf("Document ID: %s", textDoc.ID())
	log.Printf("Document Source: %s", textDoc.Source())
	log.Printf("Document Tags: %v", textDoc.Tags())
	log.Printf("Document Metadata: %v", textDoc.Metadata())
	log.Printf("Content Length: %d", len(content))
	log.Printf("Full Content: %s", content)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	log.Printf("Sending to LLM - System prompt length: %d, User message length: %d", len(FactExtractionPrompt), len(content))

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		log.Printf("LLM completion FAILED for document %s: %v", textDoc.ID(), err)
		return nil, fmt.Errorf("LLM completion error for document %s: %w", textDoc.ID(), err)
	}

	log.Printf("LLM Response for document %s:", textDoc.ID())
	log.Printf("  Response Content: %s", llmResponse.Content)
	log.Printf("  Tool Calls Count: %d", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		log.Printf("WARNING: No tool calls returned for document %s - fact extraction may have failed", textDoc.ID())
		return nil, fmt.Errorf("LLM failed to call EXTRACT_FACTS tool for document %s - responded conversationally instead", textDoc.ID())
	}

	var extractedFacts []ExtractedFact
	for i, toolCall := range llmResponse.ToolCalls {
		log.Printf("  Tool Call %d:", i+1)
		log.Printf("    Name: %s", toolCall.Function.Name)
		log.Printf("    Arguments: %s", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			log.Printf("    SKIPPING: Wrong tool name (expected %s)", ExtractFactsToolName)
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("    FAILED to unmarshal tool arguments: %v", err)
			continue
		}

		log.Printf("    Successfully parsed %d structured facts", len(args.Facts))

		if len(args.Facts) == 0 {
			log.Printf("    WARNING: Tool call returned zero facts for document %s", textDoc.ID())
		}

		for factIdx, structuredFact := range args.Facts {
			log.Printf("    Fact %d:", factIdx+1)
			log.Printf("      Category: %s", structuredFact.Category)
			log.Printf("      Subject: %s", structuredFact.Subject)
			log.Printf("      Attribute: %s", structuredFact.Attribute)
			log.Printf("      Value: %s", structuredFact.Value)
			log.Printf("      Importance: %d", structuredFact.Importance)
			log.Printf("      Sensitivity: %s", structuredFact.Sensitivity)

			// TODO: reconsider `content` field
			extractedFacts = append(extractedFacts, ExtractedFact{
				Content:         fmt.Sprintf("%s - %s", structuredFact.Subject, structuredFact.Value), // Combined for embeddings
				Category:        structuredFact.Category,
				Subject:         structuredFact.Subject,
				Attribute:       structuredFact.Attribute,
				Value:           structuredFact.Value,
				Sensitivity:     structuredFact.Sensitivity,
				Importance:      structuredFact.Importance,
				TemporalContext: structuredFact.TemporalContext,
			})
		}
	}

	if len(extractedFacts) == 0 {
		log.Printf("=== NO FACTS EXTRACTED from document %s ===", textDoc.ID())
	} else {
		log.Printf("=== Document %s: Extracted %d total facts ===", textDoc.ID(), len(extractedFacts))
	}

	return extractedFacts, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PrepareDocuments is THE function that does all document processing before memory storage.
// It handles chunking, truncation, and metadata preparation in one clean orchestrated flow.
func PrepareDocuments(docs []memory.Document, currentTime time.Time) ([]PreparedDocument, error) {
	var prepared []PreparedDocument
	var errors []error

	for _, doc := range docs {
		// Use the new polymorphic Chunk() method - much cleaner!
		chunks := doc.Chunk()

		for _, chunk := range chunks {
			var docType DocumentType
			switch chunk.(type) {
			case *memory.ConversationDocument:
				docType = DocumentTypeConversation
			case *memory.TextDocument:
				docType = DocumentTypeText
			default:
				errors = append(errors, fmt.Errorf("unknown document type: %T", chunk))
				continue
			}

			prep := addDocumentMetadata(chunk, docType, currentTime)
			prepared = append(prepared, prep)
		}
	}

	if len(errors) > 0 {
		return nil, aggregateErrors(errors)
	}

	return prepared, nil
}

// addDocumentMetadata adds all the metadata, timestamps, and speaker info. Pure function.
func addDocumentMetadata(doc memory.Document, docType DocumentType, currentTime time.Time) PreparedDocument {
	// Use document timestamp as primary source
	timestamp := currentTime
	if ts := doc.Timestamp(); ts != nil && !ts.IsZero() {
		timestamp = *ts
	}

	prep := PreparedDocument{
		Original:   doc,
		Type:       docType,
		Timestamp:  timestamp,
		DateString: getCurrentDateForPrompt(),
	}

	return prep
}
