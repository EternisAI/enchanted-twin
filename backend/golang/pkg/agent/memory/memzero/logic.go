package newmem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

	// Tool Names (matching function names in tools.go)
	AddMemoryToolName    = "ADD"
	UpdateMemoryToolName = "UPDATE"
	DeleteMemoryToolName = "DELETE"
	NoneMemoryToolName   = "NONE"
	ExtractFactsToolName = "EXTRACT_FACTS"
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

// ExtractFactsToolArguments matches the parameters defined in extractFactsTool in tools.go
type ExtractFactsToolArguments struct {
	Facts []string `json:"facts"`
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
	properties := []*models.Property{
		{
			Name:     contentProperty,
			DataType: []string{"text"},
		},
		{
			Name:     timestampProperty, // Added for storing the event timestamp of the memory
			DataType: []string{"date"},
		},
		{
			Name:     tagsProperty,       // For categorization or keyword tagging
			DataType: []string{"text[]"}, // Array of strings
		},
		{
			Name:     metadataProperty, // For any other structured metadata
			DataType: []string{"text"}, // Storing as JSON string
		},
	}

	classObj := &models.Class{
		Class:      className,
		Properties: properties,
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

	batcher := s.client.Batch().ObjectsBatcher()
	var objectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		return nil
	}

	factExtractionTools := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}
	memoryDecisionTools := []openai.ChatCompletionToolParam{
		addMemoryTool,
		updateMemoryTool,
		deleteMemoryTool,
		noneMemoryTool,
	}

	currentSystemDate := getCurrentDateForPrompt()

	for i, sessionDoc := range documents { // Each 'sessionDoc' is an aggregated session
		s.logger.Infof("Processing session document %d of %d. Session Doc ID (if any): '%s'", i+1, totalDocs, sessionDoc.ID)

		var speakerIterationCandidates []string
		if speakerA, ok := sessionDoc.Metadata["dataset_speaker_a"]; ok && speakerA != "" {
			speakerIterationCandidates = append(speakerIterationCandidates, speakerA)
		}
		if speakerB, ok := sessionDoc.Metadata["dataset_speaker_b"]; ok && speakerB != "" {
			addSpeakerB := true
			if len(speakerIterationCandidates) == 1 && speakerIterationCandidates[0] == speakerB {
				addSpeakerB = false
			}
			if addSpeakerB {
				speakerIterationCandidates = append(speakerIterationCandidates, speakerB)
			}
		}

		if len(speakerIterationCandidates) > 0 {
			s.logger.Debugf("Identified speaker iteration candidates: %v", speakerIterationCandidates)
		} else {
			s.logger.Warn("Could not identify speakers from 'dataset_speaker_a' or 'dataset_speaker_b' in sessionDoc.Metadata. Fact extraction might be limited or speaker-agnostic.")
		}

		if len(speakerIterationCandidates) == 0 {
			s.logger.Warn("No speaker candidates identified for session doc ID '%s'. Skipping speaker-focused fact extraction for this document.", sessionDoc.ID)
			if progressChan != nil {
				progressChan <- memory.ProgressUpdate{
					Processed: i + 1,
					Total:     totalDocs,
				}
			}
			continue
		}

		factsProcessedForSession := 0

		for _, speakerID := range speakerIterationCandidates {
			currentSpeakerIDForLog := speakerID
			s.logger.Infof("== Starting Fact Extraction for Speaker: %s == (Session Doc %d of %d)", currentSpeakerIDForLog, i+1, totalDocs)

			speakerFactExtractionSystemPrompt := strings.ReplaceAll(SpeakerFocusedFactExtractionPrompt, "{primary_speaker_name}", speakerID)
			speakerFactExtractionSystemPrompt = strings.ReplaceAll(speakerFactExtractionSystemPrompt, "{current_system_date}", currentSystemDate)
			docEventDateStr := "Unknown"
			if sessionDoc.Timestamp != nil && !sessionDoc.Timestamp.IsZero() {
				docEventDateStr = sessionDoc.Timestamp.Format("2006-01-02")
			}
			speakerFactExtractionSystemPrompt = strings.ReplaceAll(speakerFactExtractionSystemPrompt, "{document_event_date}", docEventDateStr)

			llmMsgs := []openai.ChatCompletionMessageParamUnion{
				{
					OfSystem: &openai.ChatCompletionSystemMessageParam{
						Content: openai.ChatCompletionSystemMessageParamContentUnion{
							OfString: openai.String(speakerFactExtractionSystemPrompt),
						},
					},
				},
			}

			conversationLines := strings.Split(strings.TrimSpace(sessionDoc.Content), "\\n")

			parsedTurnsCount := 0
			for _, line := range conversationLines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine == "" {
					continue
				}
				parts := strings.SplitN(trimmedLine, ":", 2)
				if len(parts) < 2 {
					s.logger.Warnf("Skipping malformed line (no speaker colon): '%s'", trimmedLine)
					continue
				}
				turnSpeaker := strings.TrimSpace(parts[0])
				turnText := strings.TrimSpace(parts[1])

				if turnText == "" {
					continue
				}

				parsedTurnsCount++
				fullTurnContent := fmt.Sprintf("%s: %s", turnSpeaker, turnText)

				if turnSpeaker == speakerID {
					llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
						OfUser: &openai.ChatCompletionUserMessageParam{
							Content: openai.ChatCompletionUserMessageParamContentUnion{
								OfString: openai.String(fullTurnContent),
							},
						},
					})
				} else {
					llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
						OfAssistant: &openai.ChatCompletionAssistantMessageParam{
							Content: openai.ChatCompletionAssistantMessageParamContentUnion{
								OfString: openai.String(fullTurnContent),
							},
						},
					})
				}
			}
			if parsedTurnsCount == 0 && len(conversationLines) > 0 {
				s.logger.Warnf("No valid turns were parsed from %d conversation lines for speaker %s. LLM might not have sufficient context.", len(conversationLines), currentSpeakerIDForLog)
			}
			if len(llmMsgs) <= 1 {
				s.logger.Warnf("llmMsgs only contains system prompt for speaker %s. No conversational turns added. Skipping LLM call for fact extraction.", currentSpeakerIDForLog)
				continue
			}

			s.logger.Debugf("Calling LLM for Speaker-Focused Fact Extraction (%s). Model: %s, Tools: %d tools", currentSpeakerIDForLog, openAIChatModel, len(factExtractionTools))

			llmResponse, err := s.aiService.Completions(ctx, llmMsgs, factExtractionTools, openAIChatModel)

			if err != nil {
				s.logger.Errorf("LLM completion error during fact extraction for speaker %s: %v", currentSpeakerIDForLog, err)
				continue
			}

			var extractedFacts []string
			if len(llmResponse.ToolCalls) > 0 {
				for _, toolCall := range llmResponse.ToolCalls {
					if toolCall.Function.Name == ExtractFactsToolName {
						var args ExtractFactsToolArguments
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
							extractedFacts = append(extractedFacts, args.Facts...)
							s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s.", len(args.Facts), currentSpeakerIDForLog)
						} else {
							s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s: %v. Arguments: %s", currentSpeakerIDForLog, err, toolCall.Function.Arguments)
						}
					} else {
						s.logger.Warnf("LLM called an unexpected tool '%s' during fact extraction for speaker %s.", toolCall.Function.Name, currentSpeakerIDForLog)
					}
				}
			} else {
				s.logger.Info("LLM response for fact extraction for speaker %s did not contain tool calls. No facts extracted by tool.", currentSpeakerIDForLog)
			}

			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted by LLM for speaker %s. Skipping memory operations for this speaker for this session doc.", currentSpeakerIDForLog)
				if progressChan != nil {
					s.logger.Debugf("Progress update: No specific facts to process for speaker %s in session %d", speakerID, i+1)
				}
				continue
			}

			s.logger.Infof("Total facts to process for speaker '%s': %d", currentSpeakerIDForLog, len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				factsProcessedForSession++
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "speaker", currentSpeakerIDForLog)
					continue
				}
				s.logger.Infof("Processing fact %d for speaker %s: \"%s...\"", factIdx+1, currentSpeakerIDForLog, firstNChars(factContent, 70))

				queryOptions := map[string]interface{}{"speakerID": currentSpeakerIDForLog}
				existingMemoriesResult, err := s.Query(ctx, factContent, queryOptions)
				if err != nil {
					s.logger.Errorf("Error querying existing memories for fact processing for speaker %s: %v. Fact: \"%s...\"", currentSpeakerIDForLog, err, firstNChars(factContent, 50))
					continue
				}

				existingMemoriesContentForPrompt := []string{}
				existingMemoriesForPromptStr := "No existing relevant memories found."
				if existingMemoriesResult != nil && len(existingMemoriesResult.Documents) > 0 {
					s.logger.Debugf("Retrieved %d existing memories for decision prompt for speaker %s.", len(existingMemoriesResult.Documents), currentSpeakerIDForLog)
					for _, memDoc := range existingMemoriesResult.Documents {
						memContext := fmt.Sprintf("ID: %s, Content: %s", memDoc.ID, memDoc.Content)
						existingMemoriesContentForPrompt = append(existingMemoriesContentForPrompt, memContext)
					}
					existingMemoriesForPromptStr = strings.Join(existingMemoriesContentForPrompt, "\n---\n")
				} else {
					s.logger.Debug("No existing relevant memories found for this fact for speaker %s.", currentSpeakerIDForLog)
				}

				var decisionPromptBuilder strings.Builder
				decisionPromptBuilder.WriteString(DefaultUpdateMemoryPrompt)
				decisionPromptBuilder.WriteString("\n\n")

				var contextSb strings.Builder
				contextSb.WriteString(fmt.Sprintf("Current System Date: %s\n", currentSystemDate))
				contextSb.WriteString(fmt.Sprintf("Document Event Date (when the new information occurred): %s\n", docEventDateStr))
				contextSb.WriteString(fmt.Sprintf("Primary Speaker for this fact: %s\n\n", currentSpeakerIDForLog))
				contextSb.WriteString(fmt.Sprintf("Existing Memories for %s (if any, related to the new fact):\n%s\n\n", currentSpeakerIDForLog, existingMemoriesForPromptStr))
				contextSb.WriteString(fmt.Sprintf("New Fact to consider for %s:\n%s\n\n", currentSpeakerIDForLog, factContent))
				contextSb.WriteString("Based on the guidelines, what action should be taken for the NEW FACT?")
				dynamicContextPart := contextSb.String()

				decisionPromptBuilder.WriteString(dynamicContextPart)
				fullDecisionPrompt := decisionPromptBuilder.String()

				decisionMessages := []openai.ChatCompletionMessageParamUnion{
					{
						OfSystem: &openai.ChatCompletionSystemMessageParam{
							Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(fullDecisionPrompt)},
						},
					},
				}

				s.logger.Info("Calling LLM for Memory Update Decision.", "speaker", currentSpeakerIDForLog, "fact_snippet", firstNChars(factContent, 30))
				llmDecisionResponse, err := s.aiService.Completions(ctx, decisionMessages, memoryDecisionTools, openAIChatModel)
				if err != nil {
					s.logger.Errorf("Error calling OpenAI for memory update decision for speaker %s: %v. Fact: \"%s...\"", currentSpeakerIDForLog, err, firstNChars(factContent, 50))
					continue
				}

				chosenToolName := ""
				var toolArgsJSON string

				if len(llmDecisionResponse.ToolCalls) > 0 {
					chosenToolName = llmDecisionResponse.ToolCalls[0].Function.Name
					toolArgsJSON = llmDecisionResponse.ToolCalls[0].Function.Arguments
					s.logger.Infof("LLM chose memory action: '%s' for speaker %s. Fact: \"%s...\"", chosenToolName, currentSpeakerIDForLog, firstNChars(factContent, 30))
				} else {
					s.logger.Warn("LLM made no tool call for memory decision. Defaulting to ADD for safety.", "speaker", currentSpeakerIDForLog, "fact_snippet", firstNChars(factContent, 30))
				}

				switch chosenToolName {
				case AddMemoryToolName:
					s.logger.Info("ACTION: ADD Memory", "speaker", currentSpeakerIDForLog)
					newFactEmbedding64, embedErr := s.aiService.Embedding(ctx, factContent, openAIEmbedModel)
					if embedErr != nil {
						s.logger.Errorf("Error generating embedding for new fact (ADD), skipping for speaker %s: %v. Fact: \"%s...\"", currentSpeakerIDForLog, embedErr, firstNChars(factContent, 50))
						continue
					}
					newFactEmbedding32 := make([]float32, len(newFactEmbedding64))
					for j, val := range newFactEmbedding64 {
						newFactEmbedding32[j] = float32(val)
					}

					factMetadata := make(map[string]string)
					for k, v := range sessionDoc.Metadata {
						if k != "dataset_speaker_a" && k != "dataset_speaker_b" {
							factMetadata[k] = v
						}
					}
					factMetadata["speakerID"] = currentSpeakerIDForLog

					metadataBytes, jsonErr := json.Marshal(factMetadata)
					if jsonErr != nil {
						s.logger.Errorf("Error marshalling metadata for ADD for speaker %s: %v. Storing with empty metadata.", currentSpeakerIDForLog, jsonErr)
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
						Class:      className,
						Properties: data,
						Vector:     newFactEmbedding32,
					}
					batcher.WithObjects(addObject)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for speaker %s. Fact: \"%s...\"", currentSpeakerIDForLog, firstNChars(factContent, 50))

				case UpdateMemoryToolName:
					s.logger.Info("ACTION: UPDATE Memory", "speaker", currentSpeakerIDForLog)
					var updateArgs UpdateToolArguments
					if err = json.Unmarshal([]byte(toolArgsJSON), &updateArgs); err != nil {
						s.logger.Errorf("Error unmarshalling UPDATE arguments for speaker %s: %v. Args: %s", currentSpeakerIDForLog, err, toolArgsJSON)
						continue
					}
					s.logger.Debugf("Parsed UPDATE arguments: ID=%s, UpdatedMemory (snippet)='%s', Reason='%s'", updateArgs.MemoryID, firstNChars(updateArgs.UpdatedMemory, 100), updateArgs.Reason)

					originalDoc, getErr := s.GetByID(ctx, updateArgs.MemoryID)
					if getErr != nil || originalDoc == nil {
						s.logger.Errorf("Failed to get original document for UPDATE (ID: %s) for speaker %s: %v. Skipping update.", updateArgs.MemoryID, currentSpeakerIDForLog, getErr)
						continue
					}

					if originalDoc.Metadata["speakerID"] != "" && originalDoc.Metadata["speakerID"] != currentSpeakerIDForLog {
						s.logger.Warn("LLM attempted to UPDATE a memory of a different/unspecified speaker. Skipping update.",
							"target_id", updateArgs.MemoryID, "target_speaker_in_meta", originalDoc.Metadata["speakerID"],
							"current_processing_speaker", currentSpeakerIDForLog)
						continue
					}

					updatedEmbedding64, embedErr := s.aiService.Embedding(ctx, updateArgs.UpdatedMemory, openAIEmbedModel)
					if embedErr != nil {
						s.logger.Errorf("Error generating embedding for updated memory (UPDATE) for speaker %s, skipping: %v", currentSpeakerIDForLog, embedErr, "memory_id", updateArgs.MemoryID)
						continue
					}
					updatedEmbedding32 := make([]float32, len(updatedEmbedding64))
					for j, val := range updatedEmbedding64 {
						updatedEmbedding32[j] = float32(val)
					}

					updatedFactMetadata := make(map[string]string)
					for k, v := range originalDoc.Metadata {
						updatedFactMetadata[k] = v
					}
					updatedFactMetadata["speakerID"] = currentSpeakerIDForLog

					docToUpdate := memory.TextDocument{
						ID:        updateArgs.MemoryID,
						Content:   updateArgs.UpdatedMemory,
						Timestamp: sessionDoc.Timestamp,
						Metadata:  updatedFactMetadata,
					}

					if err = s.Update(ctx, updateArgs.MemoryID, docToUpdate, updatedEmbedding32); err != nil {
						s.logger.Errorf("Error performing UPDATE operation for speaker %s: %v", currentSpeakerIDForLog, err, "memory_id", updateArgs.MemoryID)
					} else {
						s.logger.Infof("Fact UPDATED successfully for speaker %s. Memory ID: %s", currentSpeakerIDForLog, updateArgs.MemoryID)
					}

				case DeleteMemoryToolName:
					s.logger.Info("ACTION: DELETE Memory", "speaker", currentSpeakerIDForLog)
					var deleteArgs DeleteToolArguments
					if err = json.Unmarshal([]byte(toolArgsJSON), &deleteArgs); err != nil {
						s.logger.Errorf("Error unmarshalling DELETE arguments for speaker %s: %v. Args: %s", currentSpeakerIDForLog, err, toolArgsJSON)
						continue
					}
					s.logger.Debugf("Parsed DELETE arguments: ID=%s, Reason='%s'", deleteArgs.MemoryID, deleteArgs.Reason)

					if err = s.Delete(ctx, deleteArgs.MemoryID); err != nil {
						s.logger.Errorf("Error performing DELETE operation for speaker %s: %v", currentSpeakerIDForLog, err, "memory_id", deleteArgs.MemoryID)
					} else {
						s.logger.Infof("Fact DELETED successfully for speaker %s. Memory ID: %s", currentSpeakerIDForLog, deleteArgs.MemoryID)
					}

				case NoneMemoryToolName:
					s.logger.Info("ACTION: NONE", "speaker", currentSpeakerIDForLog)
					var noneArgs NoneToolArguments
					if err = json.Unmarshal([]byte(toolArgsJSON), &noneArgs); err != nil {
						s.logger.Warnf("Error unmarshalling NONE arguments for speaker %s: %v. Args: %s. Proceeding with NONE action.", currentSpeakerIDForLog, err, toolArgsJSON)
					}
					s.logger.Infof("LLM chose NONE action for fact for speaker %s. Reason: '%s'. Fact: \"%s...\"", currentSpeakerIDForLog, noneArgs.Reason, firstNChars(factContent, 50))

				default:
					s.logger.Warn("ACTION: DEFAULT to ADD (LLM decision unrecognized or no tool called)", "chosen_tool", chosenToolName, "speaker", currentSpeakerIDForLog)
					newFactEmbedding64, embedErr := s.aiService.Embedding(ctx, factContent, openAIEmbedModel)
					if embedErr != nil {
						s.logger.Errorf("Error generating embedding for default ADD fact for speaker %s, skipping: %v. Fact: \"%s...\"", currentSpeakerIDForLog, embedErr, firstNChars(factContent, 50))
						continue
					}
					newFactEmbedding32 := make([]float32, len(newFactEmbedding64))
					for j, val := range newFactEmbedding64 {
						newFactEmbedding32[j] = float32(val)
					}
					factMetadataDefault := make(map[string]string)
					for k, v := range sessionDoc.Metadata {
						if k != "dataset_speaker_a" && k != "dataset_speaker_b" {
							factMetadataDefault[k] = v
						}
					}
					factMetadataDefault["speakerID"] = currentSpeakerIDForLog
					metadataBytesDefault, jsonErrDefault := json.Marshal(factMetadataDefault)
					if jsonErrDefault != nil {
						s.logger.Errorf("Error marshalling metadata for default ADD for speaker %s: %v. Storing with empty metadata.", currentSpeakerIDForLog, jsonErrDefault)
						metadataBytesDefault = []byte("{}")
					}
					dataDefault := map[string]interface{}{
						contentProperty:  factContent,
						metadataProperty: string(metadataBytesDefault),
					}
					if sessionDoc.Timestamp != nil {
						dataDefault[timestampProperty] = sessionDoc.Timestamp.Format(time.RFC3339)
					}

					defaultAddObject := &models.Object{
						Class:      className,
						Properties: dataDefault,
						Vector:     newFactEmbedding32,
					}
					batcher.WithObjects(defaultAddObject)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch (default action) for speaker %s. Fact: \"%s...\"", currentSpeakerIDForLog, firstNChars(factContent, 50))
				}
			}
		}

		if progressChan != nil {
			progressChan <- memory.ProgressUpdate{Processed: (i + 1), Total: totalDocs}
		}
	}

	if objectsAddedToBatch > 0 {
		s.logger.Infof("Flushing batcher with %d objects at the end of Store method.", objectsAddedToBatch)
		resp, err := batcher.Do(ctx)
		if err != nil {
			s.logger.Errorf("Error final batch storing facts to Weaviate: %v", err)
		} else {
			s.logger.Info("Final fact batch storage call completed.")
		}

		var successCount, failureCount int
		if resp != nil {
			for itemIdx, res := range resp {
				if res.Result != nil && res.Result.Status != nil && *res.Result.Status == "SUCCESS" {
					successCount++
				} else {
					failureCount++
					errorMsg := "unknown error during final batch item processing"
					if res.Result != nil && res.Result.Errors != nil && len(res.Result.Errors.Error) > 0 {
						errorMsg = res.Result.Errors.Error[0].Message
					}
					s.logger.Warnf("Failed to store a fact in final batch (Item %d). Error: %s.", itemIdx, errorMsg)
				}
			}
			s.logger.Infof("Final fact batch storage completed: %d successful, %d failed.", successCount, failureCount)
		} else if err != nil {
			s.logger.Warn("Batcher.Do() returned an error and a nil response. Cannot determine individual item statuses.")
		} else {
			s.logger.Info("Batcher.Do() returned no error and a nil response. Assuming batched items were processed if objectsAddedToBatch > 0.")
		}
	} else {
		s.logger.Info("No objects were added to the batcher during this Store() call. Nothing to flush.")
	}

	s.logger.Info("Store method finished processing all documents.")
	return nil
}

// Helper function to get first N float64 values for logging vector snippets
func firstNFloats(v []float64, n int) []float64 {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// Helper function to get first N float32 values for logging vector snippets
func firstNFloat32s(v []float32, n int) []float32 {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// firstNChars is a helper to get the first N characters of a string for logging.
func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Query retrieves memories relevant to the query text.
// For this iteration, it will do a simple semantic search.
// Later, it will need to be adapted for speaker-specific queries if we store facts with speakerID.
func (s *WeaviateStorage) Query(ctx context.Context, queryText string, options interface{}) (*memory.QueryResult, error) {
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure schema before querying: %w", err)
	}

	var filterBySpeakerID string
	if opts, ok := options.(map[string]interface{}); ok {
		if speakerToFilter, okS := opts["speakerID"].(string); okS && speakerToFilter != "" {
			filterBySpeakerID = speakerToFilter
			s.logger.Info("Query results will be filtered in Go by speakerID", "speakerID", filterBySpeakerID)
		}
	}

	s.logger.Info("Query method called", "query_text", queryText)

	vector, err := s.aiService.Embedding(ctx, queryText, openAIEmbedModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding for query: %w", err)
	}
	queryVector32 := make([]float32, len(vector))
	for i, val := range vector {
		queryVector32[i] = float32(val)
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(queryVector32)

	// Define the fields to retrieve - speakerID is NOT a top-level field here
	contentField := graphql.Field{Name: contentProperty}
	timestampField := graphql.Field{Name: timestampProperty}
	metaField := graphql.Field{Name: metadataProperty} // metadataJson contains speakerID
	tagsField := graphql.Field{Name: tagsProperty}
	additionalFields := graphql.Field{
		Name: "_additional",
		Fields: []graphql.Field{
			{Name: "id"},
			{Name: "distance"},
			// {Name: "vector"}, // Optionally retrieve vector
		},
	}

	queryBuilder := s.client.GraphQL().Get().
		WithClassName(className).
		WithNearVector(nearVector).
		WithLimit(10). // Default limit, can be configured via options if needed
		// NO WithWhere filter for speakerID at Weaviate level
		WithFields(contentField, timestampField, metaField, tagsField, additionalFields)

	resp, err := queryBuilder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Weaviate query: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL query errors: %v", resp.Errors)
	}

	finalResults := []memory.TextDocument{}
	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No 'Get' field in GraphQL response or not a map.")
		return &memory.QueryResult{Documents: finalResults}, nil
	}

	classData, ok := data[className].([]interface{})
	if !ok {
		s.logger.Warn("No class data in GraphQL response or not a slice.", "class_name", className)
		return &memory.QueryResult{Documents: finalResults}, nil
	}
	s.logger.Info("Retrieved documents from Weaviate (pre-filtering)", "count", len(classData))

	for _, item := range classData {
		obj, okMap := item.(map[string]interface{})
		if !okMap {
			s.logger.Warn("Retrieved item is not a map, skipping", "item", item)
			continue
		}

		content, _ := obj[contentProperty].(string)
		metadataJSON, _ := obj[metadataProperty].(string)

		var parsedTimestamp *time.Time
		if tsStr, tsOk := obj[timestampProperty].(string); tsOk {
			t, pErr := time.Parse(time.RFC3339, tsStr)
			if pErr == nil {
				parsedTimestamp = &t
			} else {
				s.logger.Warn("Failed to parse timestamp from Weaviate", "timestamp_str", tsStr, "error", pErr)
			}
		}

		additional, _ := obj["_additional"].(map[string]interface{})
		id, _ := additional["id"].(string)
		// distanceFloat, _ := additional["distance"].(float64) // Distance not part of TextDocument

		metaMap := make(map[string]string)
		if metadataJSON != "" {
			if errJson := json.Unmarshal([]byte(metadataJSON), &metaMap); errJson != nil {
				s.logger.Debug("Could not unmarshal metadataJson for retrieved doc, using empty map", "id", id, "error", errJson)
			}
		}

		docSpeakerID := metaMap["speakerID"] // Get speakerID from the unmarshalled metadata

		if filterBySpeakerID != "" && docSpeakerID != filterBySpeakerID {
			s.logger.Debug("Document filtered out by speakerID mismatch", "doc_id", id, "doc_speaker_id", docSpeakerID, "filter_speaker_id", filterBySpeakerID)
			continue // Skip this document
		}

		var tags []string
		if tagsInterface, tagsOk := obj[tagsProperty].([]interface{}); tagsOk {
			for _, tagInterfaceItem := range tagsInterface {
				if tagStr, okTag := tagInterfaceItem.(string); okTag {
					tags = append(tags, tagStr)
				}
			}
		}

		finalResults = append(finalResults, memory.TextDocument{
			ID:        id,
			Content:   content,
			Timestamp: parsedTimestamp,
			Metadata:  metaMap, // This map contains the speakerID
			Tags:      tags,
		})
	}
	s.logger.Info("Query processed successfully.", "num_results_returned_after_filtering", len(finalResults))
	return &memory.QueryResult{Documents: finalResults}, nil
}

// GetByID retrieves a document by its Weaviate ID.
// speakerID (if present) will be within the Metadata map after unmarshalling metadataJson.
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return nil, fmt.Errorf("pre-getbyid schema check failed: %w", err)
	}

	s.logger.Debugf("Attempting to get document by ID: %s", id)

	result, err := s.client.Data().ObjectsGetter().
		WithClassName(className).
		WithID(id).
		// No WithAdditionalParameters needed for ObjectsGetter for standard properties
		Do(ctx)

	if err != nil {
		// Weaviate client might return a specific error for not found (e.g., status code 404 in the error details)
		// For now, returning the generic error. Could inspect err for specific handling of "not found".
		return nil, fmt.Errorf("getting document by ID '%s': %w", id, err)
	}

	if len(result) == 0 {
		s.logger.Warnf("No document found with ID: %s (empty result array)", id)
		return nil, nil // Or an error like fmt.Errorf("document with ID '%s' not found", id)
	}

	obj := result[0]
	if obj.Properties == nil {
		return nil, fmt.Errorf("document with ID '%s' has nil properties", id)
	}

	props, ok := obj.Properties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast properties to map[string]interface{} for ID '%s'", id)
	}

	content, _ := props[contentProperty].(string)
	docTimestampStr, _ := props[timestampProperty].(string)
	tagsInterface, _ := props[tagsProperty].([]interface{})
	metadataJSON, _ := props[metadataProperty].(string) // This string contains all metadata, including speakerID

	var docTimestampP *time.Time
	if docTimestampStr != "" {
		parsedTime, pErr := time.Parse(time.RFC3339, docTimestampStr)
		if pErr != nil {
			s.logger.Warnf("Failed to parse timestamp for document ID '%s': %v. Setting to nil.", id, pErr)
		} else {
			docTimestampP = &parsedTime
		}
	}

	var tags []string
	for _, tagInterface := range tagsInterface {
		if tagStr, okT := tagInterface.(string); okT {
			tags = append(tags, tagStr)
		}
	}

	metadataMap := make(map[string]string) // This will hold all metadata, including speakerID if present
	if metadataJSON != "" {
		if errJson := json.Unmarshal([]byte(metadataJSON), &metadataMap); errJson != nil {
			s.logger.Warnf("Failed to unmarshal metadataJson for document ID '%s': %v. Metadata will be empty.", id, errJson)
			// metadataMap will remain empty or partially filled if unmarshalling failed mid-way (unlikely for simple map[string]string)
		}
	}

	doc := &memory.TextDocument{
		ID:        obj.ID.String(), // Use the ID from Weaviate's object, converting to string
		Content:   content,
		Timestamp: docTimestampP,
		Tags:      tags,
		Metadata:  metadataMap, // speakerID is now part of this map if it was stored
	}

	s.logger.Debugf("Successfully retrieved document by ID: %s. speakerID from metadata: '%s'", id, metadataMap["speakerID"])
	return doc, nil
}

// Update updates an existing document in Weaviate.
// speakerID (if present) is expected to be within doc.Metadata, which is marshalled to metadataJson.
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	if err := s.ensureSchemaExistsInternal(ctx); err != nil {
		return fmt.Errorf("pre-update schema check failed: %w", err)
	}

	s.logger.Debugf("Attempting to update document ID: %s", id)

	data := map[string]interface{}{
		contentProperty: doc.Content,
	}

	if doc.Timestamp != nil {
		data[timestampProperty] = doc.Timestamp.Format(time.RFC3339)
	}
	if len(doc.Tags) > 0 {
		data[tagsProperty] = doc.Tags
	} else {
		data[tagsProperty] = []string{} // Explicitly clear tags if doc.Tags is empty
	}

	// All metadata, including speakerID, is expected to be in doc.Metadata.
	// This map will be marshalled into the metadataJson field.
	if len(doc.Metadata) > 0 {
		metadataBytes, err := json.Marshal(doc.Metadata) // doc.Metadata should contain speakerID if it's set
		if err != nil {
			s.logger.Errorf("Failed to marshal metadata for document ID '%s': %v", id, err)
			return fmt.Errorf("marshaling metadata for update: %w", err)
		}
		data[metadataProperty] = string(metadataBytes)
		s.logger.Debugf("Updating doc %s, speakerID in marshalled metadata: '%s'", id, doc.Metadata["speakerID"])
	} else {
		data[metadataProperty] = "{}" // Store an empty JSON object string if no metadata
		s.logger.Debugf("Updating doc %s with empty metadataJson", id)
	}

	updater := s.client.Data().Updater().
		WithClassName(className).
		WithID(id).
		WithProperties(data)

	if len(vector) > 0 {
		updater = updater.WithVector(vector)
	}

	err := updater.Do(ctx)
	if err != nil {
		return fmt.Errorf("updating document ID '%s': %w", id, err)
	}

	s.logger.Infof("Successfully updated document ID: %s", id)
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
