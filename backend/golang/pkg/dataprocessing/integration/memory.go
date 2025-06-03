package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// go run cmd/integration-test/main.go.
type IntegrationTestMemoryConfig struct {
	Source            string
	InputPath         string
	OutputPath        string
	CompletionsModel  string
	CompletionsApiKey string
	CompletionsApiUrl string
	EmbeddingsModel   string
	EmbeddingsApiKey  string
	EmbeddingsApiUrl  string
}

func IntegrationTestMemory(parentCtx context.Context, config IntegrationTestMemoryConfig) error {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	batchSize := 20

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
		Prefix:          "[integration-test] ",
	})

	tempDir, err := os.MkdirTemp("", "integration-test-memory-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Error("Failed to clean up temp directory", "error", err)
			fmt.Printf("Warning: failed to clean up temp directory %s: %v\n", tempDir, err)
		}
	}()

	storePath := filepath.Join(tempDir, "test.db")
	weaviateDataPath := filepath.Join(tempDir, "weaviate-data")

	weaviatePort := "51414"

	weaviateServer, err := bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviateDataPath)
	if err != nil {
		logger.Error("Error starting weaviate server", "error", err)
		return err
	}
	defer func() {
		logger.Info("Waiting for background operations to complete before cleanup...")
		time.Sleep(3 * time.Second)

		if weaviateServer != nil {
			if err := weaviateServer.Shutdown(); err != nil {
				logger.Error("Failed to shutdown Weaviate server", "error", err)
			} else {
				logger.Info("Weaviate server shutdown successfully")
			}
		}
	}()

	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Error creating weaviate client", "error", err)
		return err
	}

	openAiService := ai.NewOpenAIService(logger, config.CompletionsApiKey, config.CompletionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(logger, config.EmbeddingsApiKey, config.EmbeddingsApiUrl)

	schemaInitStart := time.Now()
	if err := bootstrap.InitSchema(weaviateClient, logger, aiEmbeddingsService); err != nil {
		logger.Error("Failed to initialize Weaviate schema", "error", err)
		return fmt.Errorf("failed to initialize Weaviate schema: %w", err)
	}
	logger.Info("Weaviate schema initialized", "elapsed", time.Since(schemaInitStart))

	store, err := db.NewStore(ctx, storePath)
	if err != nil {
		logger.Error("Error creating store", "error", err)
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		} else {
			logger.Info("Store closed successfully")
		}
	}()

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService, config.CompletionsModel, store)

	_, err = dataprocessingService.ProcessSource(ctx, config.Source, config.InputPath, config.OutputPath)
	if err != nil {
		logger.Error("Error processing source", "source", config.Source)
		return err
	}

	records, err := helpers.ReadJSONL[types.Record](config.OutputPath)
	if err != nil {
		logger.Error("Error loading records", "error", err)
		return err
	}

	documents, err := dataprocessingService.ToDocuments(ctx, config.Source, records)
	if err != nil {
		logger.Error("Error processing source", "source", config.Source)
		return err
	}

	storageInterface := storage.New(weaviateClient, logger, aiEmbeddingsService)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: openAiService,
		EmbeddingsService:  aiEmbeddingsService,
	})
	if err != nil {
		logger.Error("Error creating memory", "error", err)
		return err
	}

	if len(documents) == 0 {
		logger.Error("No documents to store", "source", config.Source)
		return fmt.Errorf("no documents to store")
	}
	logger.Info("Storing documents", "source", config.Source, "count", len(documents))

	for i := 0; i < len(documents); i += batchSize {
		if err := ctx.Err(); err != nil {
			logger.Info("Context canceled during document storage", "error", err)
			return err
		}

		batch := documents[i:min(i+batchSize, len(documents))]

		logger.Info("Storing documents batch", "index", i, "batch_size", len(batch))

		err = mem.Store(ctx, batch, nil)
		if err != nil {
			logger.Error("Error storing documents", "error", err)
			return err
		}
	}

	logger.Info("Waiting for memory processing to complete...")
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		logger.Info("Context canceled while waiting for processing to complete")
		return ctx.Err()
	}

	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source))
	if err != nil {
		logger.Error("Error querying memory", "error", err)
		return err
	}

	logger.Info("Query result", "result", result)

	logger.Info("Waiting for all background fact processing to complete...")
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		logger.Info("Context canceled during final wait")
		return ctx.Err()
	}

	logger.Info("Integration test completed successfully")

	return nil
}
