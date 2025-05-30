package integration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// go run cmd/integration-test/main.go.
func IntegrationTest() error {
	source := "telegram" // see pkg dataprocessing for supported sources
	inputPath := "data_input/telegram_export.json.zip"

	id := uuid.New().String()
	outputPath := fmt.Sprintf(
		"%s/%s_%s.jsonl",
		"./output",
		source,
		id,
	)

	completionsModel := "gpt-4o-mini"
	completionsApiUrl := "https://openrouter.ai/api/v1"
	// embeddingModel := "text-embedding-3-small"

	embeddingApiKey := "<your-openai-api-key>"
	completionsApiKey := "<your-openai-api-key>"

	storePath := "./output/test.db"

	weaviatePort := "8080"
	ctx := context.Background()

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, "weaviate")

	openAiService := ai.NewOpenAIService(logger, completionsApiKey, completionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(logger, embeddingApiKey, "https://api.openai.com/v1")

	fmt.Println("aiEmbeddingsService  ", aiEmbeddingsService)
	store, err := db.NewStore(ctx, storePath)
	if err != nil {
		logger.Error("Error creating store", "error", err)
		return err
	}

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService, completionsModel, store)

	_, err = dataprocessingService.ProcessSource(ctx, source, inputPath, outputPath)
	if err != nil {
		logger.Error("Error processing source", "source", source)
		return err
	}

	records, err := helpers.ReadJSONL[types.Record](outputPath)
	if err != nil {
		logger.Error("Error loading records", "error", err)
		return err
	}

	telegramProcessor := telegram.NewTelegramProcessor()
	documents, err := telegramProcessor.ToDocuments(records)
	if err != nil {
		logger.Error("Error processing telegram", "error", err)
	}

	fmt.Println("documents ", documents[0:10])

	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
	})

	mem, err := evolvingmemory.New(logger, weaviateClient, openAiService, aiEmbeddingsService)

	err = mem.Store(ctx, documents, nil)
	if err != nil {
		logger.Error("Error storing documents", "error", err)
		return err
	}

	fmt.Println(records)

	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", source))
	if err != nil {
		logger.Error("Error querying memory", "error", err)
		return err
	}

	fmt.Println(result)

	return nil
}
