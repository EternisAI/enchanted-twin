package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportCaller: true, Level: log.InfoLevel, TimeFormat: time.Kitchen,
})

func main() {
	_ = godotenv.Load("../../.env")

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "whatsapp":
		runWhatsApp()
	case "telegram":
		runTelegram()
	case "--steps":
		if len(os.Args) < 3 {
			logger.Error("--steps requires a value")
			os.Exit(1)
		}
		runStep(os.Args[2])
	default:
		printUsage()
	}
}

// Shared file utilities.
func findInputFile(pattern string) string {
	files, _ := filepath.Glob(pattern)
	if len(files) > 0 {
		return files[0]
	}
	return ""
}

func findX0File() string {
	if f := findInputFile("pipeline_output/X_0_whatsapp.json"); f != "" {
		return f
	}
	return findInputFile("pipeline_output/X_0_telegram.json")
}

func saveJSON(data interface{}, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0o644)
}

func loadJSON(filename string, target interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Data source processors.
func runWhatsApp() {
	inputFile := findInputFile("pipeline_input/*.sqlite")
	if inputFile == "" {
		inputFile = findInputFile("pipeline_input/*.db")
	}
	if inputFile == "" {
		logger.Error("No SQLite file found in pipeline_input/")
		os.Exit(1)
	}

	logger.Info("Converting WhatsApp", "file", inputFile)

	ctx := context.Background()
	store, _ := db.NewStore(ctx, ":memory:")
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := whatsapp.NewWhatsappProcessor(store, logger)
	if err != nil {
		logger.Error("Failed to create processor", "error", err)
		os.Exit(1)
	}

	documents, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		logger.Error("Processing failed", "error", err)
		os.Exit(1)
	}

	if err := saveJSON(documents, "pipeline_output/X_0_whatsapp.json"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("WhatsApp conversion done", "documents", len(documents))
}

func runTelegram() {
	inputFile := findInputFile("pipeline_input/*.json")
	if inputFile == "" {
		logger.Error("No JSON file found in pipeline_input/")
		os.Exit(1)
	}

	logger.Info("Converting Telegram", "file", inputFile)

	ctx := context.Background()
	store, _ := db.NewStore(ctx, ":memory:")
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := telegram.NewTelegramProcessor(store, logger)
	if err != nil {
		logger.Error("Failed to create processor", "error", err)
		os.Exit(1)
	}

	documents, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		logger.Error("Processing failed", "error", err)
		os.Exit(1)
	}

	if err := saveJSON(documents, "pipeline_output/X_0_telegram.json"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Telegram conversion done", "documents", len(documents))
}

// Pipeline steps.
func runStep(step string) {
	switch step {
	case "chunks_only":
		runChunks()
	case "facts_only":
		runFacts()
	default:
		logger.Error("Unknown step", "step", step)
		os.Exit(1)
	}
}

func runChunks() {
	inputFile := findX0File()
	if inputFile == "" {
		logger.Error("No X_0 file found")
		os.Exit(1)
	}

	logger.Info("Chunking documents", "input", inputFile)

	documents, err := memory.LoadConversationDocumentsFromJSON(inputFile)
	if err != nil {
		logger.Error("Load failed", "error", err)
		os.Exit(1)
	}

	var chunkedDocs []memory.Document
	for _, doc := range documents {
		docCopy := doc
		chunks := docCopy.Chunk()
		chunkedDocs = append(chunkedDocs, chunks...)
	}

	output := map[string]interface{}{
		"chunked_documents": chunkedDocs,
		"metadata": map[string]interface{}{
			"processed_at":   time.Now().Format(time.RFC3339),
			"step":           "document_to_chunks",
			"original_count": len(documents),
			"chunked_count":  len(chunkedDocs),
		},
	}

	if err := saveJSON(output, "pipeline_output/X_1_chunked_documents.json"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Chunking done", "original", len(documents), "chunks", len(chunkedDocs))
}

func runFacts() {
	if os.Getenv("COMPLETIONS_API_KEY") == "" {
		logger.Error("COMPLETIONS_API_KEY required for fact extraction")
		os.Exit(1)
	}

	logger.Info("Extracting facts")

	var chunkedData struct {
		ChunkedDocuments []json.RawMessage `json:"chunked_documents"`
	}
	if err := loadJSON("pipeline_output/X_1_chunked_documents.json", &chunkedData); err != nil {
		logger.Error("Load failed", "error", err)
		os.Exit(1)
	}

	// Convert raw JSON to Document instances
	var documents []memory.Document
	for _, rawDoc := range chunkedData.ChunkedDocuments {
		var convDoc memory.ConversationDocument
		if err := json.Unmarshal(rawDoc, &convDoc); err == nil && len(convDoc.Conversation) > 0 {
			documents = append(documents, &convDoc)
			continue
		}
		var textDoc memory.TextDocument
		if err := json.Unmarshal(rawDoc, &textDoc); err == nil && textDoc.Content() != "" {
			documents = append(documents, &textDoc)
		}
	}

	aiService := ai.NewOpenAIService(
		logger,
		os.Getenv("COMPLETIONS_API_KEY"),
		getEnvOrDefault("COMPLETIONS_API_URL", "https://openrouter.ai/api/v1"),
	)

	var allFacts []*memory.MemoryFact
	for _, doc := range documents {
		facts, err := evolvingmemory.ExtractFactsFromDocument(
			context.Background(),
			doc,
			aiService,
			getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
			logger,
		)
		if err != nil {
			logger.Error("Fact extraction failed", "id", doc.ID(), "error", err)
			continue
		}
		allFacts = append(allFacts, facts...)
	}

	output := map[string]interface{}{
		"facts": allFacts,
		"metadata": map[string]interface{}{
			"processed_at":      time.Now().Format(time.RFC3339),
			"step":              "chunks_to_facts",
			"documents_count":   len(documents),
			"facts_count":       len(allFacts),
			"completions_model": getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
		},
	}

	if err := saveJSON(output, "pipeline_output/X_2_extracted_facts.json"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Fact extraction done", "documents", len(documents), "facts", len(allFacts))
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
	fmt.Println("Usage:")
	fmt.Println("  memory-processor-test whatsapp")
	fmt.Println("  memory-processor-test telegram")
	fmt.Println("  memory-processor-test --steps chunks_only")
	fmt.Println("  memory-processor-test --steps facts_only")
	fmt.Println()
	fmt.Println("Or use make commands:")
	fmt.Println("  make whatsapp  # Convert WhatsApp SQLite")
	fmt.Println("  make telegram  # Convert Telegram JSON")
	fmt.Println("  make chunks    # X_0 → X_1")
	fmt.Println("  make facts     # X_1 → X_2")
}
