package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-openapi/strfmt"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const (
	ClassName         = "MemoryFact"
	contentProperty   = "content"
	timestampProperty = "timestamp"
	tagsProperty      = "tags"
	metadataProperty  = "metadataJson"

	ExtractFactsToolName = "EXTRACT_FACTS"
)

// Document types.
type DocumentType string

const (
	DocumentTypeConversation DocumentType = "conversation"
	DocumentTypeText         DocumentType = "text"
)

// Configuration for the storage system.
type Config struct {
	// Parallelism
	Workers        int
	FactsPerWorker int

	// Batching
	BatchSize     int
	FlushInterval time.Duration

	// Timeouts
	FactExtractionTimeout time.Duration
	StorageTimeout        time.Duration

	// Features
	EnableRichContext      bool
	ParallelFactExtraction bool
	StreamingProgress      bool
}

// ExtractMemoryFactsToolArguments is used for the LLM fact extraction tool.
type ExtractMemoryFactsToolArguments struct {
	Facts []memory.MemoryFact `json:"facts"`
}

// Simplified processing result - only contains what we actually use.
type FactResult struct {
	Fact   *memory.MemoryFact
	Source memory.Document // Source document for the fact
}

// Progress reporting.
type Progress struct {
	Processed int
	Total     int
	Stage     string
	Error     error
}

// Memory search results.
type ExistingMemory struct {
	ID        string
	Content   string
	Timestamp time.Time
	Score     float64
	Metadata  map[string]string
}

type MemoryStorage interface {
	memory.Storage // Inherit the base storage interface

	// Document reference operations - now supports multiple references per memory
	GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error)

	// Direct fact storage - bypasses extraction for testing and consolidation
	StoreFactsDirectly(ctx context.Context, facts []*memory.MemoryFact, progressCallback memory.ProgressCallback) error

	// Batch fact retrieval for intelligent querying
	GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error)

	// Intelligent 3-stage query system
	IntelligentQuery(ctx context.Context, queryText string, filter *memory.Filter) (*memory.IntelligentQueryResult, error)

	// Manual consolidation - should be called periodically rather than after every storage operation
	RunConsolidation(ctx context.Context) error
}

// Dependencies holds all the required dependencies for creating a MemoryStorage instance.
type Dependencies struct {
	Logger             *log.Logger
	Storage            storage.Interface
	CompletionsService *ai.Service
	CompletionsModel   string
	EmbeddingsWrapper  *storage.EmbeddingWrapper
}

// MemoryEngine implements pure business logic for memory operations.
// This contains no orchestration concerns (channels, workers, progress reporting).
type MemoryEngine struct {
	CompletionsService *ai.Service
	EmbeddingsWrapper  *storage.EmbeddingWrapper
	storage            storage.Interface
	CompletionsModel   string
}

// NewMemoryEngine creates a new MemoryEngine instance.
func NewMemoryEngine(completionsService *ai.Service, embeddingsWrapper *storage.EmbeddingWrapper, storage storage.Interface, completionsModel string) (*MemoryEngine, error) {
	if completionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if embeddingsWrapper == nil {
		return nil, fmt.Errorf("embeddings wrapper cannot be nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}
	if completionsModel == "" {
		return nil, fmt.Errorf("completions model cannot be empty")
	}

	return &MemoryEngine{
		CompletionsService: completionsService,
		EmbeddingsWrapper:  embeddingsWrapper,
		storage:            storage,
		CompletionsModel:   completionsModel,
	}, nil
}

// StorageImpl is the main implementation of the MemoryStorage interface.
// It orchestrates memory operations using a clean 3-layer architecture:
// StorageImpl (public API) -> MemoryOrchestrator (coordination) -> MemoryEngine (business logic).
type StorageImpl struct {
	logger       *log.Logger
	orchestrator *MemoryOrchestrator
	storage      storage.Interface
	engine       *MemoryEngine
}

// New creates a new StorageImpl instance that can work with any storage backend.
func New(deps Dependencies) (MemoryStorage, error) {
	if deps.Storage == nil {
		return nil, fmt.Errorf("storage interface cannot be nil")
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if deps.CompletionsService == nil {
		return nil, fmt.Errorf("completions service cannot be nil")
	}
	if deps.EmbeddingsWrapper == nil {
		return nil, fmt.Errorf("embeddings wrapper cannot be nil")
	}

	// Create the memory engine with business logic
	engine, err := NewMemoryEngine(deps.CompletionsService, deps.EmbeddingsWrapper, deps.Storage, deps.CompletionsModel)
	if err != nil {
		return nil, fmt.Errorf("creating memory engine: %w", err)
	}

	// Create the orchestrator with coordination logic
	orchestrator, err := NewMemoryOrchestrator(engine, deps.Storage, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating memory orchestrator: %w", err)
	}

	return &StorageImpl{
		logger:       deps.Logger,
		orchestrator: orchestrator,
		storage:      deps.Storage,
		engine:       engine,
	}, nil
}

// Store implements the memory.Storage interface.
func (s *StorageImpl) Store(ctx context.Context, documents []memory.Document, callback memory.ProgressCallback) error {
	// Separate documents by processing strategy
	var factExtractionDocs []memory.Document
	var fileDocs []memory.Document

	for _, doc := range documents {
		switch doc.(type) {
		case *memory.FileDocument:
			fileDocs = append(fileDocs, doc)
		default:
			factExtractionDocs = append(factExtractionDocs, doc)
		}
	}

	s.logger.Info("Type-based document routing",
		"total", len(documents),
		"fact_extraction", len(factExtractionDocs),
		"file_direct", len(fileDocs))

	// Process fact extraction documents (existing pipeline)
	if len(factExtractionDocs) > 0 {
		if err := s.storeFactExtractionDocuments(ctx, factExtractionDocs, callback); err != nil {
			return fmt.Errorf("storing fact extraction documents: %w", err)
		}
	}

	// Process file documents directly (new simple pipeline)
	if len(fileDocs) > 0 {
		if err := s.storeFileDocumentsDirectly(ctx, fileDocs, callback); err != nil {
			return fmt.Errorf("storing file documents: %w", err)
		}
	}

	return nil
}

// storeFactExtractionDocuments handles the existing fact extraction pipeline.
func (s *StorageImpl) storeFactExtractionDocuments(ctx context.Context, documents []memory.Document, callback memory.ProgressCallback) error {
	// Use default configuration
	config := DefaultConfig()

	// Get the channels directly from orchestrator
	progressCh, errorCh := s.orchestrator.ProcessDocuments(ctx, documents, config)

	// Convert internal Progress to external ProgressUpdate and handle callback
	total := len(documents)
	processed := 0

	// Collect all errors
	var errors []error

	// Process results from both channels
	for progressCh != nil || errorCh != nil {
		select {
		case progress, ok := <-progressCh:
			if !ok {
				progressCh = nil
				continue
			}
			processed = progress.Processed
			if callback != nil {
				callback(processed, total)
			}

		case err, ok := <-errorCh:
			if !ok {
				errorCh = nil
				continue
			}
			errors = append(errors, err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// If any errors occurred, return meaningful error information
	if len(errors) > 0 {
		if len(errors) == 1 {
			s.logger.Errorf("Store encountered error: %v", errors[0])
			return errors[0]
		}
		// Multiple errors - provide comprehensive information
		s.logger.Errorf("Store encountered %d errors, returning first with details of others: %v", len(errors), errors[0])
		return fmt.Errorf("multiple errors occurred (%d total): %w, and %d more",
			len(errors), errors[0], len(errors)-1)
	}

	s.logger.Info("Facts stored successfully")

	return nil
}

// storeFileDocumentsDirectly implements the new direct file storage pipeline.
func (s *StorageImpl) storeFileDocumentsDirectly(ctx context.Context, documents []memory.Document, callback memory.ProgressCallback) error {
	s.logger.Info("Processing file documents directly", "count", len(documents))

	// Step 1: Chunk documents (reuse existing logic)
	var chunks []memory.Document
	for _, doc := range documents {
		docChunks := doc.Chunk()
		chunks = append(chunks, docChunks...)
	}

	s.logger.Info("Chunked file documents", "original_count", len(documents), "chunk_count", len(chunks))

	// Step 2: Convert chunks to MemoryFact objects (no LLM extraction)
	var facts []*memory.MemoryFact
	for _, chunk := range chunks {
		// Extract filename from metadata for natural subject
		subject := "document"
		if filename, exists := chunk.Metadata()["path"]; exists {
			// Use just the filename, not the full path
			if lastSlash := strings.LastIndex(filename, "/"); lastSlash != -1 {
				subject = filename[lastSlash+1:]
			} else {
				subject = filename
			}
		}

		fact := &memory.MemoryFact{
			ID:      "",              // Let Weaviate auto-generate UUID for file chunks
			Content: chunk.Content(), // Direct content as searchable text
			Timestamp: func() time.Time {
				if chunk.Timestamp() != nil {
					return *chunk.Timestamp()
				}
				return time.Now()
			}(),
			// Structured fields for file documents
			Category:           "document",      // Mark as document type
			Subject:            subject,         // Use filename as subject
			Attribute:          "file_chunk",    // Attribute is file_chunk
			Value:              chunk.Content(), // Value is the actual content
			TemporalContext:    nil,             // No temporal context for files
			Sensitivity:        "low",           // Files are typically low sensitivity
			Importance:         1,               // Default importance
			Source:             chunk.Source(),
			DocumentReferences: []string{}, // No document references for direct storage
			Tags:               chunk.Tags(),
			Metadata:           chunk.Metadata(),
		}
		facts = append(facts, fact)
	}

	s.logger.Info("Converted file chunks to memory facts", "fact_count", len(facts))

	// Step 3: Store using existing fact storage pipeline (includes embeddings)
	err := s.StoreFactsDirectly(ctx, facts, callback)
	if err != nil {
		return fmt.Errorf("storing file facts: %w", err)
	}

	// 🚀 FILE UPLOAD CONSOLIDATION - Only run for file uploads, not chat messages
	s.logger.Info("File documents stored successfully, starting consolidation pipeline")

	// Get required dependencies for consolidation
	deps := ConsolidationDependencies{
		Logger:             s.logger,
		Storage:            s, // Use the MemoryStorage interface (self)
		CompletionsService: s.engine.CompletionsService,
		CompletionsModel:   s.engine.CompletionsModel,
	}

	// Run consolidation for all canonical semantic subjects
	consolidationSubjects := ConsolidationSubjects[:]

	var allReports []*ConsolidationReport

	for _, subject := range consolidationSubjects {
		// Use semantic search with filtering for better results
		filter := &memory.Filter{
			Distance:          0.75,                                                  // Allow fairly broad semantic matches
			Limit:             func() *int { limit := 30; return &limit }(),          // Reasonable limit
			FactImportanceMin: func() *int { importance := 2; return &importance }(), // Only meaningful facts
		}

		report, err := ConsolidateMemoriesBySemantic(ctx, subject, filter, deps)
		if err != nil {
			s.logger.Warn("Consolidation failed for subject", "subject", subject, "error", err)
			continue // Don't fail entire process for one subject
		}

		// Only add reports that actually found facts to consolidate
		if report.SourceFactCount > 0 {
			s.logger.Info("Subject consolidation completed",
				"subject", subject,
				"source_facts", report.SourceFactCount,
				"consolidated_facts", len(report.ConsolidatedFacts))
			allReports = append(allReports, report)
		}
	}

	// Store consolidations if any were created
	if len(allReports) > 0 {
		err := StoreConsolidationReports(ctx, allReports, s, func(processed, total int) {
			s.logger.Debug("Storing consolidated facts", "progress", fmt.Sprintf("%d/%d", processed, total))
		})
		if err != nil {
			s.logger.Error("Failed to store consolidation reports", "error", err)
			// Don't fail the entire Store operation for consolidation issues
		} else {
			// Calculate totals for logging
			totalSourceFacts := 0
			totalConsolidatedFacts := 0
			for _, report := range allReports {
				totalSourceFacts += report.SourceFactCount
				totalConsolidatedFacts += len(report.ConsolidatedFacts)
			}

			s.logger.Info("File upload consolidation completed",
				"subjects_processed", len(allReports),
				"total_source_facts", totalSourceFacts,
				"total_consolidated_facts", totalConsolidatedFacts)
		}
	} else {
		s.logger.Info("No consolidations created (no qualifying facts found)")
	}

	return nil
}

// Query implements the memory.Storage interface by delegating to the storage interface.
func (s *StorageImpl) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	return s.storage.Query(ctx, queryText, filter)
}

// GetDocumentReferences retrieves all document references for a memory.
func (s *StorageImpl) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	return s.storage.GetDocumentReferences(ctx, memoryID)
}

// GetFactsByIDs retrieves multiple memory facts by their IDs.
func (s *StorageImpl) GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error) {
	return s.storage.GetFactsByIDs(ctx, factIDs)
}

// StoreFactsDirectly stores memory facts directly without going through the extraction pipeline.
// This is useful for:
// - Test suite: storing pre-extracted facts from JSON
// - Consolidation: storing consolidated facts
// - Production: final step of the pipeline (called by orchestrator).
func (s *StorageImpl) StoreFactsDirectly(ctx context.Context, facts []*memory.MemoryFact, progressCallback memory.ProgressCallback) error {
	if len(facts) == 0 {
		s.logger.Info("No facts to store")
		return nil
	}

	config := DefaultConfig()

	// Convert facts to storage objects with embeddings
	var objects []*models.Object
	processed := 0

	// Batch generate embeddings for efficiency
	var factContents []string
	for _, fact := range facts {
		factContents = append(factContents, fact.GenerateContent())
	}

	// Generate ALL embeddings in batch - MUCH FASTER! 🚀
	s.logger.Info("Generating embeddings in batch", "count", len(factContents))
	embeddings, err := s.engine.EmbeddingsWrapper.Embeddings(ctx, factContents)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	// Create Weaviate objects with pre-generated embeddings
	for i, fact := range facts {
		// Build tags
		tags := fact.Tags
		if len(tags) == 0 {
			tags = []string{fact.Category} // Fallback to category
		}

		// Build metadata JSON
		metadata := fact.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["fact_id"] = fact.ID
		if fact.TemporalContext != nil {
			metadata["temporal_context"] = *fact.TemporalContext
		}
		metadataJSON, _ := json.Marshal(metadata)

		// Create Weaviate object with structured fields
		properties := map[string]interface{}{
			"content":            fact.GenerateContent(),
			"timestamp":          fact.Timestamp.Format(time.RFC3339),
			"source":             fact.Source,
			"tags":               tags,
			"documentReferences": fact.DocumentReferences,
			"metadataJson":       string(metadataJSON),
			// Structured fact fields for precise filtering
			"factCategory":    fact.Category,
			"factSubject":     fact.Subject,
			"factAttribute":   fact.Attribute,
			"factValue":       fact.Value,
			"factSensitivity": fact.Sensitivity,
			"factImportance":  fact.Importance,
		}

		// Add temporal context if present
		if fact.TemporalContext != nil {
			properties["factTemporalContext"] = *fact.TemporalContext
		}

		obj := &models.Object{
			Class:      "MemoryFact",
			Properties: properties,
			Vector:     embeddings[i], // Use pre-generated embedding
		}

		// Only set ID if it's a valid UUID format
		if fact.ID != "" {
			obj.ID = strfmt.UUID(fact.ID)
		}

		objects = append(objects, obj)

		// Batch storage when we reach the batch size
		if len(objects) >= config.BatchSize {
			if err := s.storage.StoreBatch(ctx, objects); err != nil {
				return fmt.Errorf("batch storage failed: %w", err)
			}

			processed += len(objects)
			objects = objects[:0] // Reset slice

			// Report progress
			if progressCallback != nil {
				progressCallback(processed, len(facts))
			}

			s.logger.Infof("Stored batch of %d memory objects", config.BatchSize)
		}
	}

	// Store remaining objects
	if len(objects) > 0 {
		if err := s.storage.StoreBatch(ctx, objects); err != nil {
			return fmt.Errorf("final batch storage failed: %w", err)
		}
		processed += len(objects)

		// Final progress callback
		if progressCallback != nil {
			progressCallback(processed, len(facts))
		}

		s.logger.Infof("Stored final batch of %d memory objects", len(objects))
	}

	s.logger.Infof("Successfully stored %d memory objects", processed)
	return nil
}

// IntelligentQuery executes a 3-stage intelligent query that prioritizes consolidated insights
// with supporting evidence and additional context.
func (s *StorageImpl) IntelligentQuery(ctx context.Context, queryText string, filter *memory.Filter) (*memory.IntelligentQueryResult, error) {
	s.logger.Info("Starting intelligent query", "query", queryText)

	// Stage 1: Find relevant consolidated insights
	consolidatedFilter := &memory.Filter{
		Distance: 0.7,
		Limit:    func() *int { limit := 10; return &limit }(),
		Tags:     &memory.TagsFilter{Any: []string{"consolidated"}},
	}

	// Merge user-provided filter options
	if filter != nil {
		if filter.Distance > 0 {
			consolidatedFilter.Distance = filter.Distance
		}
		if filter.Source != nil {
			consolidatedFilter.Source = filter.Source
		}
		if filter.Subject != nil {
			consolidatedFilter.Subject = filter.Subject
		}
	}

	consolidatedResults, err := s.storage.Query(ctx, queryText, consolidatedFilter)
	if err != nil {
		return nil, fmt.Errorf("stage 1 - consolidated query failed: %w", err)
	}

	s.logger.Info("Stage 1 completed", "consolidated_insights", len(consolidatedResults.Facts))

	// Stage 2: Retrieve cited source facts from consolidation metadata
	var citedFactIDs []string
	var citedFacts []memory.MemoryFact

	for _, fact := range consolidatedResults.Facts {
		// Extract source fact IDs from consolidation metadata
		for key, value := range fact.Metadata {
			if strings.HasPrefix(key, "source_fact_") {
				citedFactIDs = append(citedFactIDs, value)
			}
		}
	}

	if len(citedFactIDs) > 0 {
		citedFactsResult, err := s.GetFactsByIDs(ctx, citedFactIDs)
		if err != nil {
			s.logger.Warn("Failed to retrieve cited facts", "error", err, "fact_ids", citedFactIDs)
		} else {
			// Convert pointers to values
			for _, factPtr := range citedFactsResult {
				if factPtr != nil {
					citedFacts = append(citedFacts, *factPtr)
				}
			}
		}
	}

	s.logger.Info("Stage 2 completed", "cited_evidence", len(citedFacts))

	// Stage 3: Additional context - query raw facts and exclude already included ones
	rawFilter := &memory.Filter{
		Distance: 0.75, // Slightly higher threshold for context
		Limit:    func() *int { limit := 15; return &limit }(),
	}

	// Copy user filter settings
	if filter != nil {
		if filter.Source != nil {
			rawFilter.Source = filter.Source
		}
		if filter.Subject != nil {
			rawFilter.Subject = filter.Subject
		}
	}

	rawResults, err := s.storage.Query(ctx, queryText, rawFilter)
	if err != nil {
		s.logger.Warn("Stage 3 - raw query failed", "error", err)
		rawResults = memory.QueryResult{Facts: []memory.MemoryFact{}}
	}

	// Deduplicate against existing results
	existingIDs := make(map[string]bool)
	for _, fact := range consolidatedResults.Facts {
		existingIDs[fact.ID] = true
	}
	for _, fact := range citedFacts {
		existingIDs[fact.ID] = true
	}

	var additionalContext []memory.MemoryFact
	for _, fact := range rawResults.Facts {
		// Skip if already included or if it's a consolidated fact
		if existingIDs[fact.ID] {
			continue
		}

		// Skip consolidated facts (double-check via tags)
		isConsolidated := false
		for _, tag := range fact.Tags {
			if tag == "consolidated" {
				isConsolidated = true
				break
			}
		}

		if !isConsolidated {
			additionalContext = append(additionalContext, fact)
		}
	}

	s.logger.Info("Stage 3 completed", "additional_context", len(additionalContext))

	// Build result using memory package types
	result := &memory.IntelligentQueryResult{
		Query:                queryText,
		ConsolidatedInsights: consolidatedResults.Facts,
		CitedEvidence:        citedFacts,
		AdditionalContext:    additionalContext,
		Metadata: memory.QueryMetadata{
			QueriedAt:                time.Now().Format(time.RFC3339),
			ConsolidatedInsightCount: len(consolidatedResults.Facts),
			CitedEvidenceCount:       len(citedFacts),
			AdditionalContextCount:   len(additionalContext),
			TotalResults:             len(consolidatedResults.Facts) + len(citedFacts) + len(additionalContext),
			QueryStrategy:            "3-stage-intelligent",
		},
	}

	s.logger.Info("Intelligent query completed",
		"query", queryText,
		"insights", len(result.ConsolidatedInsights),
		"evidence", len(result.CitedEvidence),
		"context", len(result.AdditionalContext),
		"total", result.Metadata.TotalResults)

	return result, nil
}

// RunConsolidation performs manual consolidation across all canonical subjects.
// This should be called periodically (e.g., after bulk imports complete) rather than
// after every single document storage operation.
func (s *StorageImpl) RunConsolidation(ctx context.Context) error {
	s.logger.Info("Starting manual consolidation pipeline")

	// Get required dependencies for consolidation
	deps := ConsolidationDependencies{
		Logger:             s.logger,
		Storage:            s, // Use the MemoryStorage interface (self)
		CompletionsService: s.engine.CompletionsService,
		CompletionsModel:   s.engine.CompletionsModel,
	}

	// Run consolidation for all canonical semantic subjects
	consolidationSubjects := ConsolidationSubjects[:]

	var allReports []*ConsolidationReport

	for _, subject := range consolidationSubjects {
		// Use semantic search with filtering for better results (same as test harness)
		filter := &memory.Filter{
			Distance:          0.75,                                                  // Allow fairly broad semantic matches
			Limit:             func() *int { limit := 30; return &limit }(),          // Reasonable limit
			FactImportanceMin: func() *int { importance := 2; return &importance }(), // Only meaningful facts
		}

		report, err := ConsolidateMemoriesBySemantic(ctx, subject, filter, deps)
		if err != nil {
			s.logger.Warn("Consolidation failed for subject", "subject", subject, "error", err)
			continue // Don't fail entire process for one subject
		}

		// Only add reports that actually found facts to consolidate
		if report.SourceFactCount > 0 {
			s.logger.Info("Subject consolidation completed",
				"subject", subject,
				"source_facts", report.SourceFactCount,
				"consolidated_facts", len(report.ConsolidatedFacts))
			allReports = append(allReports, report)
		}
	}

	// Store consolidations if any were created
	if len(allReports) > 0 {
		err := StoreConsolidationReports(ctx, allReports, s, func(processed, total int) {
			s.logger.Debug("Storing consolidated facts", "progress", fmt.Sprintf("%d/%d", processed, total))
		})
		if err != nil {
			s.logger.Error("Failed to store consolidation reports", "error", err)
			return fmt.Errorf("failed to store consolidation reports: %w", err)
		}

		// Calculate totals for logging
		totalSourceFacts := 0
		totalConsolidatedFacts := 0
		for _, report := range allReports {
			totalSourceFacts += report.SourceFactCount
			totalConsolidatedFacts += len(report.ConsolidatedFacts)
		}

		s.logger.Info("Manual consolidation completed",
			"subjects_processed", len(allReports),
			"total_source_facts", totalSourceFacts,
			"total_consolidated_facts", totalConsolidatedFacts)
	} else {
		s.logger.Info("No consolidations created (no qualifying facts found)")
	}

	return nil
}
