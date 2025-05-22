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

// updateMemories decides and executes memory operations (ADD, UPDATE, DELETE, NONE) for a given fact.
func (s *WeaviateStorage) updateMemories(ctx context.Context, factContent string, speakerID string, currentSystemDate string, docEventDateStr string, sessionDoc memory.TextDocument) (string, *models.Object, error) {
	s.logger.Infof("Processing fact for speaker %s: \"%s...\"", speakerID, firstNChars(factContent, 70))

	// queryOptions := map[string]interface{}{"speakerID": speakerID}
	existingMemoriesResult, err := s.Query(ctx, factContent)
	if err != nil {
		s.logger.Errorf("Error querying existing memories for fact processing for speaker %s: %v. Fact: \"%s...\"", speakerID, err, firstNChars(factContent, 50))
		return "", nil, fmt.Errorf("querying existing memories: %w", err)
	}

	existingMemoriesContentForPrompt := []string{}
	existingMemoriesForPromptStr := "No existing relevant memories found."
	if len(existingMemoriesResult.Documents) > 0 {
		s.logger.Debugf("Retrieved %d existing memories for decision prompt for speaker %s.", len(existingMemoriesResult.Documents), speakerID)
		for _, memDoc := range existingMemoriesResult.Documents {
			memContext := fmt.Sprintf("ID: %s, Content: %s", memDoc.ID, memDoc.Content)
			existingMemoriesContentForPrompt = append(existingMemoriesContentForPrompt, memContext)
		}
		existingMemoriesForPromptStr = strings.Join(existingMemoriesContentForPrompt, "\n---\n")
	} else {
		s.logger.Debug("No existing relevant memories found for this fact for speaker %s.", speakerID)
	}

	prompt := strings.ReplaceAll(DefaultUpdateMemoryPrompt, "{primary_speaker_name}", speakerID)
	prompt = strings.ReplaceAll(prompt, "{current_system_date}", currentSystemDate)
	prompt = strings.ReplaceAll(prompt, "{document_event_date}", docEventDateStr)

	var decisionPromptBuilder strings.Builder
	decisionPromptBuilder.WriteString(prompt)
	decisionPromptBuilder.WriteString("\n\nContext:\n")
	decisionPromptBuilder.WriteString(fmt.Sprintf("Existing Memories for %s (if any, related to the new fact):\n%s\n\n", speakerID, existingMemoriesForPromptStr))
	decisionPromptBuilder.WriteString(fmt.Sprintf("New Fact to consider for %s:\n%s\n\n", speakerID, factContent))
	decisionPromptBuilder.WriteString("Based on the guidelines and context, what action should be taken for the NEW FACT?")
	fullDecisionPrompt := decisionPromptBuilder.String()

	decisionMessages := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(fullDecisionPrompt)},
			},
		},
	}

	memoryDecisionToolsList := []openai.ChatCompletionToolParam{
		addMemoryTool, updateMemoryTool, deleteMemoryTool, noneMemoryTool,
	}

	s.logger.Info("Calling LLM for Memory Update Decision.", "speaker", speakerID, "fact_snippet", firstNChars(factContent, 30))
	llmDecisionResponse, err := s.completionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("Error calling OpenAI for memory update decision for speaker %s: %v. Fact: \"%s...\"", speakerID, err, firstNChars(factContent, 50))
		return "", nil, fmt.Errorf("LLM decision for memory update: %w", err)
	}

	chosenToolName := ""
	var toolArgsJSON string

	if len(llmDecisionResponse.ToolCalls) > 0 {
		chosenToolName = llmDecisionResponse.ToolCalls[0].Function.Name
		toolArgsJSON = llmDecisionResponse.ToolCalls[0].Function.Arguments
		s.logger.Infof("LLM chose memory action: '%s' for speaker %s. Fact: \"%s...\"", chosenToolName, speakerID, firstNChars(factContent, 30))
	} else {
		s.logger.Warn("LLM made no tool call for memory decision. Defaulting to ADD for safety.", "speaker", speakerID, "fact_snippet", firstNChars(factContent, 30))
		chosenToolName = AddMemoryToolName // Default to ADD
	}

	switch chosenToolName {
	case AddMemoryToolName:
		s.logger.Info("ACTION: ADD Memory", "speaker", speakerID)
		newFactEmbedding64, embedErr := s.embeddingsService.Embedding(ctx, factContent, openAIEmbedModel)
		if embedErr != nil {
			s.logger.Errorf("Error generating embedding for new fact (ADD), skipping for speaker %s: %v. Fact: \"%s...\"", speakerID, embedErr, firstNChars(factContent, 50))
			return AddMemoryToolName, nil, fmt.Errorf("embedding for ADD failed: %w", embedErr) // Return action name but nil object due to error
		}
		newFactEmbedding32 := make([]float32, len(newFactEmbedding64))
		for j, val := range newFactEmbedding64 {
			newFactEmbedding32[j] = float32(val)
		}

		factMetadata := make(map[string]string)
		for k, v := range sessionDoc.Metadata {
			if k != "dataset_speaker_a" && k != "dataset_speaker_b" { // Filter out helper metadata
				factMetadata[k] = v
			}
		}
		factMetadata["speakerID"] = speakerID

		metadataBytes, jsonErr := json.Marshal(factMetadata)
		if jsonErr != nil {
			s.logger.Errorf("Error marshaling metadata for ADD for speaker %s: %v. Storing with empty metadata.", speakerID, jsonErr)
			metadataBytes = []byte("{}")
		}

		data := map[string]interface{}{
			contentProperty:  factContent,
			metadataProperty: string(metadataBytes),
		}
		if sessionDoc.Timestamp != nil {
			data[timestampProperty] = sessionDoc.Timestamp.Format(time.RFC3339)
		}

		addObject := &models.Object{
			Class:      ClassName,
			Properties: data,
			Vector:     newFactEmbedding32,
		}
		return AddMemoryToolName, addObject, nil

	case UpdateMemoryToolName:
		s.logger.Info("ACTION: UPDATE Memory", "speaker", speakerID)
		var updateArgs UpdateToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &updateArgs); err != nil {
			s.logger.Errorf("Error unmarshalling UPDATE arguments for speaker %s: %v. Args: %s", speakerID, err, toolArgsJSON)
			return UpdateMemoryToolName, nil, fmt.Errorf("unmarshal UPDATE args: %w", err)
		}
		s.logger.Debugf("Parsed UPDATE arguments: ID=%s, UpdatedMemory (snippet)='%s', Reason='%s'", updateArgs.MemoryID, firstNChars(updateArgs.UpdatedMemory, 100), updateArgs.Reason)

		originalDoc, getErr := s.GetByID(ctx, updateArgs.MemoryID) // GetByID is in evolvingmemory.go
		if getErr != nil || originalDoc == nil {
			s.logger.Errorf("Failed to get original document for UPDATE (ID: %s) for speaker %s: %v. Skipping update.", updateArgs.MemoryID, speakerID, getErr)
			return UpdateMemoryToolName, nil, fmt.Errorf("get original for UPDATE failed: %w", getErr)
		}

		if originalDoc.Metadata["speakerID"] != "" && originalDoc.Metadata["speakerID"] != speakerID {
			s.logger.Warn("LLM attempted to UPDATE a memory of a different/unspecified speaker. Skipping update.",
				"target_id", updateArgs.MemoryID, "target_speaker_in_meta", originalDoc.Metadata["speakerID"],
				"current_processing_speaker", speakerID)
			return UpdateMemoryToolName, nil, fmt.Errorf("speaker ID mismatch for UPDATE on memory %s", updateArgs.MemoryID)
		}

		updatedEmbedding64, embedErr := s.embeddingsService.Embedding(ctx, updateArgs.UpdatedMemory, openAIEmbedModel)
		if embedErr != nil {
			s.logger.Error("Error generating embedding for updated memory (UPDATE)", "current_speaker", speakerID, "error", embedErr, "memory_id", updateArgs.MemoryID)
			return UpdateMemoryToolName, nil, fmt.Errorf("embedding for UPDATE failed: %w", embedErr)
		}
		updatedEmbedding32 := make([]float32, len(updatedEmbedding64))
		for j, val := range updatedEmbedding64 {
			updatedEmbedding32[j] = float32(val)
		}

		updatedFactMetadata := make(map[string]string)
		for k, v := range originalDoc.Metadata {
			updatedFactMetadata[k] = v
		}
		updatedFactMetadata["speakerID"] = speakerID // Ensure current speakerID is set

		docToUpdate := memory.TextDocument{
			ID:        updateArgs.MemoryID,
			Content:   updateArgs.UpdatedMemory,
			Timestamp: sessionDoc.Timestamp, // Update timestamp to the current sessionDoc's timestamp
			Metadata:  updatedFactMetadata,
			Tags:      originalDoc.Tags, // Preserve original tags unless LLM specifies changes
		}

		if err = s.Update(ctx, updateArgs.MemoryID, docToUpdate, updatedEmbedding32); err != nil { // Update is in evolvingmemory.go
			s.logger.Error("Error performing UPDATE operation", "current_speaker", speakerID, "error", err, "memory_id", updateArgs.MemoryID)
			return UpdateMemoryToolName, nil, fmt.Errorf("store UPDATE failed: %w", err)
		} else {
			s.logger.Infof("Fact UPDATED successfully for speaker %s. Memory ID: %s", speakerID, updateArgs.MemoryID)
		}
		return UpdateMemoryToolName, nil, nil

	case DeleteMemoryToolName:
		s.logger.Info("ACTION: DELETE Memory", "speaker", speakerID)
		var deleteArgs DeleteToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &deleteArgs); err != nil {
			s.logger.Errorf("Error unmarshalling DELETE arguments for speaker %s: %v. Args: %s", speakerID, err, toolArgsJSON)
			return DeleteMemoryToolName, nil, fmt.Errorf("unmarshal DELETE args: %w", err)
		}
		s.logger.Debugf("Parsed DELETE arguments: ID=%s, Reason='%s'", deleteArgs.MemoryID, deleteArgs.Reason)

		if err = s.Delete(ctx, deleteArgs.MemoryID); err != nil { // Delete is in evolvingmemory.go
			s.logger.Error("Error performing DELETE operation", "current_speaker", speakerID, "error", err, "memory_id", deleteArgs.MemoryID)
			return DeleteMemoryToolName, nil, fmt.Errorf("store DELETE failed: %w", err)
		} else {
			s.logger.Infof("Fact DELETED successfully for speaker %s. Memory ID: %s", speakerID, deleteArgs.MemoryID)
		}
		return DeleteMemoryToolName, nil, nil

	case NoneMemoryToolName:
		s.logger.Info("ACTION: NONE", "speaker", speakerID)
		var noneArgs NoneToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &noneArgs); err != nil {
			s.logger.Warnf("Error unmarshalling NONE arguments for speaker %s: %v. Args: %s. Proceeding with NONE action.", speakerID, err, toolArgsJSON)
			// Non-fatal, proceed with NONE
		}
		s.logger.Infof("LLM chose NONE action for fact for speaker %s. Reason: '%s'. Fact: \"%s...\"", speakerID, noneArgs.Reason, firstNChars(factContent, 50))
		return NoneMemoryToolName, nil, nil

	default:
		s.logger.Warn("LLM decision unrecognized or no tool called, and did not default to ADD earlier.", "chosen_tool", chosenToolName, "speaker", speakerID)
		// This case should ideally not be reached if default to ADD is handled prior to switch.
		// However, as a fallback, treat as NONE to avoid unintended operations.
		return NoneMemoryToolName, nil, fmt.Errorf("unrecognized tool choice: %s", chosenToolName)
	}
}
