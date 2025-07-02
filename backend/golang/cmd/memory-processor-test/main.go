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
	// Check for explicit file argument first (e.g., from Makefile FILE parameter)
	var inputFile string
	if len(os.Args) > 2 {
		// Second argument is the file path (e.g., "go run . telegram /path/to/file.json")
		candidateFile := os.Args[2]
		if candidateFile != "--senders" { // Ignore special flags
			if _, err := os.Stat(candidateFile); err == nil {
				inputFile = candidateFile
				logger.Info("Using explicit file argument", "file", inputFile)
			}
		}
	}

	// Fall back to pattern matching if no explicit file provided
	if inputFile == "" {
		for _, pattern := range filePatterns {
			if file := findInputFile(pattern); file != "" {
				inputFile = file
				break
			}
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

	// Save output using memory package helper
	if err := memory.ExportConversationDocumentsJSON(documents, outputFile); err != nil {
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
	if f := findInputFile("pipeline_output/X_0_whatsapp.jsonl"); f != "" {
		return f
	}
	if f := findInputFile("pipeline_output/X_0_telegram.jsonl"); f != "" {
		return f
	}
	if f := findInputFile("pipeline_output/X_0_gmail.jsonl"); f != "" {
		return f
	}
	return findInputFile("pipeline_output/X_0_chatgpt.jsonl")
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

func exportConsolidationReportsJSONL(reports []*evolvingmemory.ConsolidationReport, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("Failed to close file", "error", closeErr, "filename", filename)
		}
	}()

	encoder := json.NewEncoder(file)
	for _, report := range reports {
		if err := encoder.Encode(report); err != nil {
			return err
		}
	}

	return nil
}

func loadConsolidationReportsJSONL(filename string) ([]*evolvingmemory.ConsolidationReport, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("Failed to close file", "error", closeErr, "filename", filename)
		}
	}()

	var reports []*evolvingmemory.ConsolidationReport
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var report evolvingmemory.ConsolidationReport
		if err := decoder.Decode(&report); err != nil {
			return nil, err
		}
		reports = append(reports, &report)
	}

	return reports, nil
}

// Data source processors using polymorphic function.
func runWhatsApp() {
	runDataProcessor(
		"WhatsApp",
		[]string{"pipeline_input/*.sqlite", "pipeline_input/*.db"},
		"pipeline_output/X_0_whatsapp.jsonl",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return whatsapp.NewWhatsappProcessor(store, logger)
		},
	)
}

func runTelegram() {
	runDataProcessor(
		"Telegram",
		[]string{"pipeline_input/*.json"},
		"pipeline_output/X_0_telegram.jsonl",
		func(store *db.Store) (dataprocessing.DocumentProcessor, error) {
			return telegram.NewTelegramProcessor(store, logger)
		},
	)
}

func runChatGPT() {
	runDataProcessor(
		"ChatGPT",
		[]string{"pipeline_input/*.json", "pipeline_input/conversations.json"},
		"pipeline_output/X_0_chatgpt.jsonl",
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
		"pipeline_output/X_0_gmail.jsonl",
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

	var chunkedDocs []memory.ConversationDocument
	for _, doc := range documents {
		docCopy := doc
		chunks := docCopy.Chunk()
		for _, chunk := range chunks {
			if convDoc, ok := chunk.(*memory.ConversationDocument); ok {
				chunkedDocs = append(chunkedDocs, *convDoc)
			}
		}
	}

	// Save chunked documents as JSONL using memory package helper
	if err := memory.ExportConversationDocumentsJSON(chunkedDocs, "pipeline_output/X_1_chunked_documents.jsonl"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Chunking done", "original", len(documents), "chunks", len(chunkedDocs))
}

// 🔥 PARALLEL FACT EXTRACTION WORKER POOL.
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

func runFacts() {
	if os.Getenv("COMPLETIONS_API_KEY") == "" {
		logger.Error("COMPLETIONS_API_KEY required for fact extraction")
		os.Exit(1)
	}

	logger.Info("Extracting facts")

	// Load chunked documents directly from JSONL using memory package helper
	conversationDocs, err := memory.LoadConversationDocumentsFromJSON("pipeline_output/X_1_chunked_documents.jsonl")
	if err != nil {
		logger.Error("Load failed", "error", err)
		os.Exit(1)
	}

	// Convert to Document interface
	var documents []memory.Document
	for i := range conversationDocs {
		documents = append(documents, &conversationDocs[i])
	}

	logger.Info("Starting parallel fact extraction", "documents", len(documents))

	// 🚀 PARALLEL FACT EXTRACTION WITH WORKER POOL
	numWorkers := 100 // YOLO mode - maximize those OpenAI credits! 💸
	allFacts := extractFactsParallel(documents, numWorkers)

	// Convert from []*MemoryFact to []MemoryFact for the helper
	facts := make([]memory.MemoryFact, len(allFacts))
	for i, fact := range allFacts {
		facts[i] = *fact
	}

	// Save facts as JSONL using memory package helper
	if err := memory.ExportMemoryFactsJSON(facts, "pipeline_output/X_2_extracted_facts.jsonl"); err != nil {
		logger.Error("Save failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Fact extraction done", "documents", len(documents), "facts", len(facts))
}

func runStore() {
	logger.Info("Storing facts using production storage module")

	// Find X_2 facts file
	inputFile := findInputFile("pipeline_output/X_2_*.jsonl")
	if inputFile == "" {
		logger.Error("No X_2 facts JSONL file found")
		os.Exit(1)
	}

	logger.Info("Loading facts", "file", inputFile)

	// Load facts from JSONL using memory package helper
	facts, err := memory.LoadMemoryFactsFromJSON(inputFile)
	if err != nil {
		logger.Error("Load failed", "error", err)
		os.Exit(1)
	}

	if len(facts) == 0 {
		logger.Warn("No facts found to store")
		return
	}

	logger.Info("Found facts to store", "count", len(facts))

	// Set up Weaviate infrastructure with schema initialization
	infra, err := setupWeaviateMemoryInfrastructureWithSchema()
	if err != nil {
		logger.Error("Failed to setup Weaviate infrastructure", "error", err)
		os.Exit(1)
	}

	// Convert facts to pointers for the interface
	var factsPtr []*memory.MemoryFact
	for i := range facts {
		factsPtr = append(factsPtr, &facts[i])
	}

	// Use the PRODUCTION StoreFactsDirectly method! 🚀
	logger.Info("Storing facts using production storage module", "count", len(factsPtr))

	err = infra.MemoryStorage.StoreFactsDirectly(infra.Context, factsPtr, func(processed, total int) {
		logger.Info("Storage progress", "processed", processed, "total", total)
	})
	if err != nil {
		logger.Error("Failed to store facts", "error", err)
		os.Exit(1)
	}

	logger.Info("Facts stored successfully using PRODUCTION code", "count", len(factsPtr))
	logger.Info("Storage complete - ready for consolidation")
}

// 🔥 PARALLEL CONSOLIDATION WORKER POOL.
func consolidateSubjectsParallel(ctx context.Context, subjects []string, consolidationDeps evolvingmemory.ConsolidationDependencies, numWorkers int) []*evolvingmemory.ConsolidationReport {
	// Input channel for subjects
	subjectChan := make(chan string, len(subjects))

	// Results channel for consolidation reports
	type consolidationResult struct {
		report  *evolvingmemory.ConsolidationReport
		subject string
		err     error
	}
	resultChan := make(chan consolidationResult, len(subjects))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for subject := range subjectChan {
				start := time.Now()
				logger.Debug("Processing consolidation subject", "worker", workerID, "subject", subject)

				// Use semantic search with filtering for better results
				filter := &memory.Filter{
					Distance:          0.75,                                                  // Allow fairly broad semantic matches
					Limit:             func() *int { limit := 30; return &limit }(),          // Reasonable limit
					FactImportanceMin: func() *int { importance := 2; return &importance }(), // Only meaningful facts
				}

				report, err := evolvingmemory.ConsolidateMemoriesBySemantic(ctx, subject, filter, consolidationDeps)

				duration := time.Since(start)
				if err != nil {
					logger.Error("Consolidation failed",
						"worker", workerID,
						"subject", subject,
						"duration", duration,
						"error", err)
				} else {
					logger.Info("Subject consolidation completed",
						"worker", workerID,
						"subject", subject,
						"source_facts", report.SourceFactCount,
						"consolidated_facts", len(report.ConsolidatedFacts),
						"duration", duration)
				}

				resultChan <- consolidationResult{
					report:  report,
					subject: subject,
					err:     err,
				}
			}
		}(i)
	}

	// Send all subjects to workers
	for _, subject := range subjects {
		subjectChan <- subject
	}
	close(subjectChan)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results with progress tracking
	var allReports []*evolvingmemory.ConsolidationReport
	processed := 0
	totalSubjects := len(subjects)

	for result := range resultChan {
		processed++
		if result.err == nil {
			allReports = append(allReports, result.report)
		}

		// Progress logging every 5 subjects or at completion
		if processed%5 == 0 || processed == totalSubjects {
			logger.Info("Consolidation progress",
				"processed", processed,
				"total", totalSubjects,
				"successful", len(allReports),
				"percent", fmt.Sprintf("%.1f%%", float64(processed)/float64(totalSubjects)*100))
		}
	}

	return allReports
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

	logger.Info("Starting parallel comprehensive consolidation", "subjects", len(evolvingmemory.ConsolidationSubjects))

	// Run consolidation for all 20 canonical subjects in parallel
	numWorkers := 20 // One worker per subject for maximum parallelism
	allReports := consolidateSubjectsParallel(ctx, evolvingmemory.ConsolidationSubjects[:], consolidationDeps, numWorkers)

	// Export consolidation reports as JSONL (consistent with X_0, X_1, X_2 format)
	outputFile := "pipeline_output/X_3_consolidation_reports.jsonl"
	if err := exportConsolidationReportsJSONL(allReports, outputFile); err != nil {
		logger.Error("Failed to export consolidation reports", "error", err)
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

	// Check if consolidation reports exist (new JSONL format)
	consolidationFile := "pipeline_output/X_3_consolidation_reports.jsonl"
	if _, err := os.Stat(consolidationFile); os.IsNotExist(err) {
		logger.Error("Consolidation reports not found. Run 'make consolidation' first.", "file", consolidationFile)
		os.Exit(1)
	}

	// Load consolidation reports from JSONL format (consistent with pipeline)
	reports, err := loadConsolidationReportsJSONL(consolidationFile)
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

	// Calculate totals for logging
	totalSourceFacts := 0
	totalConsolidatedFacts := 0
	for _, report := range reports {
		totalSourceFacts += report.SourceFactCount
		totalConsolidatedFacts += len(report.ConsolidatedFacts)
	}

	logger.Info("Consolidation storage completed",
		"reports_processed", len(reports),
		"total_source_facts", totalSourceFacts,
		"total_consolidated_facts", totalConsolidatedFacts)
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
	outputFile := fmt.Sprintf("pipeline_output/X_4_query_results_%d.json", time.Now().Unix())
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
