package evolvingmemory

import (
	"context"
	"fmt"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Store orchestrates the process of extracting facts from documents and updating memories.
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

	currentSystemDate := getCurrentDateForPrompt()

	for i, sessionDoc := range documents {
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

		for _, speakerID := range speakerIterationCandidates {
			currentSpeakerIDForLog := speakerID
			s.logger.Infof("== Processing for Speaker: %s == (Session Doc %d of %d)", currentSpeakerIDForLog, i+1, totalDocs)

			docEventDateStr := "Unknown"
			if sessionDoc.Timestamp != nil && !sessionDoc.Timestamp.IsZero() {
				docEventDateStr = sessionDoc.Timestamp.Format("2006-01-02")
			}

			extractedFacts, err := s.extractFactsFromTextDocument(ctx, sessionDoc, speakerID, currentSystemDate, docEventDateStr)
			if err != nil {
				s.logger.Errorf("Error during fact extraction for speaker %s: %v. Skipping speaker.", currentSpeakerIDForLog, err)
				continue
			}
			if len(extractedFacts) == 0 {
				s.logger.Infof("No facts extracted for speaker %s. Skipping memory operations for this speaker.", currentSpeakerIDForLog)
				continue
			}
			s.logger.Infof("Total facts to process for speaker '%s': %d", currentSpeakerIDForLog, len(extractedFacts))

			for factIdx, factContent := range extractedFacts {
				if strings.TrimSpace(factContent) == "" {
					s.logger.Debug("Skipping empty fact text.", "speaker", currentSpeakerIDForLog)
					continue
				}
				s.logger.Infof("Processing fact %d for speaker %s: \"%s...\"", factIdx+1, currentSpeakerIDForLog, firstNChars(factContent, 70))

				action, objectToAdd, err := s.updateMemories(ctx, factContent, speakerID, currentSystemDate, docEventDateStr, sessionDoc)
				if err != nil {
					s.logger.Errorf("Error processing fact for speaker %s: %v. Fact: \"%s...\"", currentSpeakerIDForLog, err, firstNChars(factContent, 50))
					continue
				}

				if action == AddMemoryToolName && objectToAdd != nil {
					batcher.WithObjects(objectToAdd)
					objectsAddedToBatch++
					s.logger.Infof("Fact ADDED to batch for speaker %s. Fact: \"%s...\"", currentSpeakerIDForLog, firstNChars(factContent, 50))
				} else if action != AddMemoryToolName {
					s.logger.Infof("Action '%s' for speaker %s (Fact: \"%s...\") handled directly, not added to batch.", action, currentSpeakerIDForLog, firstNChars(factContent, 30))
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
