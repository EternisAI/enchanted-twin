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

		// Chunk documents first
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

		// Create channels for the pipeline
		documentQueue := make(chan memory.Document, len(chunkedDocs))
		factStream := make(chan FactResult, 1000)
		resultStream := make(chan FactResult, 1000)
		objectStream := make(chan []*models.Object, 100)

		// Load all documents into the queue
		for _, doc := range chunkedDocs {
			documentQueue <- doc
		}
		close(documentQueue)

		// Start extraction workers that pull from the shared queue
		var extractWg sync.WaitGroup
		for i := range config.Workers {
			extractWg.Add(1)
			go o.extractFactsWorkerDynamic(ctx, documentQueue, factStream, &extractWg, i, config)
		}

		var processWg sync.WaitGroup
		for range config.Workers {
			processWg.Add(1)
			go o.processFactsWorker(ctx, factStream, resultStream, &processWg, config)
		}

		var aggregateWg sync.WaitGroup
		aggregateWg.Add(1)
		go o.aggregateResults(ctx, resultStream, objectStream, &aggregateWg, config)

		var storeWg sync.WaitGroup
		storeWg.Add(1)
		go o.streamingStore(ctx, objectStream, progressCh, errorCh, &storeWg, config, totalDocuments)

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

// extractFactsWorkerDynamic processes documents from a shared queue (work-stealing pattern).
func (o *MemoryOrchestrator) extractFactsWorkerDynamic(
	ctx context.Context,
	documentQueue <-chan memory.Document,
	out chan<- FactResult,
	wg *sync.WaitGroup,
	workerID int,
	config Config,
) {
	defer wg.Done()

	processedCount := 0
	failedCount := 0
	startTime := time.Now()

	for doc := range documentQueue {
		docStartTime := time.Now()
		o.logger.Debugf("Worker %d: Starting document %s (queue depth: ~%d)", workerID, doc.ID(), len(documentQueue))

		extractCtx, cancel := context.WithTimeout(ctx, config.FactExtractionTimeout)

		facts, err := ExtractFactsFromDocument(extractCtx, doc, o.engine.CompletionsService, o.engine.CompletionsModel, o.logger)
		cancel()

		if err != nil {
			o.logger.Errorf("Worker %d: Failed to extract facts from document %s: %v", workerID, doc.ID(), err)
			failedCount++

			// Send error as a FactResult with error field
			result := FactResult{
				Source: doc,
				Error:  fmt.Errorf("fact extraction failed for document %s: %w", doc.ID(), err),
			}
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
			continue
		}

		processedCount++
		processingTime := time.Since(docStartTime)
		o.logger.Debugf("Worker %d: Completed document %s in %v (extracted %d facts)", workerID, doc.ID(), processingTime, len(facts))

		for _, fact := range facts {
			if fact.GenerateContent() == "" {
				continue
			}
			// Create FactResult with the source document
			result := FactResult{
				Fact:   fact,
				Source: doc,
			}
			select {
			case out <- result:
			case <-ctx.Done():
				o.logger.Infof("Worker %d: Processed %d documents before context cancellation", workerID, processedCount)
				return
			}
		}
	}

	totalTime := time.Since(startTime)
	if processedCount > 0 || failedCount > 0 {
		o.logger.Infof("Worker %d: Completed, processed %d documents, %d failed in %v (avg: %v/doc)",
			workerID, processedCount, failedCount, totalTime, totalTime/time.Duration(processedCount+failedCount))
	} else {
		o.logger.Infof("Worker %d: Completed, processed 0 documents", workerID)
	}
}

// processFactsWorker processes facts through memory decisions using the engine.
func (o *MemoryOrchestrator) processFactsWorker(
	ctx context.Context,
	facts <-chan FactResult,
	out chan<- FactResult,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	for factResult := range facts {
		// Process the fact and update the result
		processedResult := o.processSingleFact(ctx, factResult, config)

		select {
		case out <- processedResult:
		case <-ctx.Done():
			return
		}
	}
}

// processSingleFact encapsulates the memory update logic using the engine.
func (o *MemoryOrchestrator) processSingleFact(
	ctx context.Context,
	factResult FactResult,
	config Config,
) FactResult {
	processCtx, processCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	defer processCancel()

	// Pass both the fact and source to ProcessFact
	result, err := o.engine.ProcessFact(processCtx, factResult.Fact, factResult.Source)
	if err != nil {
		o.logger.Errorf("Failed to process fact: %v", err)
		factResult.Error = err
		return factResult
	}

	// Update the existing factResult with the processing results
	factResult.Decision = result.Decision
	factResult.Object = result.Object
	factResult.Error = result.Error

	return factResult
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
