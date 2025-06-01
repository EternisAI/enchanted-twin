package integration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// go run cmd/integration-test/main.go.
type IntegrationTestConfig struct {
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

func IntegrationTest(config IntegrationTestConfig) error {
	storePath := "./output/test.db"
	weaviatePort := "8080"
	ctx := context.Background()

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	_, err := bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, "weaviate")
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

	schemaInitStart := time.Now()
	if err := bootstrap.InitSchema(weaviateClient, logger); err != nil {
		logger.Error("Failed to initialize Weaviate schema", "error", err)
		panic(errors.Wrap(err, "Failed to initialize Weaviate schema"))
	}
	logger.Info("Weaviate schema initialized", "elapsed", time.Since(schemaInitStart))

	openAiService := ai.NewOpenAIService(logger, config.CompletionsApiKey, config.CompletionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(logger, config.EmbeddingsApiKey, config.EmbeddingsApiUrl)

	fmt.Println("aiEmbeddingsService  ", aiEmbeddingsService)
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

	var documents []memory.Document
	switch config.Source {
	case "telegram":
		telegramProcessor := telegram.NewTelegramProcessor()
		documents, err = telegramProcessor.ToDocuments(records)
		if err != nil {
			logger.Error("Error processing telegram", "error", err)
			return err
		}
	case "chatgpt":
		chatgptProcessor := chatgpt.NewChatGPTProcessor(config.InputPath)
		documents, err = chatgptProcessor.ToDocuments(records)
		if err != nil {
			logger.Error("Error processing chatgpt", "error", err)
			return err
		}
	default:
		return fmt.Errorf("unsupported source type: %s", config.Source)
	}

	fmt.Println("===================")
	fmt.Println("documents ", documents[0].Content())

	return nil

	mem, err := evolvingmemory.New(logger, weaviateClient, openAiService, aiEmbeddingsService)
	if err != nil {
		logger.Error("Error processing telegram", "error", err)
		return err
	}

	err = mem.Store(ctx, documents, nil)
	if err != nil {
		logger.Error("Error storing documents", "error", err)
		return err
	}

	fmt.Println(records)

	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source))
	if err != nil {
		logger.Error("Error querying memory", "error", err)
		return err
	}

	fmt.Println(result)

	return nil
}
