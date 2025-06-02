package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// factExtractorAdapter wraps WeaviateStorage to implement FactExtractor interface.
type factExtractorAdapter struct {
	storage *WeaviateStorage
}

// NewFactExtractor creates a new FactExtractor implementation.
func NewFactExtractor(storage *WeaviateStorage) (FactExtractor, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if storage.completionsService == nil {
		return nil, fmt.Errorf("completions service not initialized")
	}
	return &factExtractorAdapter{storage: storage}, nil
}

// ExtractFacts routes to the appropriate extraction method based on document type.
func (f *factExtractorAdapter) ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error) {
	currentDate := getCurrentDateForPrompt()

	switch doc.Type {
	case DocumentTypeConversation:
		convDoc, ok := doc.Original.(*memory.ConversationDocument)
		if !ok {
			return nil, fmt.Errorf("document is not a ConversationDocument")
		}

		return f.storage.extractFactsFromConversation(ctx, *convDoc, doc.SpeakerID, currentDate, doc.DateString)

	case DocumentTypeText:
		textDoc, ok := doc.Original.(*memory.TextDocument)
		if !ok {
			return nil, fmt.Errorf("document is not a TextDocument")
		}

		return f.storage.extractFactsFromTextDocument(ctx, *textDoc, doc.SpeakerID, currentDate, doc.DateString)

	default:
		return nil, fmt.Errorf("unknown document type: %s", doc.Type)
	}
}

// memoryOperationsAdapter wraps WeaviateStorage to implement MemoryOperations interface.
type memoryOperationsAdapter struct {
	storage *WeaviateStorage
}

// NewMemoryOperations creates a new MemoryOperations implementation.
func NewMemoryOperations(storage *WeaviateStorage) (MemoryOperations, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if storage.client == nil {
		return nil, fmt.Errorf("weaviate client not initialized")
	}
	if storage.completionsService == nil {
		return nil, fmt.Errorf("completions service not initialized")
	}
	return &memoryOperationsAdapter{storage: storage}, nil
}

func (m *memoryOperationsAdapter) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
	result, err := m.storage.Query(ctx, fact)
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

func (m *memoryOperationsAdapter) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
	fullDecisionPrompt := m.buildDecisionPrompt(fact, similar)

	decisionMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fullDecisionPrompt),
	}

	memoryDecisionToolsList := []openai.ChatCompletionToolParam{
		addMemoryTool, updateMemoryTool, deleteMemoryTool, noneMemoryTool,
	}

	llmResponse, err := m.storage.completionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, openAIChatModel)
	if err != nil {
		return MemoryDecision{}, fmt.Errorf("LLM decision for memory update: %w", err)
	}

	return m.parseToolCallResponse(llmResponse)
}

func (m *memoryOperationsAdapter) buildDecisionPrompt(fact string, similar []ExistingMemory) string {
	existingMemoriesContentForPrompt := []string{}
	existingMemoriesForPromptStr := "No existing relevant memories found."

	if len(similar) > 0 {
		for _, mem := range similar {
			memContext := fmt.Sprintf("ID: %s, Content: %s", mem.ID, mem.Content)
			existingMemoriesContentForPrompt = append(existingMemoriesContentForPrompt, memContext)
		}
		existingMemoriesForPromptStr = strings.Join(existingMemoriesContentForPrompt, "\n---\n")
	}

	var decisionPromptBuilder strings.Builder
	// Default to conversation prompt - in real pipeline this would be determined by document type
	decisionPromptBuilder.WriteString(ConversationMemoryUpdatePrompt)
	decisionPromptBuilder.WriteString("\n\nContext:\n")
	decisionPromptBuilder.WriteString(fmt.Sprintf("Existing Memories for the primary user (if any, related to the new fact):\n%s\n\n", existingMemoriesForPromptStr))
	decisionPromptBuilder.WriteString(fmt.Sprintf("New Fact to consider for the primary user:\n%s\n\n", fact))
	decisionPromptBuilder.WriteString("Based on the guidelines and context, what action should be taken for the NEW FACT?")

	return decisionPromptBuilder.String()
}

func (m *memoryOperationsAdapter) parseToolCallResponse(llmResponse openai.ChatCompletionMessage) (MemoryDecision, error) {
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
			// Non-fatal for NONE
			m.storage.logger.Warnf("Error unmarshaling NONE arguments: %v", err)
		}
		decision.Reason = args.Reason
	}

	return decision, nil
}

// UpdateMemory updates an existing memory with new content.
func (m *memoryOperationsAdapter) UpdateMemory(ctx context.Context, memoryID string, newContent string, embedding []float32) error {
	originalDoc, err := m.storage.storage.GetByID(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("getting original document for update: %w", err)
	}

	// Create updated document preserving metadata
	docToUpdate := memory.TextDocument{
		FieldID:        memoryID,
		FieldContent:   newContent,
		FieldTimestamp: originalDoc.Timestamp(),
		FieldMetadata:  originalDoc.Metadata(),
		FieldTags:      originalDoc.Tags(),
	}

	return m.storage.storage.Update(ctx, memoryID, docToUpdate, embedding)
}

// DeleteMemory removes a memory by ID.
func (m *memoryOperationsAdapter) DeleteMemory(ctx context.Context, memoryID string) error {
	return m.storage.storage.Delete(ctx, memoryID)
}
