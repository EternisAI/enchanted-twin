package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// MemoryOrchestrator handles coordination logic for memory operations.
// This handles orchestration concerns (channels, workers, progress reporting).
type MemoryOrchestrator struct {
	engine  *MemoryEngine
	storage storage.Interface
	logger  *log.Logger
}

// NewMemoryOrchestrator creates a new MemoryOrchestrator instance.
func NewMemoryOrchestrator(engine *MemoryEngine, storage storage.Interface, logger *log.Logger) (*MemoryOrchestrator, error) {
	if engine == nil {
		return nil, fmt.Errorf("memory engine cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &MemoryOrchestrator{
		engine:  engine,
		storage: storage,
		logger:  logger,
	}, nil
}

// DocumentExtractionJob wraps a document for the worker pool.
type DocumentExtractionJob struct {
	Document memory.Document
	Service  *ai.Service
	Model    string
	Logger   *log.Logger
}

func (j DocumentExtractionJob) Process(ctx context.Context) ([]FactResult, error) {
	facts, err := ExtractFactsFromDocument(ctx, j.Document, j.Service, j.Model, j.Logger)
	if err != nil {
		return nil, fmt.Errorf("fact extraction failed for document %s: %w", j.Document.ID(), err)
	}

	results := make([]FactResult, 0, len(facts))
	for _, fact := range facts {
		if fact.GenerateContent() != "" {
			results = append(results, FactResult{
				Fact:   fact,
				Source: j.Document,
			})
		}
	}

	return results, nil
}

// FactProcessingJob is no longer used in the simplified flow.
// Complex fact processing has been replaced with direct storage.

// ProcessDocuments implements the simplified processing pipeline with streaming progress.
// This version removes the computational explosion by storing extracted facts directly
// without complex decision-making, similarity searches, or per-fact processing.
func (o *MemoryOrchestrator) ProcessDocuments(ctx context.Context, documents []memory.Document, config Config) (<-chan Progress, <-chan error) {
	progressCh := make(chan Progress, 100)
	errorCh := make(chan error, 100)

	go func() {
		defer close(progressCh)
		defer close(errorCh)

		o.logger.Info("ProcessDocuments: Starting processing pipeline",
			"documentCount", len(documents),
			"workers", config.Workers,
			"timeout", config.FactExtractionTimeout)

		if config.Workers <= 0 {
			errorCh <- fmt.Errorf("invalid worker count: %d, must be > 0", config.Workers)
			return
		}

		var chunkedDocs []memory.Document
		for _, doc := range documents {
			chunks := doc.Chunk()
			chunkedDocs = append(chunkedDocs, chunks...)
		}

		o.logger.Info("ProcessDocuments: Document chunking completed",
			"originalDocs", len(documents),
			"totalChunks", len(chunkedDocs))

		totalDocuments := len(chunkedDocs)
		if totalDocuments == 0 {
			o.logger.Warn("ProcessDocuments: No chunks created from documents")
			progressCh <- Progress{
				Processed: 0,
				Total:     len(documents),
				Stage:     "preparation",
			}
			return
		}

		extractJobs := make([]DocumentExtractionJob, len(chunkedDocs))
		for i, doc := range chunkedDocs {
			extractJobs[i] = DocumentExtractionJob{
				Document: doc,
				Service:  o.engine.CompletionsService,
				Model:    o.engine.CompletionsModel,
				Logger:   o.logger,
			}
		}

		o.logger.Info("ProcessDocuments: Created extraction jobs",
			"jobCount", len(extractJobs),
			"workers", config.Workers,
			"timeout", config.FactExtractionTimeout)

		extractPool := NewWorkerPool[DocumentExtractionJob](config.Workers, o.logger)
		extractionResults := extractPool.Process(ctx, extractJobs, config.FactExtractionTimeout)

		o.logger.Info("ProcessDocuments: Worker pool created, starting result collection")

		var currentBatch []FactResult
		var totalStoredFacts int

		o.logger.Info("ProcessDocuments: Starting to collect extraction results with batching",
			"expectedResults", len(extractJobs),
			"batchSize", config.BatchSize)

		collectionCtx, cancel := context.WithTimeout(ctx, config.FactExtractionTimeout)
		defer cancel()

		resultCount := 0
		maxResults := len(extractJobs)

		storageImpl := &StorageImpl{
			logger:       o.logger,
			orchestrator: o,
			storage:      o.storage,
			engine:       o.engine,
		}

		storeBatch := func(batch []FactResult) error {
			if len(batch) == 0 {
				return nil
			}

			var facts []*memory.MemoryFact
			for _, factResult := range batch {
				if factResult.Fact != nil {
					facts = append(facts, factResult.Fact)
				}
			}

			if len(facts) == 0 {
				return nil
			}

			o.logger.Info("ProcessDocuments: Storing batch of facts",
				"batchSize", len(facts),
				"totalStoredSoFar", totalStoredFacts)

			if err := storageImpl.StoreFactsDirectly(ctx, facts, func(processed, total int) {
				progressCh <- Progress{
					Processed: totalStoredFacts + processed,
					Total:     maxResults,
					Stage:     "storage",
				}
			}); err != nil {
				return fmt.Errorf("failed to store batch: %w", err)
			}

			totalStoredFacts += len(facts)
			o.logger.Info("ProcessDocuments: Successfully stored batch",
				"batchSize", len(facts),
				"totalStoredFacts", totalStoredFacts)

			return nil
		}

		for {
			select {
			case result, ok := <-extractionResults:
				if !ok {
					o.logger.Info("ProcessDocuments: All extraction results collected",
						"totalResults", resultCount,
						"expectedResults", maxResults)
					break
				}

				resultCount++
				o.logger.Debug("ProcessDocuments: Received extraction result",
					"resultIndex", resultCount,
					"totalExpected", maxResults,
					"hasError", result.Error != nil)

				if result.Error != nil {
					o.logger.Errorf("Extraction failed: %v", result.Error)
					continue
				}

				currentBatch = append(currentBatch, result.Result...)
				o.logger.Debug("ProcessDocuments: Added facts from result",
					"resultIndex", resultCount,
					"factsInResult", len(result.Result),
					"currentBatchSize", len(currentBatch))

				if len(currentBatch) >= config.BatchSize {
					if err := storeBatch(currentBatch); err != nil {
						o.logger.Error("ProcessDocuments: Failed to store batch",
							"error", err,
							"batchSize", len(currentBatch))
						select {
						case errorCh <- fmt.Errorf("batch storage failed: %w", err):
						case <-ctx.Done():
						}
						return
					}
					currentBatch = nil
				}

				if resultCount >= maxResults {
					o.logger.Info("ProcessDocuments: Received all expected results, will store remaining batch")
					break
				}

			case <-collectionCtx.Done():
				o.logger.Warn("ProcessDocuments: Result collection timed out, will store remaining batch",
					"collectedResults", resultCount,
					"expectedResults", maxResults,
					"remainingBatchSize", len(currentBatch))
				break

			case <-ctx.Done():
				o.logger.Warn("ProcessDocuments: Main context cancelled during result collection")
				return
			}

			if resultCount >= maxResults || collectionCtx.Err() != nil {
				break
			}
		}

		if len(currentBatch) > 0 {
			o.logger.Info("ProcessDocuments: Storing final batch",
				"finalBatchSize", len(currentBatch),
				"totalStoredSoFar", totalStoredFacts)

			if err := storeBatch(currentBatch); err != nil {
				o.logger.Error("ProcessDocuments: Failed to store final batch",
					"error", err,
					"batchSize", len(currentBatch))
				select {
				case errorCh <- fmt.Errorf("final batch storage failed: %w", err):
				case <-ctx.Done():
				}
				return
			}
		}

		o.logger.Info("ProcessDocuments: Finished processing and storing all facts",
			"totalResults", resultCount,
			"totalStoredFacts", totalStoredFacts,
			"expectedResults", len(extractJobs))

		progressCh <- Progress{
			Processed: totalStoredFacts,
			Total:     totalStoredFacts,
			Stage:     "completed",
		}
	}()

	return progressCh, errorCh
}
