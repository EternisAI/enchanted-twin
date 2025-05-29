package evolvingmemory

import (
	"context"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
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
		s.logger.Infof("Processing document %d of %d. ID: '%s'", i+1, totalDocs, doc.ID())

		// Handle different document types
		if convDoc, ok := doc.(*memory.ConversationDocument); ok {
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
		} else if textDoc, ok := doc.(*memory.TextDocument); ok {
			// Handle TextDocument - convert to ConversationDocument for processing
			s.logger.Infof("Processing text document as single-speaker conversation. ID: '%s'", textDoc.ID())

			// Convert TextDocument to a simple ConversationDocument for processing
			timestamp := textDoc.Timestamp()
			if timestamp == nil {
				now := time.Now()
				timestamp = &now
			}

			// Create a simple conversation with the content as a single message
			// Use "user" as the default speaker for text documents
			convDocFromText := memory.ConversationDocument{
				FieldID: textDoc.ID(),
				Conversation: memory.StructuredConversation{
					Source: textDoc.Metadata()["source"],
					People: []string{"user"},
					User:   "user",
					Conversation: []memory.ConversationMessage{
						{
							Speaker: "user",
							Content: textDoc.Content(),
							Time:    *timestamp,
						},
					},
				},
				FieldTags:     textDoc.Tags(),
				FieldMetadata: textDoc.Metadata(),
			}

			docEventDateStr := timestamp.Format("2006-01-02")

			// Process as single speaker
			speakerID := "user"
			s.logger.Infof("== Processing text document for Speaker: %s == (Document %d of %d)", speakerID, i+1, totalDocs)

			extractedFacts, err := s.extractFactsFromConversation(ctx, convDocFromText, speakerID, currentSystemDate, docEventDateStr)
			if err != nil {
				s.logger.Errorf("Error during fact extraction for text document %s: %v. Skipping.", textDoc.ID(), err)
				continue
			}
			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted for text document %s. Skipping memory operations.", textDoc.ID())
				continue
			}
			s.logger.Infof("Total facts to process for text document '%s': %d", textDoc.ID(), len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "document", textDoc.ID())
					continue
				}
				s.logger.Infof("Processing fact %d for text document %s: \"%s...\"", factIdx+1, textDoc.ID(), firstNChars(factContent, 70))

				action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDocFromText)
				if err != nil {
					s.logger.Errorf("Error processing fact for text document %s: %v. Fact: \"%s...\"", textDoc.ID(), err, firstNChars(factContent, 50))
					continue
				}

				if action == AddMemoryToolName && objectToAdd != nil {
					batcher.WithObjects(objectToAdd)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for text document %s. Fact: \"%s...\"", textDoc.ID(), firstNChars(factContent, 50))
				} else if action != AddMemoryToolName {
					s.logger.Infof("Action '%s' for text document %s (Fact: \"%s...\") handled directly, not added to batch.", action, textDoc.ID(), firstNChars(factContent, 30))
				}
			}
		} else {
			s.logger.Warnf("Document with ID '%s' is neither a ConversationDocument nor a TextDocument. Skipping.", doc.ID())
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
		vector, err := s.embeddingsService.Embedding(ctx, doc.Content(), openAIEmbedModel)
		if err != nil {
			s.logger.Errorf("Error generating embedding for document %s: %v", doc.ID(), err)
			continue
		}

		vector32 := make([]float32, len(vector))
		for j, val := range vector {
			vector32[j] = float32(val)
		}

		data := map[string]interface{}{
			contentProperty: doc.Content(),
		}

		originalTimestamp := doc.Timestamp()
		if originalTimestamp != nil {
			data[timestampProperty] = originalTimestamp.Format(time.RFC3339)
		}

		originalTags := doc.Tags()
		if len(originalTags) > 0 {
			data[tagsProperty] = originalTags
		}

		originalMetadata := doc.Metadata()
		if len(originalMetadata) > 0 {
			data[metadataProperty] = originalMetadata
		} else {
			data[metadataProperty] = "{}"
		}

		obj := &models.Object{
			Class:      ClassName,
			ID:         strfmt.UUID(doc.ID()),
			Vector:     vector32,
			Properties: data,
		}

		batcher.WithObjects(obj)
		objectsAddedToBatch++
		s.logger.Infof("Processed and added document %s to batch (%d/%d)", doc.ID(), i+1, totalDocs)

		if progressChan != nil {
			progressChan <- memory.ProgressUpdate{Processed: (i + 1), Total: totalDocs}
		}
	}

	if objectsAddedToBatch > 0 {
		s.logger.Infof("Flushing batcher with %d objects at the end of StoreRawData method.", objectsAddedToBatch)
		resp, err := batcher.Do(ctx)
		if err != nil {
			s.logger.Errorf("Error batch storing raw data to Weaviate: %v", err)
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
					errorMsg := "unknown error during batch item processing"
					if res.Result != nil && res.Result.Errors != nil && len(res.Result.Errors.Error) > 0 {
						errorMsg = res.Result.Errors.Error[0].Message
					}
					s.logger.Warnf("Failed to store raw data item in batch (Item %d). Error: %s.", itemIdx, errorMsg)
				}
			}
			s.logger.Infof("Raw data batch storage completed: %d successful, %d failed.", successCount, failureCount)
		} else if err != nil {
			s.logger.Warn("StoreRawData: Batcher.Do() returned an error and a nil response.")
		} else {
			s.logger.Info("StoreRawData: Batcher.Do() returned no error and a nil response.")
		}
	} else {
		s.logger.Info("No raw data objects were added to the batcher. Nothing to flush.")
	}

	s.logger.Info("StoreRawData method finished.")
	s.logger.Info("=== EVOLVINGMEMORY STORE RAW DATA END ===")
	return nil
}
