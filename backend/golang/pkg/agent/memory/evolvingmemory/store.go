package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Store orchestrates the process of extracting facts from documents and updating memories.
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.ConversationDocument, progressChan chan<- memory.ProgressUpdate) error {
	defer func() {
		if progressChan != nil {
			close(progressChan)
		}
	}()

	batcher := s.client.Batch().ObjectsBatcher()
	var objectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		return nil
	}

	currentSystemDate := getCurrentDateForPrompt()

	for i, convDoc := range documents {
		s.logger.Infof("Processing conversation document %d of %d. ID: '%s'", i+1, totalDocs, convDoc.ID)

		// Get conversation date from first message
		docEventDateStr := "Unknown"
		if len(convDoc.Conversation.Conversation) > 0 {
			docEventDateStr = convDoc.Conversation.Conversation[0].Time.Format("2006-01-02")
		}

		// Use the explicit People array from structured conversations
		speakersToProcess := convDoc.Conversation.People
		s.logger.Infof("Processing %d speakers from conversation: %v", len(speakersToProcess), speakersToProcess)

		// Process each speaker in the conversation
		for _, speakerID := range speakersToProcess {
			s.logger.Infof("== Processing for Speaker: %s == (Conversation %d of %d)", speakerID, i+1, totalDocs)

			extractedFacts, err := s.extractFactsFromConversation(ctx, convDoc, speakerID, currentSystemDate, docEventDateStr)
			if err != nil {
				s.logger.Errorf("Error during fact extraction for speaker %s: %v. Skipping this speaker.", speakerID, err)
				continue
			}
			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted for speaker %s. Skipping memory operations for this speaker.", speakerID)
				continue
			}
			s.logger.Infof("Total facts to process for speaker '%s': %d", speakerID, len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "speaker", speakerID)
					continue
				}
				s.logger.Infof("Processing fact %d for speaker %s: \"%s...\"", factIdx+1, speakerID, firstNChars(factContent, 70))

				action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDoc)
				if err != nil {
					s.logger.Errorf("Error processing fact for speaker %s: %v. Fact: \"%s...\"", speakerID, err, firstNChars(factContent, 50))
					continue
				}

				if action == AddMemoryToolName && objectToAdd != nil {
					batcher.WithObjects(objectToAdd)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for speaker %s. Fact: \"%s...\"", speakerID, firstNChars(factContent, 50))
				} else if action != AddMemoryToolName {
					s.logger.Infof("Action '%s' for speaker %s (Fact: \"%s...\") handled directly, not added to batch.", action, speakerID, firstNChars(factContent, 30))
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

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func (s *WeaviateStorage) extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	s.logger.Infof("== Starting Fact Extraction for Speaker: %s == (Conversation ID: '%s')", speakerID, convDoc.ID)

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	// Use the new FactExtractionPrompt for structured conversations
	sysPrompt := strings.ReplaceAll(FactExtractionPrompt, "{speaker_name}", speakerID)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{current_date}", currentSystemDate)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{conversation_date}", docEventDateStr)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(sysPrompt),
				},
			},
		},
	}

	// Build conversation context from structured messages
	parsedTurnsCount := 0
	for _, msg := range convDoc.Conversation.Conversation {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}

		parsedTurnsCount++
		fullTurnContent := fmt.Sprintf("%s: %s", msg.Speaker, msg.Content)

		if msg.Speaker == speakerID { // Target speaker's turn for the LLM context
			llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(fullTurnContent),
					},
				},
			})
		} else { // Other speaker's turn, context for LLM
			llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(fullTurnContent),
					},
				},
			})
		}
	}

	if parsedTurnsCount == 0 {
		s.logger.Warnf("No valid turns found in conversation for speaker %s in conversation %s.", speakerID, convDoc.ID)
	}
	if len(llmMsgs) <= 1 { // Only system prompt
		s.logger.Warnf("llmMsgs only contains system prompt for speaker %s in conversation %s. No conversational turns added. Skipping LLM call for fact extraction.", speakerID, convDoc.ID)
		return []string{}, nil // No error, but no facts
	}

	s.logger.Debugf("Calling LLM for Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during fact extraction for speaker %s in conversation %s: %v", speakerID, convDoc.ID, err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, conversation %s: %w", speakerID, convDoc.ID, err)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from conversation %s.", len(args.Facts), speakerID, convDoc.ID)
				} else {
					s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s from conversation %s: %v. Arguments: %s", speakerID, convDoc.ID, err, toolCall.Function.Arguments)
				}
			} else {
				s.logger.Warnf("LLM called an unexpected tool '%s' during fact extraction for speaker %s from conversation %s.", toolCall.Function.Name, speakerID, convDoc.ID)
			}
		}
	} else {
		s.logger.Info("LLM response for fact extraction for speaker %s from conversation %s did not contain tool calls. No facts extracted by tool.", speakerID, convDoc.ID)
	}

	return extractedFacts, nil
}
