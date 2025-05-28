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
)

// extractFactsForSpeaker extracts facts about a specific speaker from the conversation
func (s *WeaviateStorage) extractFactsForSpeaker(ctx context.Context, convDoc memory.ConversationDocument, speaker string, currentDate string, conversationDate string) ([]string, error) {
	// Build conversation context for the LLM
	var conversationContext strings.Builder
	for _, msg := range convDoc.Conversation.Conversation {
		conversationContext.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
	}

	// Prepare the prompt
	prompt := strings.ReplaceAll(FactExtractionPrompt, "{speaker_name}", speaker)
	prompt = strings.ReplaceAll(prompt, "{current_date}", currentDate)
	prompt = strings.ReplaceAll(prompt, "{conversation_date}", conversationDate)

	fullPrompt := prompt + "\n\nConversation:\n" + conversationContext.String()

	// Call LLM for fact extraction
	messages := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(fullPrompt),
				},
			},
		},
	}

	tools := []openai.ChatCompletionToolParam{extractFactsTool}

	response, err := s.completionsService.Completions(ctx, messages, tools, openAIChatModel)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse extracted facts
	var facts []string
	if len(response.ToolCalls) > 0 {
		for _, toolCall := range response.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					facts = append(facts, args.Facts...)
				}
			}
		}
	}

	return facts, nil
}

// processFactForSpeaker processes a single fact through the memory management system
func (s *WeaviateStorage) processFactForSpeaker(ctx context.Context, fact string, speaker string, currentDate string, conversationDate string, convDoc memory.ConversationDocument) (string, *models.Object, error) {
	// Query existing memories for this speaker
	existingMemories, err := s.Query(ctx, fact)
	if err != nil {
		return "", nil, fmt.Errorf("querying existing memories: %w", err)
	}

	// Build existing memories context
	var existingMemoriesStr string
	if len(existingMemories.Facts) > 0 {
		var memoryStrings []string
		for _, mem := range existingMemories.Facts {
			memoryStrings = append(memoryStrings, fmt.Sprintf("ID: %s, Content: %s", mem.ID, mem.Content))
		}
		existingMemoriesStr = strings.Join(memoryStrings, "\n")
	} else {
		existingMemoriesStr = "No existing memories found."
	}

	// Prepare memory update prompt
	prompt := strings.ReplaceAll(MemoryUpdatePrompt, "{speaker_name}", speaker)
	prompt = strings.ReplaceAll(prompt, "{existing_memories}", existingMemoriesStr)
	prompt = strings.ReplaceAll(prompt, "{new_fact}", fact)

	// Call LLM for memory decision
	messages := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(prompt),
				},
			},
		},
	}

	tools := []openai.ChatCompletionToolParam{
		addMemoryTool, updateMemoryTool, deleteMemoryTool, noneMemoryTool,
	}

	response, err := s.completionsService.Completions(ctx, messages, tools, openAIChatModel)
	if err != nil {
		return "", nil, fmt.Errorf("LLM decision failed: %w", err)
	}

	// Process the decision
	if len(response.ToolCalls) == 0 {
		s.logger.Warn("No tool call made, defaulting to ADD")
		return s.handleAddMemory(ctx, fact, speaker, convDoc)
	}

	toolCall := response.ToolCalls[0]

	switch toolCall.Function.Name {
	case AddMemoryToolName:
		return s.handleAddMemory(ctx, fact, speaker, convDoc)
	case UpdateMemoryToolName:
		return s.handleUpdateMemory(ctx, toolCall.Function.Arguments, fact, speaker, convDoc)
	case DeleteMemoryToolName:
		return s.handleDeleteMemory(ctx, toolCall.Function.Arguments, speaker)
	case NoneMemoryToolName:
		s.logger.Info("No memory action needed")
		return NoneMemoryToolName, nil, nil
	default:
		s.logger.Warnf("Unknown tool call: %s, defaulting to ADD", toolCall.Function.Name)
		return s.handleAddMemory(ctx, fact, speaker, convDoc)
	}
}

// handleAddMemory creates a new memory fact
func (s *WeaviateStorage) handleAddMemory(ctx context.Context, fact string, speaker string, convDoc memory.ConversationDocument) (string, *models.Object, error) {
	// Generate embedding
	embedding64, err := s.embeddingsService.Embedding(ctx, fact, openAIEmbedModel)
	if err != nil {
		return AddMemoryToolName, nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	embedding32 := make([]float32, len(embedding64))
	for i, val := range embedding64 {
		embedding32[i] = float32(val)
	}

	// Create memory fact
	memoryFact := memory.MemoryFact{
		Speaker:   speaker,
		Content:   fact,
		Timestamp: time.Now(),
		Source:    convDoc.Conversation.Source,
		Metadata: map[string]string{
			"conversation_id": convDoc.ID,
			"speaker":         speaker,
			"source":          convDoc.Conversation.Source,
		},
	}

	// Convert to storage format
	data := map[string]interface{}{
		"content":   memoryFact.Content,
		"speaker":   memoryFact.Speaker,
		"timestamp": memoryFact.Timestamp.Format(time.RFC3339),
		"source":    memoryFact.Source,
		"metadata":  mustMarshalJSON(memoryFact.Metadata),
	}

	object := &models.Object{
		Class:      ClassName,
		Properties: data,
		Vector:     embedding32,
	}

	return AddMemoryToolName, object, nil
}

// handleUpdateMemory updates an existing memory
func (s *WeaviateStorage) handleUpdateMemory(ctx context.Context, argsJSON string, newFact string, speaker string, convDoc memory.ConversationDocument) (string, *models.Object, error) {
	var args UpdateToolArguments
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return UpdateMemoryToolName, nil, fmt.Errorf("unmarshal UPDATE args: %w", err)
	}

	// Generate new embedding
	embedding64, err := s.embeddingsService.Embedding(ctx, args.UpdatedMemory, openAIEmbedModel)
	if err != nil {
		return UpdateMemoryToolName, nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	embedding32 := make([]float32, len(embedding64))
	for i, val := range embedding64 {
		embedding32[i] = float32(val)
	}

	// Update the memory
	data := map[string]interface{}{
		"content":   args.UpdatedMemory,
		"speaker":   speaker,
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    convDoc.Conversation.Source,
		"metadata": mustMarshalJSON(map[string]string{
			"conversation_id": convDoc.ID,
			"speaker":         speaker,
			"source":          convDoc.Conversation.Source,
			"update_reason":   args.Reason,
		}),
	}

	err = s.client.Data().Updater().
		WithClassName(ClassName).
		WithID(args.MemoryID).
		WithProperties(data).
		WithVector(embedding32).
		Do(ctx)

	if err != nil {
		return UpdateMemoryToolName, nil, fmt.Errorf("update failed: %w", err)
	}

	return UpdateMemoryToolName, nil, nil
}

// handleDeleteMemory deletes a memory
func (s *WeaviateStorage) handleDeleteMemory(ctx context.Context, argsJSON string, speaker string) (string, *models.Object, error) {
	var args DeleteToolArguments
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return DeleteMemoryToolName, nil, fmt.Errorf("unmarshal DELETE args: %w", err)
	}

	err := s.client.Data().Deleter().
		WithClassName(ClassName).
		WithID(args.MemoryID).
		Do(ctx)

	if err != nil {
		return DeleteMemoryToolName, nil, fmt.Errorf("delete failed: %w", err)
	}

	return DeleteMemoryToolName, nil, nil
}

// mustMarshalJSON marshals to JSON or returns empty object string
func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
