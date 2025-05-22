package evolvingmemory

import (
	"context"
	"strings"

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
func (s *WeaviateStorage) Store(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
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

	for i, sessionDoc := range documents {
		s.logger.Infof("Processing session document %d of %d. Session Doc ID (if any): '%s'", i+1, totalDocs, sessionDoc.ID)

		// Attempt to identify specific speakers using the helper method
		specificSpeakerCandidates := s.identifySpeakersInMetadata(sessionDoc.Metadata)

		var speakersToProcess []string
		if len(specificSpeakerCandidates) > 0 {
			speakersToProcess = specificSpeakerCandidates
			s.logger.Debugf("Identified specific speakers: %v. Proceeding with speaker-specific processing.", speakersToProcess)
		} else {
			// No specific speakers found, set up for a single document-level processing pass.
			// The empty string speakerID will signify document-level context to downstream functions.
			speakersToProcess = []string{""}
			s.logger.Infof("No specific speakers identified for session doc ID '%s'. Proceeding with document-level processing.", sessionDoc.ID)
		}

		// This loop will run once with speakerID="" if no specific speakers were found,
		// or once for each specific speaker if they were identified.
		for _, speakerID := range speakersToProcess {
			logContextEntity := "Speaker" // Default to "Speaker"
			logContextValue := speakerID

			if speakerID == "" {
				logContextEntity = "Document"
				logContextValue = "<document_context>" // For clearer logging when speakerID is empty
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
