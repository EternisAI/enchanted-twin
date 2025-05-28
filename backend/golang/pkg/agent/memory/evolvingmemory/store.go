package evolvingmemory

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Store orchestrates the process of extracting facts from documents and updating memories.
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.Document, progressChan chan<- memory.ProgressUpdate) error {
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

	for i, doc := range documents {
		s.logger.Infof("Processing document %d of %d. ID: '%s'", i+1, totalDocs, doc.GetID())

		// Handle different document types
		if doc.IsConversation() {
			convDoc, ok := doc.AsConversation()
			if !ok {
				s.logger.Errorf("Document claims to be conversation but conversion failed. ID: '%s'", doc.GetID())
				continue
			}

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

				extractedFacts, err := s.extractFactsFromConversation(ctx, *convDoc, speakerID, currentSystemDate, docEventDateStr)
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

					action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, *convDoc)
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
		} else {
			// Handle TextDocument - convert to ConversationDocument for processing
			textDoc, ok := doc.AsText()
			if !ok {
				s.logger.Errorf("Document is not conversation but conversion to text failed. ID: '%s'", doc.GetID())
				continue
			}

			s.logger.Infof("Processing text document as single-speaker conversation. ID: '%s'", textDoc.ID)

			// Convert TextDocument to a simple ConversationDocument for processing
			timestamp := textDoc.GetTimestamp()
			if timestamp == nil {
				now := time.Now()
				timestamp = &now
			}

			// Create a simple conversation with the content as a single message
			// Use "user" as the default speaker for text documents
			convDoc := memory.ConversationDocument{
				ID: textDoc.ID,
				Conversation: memory.StructuredConversation{
					Source: textDoc.GetMetadata()["source"],
					People: []string{"user"},
					User:   "user",
					Conversation: []memory.ConversationMessage{
						{
							Speaker: "user",
							Content: textDoc.Content,
							Time:    *timestamp,
						},
					},
				},
				Tags:     textDoc.Tags,
				Metadata: textDoc.Metadata,
			}

			docEventDateStr := timestamp.Format("2006-01-02")

			// Process as single speaker
			speakerID := "user"
			s.logger.Infof("== Processing text document for Speaker: %s == (Document %d of %d)", speakerID, i+1, totalDocs)

			extractedFacts, err := s.extractFactsFromConversation(ctx, convDoc, speakerID, currentSystemDate, docEventDateStr)
			if err != nil {
				s.logger.Errorf("Error during fact extraction for text document %s: %v. Skipping.", textDoc.ID, err)
				continue
			}
			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted for text document %s. Skipping memory operations.", textDoc.ID)
				continue
			}
			s.logger.Infof("Total facts to process for text document '%s': %d", textDoc.ID, len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "document", textDoc.ID)
					continue
				}
				s.logger.Infof("Processing fact %d for text document %s: \"%s...\"", factIdx+1, textDoc.ID, firstNChars(factContent, 70))

				action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDoc)
				if err != nil {
					s.logger.Errorf("Error processing fact for text document %s: %v. Fact: \"%s...\"", textDoc.ID, err, firstNChars(factContent, 50))
					continue
				}

				if action == AddMemoryToolName && objectToAdd != nil {
					batcher.WithObjects(objectToAdd)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for text document %s. Fact: \"%s...\"", textDoc.ID, firstNChars(factContent, 50))
				} else if action != AddMemoryToolName {
					s.logger.Infof("Action '%s' for text document %s (Fact: \"%s...\") handled directly, not added to batch.", action, textDoc.ID, firstNChars(factContent, 30))
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

// StoreRawData stores documents directly without fact extraction processing.
func (s *WeaviateStorage) StoreRawData(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
	defer func() {
		if progressChan != nil {
			close(progressChan)
		}
	}()

	s.logger.Info("=== EVOLVINGMEMORY STORE RAW DATA START ===")
	s.logger.Info("StoreRawData method called", "total_documents", len(documents))

	batcher := s.client.Batch().ObjectsBatcher()
	var objectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		s.logger.Info("No documents to process, returning early")
		return nil
	}

	for i, doc := range documents {
		vector, err := s.embeddingsService.Embedding(ctx, doc.Content, openAIEmbedModel)
		if err != nil {
			s.logger.Errorf("Error generating embedding for document %s: %v", doc.ID, err)
			continue
		}

		vector32 := make([]float32, len(vector))
		for j, val := range vector {
			vector32[j] = float32(val)
		}

		data := map[string]interface{}{
			contentProperty: doc.Content,
		}

		if doc.Timestamp != nil {
			data[timestampProperty] = doc.Timestamp.Format(time.RFC3339)
		}

		if len(doc.Tags) > 0 {
			data[tagsProperty] = doc.Tags
		}

		if len(doc.Metadata) > 0 {
			metadataBytes, err := json.Marshal(doc.Metadata)
			if err != nil {
				s.logger.Errorf("Error marshaling metadata for document %s: %v", doc.ID, err)
				continue
			}
			data[metadataProperty] = string(metadataBytes)
		} else {
			data[metadataProperty] = "{}"
		}

		obj := &models.Object{
			Class:      ClassName,
			Properties: data,
			Vector:     vector32,
		}

		batcher.WithObjects(obj)
		objectsAddedToBatch++
		s.logger.Infof("Document %s added to batch for raw storage", doc.ID)

		if progressChan != nil {
			progressChan <- memory.ProgressUpdate{Processed: (i + 1), Total: totalDocs}
		}
	}

	if objectsAddedToBatch > 0 {
		s.logger.Infof("Flushing batcher with %d objects for raw data storage.", objectsAddedToBatch)
		resp, err := batcher.Do(ctx)
		if err != nil {
			s.logger.Errorf("Error batch storing raw data to Weaviate: %v", err)
			return err
		} else {
			s.logger.Info("Raw data batch storage call completed.")
		}

		var successCount, failureCount int
		if resp != nil {
			for itemIdx, res := range resp {
				if res.Result != nil && res.Result.Status != nil && *res.Result.Status == "SUCCESS" {
					successCount++
				} else {
					failureCount++
					errorMsg := "unknown error during raw data batch item processing"
					if res.Result != nil && res.Result.Errors != nil && len(res.Result.Errors.Error) > 0 {
						errorMsg = res.Result.Errors.Error[0].Message
					}
					s.logger.Warnf("Failed to store raw data in batch (Item %d). Error: %s.", itemIdx, errorMsg)
				}
			}
			s.logger.Infof("Raw data batch storage completed: %d successful, %d failed.", successCount, failureCount)
		}
	} else {
		s.logger.Info("No objects were added to the batcher during this StoreRawData() call. Nothing to flush.")
	}

	s.logger.Info("StoreRawData method finished processing all documents.")
	s.logger.Info("=== EVOLVINGMEMORY STORE RAW DATA END ===")
	return nil
}
