package evolvingmemory

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate/entities/models"

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

		// Validate configuration
		if config.Workers <= 0 {
			errorCh <- fmt.Errorf("invalid worker count: %d, must be > 0", config.Workers)
			return
		}

		// Step 1: Chunk documents
		var chunkedDocs []memory.Document
		for _, doc := range documents {
			chunks := doc.Chunk()
			chunkedDocs = append(chunkedDocs, chunks...)
		}

		totalDocuments := len(chunkedDocs)
		if totalDocuments == 0 {
			progressCh <- Progress{
				Processed: 0,
				Total:     len(documents),
				Stage:     "preparation",
			}
			return
		}

		// Step 2: Create extraction jobs
		extractJobs := make([]DocumentExtractionJob, len(chunkedDocs))
		for i, doc := range chunkedDocs {
			extractJobs[i] = DocumentExtractionJob{
				Document: doc,
				Service:  o.engine.CompletionsService,
				Model:    o.engine.CompletionsModel,
				Logger:   o.logger,
			}
		}

		// Step 3: Run extraction in parallel
		extractPool := NewWorkerPool[DocumentExtractionJob](config.Workers, o.logger)
		extractionResults := extractPool.Process(ctx, extractJobs, config.FactExtractionTimeout)

		// Step 4: NEW SIMPLIFIED FLOW - Collect extracted facts and store directly
		var allFacts []FactResult
		for result := range extractionResults {
			if result.Error != nil {
				o.logger.Errorf("Extraction failed: %v", result.Error)
				continue
			}

			allFacts = append(allFacts, result.Result...)
		}

		// Step 5: Store facts directly without complex processing
		if err := o.storeFactsDirectly(ctx, allFacts, progressCh, config); err != nil {
			select {
			case errorCh <- fmt.Errorf("direct storage failed: %w", err):
			case <-ctx.Done():
			}
			return
		}

		// Final progress update
		progressCh <- Progress{
			Processed: len(allFacts),
			Total:     totalDocuments,
			Stage:     "completed",
		}
	}()

	return progressCh, errorCh
}

// storeFactsDirectly converts extracted facts to memory objects and stores them in batches.
// This replaces the complex aggregation, decision-making, and worker pool logic.
func (o *MemoryOrchestrator) storeFactsDirectly(ctx context.Context, facts []FactResult, progressCh chan<- Progress, config Config) error {
	if len(facts) == 0 {
		o.logger.Info("No facts to store")
		return nil
	}

	var objects []*models.Object
	processed := 0

	// Convert facts to memory objects
	for _, factResult := range facts {
		if factResult.Fact == nil {
			continue
		}

		// Store the source document and get its ID
		documentID, err := o.storage.UpsertDocument(ctx, factResult.Source)
		if err != nil {
			o.logger.Errorf("Failed to store document for fact: %v", err)
			continue
		}

		// Create memory object with document reference
		obj := CreateMemoryObjectWithDocumentReferences(factResult.Fact, factResult.Source, []string{documentID})

		// Generate embedding for the fact
		embedding, err := o.engine.EmbeddingsWrapper.Embedding(ctx, factResult.Fact.GenerateContent())
		if err != nil {
			o.logger.Errorf("Failed to generate embedding for fact: %v", err)
			continue
		}

		obj.Vector = embedding
		objects = append(objects, obj)

		// Batch storage when we reach the batch size
		if len(objects) >= config.BatchSize {
			if err := o.storage.StoreBatch(ctx, objects); err != nil {
				return fmt.Errorf("batch storage failed: %w", err)
			}

			processed += len(objects)
			objects = objects[:0] // Reset slice

			// Report progress
			select {
			case progressCh <- Progress{
				Processed: processed,
				Total:     len(facts),
				Stage:     "storage",
			}:
			case <-ctx.Done():
				return ctx.Err()
			}

			o.logger.Infof("Stored batch of %d memory objects", config.BatchSize)
		}
	}

	// Store remaining objects
	if len(objects) > 0 {
		if err := o.storage.StoreBatch(ctx, objects); err != nil {
			return fmt.Errorf("final batch storage failed: %w", err)
		}
		processed += len(objects)
		o.logger.Infof("Stored final batch of %d memory objects", len(objects))
	}

	o.logger.Infof("Successfully stored %d memory objects from %d facts", processed, len(facts))
	return nil
}
