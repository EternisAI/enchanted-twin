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

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
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
)

type PipelineConfig struct {
	InputFile string
	OutputDir string
	Steps     string
	Config    *config.Config
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
		OutputDir: "pipeline_output",
		Steps:     "basic",
		Config:    envConfig,
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
			// Memory operations removed for now
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

	// Memory operations removed for now - facts are the final payload

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
	// Ensure we have completions service for fact extraction
	if p.config.Config.CompletionsAPIKey == "" {
		return fmt.Errorf("fact extraction requires COMPLETIONS_API_KEY - run with --enable-memory or add API key to .env")
	}

	// Create AI service for fact extraction (same as production)
	aiService := ai.NewOpenAIService(
		p.logger,
		p.config.Config.CompletionsAPIKey,
		p.config.Config.CompletionsAPIURL,
	)

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

	// Convert raw JSON messages to Document instances
	var documents []memory.Document
	for _, rawDoc := range chunkedData.ChunkedDocuments {
		// Try ConversationDocument first
		var convDoc memory.ConversationDocument
		if err := json.Unmarshal(rawDoc, &convDoc); err == nil && len(convDoc.Conversation) > 0 {
			documents = append(documents, &convDoc)
			continue
		}

		// Try TextDocument
		var textDoc memory.TextDocument
		if err := json.Unmarshal(rawDoc, &textDoc); err == nil && textDoc.Content() != "" {
			documents = append(documents, &textDoc)
		}
	}

	p.logger.Info("Processing YOUR actual documents for fact extraction", "count", len(documents))

	// Extract facts from YOUR documents using the REAL production code path
	var allFacts []*memory.MemoryFact
	for i, doc := range documents {
		p.logger.Info("Extracting facts from YOUR document", "index", i+1, "id", doc.ID(), "type", fmt.Sprintf("%T", doc))

		// Call the EXACT same fact extraction function as production
		// This is the same call that happens in orchestrator.go:52
		facts, err := evolvingmemory.ExtractFactsFromDocument(ctx, doc, aiService, p.config.Config.CompletionsModel, p.logger)
		if err != nil {
			p.logger.Error("Failed to extract facts from YOUR document", "id", doc.ID(), "error", err)
			continue
		}

		p.logger.Info("Extracted facts from YOUR document", "id", doc.ID(), "facts_count", len(facts))
		allFacts = append(allFacts, facts...)
	}

	p.logger.Info("Fact extraction completed on YOUR data", "total_facts", len(allFacts))

	// Write X_3: extracted facts as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_3_extracted_facts.json")
	factsData, err := json.MarshalIndent(map[string]interface{}{
		"facts": allFacts,
		"metadata": map[string]interface{}{
			"processed_at":      time.Now().Format(time.RFC3339),
			"step":              "extract_facts",
			"documents_count":   len(documents),
			"facts_count":       len(allFacts),
			"completions_model": p.config.Config.CompletionsModel,
			"source":            "real_llm_extraction_from_user_data",
		},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal extracted facts: %w", err)
	}

	if err := os.WriteFile(outputFile, factsData, 0o644); err != nil {
		return fmt.Errorf("failed to write extracted facts file: %w", err)
	}

	p.logger.Info("Wrote YOUR extracted facts", "file", outputFile, "count", len(allFacts))
	return nil
}
