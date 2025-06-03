package integration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
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

func IntegrationTestMemory(config IntegrationTestMemoryConfig) error {
	storePath := "./output/test.db"
	weaviatePort := "8080"
	batchSize := 20
	ctx := context.Background()

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	_, err := bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, "weaviate-test-memory")
	if err != nil {
		logger.Error("Error starting weaviate server", "error", err)
		return err
	}

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
		panic(errors.Wrap(err, "Failed to initialize Weaviate schema"))
	}
	logger.Info("Weaviate schema initialized", "elapsed", time.Since(schemaInitStart))

	store, err := db.NewStore(ctx, storePath)
	if err != nil {
		logger.Error("Error creating store", "error", err)
		return err
	}

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

	// Create storage interface first
	storageInterface := storage.New(weaviateClient, logger, aiEmbeddingsService)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: openAiService,
		EmbeddingsService:  aiEmbeddingsService,
	})
	if err != nil {
		logger.Error("Error processing memory", "error", err)
		return err
	}

	if len(documents) == 0 {
		logger.Error("No documents to store", "source", config.Source)
		return fmt.Errorf("no documents to store")
	}
	logger.Info("Storing documents", "source", config.Source, "documents[0]", documents[0])

	for i := 0; i < len(documents); i += batchSize {
		batch := documents[i:min(i+batchSize, len(documents))]

		logger.Info("Storing documents batch", "index", i)

		err = mem.Store(ctx, batch, nil)
		if err != nil {
			logger.Error("Error storing documents", "error", err)
			return err
		}
	}

	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source))
	if err != nil {
		logger.Error("Error querying memory", "error", err)
		return err
	}

	logger.Info("Query result", "result", result)

	return nil
}
