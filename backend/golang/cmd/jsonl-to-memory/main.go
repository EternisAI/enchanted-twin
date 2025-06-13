package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
)

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

func main() {
	// Define command-line flags
	inputFile := flag.String("input", "", "Path to JSONL file (required)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	dryRun := flag.Bool("dry-run", false, "Preview without storing to memory")
	help := flag.Bool("help", false, "Show help message")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "JSONL to Memory Ingestion Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This tool reads WhatsApp conversations from a JSONL file and ingests them into the memory system.\n\n")
		fmt.Fprintf(os.Stderr, "Configuration is loaded from .env file.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -input conversations.jsonl\n", os.Args[0])
	}

	flag.Parse()

	// Show help if requested
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate required flags
	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -input flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Set up logger
	logger := log.New(os.Stdout)
	if *verbose {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	// Load configuration from env file
	logger.Info("Loading configuration...")
	envs, err := config.LoadConfig(false)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Validate input file exists
	if _, err := os.Stat(*inputFile); os.IsNotExist(err) {
		logger.Error("Input file does not exist", "file", *inputFile)
		os.Exit(1)
	}

	logger.Info("Starting JSONL to memory ingestion",
		"input", *inputFile,
		"weaviate", fmt.Sprintf("localhost:%s", envs.WeaviatePort),
		"dry-run", *dryRun)

	// Load records from JSONL file
	logger.Info("Loading records from JSONL file...")
	records, err := LoadRecords(*inputFile)
	if err != nil {
		logger.Error("Failed to load records", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully loaded records", "count", len(records))

	// Convert records to ConversationDocuments using WhatsApp processor
	logger.Info("Converting records to ConversationDocuments...")
	processor := whatsapp.NewWhatsappProcessor(nil, logger)

	ctx := context.Background()
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		logger.Error("Failed to convert to documents", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully converted to documents", "count", len(documents))

	// Display sample documents if verbose or dry-run
	if *verbose || *dryRun {
		logger.Info("Sample documents:")
		for i, doc := range documents {
			if i >= 3 { // Show first 3
				break
			}

			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				logger.Info("Document",
					"index", i+1,
					"id", convDoc.ID(),
					"source", convDoc.Source(),
					"people", convDoc.People,
					"messages", len(convDoc.Conversation))

				// Show first message
				if len(convDoc.Conversation) > 0 {
					msg := convDoc.Conversation[0]
					content := msg.Content
					if len(content) > 80 {
						content = content[:77] + "..."
					}
					logger.Info("  First message",
						"speaker", msg.Speaker,
						"content", content)
				}
			}
		}
	}

	// Skip memory storage in dry-run mode
	if *dryRun {
		logger.Info("Dry-run mode: Skipping memory storage")
		os.Exit(0)
	}

	// Initialize Weaviate client
	logger.Info("Connecting to Weaviate...")
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", envs.WeaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Failed to create Weaviate client", "error", err)
		os.Exit(1)
	}

	// Test Weaviate connection
	if ready, err := weaviateClient.Misc().ReadyChecker().Do(ctx); err != nil || !ready {
		logger.Error("Weaviate is not ready", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully connected to Weaviate")

	// Initialize AI services
	logger.Info("Initializing AI services...")

	var aiCompletionsService *ai.Service
	if envs.ProxyTeeURL != "" {
		tokenFunc := func() string { return "12345" }
		aiCompletionsService = ai.NewOpenAIServiceProxy(logger, envs.ProxyTeeURL, tokenFunc, envs.CompletionsAPIURL)
	} else {
		aiCompletionsService = ai.NewOpenAIService(logger, envs.CompletionsAPIKey, envs.CompletionsAPIURL)
	}

	var aiEmbeddingsService *ai.Service
	if envs.ProxyTeeURL != "" {
		tokenFunc := func() string { return "12345" }
		aiEmbeddingsService = ai.NewOpenAIServiceProxy(logger, envs.ProxyTeeURL, tokenFunc, envs.EmbeddingsAPIURL)
	} else {
		aiEmbeddingsService = ai.NewOpenAIService(logger, envs.EmbeddingsAPIKey, envs.EmbeddingsAPIURL)
	}

	weaviateStorage, err := storage.New(weaviateClient, logger, aiEmbeddingsService, envs.EmbeddingsModel)
	if err != nil {
		logger.Fatal("Failed to create weaviate storage", "error", err)
	}

	memoryDeps := evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            weaviateStorage,
		CompletionsService: aiCompletionsService,
		EmbeddingsService:  aiEmbeddingsService,
		CompletionsModel:   envs.CompletionsModel,
		EmbeddingsModel:    envs.EmbeddingsModel,
	}

	memoryStorage, err := evolvingmemory.New(memoryDeps)
	if err != nil {
		logger.Error("Failed to create memory storage", "error", err)
		os.Exit(1)
	}

	// Progress callback
	startTime := time.Now()
	progressCallback := func(processed, total int) {
		elapsed := time.Since(startTime)
		rate := float64(processed) / elapsed.Seconds()
		remaining := 0
		if rate > 0 && processed > 0 {
			remaining = int(float64(total-processed) / rate)
		}

		logger.Info("Progress",
			"processed", processed,
			"total", total,
			"percentage", fmt.Sprintf("%.1f%%", float64(processed)/float64(total)*100),
			"rate", fmt.Sprintf("%.1f docs/sec", rate),
			"elapsed", elapsed.Round(time.Second),
			"remaining", time.Duration(remaining)*time.Second)
	}

	// Store documents in memory
	logger.Info("Starting memory ingestion...", "documents", len(documents))

	for i := range documents {
		if convDoc, ok := documents[i].(*memory.ConversationDocument); ok {
			logger.Info("Storing document", "id", i, "length", len(convDoc.Conversation))
		}
	}

	for i, doc := range documents {
		logger.Info("Storing document", "id", i, "source", doc.Source())
		// if i != 36 {
		// 	continue
		// }
		logger.Info("Storing document", "id", doc.ID(), "source", doc.Source())
		err = memoryStorage.Store(ctx, []memory.Document{doc}, progressCallback)
		if err != nil {
			logger.Error("Failed to store documents", "error", err)
			os.Exit(1)
		}
	}

	// Calculate final statistics
	totalTime := time.Since(startTime)
	docsPerSecond := float64(len(documents)) / totalTime.Seconds()

	logger.Info("Memory ingestion completed successfully!",
		"documents", len(documents),
		"total_time", totalTime.Round(time.Second),
		"avg_rate", fmt.Sprintf("%.2f docs/sec", docsPerSecond))

	// Print conversation statistics
	totalMessages := 0
	uniquePeople := make(map[string]bool)

	for _, doc := range documents {
		if convDoc, ok := doc.(*memory.ConversationDocument); ok {
			totalMessages += len(convDoc.Conversation)
			for _, person := range convDoc.People {
				uniquePeople[person] = true
			}
		}
	}

	logger.Info("Ingestion statistics:",
		"total_conversations", len(documents),
		"total_messages", totalMessages,
		"unique_participants", len(uniquePeople),
		"avg_messages_per_conversation", fmt.Sprintf("%.1f", float64(totalMessages)/float64(len(documents))))
}
