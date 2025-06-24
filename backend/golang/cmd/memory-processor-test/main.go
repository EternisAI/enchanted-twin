package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-openapi/strfmt"
	"github.com/joho/godotenv"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
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
	case "store":
		runStore()
	case "consolidation":
		runConsolidation()
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

func runStore() {
	logger.Info("Storing facts directly in Weaviate (NO PIPELINE RE-EXTRACTION)")

	// Find X_2 facts file
	inputFile := findInputFile("pipeline_output/X_2_*.json")
	if inputFile == "" {
		logger.Error("No X_2 facts file found")
		os.Exit(1)
	}

	logger.Info("Loading facts", "file", inputFile)

	// Load facts from JSON
	var factsData struct {
		Facts []memory.MemoryFact `json:"facts"`
	}
	if err := loadJSON(inputFile, &factsData); err != nil {
		logger.Error("Load failed", "error", err)
		os.Exit(1)
	}

	if len(factsData.Facts) == 0 {
		logger.Warn("No facts found to store")
		return
	}

	logger.Info("Found facts to store", "count", len(factsData.Facts))

	// Weaviate configuration
	ctx := context.Background()
	weaviatePort := getEnvOrDefault("WEAVIATE_PORT", "51414")
	weaviatePath := filepath.Join(".", "pipeline_output", "weaviate-test-memory")

	logger.Info("Starting Weaviate bootstrap", "port", weaviatePort, "path", weaviatePath)

	if _, err := bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviatePath); err != nil {
		logger.Error("Failed to bootstrap Weaviate server", "error", err)
		os.Exit(1)
	}

	// Create Weaviate client
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Failed to create Weaviate client", "error", err)
		os.Exit(1)
	}

	// Create AI services for embeddings only (no completions needed for direct storage)
	aiEmbeddingsService := ai.NewOpenAIService(
		logger,
		getEnvOrDefault("EMBEDDINGS_API_KEY", os.Getenv("COMPLETIONS_API_KEY")),
		getEnvOrDefault("EMBEDDINGS_API_URL", "https://api.openai.com/v1"),
	)

	// Initialize Weaviate schema
	embeddingsModel := getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small")
	if err := bootstrap.InitSchema(weaviateClient, logger, aiEmbeddingsService, embeddingsModel); err != nil {
		logger.Error("Failed to initialize Weaviate schema", "error", err)
		os.Exit(1)
	}

	// Create embedding wrapper
	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiEmbeddingsService, embeddingsModel)
	if err != nil {
		logger.Error("Failed to create embedding wrapper", "error", err)
		os.Exit(1)
	}

	// Create storage interface
	storageInterface, err := storage.New(storage.NewStorageInput{
		Client:            weaviateClient,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
	})
	if err != nil {
		logger.Error("Failed to create storage interface", "error", err)
		os.Exit(1)
	}

	// Store facts directly - BYPASS THE PIPELINE COMPLETELY!
	logger.Info("Creating Weaviate objects directly from facts", "count", len(factsData.Facts))

	// FAST BATCH EMBEDDINGS - collect all content first
	var factContents []string
	for _, fact := range factsData.Facts {
		factContents = append(factContents, fact.GenerateContent())
	}

	// Generate ALL embeddings in batch - MUCH FASTER! 🚀
	logger.Info("Generating embeddings in batch", "count", len(factContents))
	embeddings, err := embeddingsWrapper.Embeddings(ctx, factContents)
	if err != nil {
		logger.Error("Failed to generate batch embeddings", "error", err)
		os.Exit(1)
	}

	logger.Info("Batch embeddings complete, creating Weaviate objects", "embeddings", len(embeddings))

	var objects []*models.Object
	for i, fact := range factsData.Facts {
		// Use pre-generated embedding
		embedding := embeddings[i]

		// Build tags
		tags := fact.Tags
		if len(tags) == 0 {
			tags = []string{fact.Category} // Fallback to category
		}

		// Build metadata JSON
		metadata := map[string]string{
			"fact_id": fact.ID,
		}
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
			Vector:     embedding,
		}

		// Only set ID if it's a valid UUID format
		if fact.ID != "" {
			obj.ID = strfmt.UUID(fact.ID)
		}
		// Otherwise let Weaviate auto-generate the UUID

		objects = append(objects, obj)
	}

	// Store batch directly - bypasses the entire pipeline!
	logger.Info("Storing facts batch directly in Weaviate", "count", len(objects))

	if err := storageInterface.StoreBatch(ctx, objects); err != nil {
		logger.Error("Failed to store facts batch", "error", err)
		os.Exit(1)
	}

	logger.Info("Facts stored successfully (DIRECT STORAGE)", "count", len(objects))

	// Create a marker file to indicate storage is complete
	statusFile := "pipeline_output/X_3_storage_complete.json"
	status := map[string]interface{}{
		"stored_at":     time.Now().Format(time.RFC3339),
		"facts_count":   len(objects),
		"weaviate_port": weaviatePort,
		"weaviate_path": weaviatePath,
		"step":          "facts_to_storage_direct",
		"method":        "direct_storage_bypass_pipeline",
	}
	if err := saveJSON(status, statusFile); err != nil {
		logger.Error("Failed to save status", "error", err)
	}

	logger.Info("Storage complete marker created", "file", statusFile)
}

func runConsolidation() {
	logger.Info("Running memory consolidation")

	// Check for required API key
	if os.Getenv("COMPLETIONS_API_KEY") == "" {
		logger.Error("COMPLETIONS_API_KEY required for consolidation")
		os.Exit(1)
	}

	// Check if storage is complete
	if findInputFile("pipeline_output/X_3_storage_complete.json") == "" {
		logger.Error("Storage not complete. Run 'make store' first.")
		os.Exit(1)
	}

	// Load storage status to get Weaviate connection info
	var storageStatus struct {
		WeaviatePort string `json:"weaviate_port"`
		WeaviatePath string `json:"weaviate_path"`
		FactsCount   int    `json:"facts_count"`
	}
	if err := loadJSON("pipeline_output/X_3_storage_complete.json", &storageStatus); err != nil {
		logger.Error("Failed to load storage status", "error", err)
		os.Exit(1)
	}

	logger.Info("Connecting to existing Weaviate instance",
		"port", storageStatus.WeaviatePort,
		"facts_stored", storageStatus.FactsCount)

	// Try to connect to existing Weaviate first
	ctx := context.Background()
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", storageStatus.WeaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Failed to create Weaviate client", "error", err)
		os.Exit(1)
	}

	// Test if Weaviate is actually running by trying a simple query
	ready, err := weaviateClient.Misc().ReadyChecker().Do(ctx)
	if err != nil || !ready {
		logger.Info("Existing Weaviate not running, restarting it", "port", storageStatus.WeaviatePort)

		// Restart Weaviate using the same settings
		weaviatePort := storageStatus.WeaviatePort
		weaviatePath := storageStatus.WeaviatePath

		logger.Info("Starting Weaviate bootstrap", "port", weaviatePort, "path", weaviatePath)

		_, err = bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviatePath)
		if err != nil {
			logger.Error("Failed to restart Weaviate", "error", err)
			os.Exit(1)
		}

		// Create new client after restart
		weaviateClient, err = weaviate.NewClient(weaviate.Config{
			Host:   fmt.Sprintf("localhost:%s", weaviatePort),
			Scheme: "http",
		})
		if err != nil {
			logger.Error("Failed to create Weaviate client after restart", "error", err)
			os.Exit(1)
		}

		logger.Info("Weaviate restarted successfully")
	} else {
		logger.Info("Connected to existing Weaviate instance")
	}

	// Create AI services
	aiEmbeddingsService := ai.NewOpenAIService(
		logger,
		getEnvOrDefault("EMBEDDINGS_API_KEY", os.Getenv("COMPLETIONS_API_KEY")),
		getEnvOrDefault("EMBEDDINGS_API_URL", "https://api.openai.com/v1"),
	)

	aiCompletionsService := ai.NewOpenAIService(
		logger,
		os.Getenv("COMPLETIONS_API_KEY"),
		getEnvOrDefault("COMPLETIONS_API_URL", "https://openrouter.ai/api/v1"),
	)

	// Create storage interface
	embeddingsModel := getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small")
	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiEmbeddingsService, embeddingsModel)
	if err != nil {
		logger.Error("Failed to create embedding wrapper", "error", err)
		os.Exit(1)
	}

	storageInterface, err := storage.New(storage.NewStorageInput{
		Client:            weaviateClient,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
	})
	if err != nil {
		logger.Error("Failed to create storage interface", "error", err)
		os.Exit(1)
	}

	// Create memory storage
	memoryStorage, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: aiCompletionsService,
		CompletionsModel:   getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
		EmbeddingsWrapper:  embeddingsWrapper,
	})
	if err != nil {
		logger.Error("Failed to create memory storage", "error", err)
		os.Exit(1)
	}

	// TODO: add more intelligent consolidation logic
	// Run semantic consolidation - much smarter than tag-based!
	consolidationTopic := "category theory"
	logger.Info("Running semantic consolidation", "topic", consolidationTopic)

	consolidationDeps := evolvingmemory.ConsolidationDependencies{
		Logger:             logger,
		Storage:            memoryStorage,
		CompletionsService: aiCompletionsService,
		CompletionsModel:   getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
	}

	// Use semantic search with filtering for better results
	filter := &memory.Filter{
		Distance:          0.75,                                                  // Allow fairly broad semantic matches
		Limit:             func() *int { limit := 30; return &limit }(),          // Reasonable limit
		FactImportanceMin: func() *int { importance := 2; return &importance }(), // Only meaningful facts
	}

	report, err := evolvingmemory.ConsolidateMemoriesBySemantic(ctx, consolidationTopic, filter, consolidationDeps)
	if err != nil {
		logger.Error("Semantic consolidation failed", "error", err)
		os.Exit(1)
	}

	// Export consolidation report
	outputFile := "pipeline_output/X_4_consolidation_report.json"
	if err := report.ExportJSON(outputFile); err != nil {
		logger.Error("Failed to export consolidation report", "error", err)
		os.Exit(1)
	}

	logger.Info("Consolidation completed",
		"source_facts", report.SourceFactCount,
		"consolidated_facts", len(report.ConsolidatedFacts),
		"output", outputFile)
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
	fmt.Println("  memory-processor-test store")
	fmt.Println("  memory-processor-test consolidation")
	fmt.Println()
	fmt.Println("Or use make commands:")
	fmt.Println("  make whatsapp # Convert WhatsApp SQLite")
	fmt.Println("  make telegram # Convert Telegram JSON")
	fmt.Println("  make chatgpt  # Convert ChatGPT JSON")
	fmt.Println("  make gmail    # Convert Gmail mbox")
	fmt.Println("  make gmail --senders # Analyze Gmail senders, create senders.json")
	fmt.Println("  make chunks   # X_0 → X_1")
	fmt.Println("  make facts    # X_1 → X_2")
	fmt.Println("  make store    # X_2 → Weaviate")
	fmt.Println("  make consolidation # Weaviate → X_4")
}
