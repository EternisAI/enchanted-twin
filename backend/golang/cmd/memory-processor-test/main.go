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
	"github.com/joho/godotenv"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type PipelineStep string

const (
	StepDocumentToChunks PipelineStep = "document_to_chunks" // X_0 → X_1: ConversationDocument to Chunks
	StepChunksToFacts    PipelineStep = "chunks_to_facts"    // X_1 → X_2: Chunks to Facts
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

// DocumentProcessor represents the clean interface that all processors implement.
type DocumentProcessor interface {
	ProcessFile(ctx context.Context, filepath string) ([]memory.ConversationDocument, error)
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
		TimeFormat:      time.Kitchen,
	})

	// Check if this is a WhatsApp conversion call
	if len(os.Args) > 1 && os.Args[1] == "whatsapp" {
		runWhatsAppConversion(logger)
		return
	}

	// Check if this is a Telegram conversion call
	if len(os.Args) > 1 && os.Args[1] == "telegram" {
		runTelegramConversion(logger)
		return
	}

	// Load minimal configuration for testing - we only need API keys
	_ = godotenv.Load("../../.env") // Load .env file from root directory, ignore errors

	// Debug what was actually loaded
	if os.Getenv("COMPLETIONS_API_KEY") != "" {
		logger.Info("Environment variables loaded",
			"COMPLETIONS_API_KEY", "configured",
			"COMPLETIONS_API_URL", os.Getenv("COMPLETIONS_API_URL"),
		)
	}

	envConfig := &config.Config{
		CompletionsAPIKey: os.Getenv("COMPLETIONS_API_KEY"),
		CompletionsAPIURL: getEnvOrDefault("COMPLETIONS_API_URL", "https://openrouter.ai/api/v1"),
		CompletionsModel:  getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
		EmbeddingsAPIKey:  os.Getenv("EMBEDDINGS_API_KEY"),
		EmbeddingsAPIURL:  getEnvOrDefault("EMBEDDINGS_API_URL", "https://api.openai.com/v1"),
		EmbeddingsModel:   getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small"),
		WeaviatePort:      getEnvOrDefault("WEAVIATE_PORT", "51414"),
		// Skip all the other config we don't need for testing
	}

	// Simple argument parsing - just input file and optional flags
	pipelineConfig := &PipelineConfig{
		OutputDir: "pipeline_output",
		Steps:     "basic",
		Config:    envConfig,
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
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
			i++ // Skip the next argument since we consumed it
		case "--output", "-o":
			if i+1 >= len(args) {
				logger.Error("--output requires a value")
				os.Exit(1)
			}
			pipelineConfig.OutputDir = args[i+1]
			i++ // Skip the next argument since we consumed it
		case "--steps", "-s":
			if i+1 >= len(args) {
				logger.Error("--steps requires a value")
				os.Exit(1)
			}
			pipelineConfig.Steps = args[i+1]
			i++ // Skip the next argument since we consumed it
		case "--enable-memory":
			// Memory operations removed for now
		}
	}

	// If no input file specified via flag, use first non-flag argument
	if pipelineConfig.InputFile == "" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		pipelineConfig.InputFile = args[0]
	}

	// Only auto-detect input file for steps that need external input
	needsInputFile := pipelineConfig.Steps == "documents_only" || pipelineConfig.Steps == ""

	// If still no input file and we need one, auto-detect from pipeline_input/ directory
	if pipelineConfig.InputFile == "" && needsInputFile {
		inputFiles, err := filepath.Glob("pipeline_input/*.json")
		if err != nil {
			logger.Error("Failed to scan pipeline_input directory", "error", err)
		} else if len(inputFiles) > 0 {
			pipelineConfig.InputFile = inputFiles[0]
			logger.Info("Auto-detected input file", "file", pipelineConfig.InputFile)
		}
	}

	// Only require input file for steps that need external input
	if pipelineConfig.InputFile == "" && needsInputFile {
		logger.Error("Input file is required. Place your data file in pipeline_input/ or use --input flag")
		printUsage()
		os.Exit(1)
	}

	// For atomic steps, set a dummy input file to avoid nil pointer issues
	if pipelineConfig.InputFile == "" {
		pipelineConfig.InputFile = "unused_for_atomic_step"
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

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printUsage() {
	fmt.Println("Memory Pipeline Tester")
	fmt.Println()
	fmt.Println("This tool tests the exact memory ingestion pipeline used by the main app.")
	fmt.Println("Configuration is loaded from .env file (same as main app).")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  memory-processor-test [options] [input_file]")
	fmt.Println()
	fmt.Println("Quick Start with Makefile:")
	fmt.Println("  1. Put your telegram export file in pipeline_input/")
	fmt.Println("  2. Run: make documents")
	fmt.Println("  3. Run: make chunks")
	fmt.Println("  4. Run: make facts")
	fmt.Println("  Or just: make all")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --input, -i     Input telegram export file (required)")
	fmt.Println("  --output, -o    Output directory (default: pipeline_output)")
	fmt.Println("  --steps, -s     Pipeline steps to run:")
	fmt.Println("                  - 'basic' (default): data_to_document (X_0 → X_1)")
	fmt.Println("                  - 'chunking': basic + document_to_chunks (X_0 → X_1 → X_1')")
	fmt.Println("                  - 'extraction': chunking + chunks_to_facts (X_0 → X_1 → X_1' → X_2)")
	fmt.Println("                    (requires COMPLETIONS_API_KEY)")
	fmt.Println("                  - 'facts_only': chunks_to_facts only (X_1' → X_2)")
	fmt.Println("                    (requires existing X_1'_chunked_documents.json)")
	fmt.Println("                  - 'all': all implemented steps (same as extraction for now)")
	fmt.Println("  --enable-memory Enable memory storage and analysis")
	fmt.Println("                  Requires: COMPLETIONS_API_KEY, EMBEDDINGS_API_KEY in .env")
	fmt.Println()
	fmt.Println("Pipeline Steps:")
	fmt.Println("  X_0 → X_1:     data_to_document   (Raw Data → memory.Documents)")
	fmt.Println("  X_1 → X_1':    document_to_chunks (Documents → Chunked Documents)")
	fmt.Println("  X_1' → X_2:    chunks_to_facts    (Document Chunks → memory.MemoryFacts)")
	fmt.Println("  X_2 → X_3:     store_memory       (Facts → Vector Database) [TODO]")
	fmt.Println("  X_3 → X_4:     query_memory       (Test queries) [TODO]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  memory-processor-test telegram_export.json")
	fmt.Println("  memory-processor-test --input telegram_export.json --steps chunking")
	fmt.Println("  memory-processor-test --steps extraction --enable-memory telegram_export.json")
	fmt.Println("  memory-processor-test --input telegram_export.json --steps all --enable-memory")
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
	case "facts_only":
		return []PipelineStep{
			StepChunksToFacts,
		}
	case "chunks_only":
		return []PipelineStep{
			StepDocumentToChunks,
		}
	default:
		return []PipelineStep{StepDocumentToChunks}
	}
}

func (p *MemoryPipeline) Run(ctx context.Context) error {
	// Ensure output directory exists
	if err := os.MkdirAll(p.config.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	stepsToRun := p.parseSteps(p.config.Steps)

	for _, step := range stepsToRun {
		p.logger.Info("Running pipeline step", "step", step)

		switch step {
		case StepDocumentToChunks:
			if err := p.stepDocumentToChunks(); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		case StepChunksToFacts:
			if err := p.stepChunksToFacts(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
		default:
			return fmt.Errorf("unknown step: %s", step)
		}

		p.logger.Info("Completed pipeline step", "step", step)
	}

	return nil
}

// X_0 -> X_1: Chunk ConversationDocuments.
func (p *MemoryPipeline) stepDocumentToChunks() error {
	// Auto-detect X_0 ConversationDocument file
	var inputFile string
	whatsappFile := filepath.Join(p.config.OutputDir, "X_0_whatsapp.json")
	telegramFile := filepath.Join(p.config.OutputDir, "X_0_telegram.json")

	if _, err := os.Stat(whatsappFile); err == nil {
		inputFile = whatsappFile
	} else if _, err := os.Stat(telegramFile); err == nil {
		inputFile = telegramFile
	} else {
		return fmt.Errorf("no X_0 ConversationDocument file found (X_0_whatsapp.json or X_0_telegram.json)")
	}

	// Load ConversationDocuments from JSON
	documents, err := memory.LoadConversationDocumentsFromJSON(inputFile)
	if err != nil {
		return fmt.Errorf("failed to load ConversationDocuments: %w", err)
	}

	p.logger.Info("Loaded ConversationDocuments", "count", len(documents))

	// PRODUCTION CODE PATH: Chunk documents (matches orchestrator.go:107-111)
	var chunkedDocs []memory.Document

	// Process conversation documents
	for _, doc := range documents {
		docCopy := doc
		chunks := docCopy.Chunk() // <-- EXACT SAME CODE PATH as production
		chunkedDocs = append(chunkedDocs, chunks...)
	}

	p.logger.Info("Chunked documents", "original_count", len(documents), "chunked_count", len(chunkedDocs))

	// Write X_1: chunked documents as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_1_chunked_documents.json")
	chunkedData, err := json.MarshalIndent(map[string]interface{}{
		"chunked_documents": chunkedDocs,
		"metadata": map[string]interface{}{
			"processed_at":   time.Now().Format(time.RFC3339),
			"step":           "document_to_chunks",
			"original_count": len(documents),
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

// X_1 -> X_2: Extract facts from document chunks.
func (p *MemoryPipeline) stepChunksToFacts(ctx context.Context) error {
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

	// Read X_1
	inputFile := filepath.Join(p.config.OutputDir, "X_1_chunked_documents.json")
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

	// Write X_2: extracted facts as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_2_extracted_facts.json")
	factsData, err := json.MarshalIndent(map[string]interface{}{
		"facts": allFacts,
		"metadata": map[string]interface{}{
			"processed_at":      time.Now().Format(time.RFC3339),
			"step":              "chunks_to_facts",
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

// convertToDocuments is the polymorphic function that works with any processor.
func convertToDocuments(processor DocumentProcessor, inputFile, outputFile string, logger *log.Logger, verbose bool) error {
	// Validate input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	logger.Info("Starting data conversion",
		"input", inputFile,
		"output", outputFile)

	ctx := context.Background()

	// Use the ProcessFile method that returns ConversationDocuments directly
	documents, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}

	logger.Info("Successfully processed data", "conversations", len(documents))

	// Print out ConversationDocument structures for inspection if verbose
	if verbose {
		logger.Info("Inspecting ConversationDocument structures:")

		for i, doc := range documents {
			if i >= 5 { // Show first 5 documents
				if len(documents) > 5 {
					logger.Info("... and more documents (showing first 5 only)")
				}
				break
			}

			logger.Info("ConversationDocument",
				"index", i+1,
				"ID", doc.ID(),
				"Source", doc.Source(),
				"Tags", doc.Tags(),
				"User", doc.User,
				"People", doc.People,
				"MessageCount", len(doc.Conversation),
				"Metadata", doc.Metadata())
		}
	}

	// Save documents as JSON
	logger.Info("Writing conversations to JSON file...")
	if err := memory.ExportConversationDocumentsJSON(documents, outputFile); err != nil {
		return fmt.Errorf("failed to save documents: %w", err)
	}

	logger.Info("Conversion completed successfully!",
		"output", outputFile,
		"conversations", len(documents))

	// Print summary statistics
	totalMessages := 0
	participants := make(map[string]bool)

	for _, doc := range documents {
		totalMessages += len(doc.Conversation)
		for _, person := range doc.People {
			participants[person] = true
		}
	}

	logger.Info("Summary statistics:",
		"total_conversations", len(documents),
		"total_messages", totalMessages,
		"unique_participants", len(participants))

	if totalMessages > 0 && len(documents) > 0 {
		avgMessagesPerConv := float64(totalMessages) / float64(len(documents))
		logger.Info("Average messages per conversation:", "average", fmt.Sprintf("%.2f", avgMessagesPerConv))
	}

	return nil
}

func runWhatsAppConversion(logger *log.Logger) {
	if len(os.Args) < 3 {
		fmt.Println("WhatsApp Conversion Tool")
		fmt.Println()
		fmt.Println("Converts WhatsApp SQLite database to JSONL format for memory pipeline")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./memory-processor-test whatsapp [input_sqlite_file]")
		fmt.Println("  make whatsapp  # Auto-detects input file from pipeline_input/")
		fmt.Println()
		fmt.Println("The output JSONL file will be saved as pipeline_output/X_0_whatsapp.jsonl")
		fmt.Println("This becomes the X_0 input for the memory pipeline.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./memory-processor-test whatsapp ChatStorage.sqlite")
		fmt.Println("  ./memory-processor-test whatsapp ~/Downloads/WhatsApp.sqlite")
		os.Exit(1)
	}

	inputFile := os.Args[2]

	// Auto-detect from pipeline_input if file not found
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		// Try to find SQLite files in pipeline_input
		sqliteFiles, _ := filepath.Glob("pipeline_input/*.sqlite")
		if len(sqliteFiles) == 0 {
			sqliteFiles, _ = filepath.Glob("pipeline_input/*.db")
		}

		if len(sqliteFiles) > 0 {
			inputFile = sqliteFiles[0]
			logger.Info("Auto-detected WhatsApp database", "file", inputFile)
		} else {
			logger.Error("WhatsApp database file not found", "attempted", os.Args[2])
			fmt.Println("\nTip: Place your WhatsApp SQLite file in pipeline_input/ directory")
			os.Exit(1)
		}
	}

	// Output to pipeline_output with WhatsApp-specific naming
	outputFile := "pipeline_output/X_0_whatsapp.json"

	// Convert WhatsApp to Documents
	logger.Info("🔄 Converting WhatsApp SQLite to ConversationDocuments...")

	// Create in-memory store for WhatsApp processor
	ctx := context.Background()
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		logger.Error("Failed to create store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := whatsapp.NewWhatsappProcessor(store, logger)
	if err != nil {
		logger.Error("Failed to create WhatsApp processor", "error", err)
		os.Exit(1)
	}

	if err := convertToDocuments(processor, inputFile, outputFile, logger, false); err != nil {
		logger.Error("WhatsApp conversion failed", "error", err)
		os.Exit(1)
	}

	logger.Info("✅ WhatsApp conversion completed!",
		"output", outputFile,
		"next_step", "Use 'make chunks' to process these documents")

	fmt.Println()
	fmt.Println("🎉 WhatsApp data converted successfully!")
	fmt.Printf("📁 Output: %s\n", outputFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the JSON file if needed")
	fmt.Println("  2. Run: make chunks")
	fmt.Println("  3. Run: make facts")
	fmt.Println("  Or just: make all")
}

func runTelegramConversion(logger *log.Logger) {
	if len(os.Args) < 3 {
		fmt.Println("Telegram Conversion Tool")
		fmt.Println()
		fmt.Println("Converts Telegram export JSON to JSONL format for memory pipeline")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./memory-processor-test telegram [input_json_file]")
		fmt.Println("  make telegram  # Auto-detects input file from pipeline_input/")
		fmt.Println()
		fmt.Println("The output JSONL file will be saved as pipeline_output/X_0_telegram.jsonl")
		fmt.Println("This becomes the X_0 input for the memory pipeline.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./memory-processor-test telegram result.json")
		fmt.Println("  ./memory-processor-test telegram ~/Downloads/telegram_export.json")
		os.Exit(1)
	}

	inputFile := os.Args[2]

	// Auto-detect from pipeline_input if file not found
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		// Try to find JSON files in pipeline_input
		jsonFiles, _ := filepath.Glob("pipeline_input/*.json")

		if len(jsonFiles) > 0 {
			inputFile = jsonFiles[0]
			logger.Info("Auto-detected Telegram export", "file", inputFile)
		} else {
			logger.Error("Telegram export file not found", "attempted", os.Args[2])
			fmt.Println("\nTip: Place your Telegram export JSON file in pipeline_input/ directory")
			os.Exit(1)
		}
	}

	// Output to pipeline_output with Telegram-specific naming
	outputFile := "pipeline_output/X_0_telegram.json"

	// Convert Telegram to Documents
	logger.Info("🔄 Converting Telegram JSON to ConversationDocuments...")

	// Create in-memory store for Telegram processor
	ctx := context.Background()
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		logger.Error("Failed to create store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := telegram.NewTelegramProcessor(store, logger)
	if err != nil {
		logger.Error("Failed to create Telegram processor", "error", err)
		os.Exit(1)
	}

	if err := convertToDocuments(processor, inputFile, outputFile, logger, false); err != nil {
		logger.Error("Telegram conversion failed", "error", err)
		os.Exit(1)
	}

	logger.Info("✅ Telegram conversion completed!",
		"output", outputFile,
		"next_step", "Use 'make chunks' to process these documents")

	fmt.Println()
	fmt.Println("🎉 Telegram data converted successfully!")
	fmt.Printf("📁 Output: %s\n", outputFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the JSON file if needed")
	fmt.Println("  2. Run: make chunks")
	fmt.Println("  3. Run: make facts")
	fmt.Println("  Or just: make all")
}
