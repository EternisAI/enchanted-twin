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

		// Step 5: Convert FactResults to MemoryFacts and use modular storage
		var facts []*memory.MemoryFact
		for _, factResult := range allFacts {
			if factResult.Fact != nil {
				facts = append(facts, factResult.Fact)
			}
		}

		// Use the new modular StoreFactsDirectly function
		storageImpl := &StorageImpl{
			logger:       o.logger,
			orchestrator: o,
			storage:      o.storage,
			engine:       o.engine,
		}

		if err := storageImpl.StoreFactsDirectly(ctx, facts, func(processed, total int) {
			progressCh <- Progress{
				Processed: processed,
				Total:     total,
				Stage:     "storage",
			}
		}); err != nil {
			select {
			case errorCh <- fmt.Errorf("modular storage failed: %w", err):
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
