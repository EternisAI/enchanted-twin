package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const (
	// Maximum allowed document size in characters.
	maxDocumentSizeChars = 20000
)

// validateAndTruncateDocument validates document size and truncates if necessary.
func validateAndTruncateDocument(doc memory.Document) memory.Document {
	content := doc.Content()

	if len(content) <= maxDocumentSizeChars {
		return doc
	}

	truncatedContent := content[:maxDocumentSizeChars]

	switch d := doc.(type) {
	case *memory.TextDocument:
		newDoc := *d
		newDoc.FieldContent = truncatedContent
		return &newDoc
	case *memory.ConversationDocument:

		metadata := d.Metadata()
		if metadata == nil {
			metadata = make(map[string]string)
		}
		return &memory.TextDocument{
			FieldID:        d.FieldID,
			FieldContent:   truncatedContent,
			FieldTimestamp: d.Timestamp(),
			FieldSource:    d.FieldSource,
			FieldTags:      d.FieldTags,
			FieldMetadata:  metadata,
		}
	default:
		return doc
	}
}

// PrepareDocuments converts raw documents into prepared documents with extracted metadata.
func PrepareDocuments(docs []memory.Document, currentTime time.Time) ([]PreparedDocument, error) {
	prepared := make([]PreparedDocument, 0, len(docs))
	errors := make([]error, 0)

	for _, doc := range docs {
		p, err := prepareDocument(doc, currentTime)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to prepare document: %w", err))
			continue
		}
		prepared = append(prepared, p)
	}

	if len(errors) > 0 {
		return nil, aggregateErrors(errors)
	}

	return prepared, nil
}

func prepareDocument(doc memory.Document, currentTime time.Time) (PreparedDocument, error) {
	validatedDoc := validateAndTruncateDocument(doc)

	prepared := PreparedDocument{
		Original:   validatedDoc,
		Timestamp:  currentTime,
		DateString: getCurrentDateForPrompt(),
	}

	switch d := validatedDoc.(type) {
	case *memory.ConversationDocument:
		prepared.Type = DocumentTypeConversation
		// Use the User field as the speaker ID for conversation documents
		if d.User != "" {
			prepared.SpeakerID = d.User
		}
	case *memory.TextDocument:
		prepared.Type = DocumentTypeText
		// Text documents are document-level (no speaker)
		// In the current implementation, speakerID is hardcoded as "user" but
		// for the new pipeline we'll treat it as document-level
	default:
		return PreparedDocument{}, fmt.Errorf("unknown document type: %T", validatedDoc)
	}

	// Override timestamp if document provides one
	if ts := validatedDoc.Timestamp(); ts != nil && !ts.IsZero() {
		prepared.Timestamp = *ts
	}

	return prepared, nil
}

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

// ValidateMemoryOperation ensures speaker context rules are followed.
func ValidateMemoryOperation(rule ValidationRule) error {
	switch rule.Action {
	case UPDATE, DELETE:
		// Document-level context cannot modify speaker-specific memories
		if rule.IsDocumentLevel && rule.TargetSpeakerID != "" {
			return fmt.Errorf("document-level context cannot %s speaker-specific memory", rule.Action)
		}

		// Speaker-specific context can only modify their own memories
		if !rule.IsDocumentLevel && rule.TargetSpeakerID != rule.CurrentSpeakerID {
			return fmt.Errorf("speaker %s cannot %s memory belonging to speaker %s",
				rule.CurrentSpeakerID, rule.Action, rule.TargetSpeakerID)
		}
	}
	return nil
}

// CreateMemoryObject builds the Weaviate object for ADD operations.
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object {
	metadata := make(map[string]string)

	for k, v := range fact.Source.Original.Metadata() {
		metadata[k] = v
	}

	if fact.SpeakerID != "" {
		metadata["speakerID"] = fact.SpeakerID
	}

	// Add document reference metadata for backward compatibility
	metadata["sourceDocumentId"] = fact.Source.Original.ID()
	metadata["sourceDocumentType"] = string(fact.Source.Type)

	// Get tags from the source document
	tags := fact.Source.Original.Tags()

	// Prepare properties with new direct fields
	properties := map[string]interface{}{
		"content":            fact.Content,
		"metadataJson":       marshalMetadata(metadata), // Keep for backward compatibility
		"timestamp":          fact.Source.Timestamp.Format(time.RFC3339),
		"tags":               tags,
		"documentReferences": []string{},
	}

	// Extract and store source as direct field
	if source := fact.Source.Original.Source(); source != "" {
		properties["source"] = source
	} else if source, exists := metadata["source"]; exists {
		properties["source"] = source
	}

	// Extract and store speakerID as direct field
	if fact.SpeakerID != "" {
		properties["speakerID"] = fact.SpeakerID
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

// toFloat32 converts a slice of float64 to float32 for Weaviate compatibility.
func toFloat32(embedding []float64) []float32 {
	result := make([]float32, len(embedding))
	for i, v := range embedding {
		result[i] = float32(v)
	}
	return result
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
func ExtractFactsFromDocument(ctx context.Context, doc PreparedDocument, completionsService *ai.Service) ([]string, error) {
	currentDate := getCurrentDateForPrompt()

	switch doc.Type {
	case DocumentTypeConversation:
		convDoc, ok := doc.Original.(*memory.ConversationDocument)
		if !ok {
			return nil, fmt.Errorf("document is not a ConversationDocument")
		}
		return extractFactsFromConversation(ctx, *convDoc, doc.SpeakerID, currentDate, doc.DateString, completionsService)

	case DocumentTypeText:
		textDoc, ok := doc.Original.(*memory.TextDocument)
		if !ok {
			return nil, fmt.Errorf("document is not a TextDocument")
		}
		return extractFactsFromTextDocument(ctx, *textDoc, doc.SpeakerID, currentDate, doc.DateString, completionsService)

	default:
		return nil, fmt.Errorf("unknown document type: %s", doc.Type)
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
func SearchSimilarMemories(ctx context.Context, fact string, speakerID string, storage storage.Interface) ([]ExistingMemory, error) {
	result, err := storage.Query(ctx, fact, nil)
	if err != nil {
		return nil, fmt.Errorf("querying similar memories: %w", err)
	}

	memories := make([]ExistingMemory, 0, len(result.Documents))
	for _, doc := range result.Documents {
		mem := ExistingMemory{
			ID:       doc.ID(),
			Content:  doc.Content(),
			Metadata: doc.Metadata(),
		}
		if doc.Timestamp() != nil {
			mem.Timestamp = *doc.Timestamp()
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// normalizeAndFormatConversation replaces primary user name with "primaryUser" and returns JSON.
func normalizeAndFormatConversation(convDoc memory.ConversationDocument) (string, error) {
	normalized := convDoc

	// Replace primary user name in conversation messages
	for i, msg := range normalized.Conversation {
		if msg.Speaker == convDoc.User {
			normalized.Conversation[i].Speaker = "primaryUser"
		}
	}

	// Replace primary user name in people list
	for i, person := range normalized.People {
		if person == convDoc.User {
			normalized.People[i] = "primaryUser"
		}
	}

	// Update the user field
	normalized.User = "primaryUser"

	// Just JSON marshal the whole thing
	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to marshal conversation: %w", err)
	}

	return string(jsonBytes), nil
}

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, speakerID string, currentSystemDate string, docEventDateStr string, completionsService *ai.Service) ([]string, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	// Normalize and format as JSON
	conversationJSON, err := normalizeAndFormatConversation(convDoc)
	if err != nil {
		return nil, fmt.Errorf("conversation normalization error: %w", err)
	}

	if len(convDoc.Conversation) == 0 {
		return []string{}, nil
	}

	// Use new structured fact extraction prompt for conversations
	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(conversationJSON),
	}

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		return nil, fmt.Errorf("LLM completion error for speaker %s, conversation %s: %w", speakerID, convDoc.ID(), err)
	}

	var extractedFacts []string
	for _, toolCall := range llmResponse.ToolCalls {
		if toolCall.Function.Name != ExtractFactsToolName {
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			continue
		}

		// Convert structured facts to enriched content strings for better semantic search
		for _, structuredFact := range args.Facts {
			// Create a comprehensive searchable string that includes context about the user
			var contentParts []string

			// Add subject context (helps with user-related queries)
			if structuredFact.Subject != "" && structuredFact.Subject != "user" {
				contentParts = append(contentParts, structuredFact.Subject)
			}

			// Add the main value
			contentParts = append(contentParts, structuredFact.Value)

			// Add attribute for context
			if structuredFact.Attribute != "" {
				contentParts = append(contentParts, "("+structuredFact.Attribute+")")
			}

			// Add temporal context
			if structuredFact.TemporalContext != nil && *structuredFact.TemporalContext != "" {
				contentParts = append(contentParts, "("+*structuredFact.TemporalContext+")")
			}

			// For user-related facts, ensure they're findable in user queries
			factString := strings.Join(contentParts, " ")
			if structuredFact.Subject == "user" || strings.Contains(strings.ToLower(factString), "user") {
				factString = "User: " + factString
			}

			extractedFacts = append(extractedFacts, factString)
		}
	}

	return extractedFacts, nil
}

// extractFactsFromTextDocument extracts facts from text documents.
func extractFactsFromTextDocument(ctx context.Context, textDoc memory.TextDocument, speakerID string, currentSystemDate string, docEventDateStr string, completionsService *ai.Service) ([]string, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := textDoc.Content()
	if content == "" {
		return []string{}, nil
	}

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		return nil, fmt.Errorf("LLM completion error for speaker %s, document %s: %w", speakerID, textDoc.ID(), err)
	}

	var extractedFacts []string
	for _, toolCall := range llmResponse.ToolCalls {
		if toolCall.Function.Name != ExtractFactsToolName {
			continue
		}

		var args ExtractStructuredFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			continue
		}

		// Convert structured facts to enriched content strings for better semantic search
		for _, structuredFact := range args.Facts {
			// Create a comprehensive searchable string that includes context about the user
			var contentParts []string

			// Add subject context (helps with user-related queries)
			if structuredFact.Subject != "" && structuredFact.Subject != "user" {
				contentParts = append(contentParts, structuredFact.Subject)
			}

			// Add the main value
			contentParts = append(contentParts, structuredFact.Value)

			// Add attribute for context
			if structuredFact.Attribute != "" {
				contentParts = append(contentParts, "("+structuredFact.Attribute+")")
			}

			// Add temporal context
			if structuredFact.TemporalContext != nil && *structuredFact.TemporalContext != "" {
				contentParts = append(contentParts, "("+*structuredFact.TemporalContext+")")
			}

			// For user-related facts, ensure they're findable in user queries
			factString := strings.Join(contentParts, " ")
			if structuredFact.Subject == "user" || strings.Contains(strings.ToLower(factString), "user") {
				factString = "User: " + factString
			}

			extractedFacts = append(extractedFacts, factString)
		}
	}

	return extractedFacts, nil
}
