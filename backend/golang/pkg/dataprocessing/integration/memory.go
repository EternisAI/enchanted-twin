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
		}
	}()

	storePath := filepath.Join(tempDir, "test.db")
	weaviateDataPath := filepath.Join(tempDir, "weaviate-data")

	weaviatePort := "51414"

	weaviateServer, err := bootstrap.BootstrapWeaviateServer(ctx, logger, weaviatePort, weaviateDataPath)
	if err != nil {
		return fmt.Errorf("failed to start weaviate server: %w", err)
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
		return fmt.Errorf("failed to create weaviate client: %w", err)
	}

	openAiService := ai.NewOpenAIService(logger, config.CompletionsApiKey, config.CompletionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(logger, config.EmbeddingsApiKey, config.EmbeddingsApiUrl)

	schemaInitStart := time.Now()
	if err := bootstrap.InitSchema(weaviateClient, logger, aiEmbeddingsService); err != nil {
		return fmt.Errorf("failed to initialize Weaviate schema: %w", err)
	}
	logger.Info("Weaviate schema initialized", "elapsed", time.Since(schemaInitStart))

	store, err := db.NewStore(ctx, storePath)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close store", "error", err)
		} else {
			logger.Info("Store closed successfully")
		}
	}()

	completionsModel := config.CompletionsModel
	if completionsModel == "" {
		completionsModel = "gpt-4o-mini"
	}

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService, completionsModel, store)

	_, err = dataprocessingService.ProcessSource(ctx, config.Source, config.InputPath, config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to process source: %w", err)
	}

	records, err := helpers.ReadJSONL[types.Record](config.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to load records: %w", err)
	}

	documents, err := dataprocessingService.ToDocuments(ctx, config.Source, records)
	if err != nil {
		return fmt.Errorf("failed to process source: %w", err)
	}

	logger.Info("==============✅ Data processed into %d documents===============", len(documents))

	storageInterface := storage.New(weaviateClient, logger, aiEmbeddingsService)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: openAiService,
		EmbeddingsService:  aiEmbeddingsService,
	})
	if err != nil {
		return fmt.Errorf("failed to create memory: %w", err)
	}

	if len(documents) == 0 {
		return fmt.Errorf("no documents to store")
	}
	logger.Info("...Storing documents", "source", config.Source, "count", len(documents))

	for i := 0; i < len(documents); i += batchSize {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context canceled during document storage: %w", err)
		}

		batch := documents[i:min(i+batchSize, len(documents))]

		logger.Info("Storing documents batch", "index", i, "batch_size", len(batch))

		for j, doc := range batch {
			logger.Info("Document being stored",
				"batch_index", i,
				"doc_index", j,
				"doc_id", doc.ID(),
				"content_length", len(doc.Content()),
				"content_preview", func() string {
					content := doc.Content()
					if len(content) > 100 {
						return content[:100] + "..."
					}
					return content
				}())
		}

		err = mem.Store(ctx, batch, nil)
		if err != nil {
			return fmt.Errorf("failed to store documents: %w", err)
		}
	}

	logger.Info("Waiting for memory processing to complete...")
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for processing to complete: %w", ctx.Err())
	}
	logger.Info("==============✅ Memories stored===============")

	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source))
	if err != nil {
		return fmt.Errorf("failed to query memory: %w", err)
	}

	if len(result.Documents) > 0 {
		for _, doc := range result.Documents[:min(3, len(result.Documents))] {
			memoryID := doc.ID()

			docRefs, err := mem.GetDocumentReferences(ctx, memoryID)
			if err != nil {
				return fmt.Errorf("failed to get document reference: %w", err)
			}

			if len(docRefs) == 0 {
				return fmt.Errorf("no document references found for memory %s", memoryID)
			}

			docRef := docRefs[0]

			if docRef.ID == "" {
				return fmt.Errorf("document reference has empty ID for memory %s and type %s", memoryID, docRef.Type)
			}

			if docRef.Content == "" {
				return fmt.Errorf("document reference has empty content for id %s and type %s", docRef.ID, docRef.Type)
			}

			if docRef.Type == "" {
				return fmt.Errorf("document reference has empty type %s for id %s and content %s", docRef.Type, docRef.ID, docRef.Content)
			}

			if docRef.Content != "" {
				contentSnippet := docRef.Content
				if len(contentSnippet) > 100 {
					contentSnippet = contentSnippet[:100] + "..."
				}
				logger.Info("Original document snippet", "snippet", contentSnippet)
			} else {
				return fmt.Errorf("no content available for this document reference (old format)")
			}
		}

		logger.Info("==============✅ Document references found===============")
	} else {
		return fmt.Errorf("no memories found in query result - skipping document reference test")
	}

	logger.Info("Waiting for all background fact processing to complete...")
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return fmt.Errorf("context canceled during final wait: %w", ctx.Err())
	}

	logger.Info("==============🟢 Integration test completed successfully===============")

	logger.Info("Waiting for all background fact processing to complete...")
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return fmt.Errorf("context canceled during final wait: %w", ctx.Err())
	}

	return nil
}
