package evolvingmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// DefaultConfig provides default configuration values.
func DefaultConfig() Config {
	return Config{
		Workers:                4,
		FactsPerWorker:         50,
		BatchSize:              100,
		FlushInterval:          30 * time.Second,
		FactExtractionTimeout:  30 * time.Second,
		MemoryDecisionTimeout:  30 * time.Second,
		StorageTimeout:         30 * time.Second,
		EnableRichContext:      true,
		ParallelFactExtraction: true,
		StreamingProgress:      true,
	}
}

// StoreV2 implements the new parallel processing pipeline with streaming progress.
func (s *StorageImpl) StoreV2(ctx context.Context, documents []memory.Document, config Config) (<-chan Progress, <-chan error) {
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

		factExtractor, err := NewFactExtractor(s)
		if err != nil {
			select {
			case errorCh <- fmt.Errorf("creating fact extractor: %w", err):
			case <-ctx.Done():
				return
			}
			return
		}

		memoryOps, err := NewMemoryOperations(s)
		if err != nil {
			select {
			case errorCh <- fmt.Errorf("creating memory operations: %w", err):
			case <-ctx.Done():
				return
			}
			return
		}

		factStream := make(chan ExtractedFact, 1000)
		resultStream := make(chan FactResult, 1000)
		objectStream := make(chan []*models.Object, 100)

		var extractWg sync.WaitGroup
		for i, chunk := range workChunks {
			extractWg.Add(1)
			go s.extractFactsWorker(ctx, chunk, factStream, &extractWg, factExtractor, i, config)
		}

		var processWg sync.WaitGroup
		for i := 0; i < config.Workers; i++ {
			processWg.Add(1)
			go s.processFactsWorker(ctx, factStream, resultStream, &processWg, memoryOps, i, config)
		}

		var aggregateWg sync.WaitGroup
		aggregateWg.Add(1)
		go s.aggregateResults(ctx, resultStream, objectStream, &aggregateWg, config)

		var storeWg sync.WaitGroup
		storeWg.Add(1)
		go s.streamingStore(ctx, objectStream, progressCh, errorCh, &storeWg, config)

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
func (s *StorageImpl) extractFactsWorker(
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
		extractCtx, cancel := context.WithTimeout(ctx, config.FactExtractionTimeout)

		facts, err := factExtractor.ExtractFacts(extractCtx, doc)
		cancel()

		if err != nil {
			s.logger.Errorf("Worker %d: Failed to extract facts from document %s: %v", workerID, doc.Original.ID(), err)
			continue
		}

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
func (s *StorageImpl) processFactsWorker(
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
func (s *StorageImpl) processSingleFact(
	ctx context.Context,
	fact ExtractedFact,
	memoryOps MemoryOperations,
	config Config,
) FactResult {
	searchCtx, searchCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	similar, err := memoryOps.SearchSimilar(searchCtx, fact.Content, fact.SpeakerID)
	searchCancel()

	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("search failed: %w", err)}
	}

	decisionCtx, decisionCancel := context.WithTimeout(ctx, config.MemoryDecisionTimeout)
	decision, err := memoryOps.DecideAction(decisionCtx, fact.Content, similar)
	decisionCancel()

	if err != nil {
		return FactResult{Fact: fact, Error: fmt.Errorf("decision failed: %w", err)}
	}

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

	// Execute immediate operations (UPDATE/DELETE)
	switch decision.Action {
	case UPDATE:
		embedding, err := s.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("embedding failed: %w", err)}
		}

		if err := memoryOps.UpdateMemory(ctx, decision.TargetID, fact.Content, toFloat32(embedding)); err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("update failed: %w", err)}
		}

		s.logger.Infof("Updated memory %s with new content", decision.TargetID)
		return FactResult{Fact: fact, Decision: decision}
	case DELETE:
		if err := memoryOps.DeleteMemory(ctx, decision.TargetID); err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("delete failed: %w", err)}
		}

		s.logger.Infof("Deleted memory %s", decision.TargetID)
		return FactResult{Fact: fact, Decision: decision}
	case ADD:
		// Prepare for batch processing
		obj, err := CreateMemoryObjectWithEmbedding(ctx, fact, decision, s.embeddingsService)
		if err != nil {
			return FactResult{Fact: fact, Decision: decision, Error: fmt.Errorf("object creation failed: %w", err)}
		}

		return FactResult{Fact: fact, Decision: decision, Object: obj}
	}

	// NONE action - do nothing
	return FactResult{Fact: fact, Decision: decision}
}

// aggregateResults collects ADD operations for batch processing.
func (s *StorageImpl) aggregateResults(
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
				s.logger.Errorf("Fact processing failed: %v", result.Error)
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

// streamingStore handles batch storage with progress reporting.
func (s *StorageImpl) streamingStore(
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
		err := s.storage.StoreBatch(storeCtx, batch)
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
			Total:     totalProcessed, // Will be updated by caller
			Stage:     "storage",
		}:
		case <-ctx.Done():
			return
		}

		s.logger.Infof("Stored batch of %d objects", len(batch))
	}
}

func findMemoryByID(memories []ExistingMemory, id string) *ExistingMemory {
	for i := range memories {
		if memories[i].ID == id {
			return &memories[i]
		}
	}
	return nil
}
