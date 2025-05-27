package evolvingmemory

import (
	"context"
	"strings"

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
