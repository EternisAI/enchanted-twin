package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/graphmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
)

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})
	// Define command line flags
	recreateSchema := flag.Bool("recreate", false, "Recreate the database schema")
	apiKey := flag.String("api-key", "", "OpenAI API key (optional, will use env var if not provided)")
	testDoc := flag.String("doc", "", "Test document to store and process (optional)")
	flag.Parse()

	// Load config to get OpenAI API key if not provided via flag
	envs, err := config.LoadConfig(true)
	if err != nil {
		logger.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// Use API key from command line if provided, otherwise use from config
	openaiKey := *apiKey
	if openaiKey == "" {
		openaiKey = envs.OpenAIAPIKey
		if openaiKey == "" {
			logger.Error("OpenAI API key not provided and not found in environment")
			os.Exit(1)
		}
	}

	// Start PostgreSQL
	postgresCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Info("Starting PostgreSQL service")
	postgresService, err := bootstrapPostgres(postgresCtx, logger)
	if err != nil {
		logger.Error("Failed to start PostgreSQL", "error", err)
		os.Exit(1)
	}

	// Set up cleanup on shutdown
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := postgresService.Stop(shutdownCtx); err != nil {
			logger.Error("Error stopping PostgreSQL", "error", err)
		}
		if err := postgresService.Remove(shutdownCtx); err != nil {
			logger.Error("Error removing PostgreSQL container", "error", err)
		}
	}()

	// Wait for PostgreSQL to be ready
	if err := postgresService.WaitForReady(postgresCtx, 60*time.Second); err != nil {
		logger.Error("Failed waiting for PostgreSQL to be ready", slog.Any("error", err))
		os.Exit(1)
	}

	// Ensure database exists
	dbName := "enchanted_twin"
	if err := postgresService.EnsureDatabase(postgresCtx, dbName); err != nil {
		logger.Error("Failed to ensure database exists", slog.Any("error", err))
		os.Exit(1)
	}

	connString := postgresService.GetConnectionString(dbName)
	logger.Info("PostgreSQL ready", slog.String("connection", connString))

	// Initialize OpenAI service
	aiService := ai.NewOpenAIService(openaiKey, envs.OpenAIBaseURL)
	logger.Info("AI service initialized")

	// Initialize graph memory
	logger.Info("Initializing graph memory", slog.Bool("recreate_schema", *recreateSchema))
	graphMem, err := graphmemory.NewGraphMemory(logger, connString, aiService, *recreateSchema, envs.CompletionsModel)
	if err != nil {
		logger.Error("Failed to initialize graph memory", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("Graph memory initialized")

	// Define test document
	testDocument := *testDoc
	if testDocument == "" {
		testDocument = "The Enchanted Twin is an AI assistant that helps users with their tasks. " +
			"It uses a graph memory system to store and retrieve information. " +
			"The system is built with Go and PostgreSQL. " +
			"It supports hierarchical tagging with ltree."
	}

	// Store document
	docs := []memory.TextDocument{
		{
			ID:      "test-doc-1",
			Content: testDocument,
			Tags:    []string{"test", "ai", "memory"},
			Timestamp: func() *time.Time {
				t := time.Now()
				return &t
			}(),
		},
	}

	logger.Info("Storing test document", slog.String("content", docs[0].Content))
	err = graphMem.Store(context.Background(), docs)
	if err != nil {
		logger.Error("Failed to store document", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("Document stored successfully")

	// Query for test document
	logger.Info("Querying for test document")
	result, err := graphMem.Query(context.Background(), "Enchanted Twin")
	if err != nil {
		logger.Error("Failed to query memory", slog.Any("error", err))
		os.Exit(1)
	}

	// Display results
	logger.Info(fmt.Sprintf("Found %d documents for query", len(result.Documents)))
	for i, doc := range result.Documents {
		logger.Info(fmt.Sprintf("Result %d:", i+1),
			slog.String("id", doc.ID),
			slog.String("content", doc.Content),
			slog.Any("tags", doc.Tags))
	}
	logger.Info("Text", "text", result.Text)

	logger.Info("Memory test completed successfully")
}

// bootstrapPostgres initializes and starts a PostgreSQL service
func bootstrapPostgres(ctx context.Context, logger *log.Logger) (*bootstrap.PostgresService, error) {
	// Get default options
	options := bootstrap.DefaultPostgresOptions()

	// Create and start PostgreSQL service
	postgresService, err := bootstrap.NewPostgresService(logger, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL service: %w", err)
	}

	logger.Info("Starting PostgreSQL service...")
	err = postgresService.Start(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL service: %w", err)
	}

	return postgresService, nil
}
