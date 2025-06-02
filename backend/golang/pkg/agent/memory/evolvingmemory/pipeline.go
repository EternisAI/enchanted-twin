package evolvingmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// DefaultConfig provides sensible defaults for the pipeline.
func DefaultConfig() Config {
	return Config{
		Workers:                4,
		FactsPerWorker:         10,
		BatchSize:              100,
		FlushInterval:          5 * time.Second,
		FactExtractionTimeout:  30 * time.Second,
		MemoryDecisionTimeout:  30 * time.Second,
		StorageTimeout:         30 * time.Second,
		EnableRichContext:      true,
		ParallelFactExtraction: true,
		StreamingProgress:      true,
	}
}

// StoreV2 implements the new parallel processing pipeline with streaming progress.
func (s *WeaviateStorage) StoreV2(ctx context.Context, documents []memory.Document, config Config) (<-chan Progress, <-chan error) {
	progressCh := make(chan Progress, 100)
	errorCh := make(chan error, 100)

	go func() {
		defer close(progressCh)
		defer close(errorCh)

		// Stage 1: Prepare documents (Pure)
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

		// Stage 2: Distribute work (Pure)
		workChunks := DistributeWork(prepared, config.Workers)

		// Stage 3: Create adapters
		factExtractor := NewFactExtractor(s)
		memoryOps := NewMemoryOperations(s)

		// Stage 4: Parallel Processing Pipeline
		factStream := make(chan ExtractedFact, 1000)
		resultStream := make(chan FactResult, 1000)
		objectStream := make(chan []*models.Object, 100)

		// Workers: Document → Facts
		var extractWg sync.WaitGroup
		for i, chunk := range workChunks {
			extractWg.Add(1)
			go s.extractFactsWorker(ctx, chunk, factStream, &extractWg, factExtractor, i, config)
		}

		// Workers: Facts → Decisions → Results
		var processWg sync.WaitGroup
		for i := 0; i < config.Workers; i++ {
			processWg.Add(1)
			go s.processFactsWorker(ctx, factStream, resultStream, &processWg, memoryOps, i, config)
		}

		// Aggregator: Results → Batches
		var aggregateWg sync.WaitGroup
		aggregateWg.Add(1)
		go s.aggregateResults(ctx, resultStream, objectStream, &aggregateWg, config)

		// Storage: Batches → Weaviate
		var storeWg sync.WaitGroup
		storeWg.Add(1)
		go s.streamingStore(ctx, objectStream, progressCh, errorCh, &storeWg, config)

		// Close channels in order
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

// extractFactsWorker processes documents and extracts facts.
func (s *WeaviateStorage) extractFactsWorker(
	ctx context.Context,
	docs []PreparedDocument,
	out chan<- ExtractedFact,
	wg *sync.WaitGroup,
	factExtractor FactExtractor,
	workerID int,
	config Config,
) {
	defer wg.Done()

	for _, doc := range docs {
		// Apply timeout for fact extraction
		extractCtx, cancel := context.WithTimeout(ctx, config.FactExtractionTimeout)

		facts, err := factExtractor.ExtractFacts(extractCtx, doc)
		cancel()

		if err != nil {
			s.logger.Errorf("Worker %d: Failed to extract facts from document %s: %v", workerID, doc.Original.ID(), err)
			continue
		}

		// Send facts to the stream
		for _, factContent := range facts {
			if factContent == "" {
				continue
			}

			fact := ExtractedFact{
				Content:   factContent,
				SpeakerID: doc.SpeakerID,
				Source:    doc,
			}

			select {
			case out <- fact:
			case <-ctx.Done():
				return
			}
		}
	}
}

// processFactsWorker processes facts through memory decisions.
func (s *WeaviateStorage) processFactsWorker(
	ctx context.Context,
	facts <-chan ExtractedFact,
	out chan<- FactResult,
	wg *sync.WaitGroup,
	memoryOps MemoryOperations,
	workerID int,
	config Config,
) {
	defer wg.Done()

	for fact := range facts {
		result := s.processSingleFact(ctx, fact, memoryOps, config)

		select {
		case out <- result:
		case <-ctx.Done():
			return
		}
	}
}

// processSingleFact encapsulates the memory update logic.
func (s *WeaviateStorage) processSingleFact(
	ctx context.Context,
	fact ExtractedFact,
	memoryOps MemoryOperations,
	config Config,
) FactResult {
	// Step 1: Search for similar memories
	searchCtx, searchCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	similar, err := memoryOps.SearchSimilar(searchCtx, fact.Content, fact.SpeakerID)
	searchCancel()

	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("search failed: %w", err)}
	}

	// Step 2: LLM decides action
	decisionCtx, decisionCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	decision, err := memoryOps.DecideAction(decisionCtx, fact.Content, similar)
	decisionCancel()

	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("decision failed: %w", err)}
	}

	// Step 3: Validate the operation
	if decision.Action == UPDATE || decision.Action == DELETE {
		targetMemory := findMemoryByID(similar, decision.TargetID)
		if targetMemory == nil {
			return FactResult{Fact: fact, Error: fmt.Errorf("target memory %s not found", decision.TargetID)}
		}

		rule := ValidationRule{
			CurrentSpeakerID: fact.SpeakerID,
			IsDocumentLevel:  fact.SpeakerID == "",
			TargetMemoryID:   decision.TargetID,
			TargetSpeakerID:  targetMemory.Metadata["speakerID"],
			Action:           decision.Action,
		}

		if err := ValidateMemoryOperation(rule); err != nil {
			s.logger.Warnf("Validation failed: %v", err)
			return FactResult{Fact: fact, Decision: decision, Error: err}
		}
	}

	// Step 4: Execute the action
	switch decision.Action {
	case UPDATE:
		// Generate new embedding
		embedding, err := s.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: err}
		}

		// Convert to float32
		embedding32 := toFloat32(embedding)

		// Execute update
		updateCtx, updateCancel := context.WithTimeout(ctx, config.StorageTimeout)
		err = memoryOps.UpdateMemory(updateCtx, decision.TargetID, fact.Content, embedding32)
		updateCancel()

		return FactResult{Fact: fact, Decision: decision, Error: err}

	case DELETE:
		// Execute delete
		deleteCtx, deleteCancel := context.WithTimeout(ctx, config.StorageTimeout)
		err := memoryOps.DeleteMemory(deleteCtx, decision.TargetID)
		deleteCancel()

		return FactResult{Fact: fact, Decision: decision, Error: err}

	case ADD:
		// Create object for batch insert
		obj := CreateMemoryObject(fact, decision)

		// Generate embedding
		embedding, err := s.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: err}
		}
		obj.Vector = toFloat32(embedding)

		return FactResult{Fact: fact, Decision: decision, Object: obj}

	case NONE:
		s.logger.Debugf("No action taken for fact: %s", fact.Content)
		return FactResult{Fact: fact, Decision: decision}

	default:
		return FactResult{Fact: fact, Error: fmt.Errorf("unknown action: %s", decision.Action)}
	}
}

// aggregateResults collects results and batches objects for storage.
func (s *WeaviateStorage) aggregateResults(
	ctx context.Context,
	results <-chan FactResult,
	out chan<- []*models.Object,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	batch := make([]*models.Object, 0, config.BatchSize)
	ticker := time.NewTicker(config.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			batchCopy := make([]*models.Object, len(batch))
			copy(batchCopy, batch)

			select {
			case out <- batchCopy:
				batch = batch[:0] // Reset batch
			case <-ctx.Done():
				return
			}
		}
	}

	for {
		select {
		case result, ok := <-results:
			if !ok {
				// Channel closed, flush remaining
				flush()
				return
			}

			// Log errors but don't stop
			if result.Error != nil {
				s.logger.Errorf("Fact processing error: %v", result.Error)
				continue
			}

			// Add object to batch if it's an ADD action
			if result.Object != nil {
				batch = append(batch, result.Object)

				// Flush if batch is full
				if len(batch) >= config.BatchSize {
					flush()
				}
			}

		case <-ticker.C:
			// Periodic flush
			flush()

		case <-ctx.Done():
			return
		}
	}
}

// streamingStore handles batch storage to Weaviate with progress reporting.
func (s *WeaviateStorage) streamingStore(
	ctx context.Context,
	batches <-chan []*models.Object,
	progress chan<- Progress,
	errors chan<- error,
	wg *sync.WaitGroup,
	config Config,
) {
	defer wg.Done()

	totalStored := 0

	for batch := range batches {
		if len(batch) == 0 {
			continue
		}

		// Store batch with timeout
		storeCtx, cancel := context.WithTimeout(ctx, config.StorageTimeout)
		err := s.StoreBatch(storeCtx, batch)
		cancel()

		if err != nil {
			select {
			case errors <- fmt.Errorf("batch storage failed: %w", err):
			case <-ctx.Done():
				return
			}
		} else {
			totalStored += len(batch)

			// Report progress
			select {
			case progress <- Progress{
				Processed: totalStored,
				Stage:     "storage",
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Helper functions

func findMemoryByID(memories []ExistingMemory, id string) *ExistingMemory {
	for i := range memories {
		if memories[i].ID == id {
			return &memories[i]
		}
	}
	return nil
}

func toFloat32(embedding []float64) []float32 {
	result := make([]float32, len(embedding))
	for i, v := range embedding {
		result[i] = float32(v)
	}
	return result
}
