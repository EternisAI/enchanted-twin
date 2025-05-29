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
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.TextDocument, progressCallback memory.ProgressCallback) error {
	batcher := s.client.Batch().ObjectsBatcher()
	var totalObjectsAddedToBatch int // Renamed for clarity

	totalDocs := len(documents)
	if totalDocs == 0 {
		return nil
	}

	currentSystemDate := getCurrentDateForPrompt()

	for i := range documents { // Iterate by index
		sessionDoc := &documents[i] // sessionDoc is *memory.TextDocument
		s.logger.Infof("Processing document %d of %d. Doc ID: '%s'", i+1, totalDocs, sessionDoc.ID())

		// 1. Prepare ConversationDocument and common parameters
		timestamp := sessionDoc.Timestamp()
		if timestamp == nil {
			now := time.Now()
			timestamp = &now
		}
		docEventDateStr := timestamp.Format("2006-01-02")

		source := "unknown"
		if md := sessionDoc.Metadata(); md != nil {
			if srcVal, ok := md["source"]; ok {
				source = srcVal
			}
		}

		convDocFromText := memory.ConversationDocument{
			FieldID:     sessionDoc.ID(),
			FieldSource: source,
			People:      []string{"user"},
			User:        "user",
			Conversation: []memory.ConversationMessage{
				{
					Speaker: "user",
					Content: sessionDoc.Content(),
					Time:    *timestamp,
				},
			},
			FieldTags:     sessionDoc.Tags(),
			FieldMetadata: sessionDoc.Metadata(),
		}

		speakerID := "user" // For TextDocument, we process for the "user" speaker

		s.logger.Infof("Attempting to process document '%s' as a conversation for speaker '%s'.", convDocFromText.ID(), speakerID)

		// 2. Call the common processing logic
		objectsForThisDoc, err := s.processConversationForSpeaker(
			ctx,
			convDocFromText,
			speakerID,
			currentSystemDate,
			docEventDateStr,
		)
		if err != nil {
			// s.processConversationForSpeaker logs errors related to fact extraction.
			s.logger.Errorf("Error processing document %s for speaker %s: %v. Skipping additions from this document.", sessionDoc.ID(), speakerID, err)
			// If extractFactsFromConversation failed in the helper, objectsForThisDoc will be nil.
		}

		if len(objectsForThisDoc) > 0 {
			batcher.WithObjects(objectsForThisDoc...)
			totalObjectsAddedToBatch += len(objectsForThisDoc)
			s.logger.Infof("%d object(s) from document %s (speaker %s) added to batch.", len(objectsForThisDoc), sessionDoc.ID(), speakerID)
		} else if err == nil { // err == nil means processing happened but yielded no objects (e.g. no facts, or facts led to no-ops)
			s.logger.Infof("No objects to add to batch from document %s (speaker %s).", sessionDoc.ID(), speakerID)
		}
		// If err != nil, the error message from above covers why no objects are added.

		if progressCallback != nil {
			progressCallback(i+1, totalDocs)
		}
	} // End of document loop

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

// processConversationForSpeaker handles fact extraction and memory updates for a given conversation and speaker.
// It returns a slice of Weaviate objects to be added to the batch and an error if fact extraction fails.
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
		s.logger.Errorf("Fact extraction failed for speaker %s, doc ID %s: %v", speakerID, convDoc.ID(), err)
		return nil, err // Return error to indicate failure at this stage
	}

	if len(extractedFacts) == 0 {
		s.logger.Infof("No facts extracted for speaker %s, doc ID %s.", speakerID, convDoc.ID())
		return nil, nil // No facts, no objects, no error
	}
	s.logger.Infof("Extracted %d facts for speaker %s, doc ID %s.", len(extractedFacts), speakerID, convDoc.ID())

	for factIdx, factContent := range extractedFacts {
		if strings.TrimSpace(factContent) == "" {
			s.logger.Debugf("Skipping empty fact text for speaker %s, doc ID %s.", speakerID, convDoc.ID())
			continue
		}
		s.logger.Infof("Processing fact %d/%d for speaker %s, doc ID %s: \\\"%s...\\\"",
			factIdx+1, len(extractedFacts), speakerID, convDoc.ID(), firstNChars(factContent, 70))

		action, objectToAdd, updateErr := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, convDoc)
		if updateErr != nil {
			s.logger.Errorf("Error updating memories for fact for speaker %s, doc ID %s: %v. Fact: \\\"%s...\\\"",
				speakerID, convDoc.ID(), updateErr, firstNChars(factContent, 50))
			// Continue with other facts, even if one update fails
			continue
		}

		if action == AddMemoryToolName && objectToAdd != nil {
			objectsToAdd = append(objectsToAdd, objectToAdd)
			s.logger.Infof("Fact marked for ADDITION to batch for speaker %s, doc ID %s. Fact: \\\"%s...\\\"",
				speakerID, convDoc.ID(), firstNChars(factContent, 50))
		} else if action != AddMemoryToolName {
			s.logger.Infof("Action '%s' for speaker %s, doc ID %s (Fact: \\\"%s...\\\") handled, not adding to batch.",
				action, speakerID, convDoc.ID(), firstNChars(factContent, 30))
		}
	}
	return objectsToAdd, nil
}

// StoreRawData stores documents directly without fact extraction processing.
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
