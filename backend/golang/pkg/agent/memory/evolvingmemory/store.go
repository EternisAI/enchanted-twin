package evolvingmemory

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// identifySpeakersInMetadata attempts to find specific speaker identifiers in document metadata.
// Currently, it looks for "dataset_speaker_a" and "dataset_speaker_b".
// It returns a slice of identified speaker strings. If no specific speakers are found,
// it returns an empty slice.
func (s *WeaviateStorage) identifySpeakersInMetadata(metadata map[string]string) []string {
	var specificSpeakerCandidates []string
	if speakerA, ok := metadata["dataset_speaker_a"]; ok && speakerA != "" {
		specificSpeakerCandidates = append(specificSpeakerCandidates, speakerA)
	}
	if speakerB, ok := metadata["dataset_speaker_b"]; ok && speakerB != "" {
		addSpeakerB := true
		// Avoid adding B if it's the same as A and A was already added
		if len(specificSpeakerCandidates) == 1 && specificSpeakerCandidates[0] == speakerB {
			addSpeakerB = false
		}
		if addSpeakerB {
			specificSpeakerCandidates = append(specificSpeakerCandidates, speakerB)
		}
	}
	return specificSpeakerCandidates
}

// Store orchestrates the process of extracting facts from documents and updating memories.
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.TextDocument, progressCallback memory.ProgressCallback) error {
	batcher := s.client.Batch().ObjectsBatcher()
	var objectsAddedToBatch int

	totalDocs := len(documents)
	if totalDocs == 0 {
		return nil
	}

	currentSystemDate := getCurrentDateForPrompt()

	for i, sessionDoc := range documents {
		s.logger.Infof("Processing session document %d of %d. Session Doc ID (if any): '%s'", i+1, totalDocs, sessionDoc.ID)

		specificSpeakerCandidates := s.identifySpeakersInMetadata(sessionDoc.Metadata)

		var speakersToProcess []string
		if len(specificSpeakerCandidates) > 0 {
			speakersToProcess = specificSpeakerCandidates
			s.logger.Debugf("Identified specific speakers: %v. Proceeding with speaker-specific processing.", speakersToProcess)
		} else {
			speakersToProcess = []string{""}
			s.logger.Infof("No specific speakers identified for session doc ID '%s'. Proceeding with document-level processing.", sessionDoc.ID)
		}

		for _, speakerID := range speakersToProcess {
			logContextEntity := "Speaker"
			logContextValue := speakerID

			if speakerID == "" {
				logContextEntity = "Document"
				logContextValue = "<document_context>"
			}
			s.logger.Infof("== Processing for %s: %s == (Session Doc %d of %d)", logContextEntity, logContextValue, i+1, totalDocs)

			docEventDateStr := "Unknown"
			if sessionDoc.Timestamp != nil && !sessionDoc.Timestamp.IsZero() {
				docEventDateStr = sessionDoc.Timestamp.Format("2006-01-02")
			}

			extractedFacts, err := s.extractFactsFromTextDocument(ctx, sessionDoc, speakerID, currentSystemDate, docEventDateStr)
			if err != nil {
				s.logger.Errorf("Error during fact extraction for %s %s: %v. Skipping this processing unit.", logContextEntity, logContextValue, err)
				continue
			}
			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted for %s %s. Skipping memory operations for this unit.", logContextEntity, logContextValue)
				continue
			}
			s.logger.Infof("Total facts to process for %s '%s': %d", logContextEntity, logContextValue, len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "context", logContextValue)
					continue
				}
				s.logger.Infof("Processing fact %d for %s %s: \"%s...\"", factIdx+1, logContextEntity, logContextValue, firstNChars(factContent, 70))

				action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, sessionDoc)
				if err != nil {
					s.logger.Errorf("Error processing fact for %s %s: %v. Fact: \"%s...\"", logContextEntity, logContextValue, err, firstNChars(factContent, 50))
					continue
				}

				if action == AddMemoryToolName && objectToAdd != nil {
					batcher.WithObjects(objectToAdd)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for %s %s. Fact: \"%s...\"", logContextEntity, logContextValue, firstNChars(factContent, 50))
				} else if action != AddMemoryToolName {
					s.logger.Infof("Action '%s' for %s %s (Fact: \"%s...\") handled directly, not added to batch.", action, logContextEntity, logContextValue, firstNChars(factContent, 30))
				}
			}
		}

		if progressCallback != nil {
			progressCallback(i+1, totalDocs)
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

		if progressCallback != nil {
			progressCallback(i+1, totalDocs)
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
