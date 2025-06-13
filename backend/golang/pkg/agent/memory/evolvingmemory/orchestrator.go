package evolvingmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

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

// FactProcessingJob wraps a fact for memory decision processing.
type FactProcessingJob struct {
	FactResult FactResult
	Engine     *MemoryEngine
	Logger     *log.Logger
}

func (j FactProcessingJob) Process(ctx context.Context) (FactResult, error) {
	result, err := j.Engine.ProcessFact(ctx, j.FactResult.Fact, j.FactResult.Source)
	if err != nil {
		return FactResult{}, fmt.Errorf("failed to process fact: %w", err)
	}

	j.FactResult.Decision = result.Decision
	j.FactResult.Object = result.Object
	j.FactResult.Error = result.Error

	return j.FactResult, nil
}

// ProcessDocuments implements the new parallel processing pipeline with streaming progress.
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

		// Step 4: Collect facts and create processing jobs
		var processJobs []FactProcessingJob
		for result := range extractionResults {
			if result.Error != nil {
				o.logger.Errorf("Extraction failed: %v", result.Error)
				continue
			}

			for _, factResult := range result.Result {
				processJobs = append(processJobs, FactProcessingJob{
					FactResult: factResult,
					Engine:     o.engine,
					Logger:     o.logger,
				})
			}
		}

		// Step 5: Process facts in parallel
		processPool := NewWorkerPool[FactProcessingJob](config.Workers, o.logger)
		processingResults := processPool.Process(ctx, processJobs, config.MemoryDecisionTimeout)

		// Step 6: Convert ProcessResult channel to FactResult channel
		factResultCh := make(chan FactResult, 1000)
		go func() {
			defer close(factResultCh)
			for result := range processingResults {
				if result.Error != nil {
					o.logger.Errorf("Processing failed: %v", result.Error)
					continue
				}
				select {
				case factResultCh <- result.Result:
				case <-ctx.Done():
					return
				}
			}
		}()

		// Step 7: Aggregate and store results
		objectStream := make(chan []*models.Object, 100)

		var aggregateWg sync.WaitGroup
		aggregateWg.Add(1)
		go o.aggregateResults(ctx, factResultCh, objectStream, &aggregateWg, config)

		// Step 8: Store batches
		var storeWg sync.WaitGroup
		storeWg.Add(1)
		go o.streamingStore(ctx, objectStream, progressCh, errorCh, &storeWg, config, totalDocuments)

		aggregateWg.Wait()
		close(objectStream)
		storeWg.Wait()
	}()

	return progressCh, errorCh
}

// aggregateResults collects ADD operations for batch processing.
func (o *MemoryOrchestrator) aggregateResults(
	ctx context.Context,
	results <-chan FactResult,
	out chan<- []*models.Object,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	// Pre-allocate batch with expected capacity
	batch := make([]*models.Object, 0, config.BatchSize)
	ticker := time.NewTicker(config.FlushInterval)
	defer ticker.Stop()

	flushBatch := func() {
		if len(batch) > 0 {
			select {
			case out <- batch:
				batch = make([]*models.Object, 0, config.BatchSize)
			case <-ctx.Done():
				return
			}
		}
	}

	for {
		select {
		case result, ok := <-results:
			if !ok {
				flushBatch()
				return
			}

			if result.Error != nil {
				o.logger.Errorf("Fact processing failed: %v", result.Error)
				continue
			}

			if result.Object != nil {
				batch = append(batch, result.Object)

				if len(batch) >= config.BatchSize {
					flushBatch()
				}
			}

		case <-ticker.C:
			flushBatch()

		case <-ctx.Done():
			return
		}
	}
}

// streamingStore handles batch storage with progress reporting using the engine.
func (o *MemoryOrchestrator) streamingStore(
	ctx context.Context,
	batches <-chan []*models.Object,
	progress chan<- Progress,
	errors chan<- error,
	wg *sync.WaitGroup,
	config Config,
	totalDocuments int,
) {
	defer wg.Done()

	totalProcessed := 0

	for batch := range batches {
		if len(batch) == 0 {
			continue
		}

		storeCtx, cancel := context.WithTimeout(ctx, config.StorageTimeout)
		err := o.storage.StoreBatch(storeCtx, batch)
		cancel()

		if err != nil {
			select {
			case errors <- fmt.Errorf("batch storage failed: %w", err):
			case <-ctx.Done():
				return
			}
			return
		}

		totalProcessed += len(batch)
		select {
		case progress <- Progress{
			Processed: totalProcessed,
			Total:     totalDocuments,
			Stage:     "storage",
		}:
		case <-ctx.Done():
			return
		}

		o.logger.Infof("Stored batch of %d objects", len(batch))
	}
}
