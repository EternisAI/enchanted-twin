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
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

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

type WeaviateInfrastructure struct {
	Client             *weaviate.Client
	MemoryStorage      evolvingmemory.MemoryStorage
	EmbeddingsService  *ai.Service
	CompletionsService *ai.Service
	EmbeddingsWrapper  *storage.EmbeddingWrapper
	StorageInterface   storage.Interface
	Context            context.Context
}

// setupWeaviateMemoryInfrastructure creates a complete Weaviate + Memory setup
// Handles connection, startup if needed, AI services, and memory storage.
func setupWeaviateMemoryInfrastructure() (*WeaviateInfrastructure, error) {
	ctx := context.Background()
	weaviatePort := getEnvOrDefault("WEAVIATE_PORT", "51414")
	weaviatePath := filepath.Join(".", "pipeline_output", "weaviate-test-memory")

	logger.Info("Setting up Weaviate infrastructure", "port", weaviatePort)

	// Try to connect to existing Weaviate first
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	// Test if Weaviate is actually running by trying a simple query
	ready, err := weaviateClient.Misc().ReadyChecker().Do(ctx)
	if err != nil || !ready {
		logger.Info("Weaviate not running, starting it", "port", weaviatePort)

		// Start Weaviate
		logger.Info("Starting Weaviate bootstrap", "port", weaviatePort, "path", weaviatePath)

		_, err = bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to start Weaviate: %w", err)
		}

		// Create new client after startup
		weaviateClient, err = weaviate.NewClient(weaviate.Config{
			Host:   fmt.Sprintf("localhost:%s", weaviatePort),
			Scheme: "http",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Weaviate client after startup: %w", err)
		}

		logger.Info("Weaviate started successfully")
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
		return nil, fmt.Errorf("failed to create embedding wrapper: %w", err)
	}

	storageInterface, err := storage.New(storage.NewStorageInput{
		Client:            weaviateClient,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage interface: %w", err)
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
		return nil, fmt.Errorf("failed to create memory storage: %w", err)
	}

	return &WeaviateInfrastructure{
		Client:             weaviateClient,
		MemoryStorage:      memoryStorage,
		EmbeddingsService:  aiEmbeddingsService,
		CompletionsService: aiCompletionsService,
		EmbeddingsWrapper:  embeddingsWrapper,
		StorageInterface:   storageInterface,
		Context:            ctx,
	}, nil
}

// For runStore which needs schema initialization.
func setupWeaviateMemoryInfrastructureWithSchema() (*WeaviateInfrastructure, error) {
	infra, err := setupWeaviateMemoryInfrastructure()
	if err != nil {
		return nil, err
	}

	// Initialize Weaviate schema for fresh deployments
	embeddingsModel := getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small")
	if err := bootstrap.InitSchema(infra.Client, logger, infra.EmbeddingsService, embeddingsModel); err != nil {
		return nil, fmt.Errorf("failed to initialize Weaviate schema: %w", err)
	}

	return infra, nil
}

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
	case "store-consolidations":
		runStoreConsolidations()
	case "query-consolidations":
		runQueryConsolidations()
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
	logger.Info("Storing facts using production storage module")

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

	// Set up Weaviate infrastructure with schema initialization
	infra, err := setupWeaviateMemoryInfrastructureWithSchema()
	if err != nil {
		logger.Error("Failed to setup Weaviate infrastructure", "error", err)
		os.Exit(1)
	}

	// Convert facts to pointers for the interface
	var facts []*memory.MemoryFact
	for i := range factsData.Facts {
		facts = append(facts, &factsData.Facts[i])
	}

	// Use the PRODUCTION StoreFactsDirectly method! 🚀
	logger.Info("Storing facts using production storage module", "count", len(facts))

	err = infra.MemoryStorage.StoreFactsDirectly(infra.Context, facts, func(processed, total int) {
		logger.Info("Storage progress", "processed", processed, "total", total)
	})
	if err != nil {
		logger.Error("Failed to store facts", "error", err)
		os.Exit(1)
	}

	logger.Info("Facts stored successfully using PRODUCTION code", "count", len(facts))
	logger.Info("Storage complete - ready for consolidation")
}

func runConsolidation() {
	logger.Info("Running comprehensive memory consolidation for all subjects")

	// Check for required API key
	if os.Getenv("COMPLETIONS_API_KEY") == "" {
		logger.Error("COMPLETIONS_API_KEY required for consolidation")
		os.Exit(1)
	}

	// Weaviate configuration (use defaults since we're not tracking status anymore)
	ctx := context.Background()
	weaviatePort := getEnvOrDefault("WEAVIATE_PORT", "51414")
	weaviatePath := filepath.Join(".", "pipeline_output", "weaviate-test-memory")

	logger.Info("Connecting to Weaviate instance", "port", weaviatePort)

	// Try to connect to existing Weaviate first
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Failed to create Weaviate client", "error", err)
		os.Exit(1)
	}

	// Test if Weaviate is actually running by trying a simple query
	ready, err := weaviateClient.Misc().ReadyChecker().Do(ctx)
	if err != nil || !ready {
		logger.Info("Weaviate not running, starting it", "port", weaviatePort)

		// Start Weaviate
		logger.Info("Starting Weaviate bootstrap", "port", weaviatePort, "path", weaviatePath)

		_, err = bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviatePath)
		if err != nil {
			logger.Error("Failed to start Weaviate", "error", err)
			os.Exit(1)
		}

		// Create new client after startup
		weaviateClient, err = weaviate.NewClient(weaviate.Config{
			Host:   fmt.Sprintf("localhost:%s", weaviatePort),
			Scheme: "http",
		})
		if err != nil {
			logger.Error("Failed to create Weaviate client after startup", "error", err)
			os.Exit(1)
		}

		logger.Info("Weaviate started successfully")
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

	// Set up consolidation dependencies
	consolidationDeps := evolvingmemory.ConsolidationDependencies{
		Logger:             logger,
		Storage:            memoryStorage,
		CompletionsService: aiCompletionsService,
		CompletionsModel:   getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
	}

	logger.Info("Starting comprehensive consolidation", "subjects", len(evolvingmemory.ConsolidationSubjects))

	// Run consolidation for all 20 canonical subjects
	var allReports []*evolvingmemory.ConsolidationReport

	for i, subject := range evolvingmemory.ConsolidationSubjects {
		logger.Info("Processing consolidation subject",
			"subject", subject,
			"progress", fmt.Sprintf("%d/%d", i+1, len(evolvingmemory.ConsolidationSubjects)))

		// Use semantic search with filtering for better results
		filter := &memory.Filter{
			Distance:          0.75,                                                  // Allow fairly broad semantic matches
			Limit:             func() *int { limit := 30; return &limit }(),          // Reasonable limit
			FactImportanceMin: func() *int { importance := 2; return &importance }(), // Only meaningful facts
		}

		report, err := evolvingmemory.ConsolidateMemoriesBySemantic(ctx, subject, filter, consolidationDeps)
		if err != nil {
			logger.Error("Semantic consolidation failed", "subject", subject, "error", err)
			continue // Don't fail entire process for one subject
		}

		logger.Info("Subject consolidation completed",
			"subject", subject,
			"source_facts", report.SourceFactCount,
			"consolidated_facts", len(report.ConsolidatedFacts))

		allReports = append(allReports, report)
	}

	// Create comprehensive consolidation output
	comprehensiveOutput := map[string]interface{}{
		"consolidation_reports": allReports,
		"metadata": map[string]interface{}{
			"processed_at":       time.Now().Format(time.RFC3339),
			"step":               "comprehensive_consolidation",
			"total_subjects":     len(evolvingmemory.ConsolidationSubjects),
			"successful_reports": len(allReports),
			"completions_model":  getEnvOrDefault("COMPLETIONS_MODEL", "openai/gpt-4.1"),
			"consolidation_type": "semantic_search",
		},
	}

	// Export comprehensive consolidation report
	outputFile := "pipeline_output/X_3_consolidation_report.json"
	if err := saveJSON(comprehensiveOutput, outputFile); err != nil {
		logger.Error("Failed to export comprehensive consolidation report", "error", err)
		os.Exit(1)
	}

	// Calculate totals
	totalSourceFacts := 0
	totalConsolidatedFacts := 0
	for _, report := range allReports {
		totalSourceFacts += report.SourceFactCount
		totalConsolidatedFacts += len(report.ConsolidatedFacts)
	}

	logger.Info("Comprehensive consolidation completed",
		"subjects_processed", len(allReports),
		"total_source_facts", totalSourceFacts,
		"total_consolidated_facts", totalConsolidatedFacts,
		"output", outputFile)
}

func runStoreConsolidations() {
	logger.Info("Storing consolidated facts in Weaviate database")

	// Check if consolidation report exists
	consolidationFile := "pipeline_output/X_3_consolidation_report.json"
	if _, err := os.Stat(consolidationFile); os.IsNotExist(err) {
		logger.Error("Consolidation report not found. Run 'make consolidation' first.", "file", consolidationFile)
		os.Exit(1)
	}

	// Load consolidation reports using evolvingmemory package
	reports, err := evolvingmemory.LoadConsolidationReportsFromJSON(consolidationFile)
	if err != nil {
		logger.Error("Failed to load consolidation reports", "error", err)
		os.Exit(1)
	}

	// Set up Weaviate infrastructure (smart connection like consolidation)
	infra, err := setupWeaviateMemoryInfrastructure()
	if err != nil {
		logger.Error("Failed to setup Weaviate infrastructure", "error", err)
		os.Exit(1)
	}

	// Store all reports using evolvingmemory package! 🚀
	err = evolvingmemory.StoreConsolidationReports(infra.Context, reports, infra.MemoryStorage, func(processed, total int) {
		logger.Info("Storing consolidated facts", "progress", fmt.Sprintf("%d/%d", processed, total))
	})
	if err != nil {
		logger.Error("Failed to store consolidation reports", "error", err)
		os.Exit(1)
	}

	// Calculate totals for status
	totalSourceFacts := 0
	totalConsolidatedFacts := 0
	for _, report := range reports {
		totalSourceFacts += report.SourceFactCount
		totalConsolidatedFacts += len(report.ConsolidatedFacts)
	}

	// Create storage status report
	storageStatus := map[string]interface{}{
		"stored_at":                time.Now().Format(time.RFC3339),
		"step":                     "consolidation_storage",
		"total_reports_processed":  len(reports),
		"total_source_facts":       totalSourceFacts,
		"total_consolidated_facts": totalConsolidatedFacts,
		"storage_metadata": map[string]interface{}{
			"embeddings_model": getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small"),
			"weaviate_port":    getEnvOrDefault("WEAVIATE_PORT", "51414"),
		},
	}

	// Export storage status
	outputFile := "pipeline_output/X_4_consolidation_storage_status.json"
	if err := saveJSON(storageStatus, outputFile); err != nil {
		logger.Error("Failed to export storage status", "error", err)
		os.Exit(1)
	}

	logger.Info("Consolidation storage completed",
		"reports_processed", len(reports),
		"total_source_facts", totalSourceFacts,
		"total_consolidated_facts", totalConsolidatedFacts,
		"output", outputFile)
}

func runQueryConsolidations() {
	// Check for query parameter
	var queryText string
	if len(os.Args) >= 3 {
		queryText = os.Args[2]
	} else {
		// Check environment variable for make query QUERY="..."
		queryText = os.Getenv("QUERY")
	}

	if queryText == "" {
		logger.Error("Query required. Usage: memory-processor-test query-consolidations \"your query\" or make query QUERY=\"your query\"")
		os.Exit(1)
	}

	logger.Info("Executing intelligent 3-stage query", "query", queryText)

	// Check if consolidations are stored
	statusFile := "pipeline_output/X_4_consolidation_storage_status.json"
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		logger.Error("Consolidations not stored yet. Run 'make store-consolidations' first.", "file", statusFile)
		os.Exit(1)
	}

	// Set up Weaviate infrastructure
	infra, err := setupWeaviateMemoryInfrastructure()
	if err != nil {
		logger.Error("Failed to setup Weaviate infrastructure", "error", err)
		os.Exit(1)
	}

	logger.Info("Executing intelligent query", "query", queryText)

	// Execute the new 3-stage intelligent query
	intelligentResult, err := infra.MemoryStorage.IntelligentQuery(infra.Context, queryText, &memory.Filter{
		Distance: 0.7,
	})
	if err != nil {
		logger.Error("Intelligent query failed", "query", queryText, "error", err)
		os.Exit(1)
	}

	// Create comprehensive query result with backwards compatibility
	queryResult := map[string]interface{}{
		"query":                     queryText,
		"queried_at":                time.Now().Format(time.RFC3339),
		"intelligent_query_results": intelligentResult,
		"legacy_vector_search_results": map[string]interface{}{
			"all_facts": map[string]interface{}{
				"count":       intelligentResult.Metadata.TotalResults,
				"facts":       append(append(intelligentResult.ConsolidatedInsights, intelligentResult.CitedEvidence...), intelligentResult.AdditionalContext...),
				"description": "All facts from intelligent 3-stage query",
			},
			"consolidated_only": map[string]interface{}{
				"count":       intelligentResult.Metadata.ConsolidatedInsightCount,
				"facts":       intelligentResult.ConsolidatedInsights,
				"description": "Consolidated insights (Stage 1 results)",
			},
			"cited_evidence": map[string]interface{}{
				"count":       intelligentResult.Metadata.CitedEvidenceCount,
				"facts":       intelligentResult.CitedEvidence,
				"description": "Source facts cited by consolidated insights (Stage 2 results)",
			},
			"additional_context": map[string]interface{}{
				"count":       intelligentResult.Metadata.AdditionalContextCount,
				"facts":       intelligentResult.AdditionalContext,
				"description": "Additional raw facts for context (Stage 3 results)",
			},
		},
		"query_metadata": map[string]interface{}{
			"embeddings_model":   getEnvOrDefault("EMBEDDINGS_MODEL", "text-embedding-3-small"),
			"weaviate_port":      getEnvOrDefault("WEAVIATE_PORT", "51414"),
			"distance_threshold": 0.7,
			"query_strategy":     intelligentResult.Metadata.QueryStrategy,
		},
	}

	// Export query results
	outputFile := fmt.Sprintf("pipeline_output/X_5_query_results_%d.json", time.Now().Unix())
	if err := saveJSON(queryResult, outputFile); err != nil {
		logger.Error("Failed to export query results", "error", err)
		os.Exit(1)
	}

	logger.Info("Intelligent query completed",
		"query", queryText,
		"total_results", intelligentResult.Metadata.TotalResults,
		"consolidated_insights", intelligentResult.Metadata.ConsolidatedInsightCount,
		"cited_evidence", intelligentResult.Metadata.CitedEvidenceCount,
		"additional_context", intelligentResult.Metadata.AdditionalContextCount,
		"output", outputFile)

	// Print top results for immediate feedback
	fmt.Printf("\n🧠 Intelligent Query Results for: \"%s\"\n", queryText)
	fmt.Printf("📊 Total: %d | 🔗 Insights: %d | 🔗 Evidence: %d | 📄 Context: %d\n\n",
		intelligentResult.Metadata.TotalResults,
		intelligentResult.Metadata.ConsolidatedInsightCount,
		intelligentResult.Metadata.CitedEvidenceCount,
		intelligentResult.Metadata.AdditionalContextCount)

	if len(intelligentResult.ConsolidatedInsights) > 0 {
		fmt.Println("🔗 Top Consolidated Insights:")
		for i, fact := range intelligentResult.ConsolidatedInsights {
			if i >= 3 {
				break
			} // Show top 3
			fmt.Printf("  %d. %s\n", i+1, fact.Content)
		}
		fmt.Println()
	}

	if len(intelligentResult.CitedEvidence) > 0 {
		fmt.Println("📋 Supporting Evidence:")
		for i, fact := range intelligentResult.CitedEvidence {
			if i >= 2 {
				break
			} // Show top 2
			fmt.Printf("  %d. %s\n", i+1, fact.Content)
		}
		fmt.Println()
	}

	if len(intelligentResult.AdditionalContext) > 0 {
		fmt.Println("📄 Additional Context:")
		for i, fact := range intelligentResult.AdditionalContext {
			if i >= 2 {
				break
			} // Show top 2
			fmt.Printf("  %d. %s\n", i+1, fact.Content)
		}
		fmt.Println()
	}

	fmt.Printf("💾 Full results saved to: %s\n", outputFile)
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
	fmt.Println("  memory-processor-test store-consolidations")
	fmt.Println("  memory-processor-test query-consolidations")
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
	fmt.Println("  make consolidation # Weaviate → X_3 (all 20 subjects)")
	fmt.Println("  make store-consolidations # Weaviate → X_3 (all 20 subjects)")
	fmt.Println("  make query-consolidations # Query consolidation")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  memory-processor-test consolidation  # Comprehensive consolidation")
	fmt.Println("  make consolidation                   # Same as above")
}
