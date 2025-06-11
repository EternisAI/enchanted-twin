package evolvingmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// MemoryOrchestrator defines the coordination interface for memory operations.
// This interface handles orchestration concerns (channels, workers, progress reporting).
type MemoryOrchestrator interface {
	ProcessDocuments(ctx context.Context, docs []memory.Document, config Config) (<-chan Progress, <-chan error)
}

// memoryOrchestrator implements MemoryOrchestrator with coordination logic.
type memoryOrchestrator struct {
	engine *MemoryEngine
	logger *log.Logger
}

// NewMemoryOrchestrator creates a new MemoryOrchestrator instance.
func NewMemoryOrchestrator(engine *MemoryEngine, logger *log.Logger) (MemoryOrchestrator, error) {
	if engine == nil {
		return nil, fmt.Errorf("memory engine cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &memoryOrchestrator{
		engine: engine,
		logger: logger,
	}, nil
}

// ProcessDocuments implements the new parallel processing pipeline with streaming progress.
func (o *memoryOrchestrator) ProcessDocuments(ctx context.Context, documents []memory.Document, config Config) (<-chan Progress, <-chan error) {
	progressCh := make(chan Progress, 100)
	errorCh := make(chan error, 100)

	go func() {
		defer close(progressCh)
		defer close(errorCh)

		prepared, prepError := PrepareDocuments(documents, time.Now())
		if prepError != nil {
			select {
			case errorCh <- prepError:
			case <-ctx.Done():
				return
			}
			return
		}

		if len(prepared) == 0 {
			progressCh <- Progress{
				Processed: 0,
				Total:     len(documents),
				Stage:     "preparation",
			}
			return
		}

		workChunks := DistributeWork(prepared, config.Workers)

		factStream := make(chan ExtractedFact, 1000)
		resultStream := make(chan FactResult, 1000)
		objectStream := make(chan []*models.Object, 100)

		var extractWg sync.WaitGroup
		for i, chunk := range workChunks {
			extractWg.Add(1)
			go o.extractFactsWorker(ctx, chunk, factStream, &extractWg, i, config)
		}

		var processWg sync.WaitGroup
		for i := 0; i < config.Workers; i++ {
			processWg.Add(1)
			go o.processFactsWorker(ctx, factStream, resultStream, &processWg, i, config)
		}

		var aggregateWg sync.WaitGroup
		aggregateWg.Add(1)
		go o.aggregateResults(ctx, resultStream, objectStream, &aggregateWg, config)

		var storeWg sync.WaitGroup
		storeWg.Add(1)
		go o.streamingStore(ctx, objectStream, progressCh, errorCh, &storeWg, config)

		extractWg.Wait()
		close(factStream)

		processWg.Wait()
		close(resultStream)

		aggregateWg.Wait()
		close(objectStream)

		storeWg.Wait()
	}()

	return progressCh, errorCh
}

// extractFactsWorker processes documents and extracts facts using the engine.
func (o *memoryOrchestrator) extractFactsWorker(
	ctx context.Context,
	docs []PreparedDocument,
	out chan<- ExtractedFact,
	wg *sync.WaitGroup,
	workerID int,
	config Config,
) {
	defer wg.Done()

	for _, doc := range docs {
		extractCtx, cancel := context.WithTimeout(ctx, config.FactExtractionTimeout)

		facts, err := o.engine.ExtractFacts(extractCtx, doc)
		cancel()

		if err != nil {
			o.logger.Errorf("Worker %d: Failed to extract facts from document %s: %v", workerID, doc.Original.ID(), err)
			continue
		}

		for _, fact := range facts {
			if fact.Content == "" {
				continue
			}

			// Populate the missing fields in ExtractedFact
			fact.Source = doc

			select {
			case out <- fact:
			case <-ctx.Done():
				return
			}
		}
	}
}

// processFactsWorker processes facts through memory decisions using the engine.
func (o *memoryOrchestrator) processFactsWorker(
	ctx context.Context,
	facts <-chan ExtractedFact,
	out chan<- FactResult,
	wg *sync.WaitGroup,
	workerID int,
	config Config,
) {
	defer wg.Done()

	for fact := range facts {
		result := o.processSingleFact(ctx, fact, config)

		select {
		case out <- result:
		case <-ctx.Done():
			return
		}
	}
}

// processSingleFact encapsulates the memory update logic using the engine.
func (o *memoryOrchestrator) processSingleFact(
	ctx context.Context,
	fact ExtractedFact,
	config Config,
) FactResult {
	processCtx, processCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	defer processCancel()

	result, err := o.engine.ProcessFact(processCtx, fact)
	if err != nil {
		o.logger.Errorf("Failed to process fact: %v", err)
		return FactResult{Fact: fact, Error: err}
	}

	return result
}

// aggregateResults collects ADD operations for batch processing.
func (o *memoryOrchestrator) aggregateResults(
	ctx context.Context,
	results <-chan FactResult,
	out chan<- []*models.Object,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	var batch []*models.Object
	ticker := time.NewTicker(config.FlushInterval)
	defer ticker.Stop()

	flushBatch := func() {
		if len(batch) > 0 {
			select {
			case out <- batch:
				batch = nil
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
func (o *memoryOrchestrator) streamingStore(
	ctx context.Context,
	batches <-chan []*models.Object,
	progress chan<- Progress,
	errors chan<- error,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	totalProcessed := 0

	for batch := range batches {
		if len(batch) == 0 {
			continue
		}

		storeCtx, cancel := context.WithTimeout(ctx, config.StorageTimeout)
		err := o.engine.StoreBatch(storeCtx, batch)
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
			Total:     totalProcessed,
			Stage:     "storage",
		}:
		case <-ctx.Done():
			return
		}

		o.logger.Infof("Stored batch of %d objects", len(batch))
	}
}
