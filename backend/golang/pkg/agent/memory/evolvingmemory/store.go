package evolvingmemory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Store handles unified Document interface with internal type routing.
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	s.logger.Info("=== MEMORY STORE START ===")
	batcher := s.client.Batch().ObjectsBatcher()
	var totalObjectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		return nil
	}

	currentSystemDate := getCurrentDateForPrompt()

	for i, doc := range documents {
		s.logger.Infof("Processing document %d of %d. Doc ID: '%s'", i+1, totalDocs, doc.ID())

		timestamp := doc.Timestamp()
		if timestamp == nil {
			now := time.Now()
			timestamp = &now
		}
		docEventDateStr := timestamp.Format("2006-01-02")

		// Route to appropriate processing based on document type
		var objectsForThisDoc []*models.Object
		var err error

		switch typedDoc := doc.(type) {
		case *memory.ConversationDocument:
			s.logger.Infof("Processing ConversationDocument '%s' with Rich Context Extraction", doc.ID())

			// Use rich conversation extraction for each speaker
			for _, speakerID := range typedDoc.People {
				speakerObjects, speakerErr := s.processConversationForSpeaker(
					ctx,
					*typedDoc,
					speakerID,
					currentSystemDate,
					docEventDateStr,
				)
				if speakerErr != nil {
					s.logger.Errorf("Error processing ConversationDocument %s for speaker %s: %v", doc.ID(), speakerID, speakerErr)
					continue
				}
				objectsForThisDoc = append(objectsForThisDoc, speakerObjects...)
			}

		case *memory.TextDocument:
			s.logger.Infof("Processing TextDocument '%s' with Legacy Text Extraction", doc.ID())

			speakerID := "user"
			objectsForThisDoc, err = s.processTextDocumentForSpeaker(
				ctx,
				*typedDoc,
				speakerID,
				currentSystemDate,
				docEventDateStr,
			)
			if err != nil {
				s.logger.Errorf("Error processing TextDocument %s for speaker %s: %v", doc.ID(), speakerID, err)
			}

		default:
			s.logger.Warnf("Unknown document type for document %s, skipping", doc.ID())
			continue
		}

		if len(objectsForThisDoc) > 0 {
			batcher.WithObjects(objectsForThisDoc...)
			totalObjectsAddedToBatch += len(objectsForThisDoc)
			s.logger.Infof("%d object(s) from document %s added to batch.", len(objectsForThisDoc), doc.ID())
		} else if err == nil {
			s.logger.Infof("No objects to add to batch from document %s.", doc.ID())
		}

		if progressCallback != nil {
			progressCallback(i+1, totalDocs)
		}
	}

	// Batch flushing logic
	if totalObjectsAddedToBatch > 0 {
		s.logger.Infof("Flushing batcher with %d objects at the end of Store method.", totalObjectsAddedToBatch)
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
			s.logger.Info("Batcher.Do() returned no error and a nil response. Assuming batched items were processed if totalObjectsAddedToBatch > 0.")
		}
	} else {
		s.logger.Info("No objects were added to the batcher during this Store() call. Nothing to flush.")
	}

	s.logger.Info("Store method finished processing all documents.")
	return nil
}

// processTextDocumentForSpeaker handles legacy text document processing using the TextDocument prompt.
func (s *WeaviateStorage) processTextDocumentForSpeaker(
	ctx context.Context,
	textDoc memory.TextDocument,
	speakerID string,
	currentSystemDate string,
	docEventDateStr string,
) ([]*models.Object, error) {
	var objectsToAdd []*models.Object

	extractedFacts, err := s.extractFactsFromTextDocument(ctx, textDoc, speakerID, currentSystemDate, docEventDateStr)
	if err != nil {
		s.logger.Errorf("Legacy text fact extraction failed for speaker %s, doc ID %s: %v", speakerID, textDoc.ID(), err)
		return nil, err
	}

	if len(extractedFacts) == 0 {
		s.logger.Infof("No facts extracted for speaker %s, text doc ID %s.", speakerID, textDoc.ID())
		return nil, nil
	}
	s.logger.Infof("Extracted %d facts for speaker %s, text doc ID %s.", len(extractedFacts), speakerID, textDoc.ID())

	// Convert TextDocument to ConversationDocument for updateMemories compatibility
	convDocFromText := memory.ConversationDocument{
		FieldID:     textDoc.ID(),
		FieldSource: textDoc.Source(),
		People:      []string{speakerID},
		User:        speakerID,
		Conversation: []memory.ConversationMessage{
			{
				Speaker: speakerID,
				Content: textDoc.Content(),
				Time:    *textDoc.Timestamp(),
			},
		},
		FieldTags:     textDoc.Tags(),
		FieldMetadata: textDoc.Metadata(),
	}

	for factIdx, factContent := range extractedFacts {
		if strings.TrimSpace(factContent) == "" {
			s.logger.Debugf("Skipping empty fact text for speaker %s, text doc ID %s.", speakerID, textDoc.ID())
			continue
		}
		s.logger.Infof("Processing fact %d/%d for speaker %s, text doc ID %s: \\\"%s...\\\"",
			factIdx+1, len(extractedFacts), speakerID, textDoc.ID(), firstNChars(factContent, 70))

		action, objectToAdd, updateErr := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDocFromText)
		if updateErr != nil {
			s.logger.Errorf("Error updating memories for fact for speaker %s, text doc ID %s: %v. Fact: \\\"%s...\\\"",
				speakerID, textDoc.ID(), updateErr, firstNChars(factContent, 50))
			continue
		}

		if action == AddMemoryToolName && objectToAdd != nil {
			objectsToAdd = append(objectsToAdd, objectToAdd)
			s.logger.Infof("Fact marked for ADDITION to batch for speaker %s, text doc ID %s. Fact: \\\"%s...\\\"",
				speakerID, textDoc.ID(), firstNChars(factContent, 50))
		} else if action != AddMemoryToolName {
			s.logger.Infof("Action '%s' for speaker %s, text doc ID %s (Fact: \\\"%s...\\\") handled, not adding to batch.",
				action, speakerID, textDoc.ID(), firstNChars(factContent, 30))
		}
	}
	return objectsToAdd, nil
}

// processConversationForSpeaker handles fact extraction and memory updates for a given conversation and speaker.
func (s *WeaviateStorage) processConversationForSpeaker(
	ctx context.Context,
	convDoc memory.ConversationDocument,
	speakerID string,
	currentSystemDate string,
	docEventDateStr string,
) ([]*models.Object, error) {
	var objectsToAdd []*models.Object

	extractedFacts, err := s.extractFactsFromConversation(ctx, convDoc, speakerID, currentSystemDate, docEventDateStr)
	if err != nil {
		s.logger.Errorf("Rich conversation fact extraction failed for speaker %s, doc ID %s: %v", speakerID, convDoc.ID(), err)
		return nil, err
	}

	if len(extractedFacts) == 0 {
		s.logger.Infof("No facts extracted for speaker %s, conversation doc ID %s.", speakerID, convDoc.ID())
		return nil, nil
	}
	s.logger.Infof("Extracted %d facts for speaker %s, conversation doc ID %s.", len(extractedFacts), speakerID, convDoc.ID())

	for factIdx, factContent := range extractedFacts {
		if strings.TrimSpace(factContent) == "" {
			s.logger.Debugf("Skipping empty fact text for speaker %s, conversation doc ID %s.", speakerID, convDoc.ID())
			continue
		}
		s.logger.Infof("Processing fact %d/%d for speaker %s, conversation doc ID %s: \\\"%s...\\\"",
			factIdx+1, len(extractedFacts), speakerID, convDoc.ID(), firstNChars(factContent, 70))

		action, objectToAdd, updateErr := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDoc)
		if updateErr != nil {
			s.logger.Errorf("Error updating memories for fact for speaker %s, conversation doc ID %s: %v. Fact: \\\"%s...\\\"",
				speakerID, convDoc.ID(), updateErr, firstNChars(factContent, 50))
			continue
		}

		if action == AddMemoryToolName && objectToAdd != nil {
			objectsToAdd = append(objectsToAdd, objectToAdd)
			s.logger.Infof("Fact marked for ADDITION to batch for speaker %s, conversation doc ID %s. Fact: \\\"%s...\\\"",
				speakerID, convDoc.ID(), firstNChars(factContent, 50))
		} else if action != AddMemoryToolName {
			s.logger.Infof("Action '%s' for speaker %s, conversation doc ID %s (Fact: \\\"%s...\\\") handled, not adding to batch.",
				action, speakerID, convDoc.ID(), firstNChars(factContent, 30))
		}
	}
	return objectsToAdd, nil
}

// StoreRawData stores documents directly without fact extraction processing (internal method).
func (s *WeaviateStorage) StoreRawData(ctx context.Context, documents []memory.TextDocument, progressCallback memory.ProgressCallback) error {
	s.logger.Info("=== EVOLVINGMEMORY STORE RAW DATA START ===")
	s.logger.Info("StoreRawData method called", "total_documents", len(documents))

	batcher := s.client.Batch().ObjectsBatcher()
	var objectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		s.logger.Info("No documents to process, returning early")
		return nil
	}

	for i := range documents { // Iterate by index to get pointers
		doc := &documents[i] // doc is *memory.TextDocument
		vector, err := s.embeddingsService.Embedding(ctx, doc.Content(), openAIEmbedModel)
		if err != nil {
			s.logger.Errorf("Error generating embedding for document %s: %v", doc.ID(), err)
			if progressCallback != nil {
				progressCallback(i+1, totalDocs) // Call progress on error before continue
			}
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

		if progressCallback != nil {
			progressCallback(i+1, totalDocs)
		}
	}

	if objectsAddedToBatch > 0 {
		s.logger.Infof("Flushing batcher with %d objects at the end of StoreRawData method.", objectsAddedToBatch)
		resp, err := batcher.Do(ctx)
		if err != nil {
			s.logger.Errorf("Error batch storing raw data to Weaviate: %v", err)
			return fmt.Errorf("raw data batch storage error: %w", err)
		}

		var successCount, failureCount int
		if resp != nil {
			for itemIdx, res := range resp {
				if res.Result != nil && res.Result.Status != nil && *res.Result.Status == "SUCCESS" {
					successCount++
				} else {
					failureCount++
					errorMsg := "unknown error"
					if res.Result != nil && res.Result.Errors != nil && len(res.Result.Errors.Error) > 0 {
						errorMsg = res.Result.Errors.Error[0].Message
					}
					s.logger.Warnf("Failed to store raw document in batch (Item %d). Error: %s.", itemIdx, errorMsg)
				}
			}
			s.logger.Infof("Raw data batch storage completed: %d successful, %d failed.", successCount, failureCount)
		}
	} else {
		s.logger.Info("No objects were added to the raw data batcher. Nothing to flush.")
	}

	s.logger.Info("StoreRawData method finished.")
	s.logger.Info("=== EVOLVINGMEMORY STORE RAW DATA END ===")
	return nil
}
