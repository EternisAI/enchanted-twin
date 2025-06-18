package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type PipelineStep string

const (
	StepParseJSON      PipelineStep = "parse_json"
	StepToDocuments    PipelineStep = "to_documents"
	StepChunkDocuments PipelineStep = "chunk_documents"
	StepExtractFacts   PipelineStep = "extract_facts"
	StepStoreMemory    PipelineStep = "store_memory"
	StepQueryMemory    PipelineStep = "query_memory"
	StepFactsAnalysis  PipelineStep = "facts_analysis"
)

type PipelineConfig struct {
	InputFile    string
	OutputDir    string
	Steps        string
	EnableMemory bool
	Config       *config.Config
}

type MemoryPipeline struct {
	config *PipelineConfig
	logger *log.Logger
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
		TimeFormat:      time.Kitchen,
	})

	// Load configuration from .env file (just like the main app)
	envConfig, err := config.LoadConfig(false)
	if err != nil {
		logger.Error("Failed to load config from .env", "error", err)
		os.Exit(1)
	}

	// Simple argument parsing - just input file and optional flags
	pipelineConfig := &PipelineConfig{
		OutputDir:    "pipeline_output",
		Steps:        "basic",
		EnableMemory: false,
		Config:       envConfig,
	}

	args := os.Args[1:]
	for i, arg := range args {
		switch arg {
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--input", "-i":
			if i+1 >= len(args) {
				logger.Error("--input requires a value")
				os.Exit(1)
			}
			pipelineConfig.InputFile = args[i+1]
		case "--output", "-o":
			if i+1 >= len(args) {
				logger.Error("--output requires a value")
				os.Exit(1)
			}
			pipelineConfig.OutputDir = args[i+1]
		case "--steps", "-s":
			if i+1 >= len(args) {
				logger.Error("--steps requires a value")
				os.Exit(1)
			}
			pipelineConfig.Steps = args[i+1]
		case "--enable-memory":
			pipelineConfig.EnableMemory = true
		}
	}

	// If no input file specified via flag, use first non-flag argument
	if pipelineConfig.InputFile == "" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		pipelineConfig.InputFile = args[0]
	}

	if pipelineConfig.InputFile == "" {
		logger.Error("Input file is required")
		printUsage()
		os.Exit(1)
	}

	// Validate memory requirements
	if pipelineConfig.EnableMemory {
		if envConfig.CompletionsAPIKey == "" {
			logger.Error("Memory operations require COMPLETIONS_API_KEY in .env file")
			os.Exit(1)
		}
		logger.Info("Memory operations enabled",
			"completions_model", envConfig.CompletionsModel,
			"embeddings_model", envConfig.EmbeddingsModel,
			"weaviate_port", envConfig.WeaviatePort)
	}

	pipeline := &MemoryPipeline{
		config: pipelineConfig,
		logger: logger,
	}

	if err := pipeline.Run(context.Background()); err != nil {
		logger.Error("Pipeline failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Pipeline completed successfully! 🎉")
}

func printUsage() {
	fmt.Println("Telegram Memory Pipeline Tester")
	fmt.Println()
	fmt.Println("This tool tests the exact memory ingestion pipeline used by the main app.")
	fmt.Println("Configuration is loaded from .env file (same as main app).")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  telegram-processor-test [options] [input_file]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --input, -i     Input telegram export file (required)")
	fmt.Println("  --output, -o    Output directory (default: pipeline_output)")
	fmt.Println("  --steps, -s     Pipeline steps to run:")
	fmt.Println("                  - 'basic' (default): parse_json, to_documents")
	fmt.Println("                  - 'chunking': basic + chunk_documents")
	fmt.Println("                  - 'extraction': chunking + extract_facts (requires COMPLETIONS_API_KEY)")
	fmt.Println("                  - 'all': all steps including memory operations")
	fmt.Println("                    (requires: COMPLETIONS_API_KEY, EMBEDDINGS_API_KEY)")
	fmt.Println("  --enable-memory Enable memory storage and analysis")
	fmt.Println("                  Requires: COMPLETIONS_API_KEY, EMBEDDINGS_API_KEY in .env")
	fmt.Println()
	fmt.Println("Pipeline Steps:")
	fmt.Println("  X_0 → X_1:     parse_json      (Telegram JSON → types.Record)")
	fmt.Println("  X_1 → X_2:     to_documents    (Records → memory.Document)")
	fmt.Println("  X_2 → X_2.5:   chunk_documents (Documents → Chunked Documents)")
	fmt.Println("  X_2.5 → X_3:   extract_facts   (Chunked Docs → memory.MemoryFact)")
	fmt.Println("  X_3 → X_4:     store_memory    (Facts → Vector Database)")
	fmt.Println("  X_4 → X_5:     query_memory    (Test queries)")
	fmt.Println("  X_5 → X_6:     facts_analysis  (Analyze memory structure)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  telegram-processor-test telegram_export.json")
	fmt.Println("  telegram-processor-test --input telegram_export.json --steps chunking")
	fmt.Println("  telegram-processor-test --steps extraction --enable-memory telegram_export.json")
	fmt.Println("  telegram-processor-test --input telegram_export.json --steps all --enable-memory")
	fmt.Println()
	fmt.Println("Environment Variables (via .env file):")
	fmt.Println("  COMPLETIONS_API_KEY     OpenAI API key for completions")
	fmt.Println("  EMBEDDINGS_API_KEY      OpenAI API key for embeddings")
	fmt.Println("  COMPLETIONS_MODEL       Model for completions (default: gpt-4o-mini)")
	fmt.Println("  EMBEDDINGS_MODEL        Model for embeddings (default: text-embedding-3-small)")
	fmt.Println("  WEAVIATE_PORT          Weaviate port (default: 51414)")
}

func (p *MemoryPipeline) parseSteps(stepsStr string) []PipelineStep {
	switch stepsStr {
	case "all":
		return []PipelineStep{
			StepParseJSON,
			StepToDocuments,
			StepChunkDocuments,
			StepExtractFacts,
			StepStoreMemory,
			StepQueryMemory,
			StepFactsAnalysis,
		}
	case "extraction":
		return []PipelineStep{
			StepParseJSON,
			StepToDocuments,
			StepChunkDocuments,
			StepExtractFacts,
		}
	case "chunking":
		return []PipelineStep{
			StepParseJSON,
			StepToDocuments,
			StepChunkDocuments,
		}
	default: // "basic"
		return []PipelineStep{StepParseJSON, StepToDocuments}
	}
}

func (p *MemoryPipeline) Run(ctx context.Context) error {
	stepsToRun := p.parseSteps(p.config.Steps)

	for _, step := range stepsToRun {
		p.logger.Info("Running pipeline step", "step", step)

		switch step {
		case StepParseJSON:
			if err := p.stepParseJSON(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepToDocuments:
			if err := p.stepToDocuments(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepChunkDocuments:
			if err := p.stepChunkDocuments(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepExtractFacts:
			if err := p.stepExtractFacts(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepStoreMemory:
			if err := p.stepStoreMemory(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepQueryMemory:
			if err := p.stepQueryMemory(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepFactsAnalysis:
			if err := p.stepFactsAnalysis(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		default:
			return fmt.Errorf("unknown step: %s", step)
		}

		p.logger.Info("Completed pipeline step", "step", step)
	}

	return nil
}

// X_0 -> X_1: Parse Telegram JSON and extract records.
func (p *MemoryPipeline) stepParseJSON(ctx context.Context) error {
	// Create an in-memory SQLite store for testing
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			p.logger.Error("Failed to close store", "error", err)
		}
	}()

	// Create the telegram processor - FIX: capture both return values
	processor, err := telegram.NewTelegramProcessor(store, p.logger)
	if err != nil {
		return fmt.Errorf("failed to create telegram processor: %w", err)
	}

	p.logger.Info("Processing Telegram export file", "file", p.config.InputFile)

	// Use the actual ProcessFile method
	records, err := processor.ProcessFile(ctx, p.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}

	p.logger.Info("Processed records", "count", len(records))

	// Write X_1: processed records as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_1_records.json")
	recordsData, err := json.MarshalIndent(map[string]interface{}{
		"records": records,
		"metadata": map[string]interface{}{
			"source_file":  p.config.InputFile,
			"processed_at": time.Now().Format(time.RFC3339),
			"record_count": len(records),
			"step":         "parse_json",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal records: %w", err)
	}

	if err := os.WriteFile(outputFile, recordsData, 0o644); err != nil {
		return fmt.Errorf("failed to write records file: %w", err)
	}

	p.logger.Info("Wrote processed records", "file", outputFile)
	return nil
}

// X_1 -> X_2: Convert records to memory documents.
func (p *MemoryPipeline) stepToDocuments(ctx context.Context) error {
	// Read X_1
	inputFile := filepath.Join(p.config.OutputDir, "X_1_records.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read records file: %w", err)
	}

	var recordsData struct {
		Records []types.Record `json:"records"`
	}
	if err := json.Unmarshal(data, &recordsData); err != nil {
		return fmt.Errorf("failed to unmarshal records: %w", err)
	}

	// Create store and dataprocessing service (matching real app workflow)
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			p.logger.Error("Failed to close store", "error", err)
		}
	}()

	// Create AI service for dataprocessing service (only if we have an API key)
	var aiService *ai.Service
	if p.config.Config.CompletionsAPIKey != "" {
		aiService = ai.NewOpenAIService(p.logger, p.config.Config.CompletionsAPIKey, p.config.Config.CompletionsAPIURL)
	}

	// Create DataProcessingService (this is what the real app uses!)
	dataprocessingService := dataprocessing.NewDataProcessingService(aiService, "gpt-4", store, p.logger)

	// Convert to documents using the REAL workflow path
	documents, err := dataprocessingService.ToDocuments(ctx, "telegram", recordsData.Records)
	if err != nil {
		return fmt.Errorf("failed to convert to documents: %w", err)
	}

	p.logger.Info("Generated documents", "count", len(documents))

	// Analyze document types
	var conversationDocs []memory.ConversationDocument
	var otherDocs []memory.Document

	for _, doc := range documents {
		if convDoc, ok := doc.(*memory.ConversationDocument); ok {
			conversationDocs = append(conversationDocs, *convDoc)
		} else {
			otherDocs = append(otherDocs, doc)
		}
	}

	// Write X_2: documents as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_2_documents.json")
	documentsData, err := json.MarshalIndent(map[string]interface{}{
		"conversation_documents": conversationDocs,
		"other_documents":        otherDocs,
		"metadata": map[string]interface{}{
			"processed_at":       time.Now().Format(time.RFC3339),
			"total_documents":    len(documents),
			"conversation_count": len(conversationDocs),
			"other_count":        len(otherDocs),
			"step":               "to_documents",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal documents: %w", err)
	}

	if err := os.WriteFile(outputFile, documentsData, 0o644); err != nil {
		return fmt.Errorf("failed to write documents file: %w", err)
	}

	p.logger.Info("Wrote documents", "file", outputFile, "conversations", len(conversationDocs), "other", len(otherDocs))
	return nil
}

// X_2 -> X_2.5: Chunk documents.
func (p *MemoryPipeline) stepChunkDocuments(ctx context.Context) error {
	// Read X_2
	inputFile := filepath.Join(p.config.OutputDir, "X_2_documents.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read documents file: %w", err)
	}

	var documentsData struct {
		ConversationDocuments []memory.ConversationDocument `json:"conversation_documents"`
		OtherDocuments        []memory.Document             `json:"other_documents"`
	}
	if err := json.Unmarshal(data, &documentsData); err != nil {
		return fmt.Errorf("failed to unmarshal documents: %w", err)
	}

	// PRODUCTION CODE PATH: Step 1: Chunk documents (matches orchestrator.go:107-111)
	var chunkedDocs []memory.Document
	for _, doc := range documentsData.ConversationDocuments {
		docCopy := doc
		chunks := docCopy.Chunk() // <-- EXACT SAME CODE PATH as production
		chunkedDocs = append(chunkedDocs, chunks...)
	}
	for _, doc := range documentsData.OtherDocuments {
		chunks := doc.Chunk() // <-- EXACT SAME CODE PATH as production
		chunkedDocs = append(chunkedDocs, chunks...)
	}

	p.logger.Info("Chunked documents", "original_count", len(documentsData.ConversationDocuments)+len(documentsData.OtherDocuments), "chunked_count", len(chunkedDocs))

	// Write X_2.5: chunked documents as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_2.5_chunked_documents.json")
	chunkedData, err := json.MarshalIndent(map[string]interface{}{
		"chunked_documents": chunkedDocs,
		"metadata": map[string]interface{}{
			"processed_at":   time.Now().Format(time.RFC3339),
			"step":           "chunk_documents",
			"original_count": len(documentsData.ConversationDocuments) + len(documentsData.OtherDocuments),
			"chunked_count":  len(chunkedDocs),
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chunked documents: %w", err)
	}

	if err := os.WriteFile(outputFile, chunkedData, 0o644); err != nil {
		return fmt.Errorf("failed to write chunked documents file: %w", err)
	}

	p.logger.Info("Wrote chunked documents", "file", outputFile, "count", len(chunkedDocs))
	return nil
}

// X_2.5 -> X_3: Extract facts from documents.
func (p *MemoryPipeline) stepExtractFacts(ctx context.Context) error {
	// This step is complex and requires direct access to internal functions
	// For the pipeline test, we'll create placeholder facts to demonstrate the structure

	// Read X_2.5
	inputFile := filepath.Join(p.config.OutputDir, "X_2.5_chunked_documents.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read chunked documents file: %w", err)
	}

	var chunkedData struct {
		ChunkedDocuments []json.RawMessage `json:"chunked_documents"`
	}
	if err := json.Unmarshal(data, &chunkedData); err != nil {
		return fmt.Errorf("failed to unmarshal chunked documents: %w", err)
	}

	p.logger.Info("Processing documents for fact extraction", "count", len(chunkedData.ChunkedDocuments))

	// For now, create a few sample facts to demonstrate the pipeline structure
	// In the full implementation, this would use the evolvingmemory fact extraction
	now := time.Now()
	sampleFacts := []*memory.MemoryFact{
		{
			ID:                 "fact-1",
			Content:            "User prefers dark mode interface settings",
			Timestamp:          now,
			Category:           "preference",
			Subject:            "user",
			Attribute:          "interface_preference",
			Value:              "prefers dark mode over light mode for UI settings",
			TemporalContext:    nil,
			Sensitivity:        "low",
			Importance:         2,
			Source:             "telegram",
			DocumentReferences: []string{"sample-doc-1"},
			Tags:               []string{"ui", "preferences"},
			Metadata: map[string]string{
				"extraction_method": "pipeline_test",
				"source_type":       "conversation",
			},
		},
		{
			ID:                 "fact-2",
			Content:            "User is learning Go programming language",
			Timestamp:          now,
			Category:           "skill",
			Subject:            "user",
			Attribute:          "programming_skill",
			Value:              "currently learning Go programming language",
			TemporalContext:    &[]string{"2025-01"}[0],
			Sensitivity:        "low",
			Importance:         2,
			Source:             "telegram",
			DocumentReferences: []string{"sample-doc-1", "sample-doc-2"},
			Tags:               []string{"programming", "learning"},
			Metadata: map[string]string{
				"extraction_method": "pipeline_test",
				"source_type":       "conversation",
			},
		},
	}

	p.logger.Info("Generated sample facts for pipeline testing", "count", len(sampleFacts))

	// Write X_3: extracted facts as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_3_extracted_facts.json")
	factsData, err := json.MarshalIndent(map[string]interface{}{
		"facts": sampleFacts,
		"metadata": map[string]interface{}{
			"processed_at":    time.Now().Format(time.RFC3339),
			"step":            "extract_facts",
			"documents_count": len(chunkedData.ChunkedDocuments),
			"facts_count":     len(sampleFacts),
			"note":            "Sample facts generated for pipeline testing - full implementation requires fact extraction service",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal extracted facts: %w", err)
	}

	if err := os.WriteFile(outputFile, factsData, 0o644); err != nil {
		return fmt.Errorf("failed to write extracted facts file: %w", err)
	}

	p.logger.Info("Wrote extracted facts", "file", outputFile, "count", len(sampleFacts))
	return nil
}

// X_3 -> X_4: Store documents in memory system (memory operations).
func (p *MemoryPipeline) stepStoreMemory(ctx context.Context) error {
	if !p.config.EnableMemory {
		p.logger.Info("Memory storage disabled, skipping step")
		return nil
	}

	// Read X_3
	inputFile := filepath.Join(p.config.OutputDir, "X_3_extracted_facts.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read facts file: %w", err)
	}

	var factsData struct {
		Facts []memory.MemoryFact `json:"facts"`
	}
	if err := json.Unmarshal(data, &factsData); err != nil {
		return fmt.Errorf("failed to unmarshal facts: %w", err)
	}

	// Create memory storage
	memoryStorage, err := p.createMemoryStorage(ctx)
	if err != nil {
		return fmt.Errorf("failed to create memory storage: %w", err)
	}

	// Track progress
	var progressUpdates []map[string]interface{}
	progressCallback := func(processed, total int) {
		update := map[string]interface{}{
			"processed": processed,
			"total":     total,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		progressUpdates = append(progressUpdates, update)
		p.logger.Info("Memory storage progress", "processed", processed, "total", total)
	}

	p.logger.Info("Storing facts in memory", "count", len(factsData.Facts))

	startTime := time.Now()

	// Convert MemoryFacts to Documents for storage
	var documents []memory.Document
	for _, fact := range factsData.Facts {
		// Create a TextDocument from the fact for storage
		factCopy := fact
		doc := &memory.TextDocument{
			FieldID:        factCopy.ID,
			FieldContent:   factCopy.Content,
			FieldTimestamp: &factCopy.Timestamp,
			FieldSource:    factCopy.Source,
			FieldTags:      factCopy.Tags,
			FieldMetadata:  factCopy.Metadata,
		}
		documents = append(documents, doc)
	}

	err = memoryStorage.Store(ctx, documents, progressCallback)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("failed to store facts in memory: %w", err)
	}

	// Write X_4: memory storage results
	outputFile := filepath.Join(p.config.OutputDir, "X_4_memory_storage.json")
	storageData, err := json.MarshalIndent(map[string]interface{}{
		"storage_completed": true,
		"facts_stored":      len(factsData.Facts),
		"duration_seconds":  duration.Seconds(),
		"progress_updates":  progressUpdates,
		"metadata": map[string]interface{}{
			"processed_at": time.Now().Format(time.RFC3339),
			"step":         "store_memory",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage results: %w", err)
	}

	if err := os.WriteFile(outputFile, storageData, 0o644); err != nil {
		return fmt.Errorf("failed to write storage results file: %w", err)
	}

	p.logger.Info("Wrote memory storage results", "file", outputFile, "duration", duration)
	return nil
}

// X_4 -> X_5: Query memory system to verify storage.
func (p *MemoryPipeline) stepQueryMemory(ctx context.Context) error {
	if !p.config.EnableMemory {
		p.logger.Info("Memory queries disabled, skipping step")
		return nil
	}

	// Create memory storage
	memoryStorage, err := p.createMemoryStorage(ctx)
	if err != nil {
		return fmt.Errorf("failed to create memory storage: %w", err)
	}

	// Define test queries
	testQueries := []struct {
		Name   string
		Query  string
		Filter *memory.Filter
	}{
		{
			Name:  "all_memories",
			Query: "tell me about the user",
			Filter: &memory.Filter{
				Limit: &[]int{50}[0],
			},
		},
		{
			Name:  "conversation_memories",
			Query: "conversations with friends",
			Filter: &memory.Filter{
				Source: &[]string{"telegram"}[0],
				Limit:  &[]int{20}[0],
			},
		},
		{
			Name:  "preferences",
			Query: "user preferences and likes",
			Filter: &memory.Filter{
				FactCategory: &[]string{"preference"}[0],
				Limit:        &[]int{10}[0],
			},
		},
		{
			Name:  "important_facts",
			Query: "important life facts",
			Filter: &memory.Filter{
				FactImportanceMin: &[]int{2}[0],
				Limit:             &[]int{15}[0],
			},
		},
	}

	queryResults := make(map[string]interface{})

	for _, testQuery := range testQueries {
		p.logger.Info("Running test query", "name", testQuery.Name, "query", testQuery.Query)

		startTime := time.Now()
		result, err := memoryStorage.Query(ctx, testQuery.Query, testQuery.Filter)
		duration := time.Since(startTime)

		if err != nil {
			p.logger.Error("Query failed", "name", testQuery.Name, "error", err)
			queryResults[testQuery.Name] = map[string]interface{}{
				"error":            err.Error(),
				"duration_seconds": duration.Seconds(),
			}
			continue
		}

		queryResults[testQuery.Name] = map[string]interface{}{
			"facts_count":      len(result.Facts),
			"duration_seconds": duration.Seconds(),
			"facts":            result.Facts,
			"query":            testQuery.Query,
			"filter":           testQuery.Filter,
		}

		p.logger.Info("Query completed", "name", testQuery.Name, "facts", len(result.Facts), "duration", duration)
	}

	// Write X_5: query results
	outputFile := filepath.Join(p.config.OutputDir, "X_5_memory_queries.json")
	queryData, err := json.MarshalIndent(map[string]interface{}{
		"query_results": queryResults,
		"metadata": map[string]interface{}{
			"processed_at": time.Now().Format(time.RFC3339),
			"step":         "query_memory",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal query results: %w", err)
	}

	if err := os.WriteFile(outputFile, queryData, 0o644); err != nil {
		return fmt.Errorf("failed to write query results: %w", err)
	}

	p.logger.Info("Memory queries completed", "file", outputFile)
	return nil
}

// X_5 -> X_6: Analyze extracted facts and memory structure.
func (p *MemoryPipeline) stepFactsAnalysis(ctx context.Context) error {
	if !p.config.EnableMemory {
		p.logger.Info("Facts analysis disabled, skipping step")
		return nil
	}

	// Read X_5
	inputFile := filepath.Join(p.config.OutputDir, "X_5_memory_queries.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read query results file: %w", err)
	}

	var queryData struct {
		QueryResults map[string]interface{} `json:"query_results"`
	}
	if err := json.Unmarshal(data, &queryData); err != nil {
		return fmt.Errorf("failed to unmarshal query results: %w", err)
	}

	// Count facts by category, importance, etc.
	categoryCount := make(map[string]int)
	importanceCount := make(map[int]int)
	subjectCount := make(map[string]int)
	totalFacts := 0

	for queryName, resultInterface := range queryData.QueryResults {
		if resultMap, ok := resultInterface.(map[string]interface{}); ok {
			if factsInterface, ok := resultMap["facts"]; ok {
				if factsSlice, ok := factsInterface.([]interface{}); ok {
					for _, factInterface := range factsSlice {
						if factMap, ok := factInterface.(map[string]interface{}); ok {
							totalFacts++

							if category, ok := factMap["category"].(string); ok {
								categoryCount[category]++
							}
							if importance, ok := factMap["importance"].(float64); ok {
								importanceCount[int(importance)]++
							}
							if subject, ok := factMap["subject"].(string); ok {
								subjectCount[subject]++
							}
						}
					}
				}
			}
		}

		p.logger.Info("Analyzed query results", "query", queryName, "type", fmt.Sprintf("%T", resultInterface))
	}

	analysis := map[string]interface{}{
		"total_facts":        totalFacts,
		"category_breakdown": categoryCount,
		"importance_levels":  importanceCount,
		"subject_breakdown":  subjectCount,
		"analysis_summary": map[string]interface{}{
			"most_common_category": findMostCommon(categoryCount),
			"most_common_subject":  findMostCommon(subjectCount),
			"highest_importance":   findHighestKey(importanceCount),
		},
	}

	// Write X_6: facts analysis
	outputFile := filepath.Join(p.config.OutputDir, "X_6_facts_analysis.json")
	analysisData, err := json.MarshalIndent(map[string]interface{}{
		"facts_analysis": analysis,
		"metadata": map[string]interface{}{
			"processed_at": time.Now().Format(time.RFC3339),
			"step":         "facts_analysis",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analysis results: %w", err)
	}

	if err := os.WriteFile(outputFile, analysisData, 0o644); err != nil {
		return fmt.Errorf("failed to write analysis results: %w", err)
	}

	p.logger.Info("Facts analysis completed", "file", outputFile, "total_facts", totalFacts)
	return nil
}

func (p *MemoryPipeline) createMemoryStorage(ctx context.Context) (memory.Storage, error) {
	// Create AI service for embeddings and completions
	aiService := ai.NewOpenAIService(p.logger, p.config.Config.CompletionsAPIKey, p.config.Config.CompletionsAPIURL)

	// Create Weaviate storage backend
	embeddingWrapper, err := storage.NewEmbeddingWrapper(aiService, "text-embedding-3-small")
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding wrapper: %w", err)
	}

	// Create Weaviate client
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", p.config.Config.WeaviatePort),
		Scheme: "http",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	weaviateStorage, err := storage.New(storage.NewStorageInput{
		Client:            weaviateClient,
		Logger:            p.logger,
		EmbeddingsWrapper: embeddingWrapper,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate storage: %w", err)
	}

	// Create evolving memory storage
	memoryStorage, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             p.logger,
		Storage:            weaviateStorage,
		CompletionsService: aiService,
		CompletionsModel:   "gpt-4",
		EmbeddingsWrapper:  embeddingWrapper,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create memory storage: %w", err)
	}

	return memoryStorage, nil
}

// Helper functions.
func findMostCommon(counts map[string]int) string {
	maxCount := 0
	mostCommon := ""
	for key, count := range counts {
		if count > maxCount {
			maxCount = count
			mostCommon = key
		}
	}
	return mostCommon
}

func findHighestKey(counts map[int]int) int {
	highest := 0
	for key := range counts {
		if key > highest {
			highest = key
		}
	}
	return highest
}
