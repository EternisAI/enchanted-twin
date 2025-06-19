package main

import (
	"bufio"
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
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type PipelineStep string

const (
	StepDataToDocument   PipelineStep = "data_to_document"   // X_0 → X_1: Raw JSON to Documents (unified)
	StepDocumentToChunks PipelineStep = "document_to_chunks" // X_1 → X_1': Documents to Chunks
	StepChunksToFacts    PipelineStep = "chunks_to_facts"    // X_1' → X_2: Chunks to Facts
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
	fmt.Println("Telegram Memory Pipeline Tester")
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
	fmt.Println("  X_0 → X_1:     data_to_document   (Telegram JSON → memory.Documents)")
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
	case "documents_only":
		return []PipelineStep{
			StepDataToDocument,
		}
	default:
		return []PipelineStep{StepDataToDocument}
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
		case StepDataToDocument:
			if err := p.stepDataToDocument(ctx); err != nil {
				return fmt.Errorf("step %s failed: %w", step, err)
			}
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

// X_0 -> X_1: Process JSONL X_0 records to Documents (unified step).
func (p *MemoryPipeline) stepDataToDocument(ctx context.Context) error {
	p.logger.Info("Processing X_0 JSONL file", "file", p.config.InputFile)

	// Detect if input is JSONL (X_0) or legacy direct input
	if strings.HasSuffix(p.config.InputFile, ".jsonl") {
		// New unified approach: JSONL X_0 → Documents X_1
		return p.processJSONLToDocuments(ctx)
	}
	return nil
}

// New unified JSONL processing path.
func (p *MemoryPipeline) processJSONLToDocuments(ctx context.Context) error {
	// Load records from JSONL file
	records, err := LoadRecords(p.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to load JSONL records: %w", err)
	}

	p.logger.Info("Loaded JSONL records", "count", len(records))

	// Determine processor type based on first record source
	var processor interface {
		ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error)
	}

	if len(records) > 0 {
		source := records[0].Source
		p.logger.Info("Detected source type", "source", source)

		switch source {
		case "whatsapp":
			// Create in-memory store for WhatsApp processor
			store, err := db.NewStore(ctx, ":memory:")
			if err != nil {
				return fmt.Errorf("failed to create store: %w", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					p.logger.Error("Failed to close store", "error", err)
				}
			}()

			processor, err = whatsapp.NewWhatsappProcessor(store, p.logger)
			if err != nil {
				return fmt.Errorf("failed to create WhatsApp processor: %w", err)
			}
		case "telegram":
			// Create in-memory store for Telegram processor
			store, err := db.NewStore(ctx, ":memory:")
			if err != nil {
				return fmt.Errorf("failed to create store: %w", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					p.logger.Error("Failed to close store", "error", err)
				}
			}()

			telegramProcessor, err := telegram.NewTelegramProcessor(store, p.logger)
			if err != nil {
				return fmt.Errorf("failed to create telegram processor: %w", err)
			}
			processor = telegramProcessor
		default:
			return fmt.Errorf("unsupported source type: %s", source)
		}
	} else {
		return fmt.Errorf("no records found in JSONL file")
	}

	// Convert records to documents using the appropriate processor
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		return fmt.Errorf("failed to convert records to documents: %w", err)
	}

	p.logger.Info("Generated documents from JSONL", "count", len(documents))

	// Analyze document types and write output
	return p.writeDocumentsOutput(documents)
}

// Helper function to write documents output (shared by both paths).
func (p *MemoryPipeline) writeDocumentsOutput(documents []memory.Document) error {
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

	// Write X_1: documents as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_1_documents.json")
	documentsData, err := json.MarshalIndent(map[string]interface{}{
		"conversation_documents": conversationDocs,
		"other_documents":        otherDocs,
		"metadata": map[string]interface{}{
			"source_file":        p.config.InputFile,
			"processed_at":       time.Now().Format(time.RFC3339),
			"total_documents":    len(documents),
			"conversation_count": len(conversationDocs),
			"other_count":        len(otherDocs),
			"step":               "data_to_document",
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

// X_1 -> X_1': Chunk documents.
func (p *MemoryPipeline) stepDocumentToChunks() error {
	// Read X_1
	inputFile := filepath.Join(p.config.OutputDir, "X_1_documents.json")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read documents file: %w", err)
	}

	var documentsData struct {
		ConversationDocuments []memory.ConversationDocument `json:"conversation_documents"`
		OtherDocuments        []json.RawMessage             `json:"other_documents"`
	}
	if err := json.Unmarshal(data, &documentsData); err != nil {
		return fmt.Errorf("failed to unmarshal documents: %w", err)
	}

	// PRODUCTION CODE PATH: Step 1: Chunk documents (matches orchestrator.go:107-111)
	var chunkedDocs []memory.Document

	// Process conversation documents
	for _, doc := range documentsData.ConversationDocuments {
		docCopy := doc
		chunks := docCopy.Chunk() // <-- EXACT SAME CODE PATH as production
		chunkedDocs = append(chunkedDocs, chunks...)
	}

	// Process other documents (need to unmarshal them individually)
	for _, rawDoc := range documentsData.OtherDocuments {
		// Try TextDocument first
		var textDoc memory.TextDocument
		if err := json.Unmarshal(rawDoc, &textDoc); err == nil && textDoc.Content() != "" {
			chunks := textDoc.Chunk() // <-- EXACT SAME CODE PATH as production
			chunkedDocs = append(chunkedDocs, chunks...)
			continue
		}

		// Could add other document types here as needed
		p.logger.Warn("Could not unmarshal other document, skipping")
	}

	p.logger.Info("Chunked documents", "original_count", len(documentsData.ConversationDocuments)+len(documentsData.OtherDocuments), "chunked_count", len(chunkedDocs))

	// Write X_1': chunked documents as JSON
	outputFile := filepath.Join(p.config.OutputDir, "X_1'_chunked_documents.json")
	chunkedData, err := json.MarshalIndent(map[string]interface{}{
		"chunked_documents": chunkedDocs,
		"metadata": map[string]interface{}{
			"processed_at":   time.Now().Format(time.RFC3339),
			"step":           "document_to_chunks",
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

// X_1' -> X_2: Extract facts from document chunks.
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

	// Read X_1'
	inputFile := filepath.Join(p.config.OutputDir, "X_1'_chunked_documents.json")
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

// LoadRecords loads records from a JSONL file.
func LoadRecords(filePath string) ([]types.Record, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var records []types.Record
	scanner := bufio.NewScanner(file)

	// Increase buffer size to handle large conversation records (10MB max)
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var record types.Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("unmarshaling line %d: %w", lineNum, err)
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return records, nil
}

func convertWhatsAppToJSONL(inputFile, outputFile string, logger *log.Logger, verbose bool) error {
	// Validate input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Ensure output file has .jsonl extension
	if !strings.HasSuffix(outputFile, ".jsonl") {
		logger.Warn("Output file should have .jsonl extension, adding it", "file", outputFile)
		outputFile = outputFile + ".jsonl"
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	logger.Info("Starting WhatsApp to JSONL conversion",
		"input", inputFile,
		"output", outputFile)

	// Process the SQLite file
	ctx := context.Background()

	// Create in-memory store for WhatsApp processor
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	// Create WhatsApp processor
	processor, err := whatsapp.NewWhatsappProcessor(store, logger)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp processor: %w", err)
	}
	logger.Info("Reading WhatsApp database...")

	records, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to process WhatsApp database: %w", err)
	}

	logger.Info("Successfully read database", "conversations", len(records))

	// Call ToDocuments to convert records to memory documents
	logger.Info("Converting records to memory documents...")
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		return fmt.Errorf("failed to convert to documents: %w", err)
	}

	logger.Info("Successfully converted to documents", "documents", len(documents))

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

			// Type assert to ConversationDocument
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				logger.Info("ConversationDocument",
					"index", i+1,
					"ID", convDoc.ID(),
					"Source", convDoc.Source(),
					"Tags", convDoc.Tags(),
					"User", convDoc.User,
					"People", convDoc.People,
					"MessageCount", len(convDoc.Conversation),
					"Metadata", convDoc.Metadata())
			} else {
				logger.Warn("Document is not a ConversationDocument", "index", i+1, "type", fmt.Sprintf("%T", doc))
			}
		}
	}

	// Save records to JSONL file
	logger.Info("Writing conversations to JSONL file...")
	if err := dataprocessing.SaveRecords(records, outputFile); err != nil {
		return fmt.Errorf("failed to save records: %w", err)
	}

	logger.Info("Conversion completed successfully!",
		"output", outputFile,
		"conversations", len(records))

	// Print summary statistics
	totalMessages := 0
	participants := make(map[string]bool)

	for _, record := range records {
		if conversation, ok := record.Data["conversation"].([]map[string]interface{}); ok {
			totalMessages += len(conversation)
		}
		if people, ok := record.Data["people"].([]string); ok {
			for _, person := range people {
				participants[person] = true
			}
		}
	}

	logger.Info("Summary statistics:",
		"total_conversations", len(records),
		"total_messages", totalMessages,
		"unique_participants", len(participants))

	if totalMessages > 0 && len(records) > 0 {
		avgMessagesPerConv := float64(totalMessages) / float64(len(records))
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
	outputFile := "pipeline_output/X_0_whatsapp.jsonl"

	// Convert WhatsApp to JSONL
	logger.Info("🔄 Converting WhatsApp SQLite to JSONL...")
	if err := convertWhatsAppToJSONL(inputFile, outputFile, logger, false); err != nil {
		logger.Error("WhatsApp conversion failed", "error", err)
		os.Exit(1)
	}

	logger.Info("✅ WhatsApp conversion completed!",
		"output", outputFile,
		"next_step", "Use 'make documents' to process this JSONL file")

	fmt.Println()
	fmt.Println("🎉 WhatsApp data converted successfully!")
	fmt.Printf("📁 Output: %s\n", outputFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the JSONL file if needed")
	fmt.Println("  2. Run: make documents")
	fmt.Println("  3. Run: make chunks")
	fmt.Println("  4. Run: make facts")
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
	outputFile := "pipeline_output/X_0_telegram.jsonl"

	// Convert Telegram to JSONL
	logger.Info("🔄 Converting Telegram JSON to JSONL...")
	if err := convertTelegramToJSONL(inputFile, outputFile, logger); err != nil {
		logger.Error("Telegram conversion failed", "error", err)
		os.Exit(1)
	}

	logger.Info("✅ Telegram conversion completed!",
		"output", outputFile,
		"next_step", "Use 'make documents' to process this JSONL file")

	fmt.Println()
	fmt.Println("🎉 Telegram data converted successfully!")
	fmt.Printf("📁 Output: %s\n", outputFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review the JSONL file if needed")
	fmt.Println("  2. Run: make documents")
	fmt.Println("  3. Run: make chunks")
	fmt.Println("  4. Run: make facts")
	fmt.Println("  Or just: make all")
}

func convertTelegramToJSONL(inputFile, outputFile string, logger *log.Logger) error {
	// Validate input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Ensure output file has .jsonl extension
	if !strings.HasSuffix(outputFile, ".jsonl") {
		logger.Warn("Output file should have .jsonl extension, adding it", "file", outputFile)
		outputFile = outputFile + ".jsonl"
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	logger.Info("Starting Telegram to JSONL conversion",
		"input", inputFile,
		"output", outputFile)

	// Create an in-memory SQLite store for testing
	ctx := context.Background()
	store, err := db.NewStore(ctx, ":memory:")
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	// Create the telegram processor
	processor, err := telegram.NewTelegramProcessor(store, logger)
	if err != nil {
		return fmt.Errorf("failed to create telegram processor: %w", err)
	}

	logger.Info("Reading Telegram export file...")

	// Use the actual ProcessFile method
	records, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}

	logger.Info("Successfully processed Telegram export", "conversations", len(records))

	// Save records to JSONL file
	logger.Info("Writing conversations to JSONL file...")
	if err := dataprocessing.SaveRecords(records, outputFile); err != nil {
		return fmt.Errorf("failed to save records: %w", err)
	}

	logger.Info("Conversion completed successfully!",
		"output", outputFile,
		"conversations", len(records))

	// Print summary statistics
	totalMessages := 0
	participants := make(map[string]bool)

	for _, record := range records {
		if conversation, ok := record.Data["conversation"].([]map[string]interface{}); ok {
			totalMessages += len(conversation)
		}
		if people, ok := record.Data["people"].([]string); ok {
			for _, person := range people {
				participants[person] = true
			}
		}
	}

	logger.Info("Summary statistics:",
		"total_conversations", len(records),
		"total_messages", totalMessages,
		"unique_participants", len(participants))

	if totalMessages > 0 && len(records) > 0 {
		avgMessagesPerConv := float64(totalMessages) / float64(len(records))
		logger.Info("Average messages per conversation:", "average", fmt.Sprintf("%.2f", avgMessagesPerConv))
	}

	return nil
}
