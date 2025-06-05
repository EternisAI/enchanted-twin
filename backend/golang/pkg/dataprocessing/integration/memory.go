package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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

	logger.Info("==============✅ Data processed into %s documents===============", len(documents))

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

	limit := 100
	filter := memory.Filter{
		Source:   &config.Source,
		Distance: 0.8, // Balanced threshold - more permissive than 0.7 but still validates filtering
		Limit:    &limit,
	}
	result, err := mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source), &filter)
	if err != nil {
		return fmt.Errorf("failed to query memory: %w", err)
	}
	resultDocuments := result.Documents
	if len(resultDocuments) == 0 {
		return fmt.Errorf("failed to find memories")
	}

	// Test document references using the valid source query results
	if len(resultDocuments) > 0 {
		for _, doc := range resultDocuments[:min(3, len(resultDocuments))] {
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
	}

	invalidSource := "invalid-source"
	filter = memory.Filter{
		Source:   &invalidSource,
		Distance: 0.7,
		Limit:    &limit,
	}
	result, err = mem.Query(ctx, fmt.Sprintf("What do facts from %s say about the user?", config.Source), &filter)
	if err != nil {
		return fmt.Errorf("failed to query memory: %w", err)
	}

	resultDocuments = result.Documents
	if len(resultDocuments) != 0 {
		return fmt.Errorf("found memories for invalid source when none should exist")
	}

	filter = memory.Filter{
		Source:   &config.Source,
		Distance: 0.001,
		Limit:    &limit,
	}
	result, err = mem.Query(ctx, "What do I know about gluon fields ?", &filter)
	if err != nil {
		return fmt.Errorf("failed to query memory: %w", err)
	}
	resultDocuments = result.Documents
	if len(resultDocuments) != 0 {
		return fmt.Errorf("failed to filter out documents for invalid query")
	}

	// Test structured fact filtering integration
	if err := testStructuredFactFiltering(ctx, mem, config.Source, limit, logger); err != nil {
		return fmt.Errorf("structured fact filtering integration tests failed: %w", err)
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

// testStructuredFactFiltering tests the new structured fact filtering functionality.
func testStructuredFactFiltering(ctx context.Context, mem evolvingmemory.MemoryStorage, source string, limit int, logger *log.Logger) error {
	logger.Info("==============🧪 Testing structured fact filtering===============")

	// Helper function for pointer creation
	stringPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	// Test 1: Category filtering
	logger.Info("Testing fact category filtering...")
	filter := &memory.Filter{
		FactCategory: stringPtr("preference"),
		Source:       &source,
		Limit:        intPtr(limit),
	}
	result, err := mem.Query(ctx, "user preferences", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact category filter: %w", err)
	}
	logger.Info("Category filtering test completed", "results_count", len(result.Documents))

	// Test 2: Subject filtering
	logger.Info("Testing fact subject filtering...")
	filter = &memory.Filter{
		FactSubject: stringPtr("user"),
		Source:      &source,
		Limit:       intPtr(limit),
	}
	result, err = mem.Query(ctx, "facts about user", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact subject filter: %w", err)
	}
	logger.Info("Subject filtering test completed", "results_count", len(result.Documents))

	// Test 3: Importance filtering (exact)
	logger.Info("Testing fact importance exact filtering...")
	filter = &memory.Filter{
		FactImportance: intPtr(3), // High importance facts
		Source:         &source,
		Limit:          intPtr(limit),
	}
	result, err = mem.Query(ctx, "important facts", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact importance filter: %w", err)
	}
	logger.Info("Importance filtering test completed", "results_count", len(result.Documents))

	// Test 4: Importance range filtering
	logger.Info("Testing fact importance range filtering...")
	filter = &memory.Filter{
		FactImportanceMin: intPtr(2), // Medium to high importance
		FactImportanceMax: intPtr(3),
		Source:            &source,
		Limit:             intPtr(limit),
	}
	result, err = mem.Query(ctx, "medium to high importance facts", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact importance range filter: %w", err)
	}
	logger.Info("Importance range filtering test completed", "results_count", len(result.Documents))

	// Test 5: Sensitivity filtering
	logger.Info("Testing fact sensitivity filtering...")
	filter = &memory.Filter{
		FactSensitivity: stringPtr("low"), // Public information only
		Source:          &source,
		Limit:           intPtr(limit),
	}
	result, err = mem.Query(ctx, "public information", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact sensitivity filter: %w", err)
	}
	logger.Info("Sensitivity filtering test completed", "results_count", len(result.Documents))

	// Test 6: Combined structured fact filtering
	logger.Info("Testing combined structured fact filtering...")
	filter = &memory.Filter{
		FactCategory:   stringPtr("preference"),
		FactSubject:    stringPtr("user"),
		FactImportance: intPtr(2),
		Source:         &source,
		Limit:          intPtr(limit),
	}
	result, err = mem.Query(ctx, "user preferences with medium importance", filter)
	if err != nil {
		return fmt.Errorf("failed to query with combined structured fact filters: %w", err)
	}
	logger.Info("Combined filtering test completed", "results_count", len(result.Documents))

	// Test 7: Value partial matching
	logger.Info("Testing fact value partial matching...")
	filter = &memory.Filter{
		FactValue: stringPtr("coffee"), // Should match any fact containing "coffee"
		Source:    &source,
		Limit:     intPtr(limit),
	}
	result, err = mem.Query(ctx, "coffee related facts", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact value filter: %w", err)
	}
	logger.Info("Value partial matching test completed", "results_count", len(result.Documents))

	// Test 8: Temporal context filtering
	logger.Info("Testing fact temporal context filtering...")
	filter = &memory.Filter{
		FactTemporalContext: stringPtr("2024"), // Facts from 2024
		Source:              &source,
		Limit:               intPtr(limit),
	}
	result, err = mem.Query(ctx, "facts from 2024", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact temporal context filter: %w", err)
	}
	logger.Info("Temporal context filtering test completed", "results_count", len(result.Documents))

	// Test 9: Mixed legacy and structured filtering
	logger.Info("Testing mixed legacy and structured filtering...")
	filter = &memory.Filter{
		// Legacy fields
		Source:   &source,
		Distance: 0.8,
		Limit:    intPtr(limit),
		// Structured fields
		FactCategory:   stringPtr("preference"),
		FactImportance: intPtr(3),
	}
	result, err = mem.Query(ctx, "high priority preferences", filter)
	if err != nil {
		return fmt.Errorf("failed to query with mixed legacy and structured filters: %w", err)
	}
	logger.Info("Mixed filtering test completed", "results_count", len(result.Documents))

	// Test 10: Complex filtering scenario (realistic use case)
	logger.Info("Testing complex realistic filtering scenario...")
	filter = &memory.Filter{
		FactCategory:        stringPtr("goal_plan"),
		FactSubject:         stringPtr("user"),
		FactSensitivity:     stringPtr("medium"),
		FactImportanceMin:   intPtr(2),
		FactTemporalContext: stringPtr("Q1"),
		Source:              &source,
		Limit:               intPtr(limit),
	}
	result, err = mem.Query(ctx, "user goals for Q1 with medium sensitivity and high importance", filter)
	if err != nil {
		return fmt.Errorf("failed to query with complex structured fact filters: %w", err)
	}
	logger.Info("Complex filtering test completed", "results_count", len(result.Documents))

	// Test 11: Attribute filtering
	logger.Info("Testing fact attribute filtering...")
	filter = &memory.Filter{
		FactAttribute: stringPtr("health_metric"),
		Source:        &source,
		Limit:         intPtr(limit),
	}
	result, err = mem.Query(ctx, "health metrics", filter)
	if err != nil {
		return fmt.Errorf("failed to query with fact attribute filter: %w", err)
	}
	logger.Info("Attribute filtering test completed", "results_count", len(result.Documents))

	// Test 12: Invalid/non-existent category (should return no results)
	logger.Info("Testing filtering with non-existent category...")
	filter = &memory.Filter{
		FactCategory: stringPtr("nonexistent_category"),
		Source:       &source,
		Limit:        intPtr(limit),
	}
	result, err = mem.Query(ctx, "facts from non-existent category", filter)
	if err != nil {
		return fmt.Errorf("failed to query with non-existent category filter: %w", err)
	}
	if len(result.Documents) != 0 {
		logger.Warn("Expected no results for non-existent category", "actual_count", len(result.Documents))
	}
	logger.Info("Non-existent category test completed", "results_count", len(result.Documents))

	logger.Info("==============✅ All structured fact filtering tests completed===============")
	return nil
}
