package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
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
	case "chatgpt":
		runChatGPT()
	case "gmail":
		runGmail()
	case "chunks":
		runChunks()
	case "facts":
		runFacts()
	default:
		printUsage()
	}
}

// Polymorphic data processor - eliminates duplication.
func runDataProcessor(name string, filePatterns []string, outputFile string, processorFactory func(*db.Store) (dataprocessing.DocumentProcessor, error)) {
	// Find input file
	var inputFile string
	for _, pattern := range filePatterns {
		if file := findInputFile(pattern); file != "" {
			inputFile = file
			break
		}
	}
	if inputFile == "" {
		logger.Error("No input file found", "name", name, "patterns", filePatterns)
		os.Exit(1)
	}

	logger.Info("Converting data", "name", name, "file", inputFile)

	// Create processor
	ctx := context.Background()
	store, _ := db.NewStore(ctx, ":memory:")
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := processorFactory(store)
	if err != nil {
		logger.Error("Failed to create processor", "error", err)
		os.Exit(1)
	}

	// Process file
	documents, err := processor.ProcessFile(ctx, inputFile)
	if err != nil {
		logger.Error("Processing failed", "error", err)
		os.Exit(1)
	}

	// Save output
	if err := saveJSON(documents, outputFile); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Conversion done", "name", name, "documents", len(documents))
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
	if f := findInputFile("pipeline_output/X_0_telegram.json"); f != "" {
		return f
	}
	if f := findInputFile("pipeline_output/X_0_gmail.json"); f != "" {
		return f
	}
	return findInputFile("pipeline_output/X_0_chatgpt.json")
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

// Data source processors using polymorphic function.
func runWhatsApp() {
	runDataProcessor(
		"WhatsApp",
		[]string{"pipeline_input/*.sqlite", "pipeline_input/*.db"},
		"pipeline_output/X_0_whatsapp.json",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return whatsapp.NewWhatsappProcessor(store, logger)
		},
	)
}

func runTelegram() {
	runDataProcessor(
		"Telegram",
		[]string{"pipeline_input/*.json"},
		"pipeline_output/X_0_telegram.json",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return telegram.NewTelegramProcessor(store, logger)
		},
	)
}

func runChatGPT() {
	runDataProcessor(
		"ChatGPT",
		[]string{"pipeline_input/*.json", "pipeline_input/conversations.json"},
		"pipeline_output/X_0_chatgpt.json",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return chatgpt.NewChatGPTProcessor(store, logger)
		},
	)
}

func runGmail() {
	// Check for --senders flag
	sendersOnly := len(os.Args) > 2 && os.Args[2] == "--senders"

	if sendersOnly {
		runGmailSenders()
		return
	}

	runDataProcessor(
		"Gmail",
		[]string{"pipeline_input/*.mbox"},
		"pipeline_output/X_0_gmail.json",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return gmail.NewGmailProcessor(store, logger)
		},
	)
}

func runGmailSenders() {
	// Find input file
	var inputFile string
	for _, pattern := range []string{"pipeline_input/*.mbox"} {
		if file := findInputFile(pattern); file != "" {
			inputFile = file
			break
		}
	}
	if inputFile == "" {
		logger.Error("No mbox file found", "patterns", []string{"pipeline_input/*.mbox"})
		os.Exit(1)
	}

	logger.Info("Analyzing Gmail senders", "file", inputFile)

	// Create processor
	ctx := context.Background()
	store, _ := db.NewStore(ctx, ":memory:")
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		}
	}()

	processor, err := gmail.NewGmailProcessor(store, logger)
	if err != nil {
		logger.Error("Failed to create Gmail processor", "error", err)
		os.Exit(1)
	}

	// Process file for senders only
	if err := processor.ProcessFileForSenders(ctx, inputFile, "pipeline_output"); err != nil {
		logger.Error("Sender analysis failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Sender analysis completed. Edit pipeline_output/senders.json then run 'make gmail' again.")
}

// Pipeline steps.
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

	logger.Info("Starting parallel fact extraction", "documents", len(documents))

	// 🚀 PARALLEL FACT EXTRACTION WITH WORKER POOL
	numWorkers := 100 // YOLO mode - maximize those OpenAI credits! 💸
	allFacts := extractFactsParallel(documents, numWorkers)

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

// 🔥 PARALLEL FACT EXTRACTION WORKER POOL
func extractFactsParallel(documents []memory.Document, numWorkers int) []*memory.MemoryFact {
	aiService := ai.NewOpenAIService(
		logger,
		os.Getenv("COMPLETIONS_API_KEY"),
		getEnvOrDefault("COMPLETIONS_API_URL", "https://openrouter.ai/api/v1"),
	)

	// Input channel for documents
	docChan := make(chan memory.Document, len(documents))

	// Results channel for facts
	type factResult struct {
		facts []*memory.MemoryFact
		docID string
		err   error
	}
	resultChan := make(chan factResult, len(documents))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for doc := range docChan {
				start := time.Now()
				logger.Debug("Processing document", "worker", workerID, "doc", doc.ID())

				facts, err := evolvingmemory.ExtractFactsFromDocument(
					context.Background(),
					doc,
					aiService,
					getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
					logger,
				)

				duration := time.Since(start)
				if err != nil {
					logger.Error("Fact extraction failed",
						"worker", workerID,
						"doc", doc.ID(),
						"duration", duration,
						"error", err)
				} else {
					logger.Info("Document processed",
						"worker", workerID,
						"doc", doc.ID(),
						"facts", len(facts),
						"duration", duration)
				}

				resultChan <- factResult{
					facts: facts,
					docID: doc.ID(),
					err:   err,
				}
			}
		}(i)
	}

	// Send all documents to workers
	for _, doc := range documents {
		docChan <- doc
	}
	close(docChan)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results with progress tracking
	var allFacts []*memory.MemoryFact
	processed := 0
	totalDocs := len(documents)

	for result := range resultChan {
		processed++
		if result.err == nil {
			allFacts = append(allFacts, result.facts...)
		}

		// Progress logging every 10 documents or at completion
		if processed%10 == 0 || processed == totalDocs {
			logger.Info("Progress update",
				"processed", processed,
				"total", totalDocs,
				"facts_so_far", len(allFacts),
				"percent", fmt.Sprintf("%.1f%%", float64(processed)/float64(totalDocs)*100))
		}
	}

	return allFacts
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
	fmt.Println("  memory-processor-test chatgpt")
	fmt.Println("  memory-processor-test gmail")
	fmt.Println("  memory-processor-test gmail --senders  # Analyze senders only")
	fmt.Println("  memory-processor-test chunks")
	fmt.Println("  memory-processor-test facts")
	fmt.Println()
	fmt.Println("Or use make commands:")
	fmt.Println("  make whatsapp # Convert WhatsApp SQLite")
	fmt.Println("  make telegram # Convert Telegram JSON")
	fmt.Println("  make chatgpt  # Convert ChatGPT JSON")
	fmt.Println("  make gmail    # Convert Gmail mbox")
	fmt.Println("  make gmail --senders # Analyze Gmail senders, create senders.json")
	fmt.Println("  make chunks   # X_0 → X_1")
	fmt.Println("  make facts    # X_1 → X_2")
}
