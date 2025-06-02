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
func (s *WeaviateStorage) updateMemories(ctx context.Context, factContent string, speakerID string, currentSystemDate string, docEventDateStr string, sourceDoc memory.Document, isFromTextDocument bool) (string, *models.Object, error) {
	logContextEntity := "Speaker"
	logContextValue := speakerID

	if speakerID == "" {
		logContextEntity = "Document"
		logContextValue = "<document_context>"
	}

	s.logger.Infof("Processing fact for %s %s: \"%s...\"", logContextEntity, logContextValue, firstNChars(factContent, 70))

	// queryOptions := map[string]interface{}{"speakerID": speakerID} // Consider if query needs adaptation for speakerID == ""
	existingMemoriesResult, err := s.Query(ctx, factContent, nil)
	if err != nil {
		s.logger.Errorf("Error querying existing memories for fact processing for %s %s: %v. Fact: \"%s...\"", logContextEntity, logContextValue, err, firstNChars(factContent, 50))
		return "", nil, fmt.Errorf("querying existing memories: %w", err)
	}

	existingMemoriesContentForPrompt := []string{}
	existingMemoriesForPromptStr := "No existing relevant memories found."
	if len(existingMemoriesResult.Facts) > 0 {
		s.logger.Debugf("Retrieved %d existing memories for decision prompt for %s %s.", len(existingMemoriesResult.Facts), logContextEntity, logContextValue)
		for _, memFact := range existingMemoriesResult.Facts {
			memContext := fmt.Sprintf("ID: %s, Content: %s", memFact.ID, memFact.Content)
			// Potentially: memFact.Metadata["speakerID"] could be displayed here if relevant to the LLM's decision
			existingMemoriesContentForPrompt = append(existingMemoriesContentForPrompt, memContext)
		}
		existingMemoriesForPromptStr = strings.Join(existingMemoriesContentForPrompt, "\n---\n")
	} else {
		s.logger.Debugf("No existing relevant memories found for this fact for %s %s.", logContextEntity, logContextValue)
	}

	var decisionPromptBuilder strings.Builder

	// Use appropriate prompt based on document type
	if isFromTextDocument {
		decisionPromptBuilder.WriteString(TextMemoryUpdatePrompt)
		s.logger.Debugf("Using TextMemoryUpdatePrompt for %s %s", logContextEntity, logContextValue)
	} else {
		decisionPromptBuilder.WriteString(ConversationMemoryUpdatePrompt)
		s.logger.Debugf("Using ConversationMemoryUpdatePrompt for %s %s", logContextEntity, logContextValue)
	}

	decisionPromptBuilder.WriteString("\n\nContext:\n")
	decisionPromptBuilder.WriteString(fmt.Sprintf("Existing Memories for the primary user (if any, related to the new fact):\n%s\n\n", existingMemoriesForPromptStr))
	decisionPromptBuilder.WriteString(fmt.Sprintf("New Fact to consider for the primary user:\n%s\n\n", factContent))
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

	s.logger.Info("Calling LLM for Memory Update Decision.", "context", logContextValue, "fact_snippet", firstNChars(factContent, 30))
	llmDecisionResponse, err := s.completionsService.Completions(ctx, decisionMessages, memoryDecisionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("Error calling OpenAI for memory update decision for %s %s: %v. Fact: \"%s...\"", logContextEntity, logContextValue, err, firstNChars(factContent, 50))
		return "", nil, fmt.Errorf("LLM decision for memory update: %w", err)
	}

	chosenToolName := ""
	var toolArgsJSON string

	if len(llmDecisionResponse.ToolCalls) > 0 {
		chosenToolName = llmDecisionResponse.ToolCalls[0].Function.Name
		toolArgsJSON = llmDecisionResponse.ToolCalls[0].Function.Arguments
		s.logger.Infof("LLM chose memory action: '%s' for %s %s. Fact: \"%s...\"", chosenToolName, logContextEntity, logContextValue, firstNChars(factContent, 30))
	} else {
		s.logger.Warn("LLM made no tool call for memory decision. Defaulting to ADD for safety.", "context", logContextValue, "fact_snippet", firstNChars(factContent, 30))
		chosenToolName = AddMemoryToolName // Default to ADD
	}

	switch chosenToolName {
	case AddMemoryToolName:
		s.logger.Info("ACTION: ADD Memory", "context", logContextValue)
		newFactEmbedding64, embedErr := s.embeddingsService.Embedding(ctx, factContent, openAIEmbedModel)
		if embedErr != nil {
			s.logger.Errorf("Error generating embedding for new fact (ADD), skipping for %s %s: %v. Fact: \"%s...\"", logContextEntity, logContextValue, embedErr, firstNChars(factContent, 50))
			return AddMemoryToolName, nil, fmt.Errorf("embedding for ADD failed: %w", embedErr) // Return action name but nil object due to error
		}
		newFactEmbedding32 := make([]float32, len(newFactEmbedding64))
		for j, val := range newFactEmbedding64 {
			newFactEmbedding32[j] = float32(val)
		}

		factMetadata := make(map[string]string)
		for k, v := range sourceDoc.Metadata() {
			factMetadata[k] = v
		}
		if speakerID != "" { // Only add speakerID to metadata if it's not empty
			factMetadata["speakerID"] = speakerID
		}

		metadataBytes, jsonErr := json.Marshal(factMetadata)
		if jsonErr != nil {
			s.logger.Errorf("Error marshaling metadata for ADD for %s %s: %v. Storing with empty metadata.", logContextEntity, logContextValue, jsonErr)
			metadataBytes = []byte("{}")
		}

		data := map[string]interface{}{
			contentProperty:  factContent,
			metadataProperty: string(metadataBytes),
		}

		// Extract and store source and contactName as root-level properties for efficient indexing
		if source := sourceDoc.Source(); source != "" {
			data[sourceProperty] = source
		}
		if contactName, exists := factMetadata[contactNameProperty]; exists && contactName != "" {
			data[contactNameProperty] = contactName
		}
		// Use conversation date for timestamp
		if docEventDateStr != "Unknown" {
			if parsedTime, parseErr := time.Parse("2006-01-02", docEventDateStr); parseErr == nil {
				data[timestampProperty] = parsedTime.Format(time.RFC3339)
			}
		}

		addObject := &models.Object{
			Class:      ClassName,
			Properties: data,
			Vector:     newFactEmbedding32,
		}
		return AddMemoryToolName, addObject, nil

	case UpdateMemoryToolName:
		s.logger.Info("ACTION: UPDATE Memory", "context", logContextValue)
		var updateArgs UpdateToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &updateArgs); err != nil {
			s.logger.Errorf("Error unmarshalling UPDATE arguments for %s %s: %v. Args: %s", logContextEntity, logContextValue, err, toolArgsJSON)
			return UpdateMemoryToolName, nil, fmt.Errorf("unmarshal UPDATE args: %w", err)
		}
		s.logger.Debugf("Parsed UPDATE arguments: ID=%s, UpdatedMemory (snippet)='%s', Reason='%s'", updateArgs.MemoryID, firstNChars(updateArgs.UpdatedMemory, 100), updateArgs.Reason)

		originalDoc, getErr := s.GetByID(ctx, updateArgs.MemoryID) // GetByID is in evolvingmemory.go
		if getErr != nil || originalDoc == nil {
			s.logger.Errorf("Failed to get original document for UPDATE (ID: %s) for %s %s: %v. Skipping update.", updateArgs.MemoryID, logContextEntity, logContextValue, getErr)
			return UpdateMemoryToolName, nil, fmt.Errorf("get original for UPDATE failed: %w", getErr)
		}

		// Speaker/Context validation for UPDATE
		originalSpeakerInMeta, originalSpeakerMetaExists := originalDoc.Metadata()["speakerID"]

		if speakerID == "" { // Current context is document-level
			if originalSpeakerMetaExists && originalSpeakerInMeta != "" {
				// Document-level context attempting to update a memory that has a specific speaker
				s.logger.Warn("Document-level context attempted to UPDATE a memory with a specific speaker. Skipping update.",
					"target_id", updateArgs.MemoryID, "target_speaker_in_meta", originalSpeakerInMeta)
				return UpdateMemoryToolName, nil, fmt.Errorf("document-level context cannot update speaker-specific memory %s", updateArgs.MemoryID)
			}
		} else { // Current context is speaker-specific
			if !originalSpeakerMetaExists || originalSpeakerInMeta != speakerID {
				// Speaker-specific context attempting to update a memory that doesn't belong to them or is document-level
				s.logger.Warn("Speaker-specific context attempted to UPDATE a memory of a different/unspecified speaker. Skipping update.",
					"target_id", updateArgs.MemoryID, "target_speaker_in_meta", originalSpeakerInMeta, "original_speaker_meta_exists", originalSpeakerMetaExists,
					"current_processing_speaker", speakerID)
				return UpdateMemoryToolName, nil, fmt.Errorf("speaker ID mismatch or original speaker missing for UPDATE on memory %s", updateArgs.MemoryID)
			}
		}

		updatedEmbedding64, embedErr := s.embeddingsService.Embedding(ctx, updateArgs.UpdatedMemory, openAIEmbedModel)
		if embedErr != nil {
			s.logger.Errorf("Error generating embedding for updated memory (UPDATE) for %s %s: %v. Memory ID: %s", logContextEntity, logContextValue, embedErr, updateArgs.MemoryID)
			return UpdateMemoryToolName, nil, fmt.Errorf("embedding for UPDATE failed: %w", embedErr)
		}
		updatedEmbedding32 := make([]float32, len(updatedEmbedding64))
		for j, val := range updatedEmbedding64 {
			updatedEmbedding32[j] = float32(val)
		}

		updatedFactMetadata := make(map[string]string)
		for k, v := range originalDoc.Metadata() {
			updatedFactMetadata[k] = v
		}
		// Manage speakerID in updated metadata
		if speakerID != "" {
			updatedFactMetadata["speakerID"] = speakerID // Ensure current speakerID is set
		} else {
			delete(updatedFactMetadata, "speakerID") // If document-level, ensure no speakerID is present
		}

		// Parse conversation date for timestamp
		var updateTimestamp *time.Time
		if docEventDateStr != "Unknown" {
			if parsedTime, parseErr := time.Parse("2006-01-02", docEventDateStr); parseErr == nil {
				updateTimestamp = &parsedTime
			}
		}

		docToUpdate := memory.TextDocument{
			FieldID:        updateArgs.MemoryID,
			FieldContent:   updateArgs.UpdatedMemory,
			FieldTimestamp: updateTimestamp,
			FieldMetadata:  updatedFactMetadata,
			FieldTags:      originalDoc.Tags(), // Use Tags() method to access tags
		}

		if err = s.Update(ctx, updateArgs.MemoryID, docToUpdate, updatedEmbedding32); err != nil { // Update is in evolvingmemory.go
			s.logger.Errorf("Error performing UPDATE operation for %s %s: %v. Memory ID: %s", logContextEntity, logContextValue, err, updateArgs.MemoryID)
			return UpdateMemoryToolName, nil, fmt.Errorf("store UPDATE failed: %w", err)
		} else {
			s.logger.Infof("Fact UPDATED successfully for %s %s. Memory ID: %s", logContextEntity, logContextValue, updateArgs.MemoryID)
		}
		return UpdateMemoryToolName, nil, nil

	case DeleteMemoryToolName:
		// NOTE: Delete operations might also need speaker/context validation similar to UPDATE.
		// For now, keeping it simpler: if LLM decides to delete, we proceed.
		// Consider if a document-level context should be able to delete speaker-specific memories.
		s.logger.Info("ACTION: DELETE Memory", "context", logContextValue)
		var deleteArgs DeleteToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &deleteArgs); err != nil {
			s.logger.Errorf("Error unmarshalling DELETE arguments for %s %s: %v. Args: %s", logContextEntity, logContextValue, err, toolArgsJSON)
			return DeleteMemoryToolName, nil, fmt.Errorf("unmarshal DELETE args: %w", err)
		}
		s.logger.Debugf("Parsed DELETE arguments: ID=%s, Reason='%s'", deleteArgs.MemoryID, deleteArgs.Reason)

		// Speaker/Context validation for DELETE (similar to UPDATE)
		originalDocForDelete, getDelErr := s.GetByID(ctx, deleteArgs.MemoryID)
		if getDelErr != nil || originalDocForDelete == nil {
			s.logger.Warnf("Failed to get original document for DELETE validation (ID: %s) for %s %s: %v. Proceeding with delete cautiously.", deleteArgs.MemoryID, logContextEntity, logContextValue, getDelErr)
			// If we can't get the doc, we might still proceed with delete if LLM is trusted, or deny. For now, proceed.
		} else {
			// Use FieldMetadata() method to access metadata
			originalSpeakerInMetaDel, originalSpeakerMetaExistsDel := originalDocForDelete.Metadata()["speakerID"]
			if speakerID == "" { // Current context is document-level
				if originalSpeakerMetaExistsDel && originalSpeakerInMetaDel != "" {
					s.logger.Warn("Document-level context attempted to DELETE a memory with a specific speaker. Skipping delete.",
						"target_id", deleteArgs.MemoryID, "target_speaker_in_meta", originalSpeakerInMetaDel)
					return DeleteMemoryToolName, nil, fmt.Errorf("document-level context cannot delete speaker-specific memory %s", deleteArgs.MemoryID)
				}
			} else { // Current context is speaker-specific
				if !originalSpeakerMetaExistsDel || originalSpeakerInMetaDel != speakerID {
					s.logger.Warn("Speaker-specific context attempted to DELETE a memory of a different/unspecified speaker. Skipping delete.",
						"target_id", deleteArgs.MemoryID, "target_speaker_in_meta", originalSpeakerInMetaDel, "original_speaker_meta_exists", originalSpeakerMetaExistsDel,
						"current_processing_speaker", speakerID)
					return DeleteMemoryToolName, nil, fmt.Errorf("speaker ID mismatch or original speaker missing for DELETE on memory %s", deleteArgs.MemoryID)
				}
			}
		}

		if err = s.Delete(ctx, deleteArgs.MemoryID); err != nil { // Delete is in evolvingmemory.go
			s.logger.Errorf("Error performing DELETE operation for %s %s: %v. Memory ID: %s", logContextEntity, logContextValue, err, deleteArgs.MemoryID)
			return DeleteMemoryToolName, nil, fmt.Errorf("store DELETE failed: %w", err)
		} else {
			s.logger.Infof("Fact DELETED successfully for %s %s. Memory ID: %s", logContextEntity, logContextValue, deleteArgs.MemoryID)
		}
		return DeleteMemoryToolName, nil, nil

	case NoneMemoryToolName:
		s.logger.Info("ACTION: NONE", "context", logContextValue)
		var noneArgs NoneToolArguments // Assumes struct is in evolvingmemory.go
		if err = json.Unmarshal([]byte(toolArgsJSON), &noneArgs); err != nil {
			s.logger.Warnf("Error unmarshalling NONE arguments for %s %s: %v. Args: %s. Proceeding with NONE action.", logContextEntity, logContextValue, err, toolArgsJSON)
			// Non-fatal, proceed with NONE
		}
		s.logger.Infof("LLM chose NONE action for fact for %s %s. Reason: '%s'. Fact: \"%s...\"", logContextEntity, logContextValue, noneArgs.Reason, firstNChars(factContent, 50))
		return NoneMemoryToolName, nil, nil

	default:
		s.logger.Warn("LLM decision unrecognized or no tool called, and did not default to ADD earlier.", "chosen_tool", chosenToolName, "context", logContextValue)
		// This case should ideally not be reached if default to ADD is handled prior to switch.
		// However, as a fallback, treat as NONE to avoid unintended operations.
		return NoneMemoryToolName, nil, fmt.Errorf("unrecognized tool choice: %s", chosenToolName)
	}
}
