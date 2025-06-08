package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/adapters/handlers/rest"

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

var (
	sharedWeaviateServer *rest.Server
	sharedWeaviateClient *weaviate.Client
	sharedLogger         *log.Logger
	sharedTempDir        string
	setupOnce            sync.Once
	teardownOnce         sync.Once
)

type testConfig struct {
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

type testEnvironment struct {
	config         testConfig
	logger         *log.Logger
	tempDir        string
	store          *db.Store
	memory         evolvingmemory.MemoryStorage
	dataprocessing *dataprocessing.DataProcessingService
	documents      []memory.Document
	ctx            context.Context
	cancel         context.CancelFunc
}

// deterministicAIService wraps ai.Service to use temperature 0.0 for deterministic testing.
type deterministicAIService struct {
	*ai.Service
}

// Completions overrides the default method to use temperature 0.0 for deterministic results.
func (d *deterministicAIService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	return d.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 0.0}, // Deterministic temperature
	})
}

// newDeterministicAIService creates a test AI service that uses temperature 0.0.
func newDeterministicAIService(logger *log.Logger, apiKey, baseURL string) *deterministicAIService {
	return &deterministicAIService{
		Service: ai.NewOpenAIService(logger, apiKey, baseURL),
	}
}

func setupSharedInfrastructure() {
	setupOnce.Do(func() {
		var err error

		sharedLogger = log.NewWithOptions(os.Stdout, log.Options{
			ReportCaller:    true,
			ReportTimestamp: true,
			Level:           log.DebugLevel,
			TimeFormat:      time.Kitchen,
			Prefix:          "[memory-test] ",
		})

		sharedTempDir, err = os.MkdirTemp("", "memory-integration-*")
		if err != nil {
			panic(fmt.Sprintf("failed to create temp directory: %v", err))
		}

		weaviateDataPath := filepath.Join(sharedTempDir, "weaviate-data")
		weaviatePort := "51414"

		ctx := context.Background()

		originalArgs := os.Args
		os.Args = []string{"test"}

		sharedWeaviateServer, err = bootstrap.BootstrapWeaviateServer(ctx, sharedLogger, weaviatePort, weaviateDataPath)
		if err != nil {
			panic(fmt.Sprintf("failed to start weaviate server: %v", err))
		}

		os.Args = originalArgs

		sharedWeaviateClient, err = weaviate.NewClient(weaviate.Config{
			Host:   fmt.Sprintf("localhost:%s", weaviatePort),
			Scheme: "http",
		})
		if err != nil {
			panic(fmt.Sprintf("failed to create weaviate client: %v", err))
		}

		aiEmbeddingsService := ai.NewOpenAIService(sharedLogger, os.Getenv("EMBEDDINGS_API_KEY"), "https://api.openai.com/v1")
		err = bootstrap.InitSchema(sharedWeaviateClient, sharedLogger, aiEmbeddingsService, "text-embedding-3-small")
		if err != nil {
			panic(fmt.Sprintf("failed to initialize schema: %v", err))
		}

		sharedLogger.Info("Shared Weaviate infrastructure initialized successfully")
	})
}

func teardownSharedInfrastructure() {
	teardownOnce.Do(func() {
		if sharedWeaviateServer != nil {
			sharedLogger.Info("Shutting down shared Weaviate server...")
			if err := sharedWeaviateServer.Shutdown(); err != nil {
				sharedLogger.Error("Failed to shutdown Weaviate server", "error", err)
			}
		}

		if sharedTempDir != "" {
			if err := os.RemoveAll(sharedTempDir); err != nil {
				sharedLogger.Error("Failed to clean up temp directory", "error", err)
			}
		}
	})
}

func clearWeaviateData(t *testing.T) {
	t.Helper()

	for _, className := range []string{"TextDocument", "SourceDocument"} {
		result, err := sharedWeaviateClient.Data().ObjectsGetter().
			WithClassName(className).
			WithLimit(1000).
			Do(context.Background())
		if err != nil {
			t.Logf("Warning: Failed to get %s objects: %v", className, err)
			continue
		}

		if len(result) > 0 {
			for _, obj := range result {
				if obj.ID != "" {
					err := sharedWeaviateClient.Data().Deleter().
						WithClassName(className).
						WithID(string(obj.ID)).
						Do(context.Background())
					if err != nil {
						t.Logf("Warning: Failed to delete object %s: %v", string(obj.ID), err)
					}
				}
			}
			t.Logf("Cleared %d objects from %s", len(result), className)
		}
	}
}

func setupTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	setupSharedInfrastructure()

	clearWeaviateData(t)

	config := getTestConfig(t)

	testTimeout := 60 * time.Minute
	if localTestTimeout := os.Getenv("LOCAL_MODEL_TEST_TIMEOUT"); localTestTimeout != "" {
		if duration, err := time.ParseDuration(localTestTimeout); err == nil {
			testTimeout = duration
			t.Logf("Using custom test timeout for local model: %v", duration)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

	tempDir, err := os.MkdirTemp("", "memory-test-*")
	require.NoError(t, err)

	storePath := filepath.Join(tempDir, "test.db")
	store, err := db.NewStore(ctx, storePath)
	require.NoError(t, err)

	completionsModel := config.CompletionsModel
	if completionsModel == "" {
		completionsModel = "gpt-4o-mini"
	}

	openAiService := newDeterministicAIService(sharedLogger, config.CompletionsApiKey, config.CompletionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(sharedLogger, config.EmbeddingsApiKey, config.EmbeddingsApiUrl)

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService.Service, completionsModel, store)

	storageInterface := storage.New(sharedWeaviateClient, sharedLogger, aiEmbeddingsService)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             sharedLogger,
		Storage:            storageInterface,
		CompletionsService: openAiService.Service,
		EmbeddingsService:  aiEmbeddingsService,
		CompletionsModel:   config.CompletionsModel,
		EmbeddingsModel:    config.EmbeddingsModel,
	})
	require.NoError(t, err)

	return &testEnvironment{
		config:         config,
		logger:         sharedLogger,
		tempDir:        tempDir,
		store:          store,
		memory:         mem,
		dataprocessing: dataprocessingService,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (env *testEnvironment) cleanup(t *testing.T) {
	t.Helper()

	env.cancel()

	if env.store != nil {
		if err := env.store.Close(); err != nil {
			t.Logf("Failed to close store: %v", err)
		}
	}

	if err := os.RemoveAll(env.tempDir); err != nil {
		t.Logf("Failed to clean up temp directory: %v", err)
	}
}

func (env *testEnvironment) loadDocuments(t *testing.T, source, inputPath string) {
	t.Helper()

	_, err := env.dataprocessing.ProcessSource(env.ctx, source, inputPath, env.config.OutputPath)
	require.NoError(t, err)

	count, err := helpers.CountJSONLLines(env.config.OutputPath)
	if err != nil {
		t.Fatalf("Failed to count JSONL lines: %v", err)
	}

	batchSize := 3
	var allRecords []types.Record

	env.logger.Info("Loading documents in batches", "totalRecords", count, "batchSize", batchSize)

	for batchIndex := 0; ; batchIndex++ {
		startIndex := batchIndex * batchSize
		records, err := helpers.ReadJSONLBatch(env.config.OutputPath, startIndex, batchSize)
		require.NoError(t, err)

		if len(records) == 0 {
			break
		}

		env.logger.Info("Loaded batch", "batchIndex", batchIndex, "recordCount", len(records), "startIndex", startIndex)
		allRecords = append(allRecords, records...)

		for i, record := range records {
			env.logger.Info("Record in batch",
				"batchIndex", batchIndex,
				"recordIndex", i,
				"globalIndex", startIndex+i,
				"source", record.Source,
				"content_preview", truncateString(record.Data["content"], 100))
		}
	}

	env.logger.Info("All records processed", "totalLoaded", len(allRecords), "expectedCount", count)
	require.Equal(t, count, len(allRecords), "Should load all records across batches")

	documents, err := env.dataprocessing.ToDocuments(env.ctx, source, allRecords)
	require.NoError(t, err)

	require.NotEmpty(t, documents, "no documents to test with")
	env.documents = documents

	env.logger.Info("Documents converted", "count", len(documents))
	for i, fact := range documents {
		env.logger.Info("Document", "index", i, "id", fact.ID(), "source", fact.Source(), "content_preview", truncateString(fact.Content(), 150))
	}
}

func truncateString(s interface{}, maxLen int) string {
	str, ok := s.(string)
	if !ok {
		return fmt.Sprintf("%v", s)
	}
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}

func (env *testEnvironment) storeDocuments(t *testing.T) {
	env.logger.Info("Documents loaded successfully", "count", len(env.documents))

	env.logger.Info("Waiting for memory processing to complete...")

	config := evolvingmemory.DefaultConfig()

	if localTimeout := os.Getenv("LOCAL_MODEL_TIMEOUT"); localTimeout != "" {
		if duration, err := time.ParseDuration(localTimeout); err == nil {
			config.FactExtractionTimeout = duration
			config.MemoryDecisionTimeout = duration
			config.StorageTimeout = duration
			env.logger.Info("Using custom timeout for local model", "timeout", duration)
		}
	}

	progressCh, errorCh := env.memory.StoreV2(env.ctx, env.documents, config)

	var errors []error

	processingTimeout := 50 * time.Minute
	if localProcessingTimeout := os.Getenv("LOCAL_MODEL_PROCESSING_TIMEOUT"); localProcessingTimeout != "" {
		if duration, err := time.ParseDuration(localProcessingTimeout); err == nil {
			processingTimeout = duration
			env.logger.Info("Using custom processing timeout for local model", "timeout", duration)
		}
	}

	timeout := time.After(processingTimeout)

	for progressCh != nil || errorCh != nil {
		select {
		case progress, ok := <-progressCh:
			if !ok {
				progressCh = nil
				continue
			}
			env.logger.Infof("Progress: %d/%d (stage: %s)", progress.Processed, progress.Total, progress.Stage)

		case err, ok := <-errorCh:
			if !ok {
				errorCh = nil
				continue
			}
			errors = append(errors, err)
			env.logger.Errorf("Processing error: %v", err)

		case <-timeout:
			t.Fatalf("Memory processing timed out after %v", processingTimeout)

		case <-env.ctx.Done():
			t.Fatal("Context canceled during memory processing")
		}
	}

	if len(errors) > 0 {
		t.Fatalf("Memory processing failed with %d errors, first error: %v", len(errors), errors[0])
	}

	env.logger.Info("Documents stored successfully")
}

func getTestConfig(t *testing.T) testConfig {
	t.Helper()

	envPath := filepath.Join("..", "..", "..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		t.Logf("Could not load .env file from %s: %v", envPath, err)
		_ = godotenv.Load()
	}

	source := getEnvOrDefault("TEST_SOURCE", "misc")

	defaultInputPath := filepath.Join("testdata", "misc")
	inputPath := getEnvOrDefault("TEST_INPUT_PATH", defaultInputPath)

	outputPath := getEnvOrDefault("TEST_OUTPUT_PATH", "")
	if outputPath == "" {
		id := uuid.New().String()
		outputPath = fmt.Sprintf("./output/%s_%s.jsonl", source, id)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("Failed to create output directory %s: %v", outputDir, err)
	}

	completionsApiKey := os.Getenv("COMPLETIONS_API_KEY")

	if completionsApiKey == "" {
		t.Fatalf("No completions API key found (set COMPLETIONS_API_KEY or TEST_COMPLETIONS_API_KEY)")
	}
	embeddingsApiKey := os.Getenv("EMBEDDINGS_API_KEY")
	if embeddingsApiKey == "" {
		t.Fatalf("No embeddings API key found (set EMBEDDINGS_API_KEY or TEST_EMBEDDINGS_API_KEY)")
	}

	completionsModel := "gpt-4o-mini"

	embeddingsModel := "text-embedding-3-small"

	return testConfig{
		Source:            source,
		InputPath:         inputPath,
		OutputPath:        outputPath,
		CompletionsModel:  completionsModel,
		CompletionsApiKey: completionsApiKey,
		CompletionsApiUrl: "https://openrouter.ai/api/v1",
		EmbeddingsModel:   embeddingsModel,
		EmbeddingsApiKey:  embeddingsApiKey,
		EmbeddingsApiUrl:  "https://api.openai.com/v1",
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func TestMemoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := setupTestEnvironment(t)
	defer env.cleanup(t)

	t.Run("DataProcessingAndStorage", func(t *testing.T) {
		env.loadDocuments(t, env.config.Source, env.config.InputPath)
		assert.NotEmpty(t, env.documents)
		env.logger.Info("Documents loaded", "count", len(env.documents))
		env.logger.Info("Documents", "documents", env.documents)
		for i := 0; i < len(env.documents); i++ {
			env.logger.Info("Document", "id", env.documents[i].ID(), "content", env.documents[i].Content())
		}

		env.storeDocuments(t)
		env.logger.Info("Documents stored successfully")
	})

	t.Run("BasicQuerying", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t, env.config.Source, env.config.InputPath)
			env.storeDocuments(t)
		}

		limit := 100
		filter := memory.Filter{
			Source: &env.config.Source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do facts from %s say about the user?", env.config.Source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories with basic query")

		for _, fact := range result.Facts {
			env.logger.Info("Basic fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query chatgpt", func(t *testing.T) {
		source := "chatgpt"
		inputPath := "testdata/chatgpt.zip"

		env.loadDocuments(t, source, inputPath)
		env.storeDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query gmail", func(t *testing.T) {
		source := "gmail"
		inputPath := "testdata/google_export_sample.zip"

		env.loadDocuments(t, source, inputPath)
		env.storeDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query telegram", func(t *testing.T) {
		source := "telegram"
		inputPath := "testdata/telegram_export_sample.json"

		env.loadDocuments(t, source, inputPath)
		env.storeDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query X", func(t *testing.T) {
		source := "x"
		inputPath := "testdata/x_export_sample.zip"

		env.loadDocuments(t, source, inputPath)
		env.storeDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query slack", func(t *testing.T) {
		source := "slack"
		inputPath := "testdata/slack_export_sample.zip"

		env.loadDocuments(t, source, inputPath)
		env.storeDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Important facts", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t, env.config.Source, env.config.InputPath)
			env.storeDocuments(t)
		}

		limit := 100
		filter := memory.Filter{
			FactImportanceMin: intPtr(3),
			Limit:             &limit,
		}

		result, err := env.memory.Query(env.ctx, "What are the most important facts about me?", &filter)
		require.NoError(t, err)

		env.logger.Info("Importance filtered query result", "count", len(result.Facts))
		assert.NotEmpty(t, result.Facts, "should find important facts about the user")

		for _, fact := range result.Facts {
			env.logger.Info("Important fact", "id", fact.ID, "content", fact.Content)
		}

		foundFieldsMedal := false
		for _, fact := range result.Facts {
			if strings.Contains(strings.ToLower(fact.Content), "fields medal") {
				foundFieldsMedal = true
				break
			}
		}
		assert.True(t, foundFieldsMedal, "should find a document containing 'Fields Medal'")
	})

	t.Run("DocumentReferences", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t, env.config.Source, env.config.InputPath)
			env.storeDocuments(t)
		}

		limit := 3
		filter := memory.Filter{
			Source:   &env.config.Source,
			Distance: 0.8,
			Limit:    &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do facts from %s say about the user?", env.config.Source), &filter)
		require.NoError(t, err)
		require.NotEmpty(t, result.Facts)

		for _, fact := range result.Facts[:min(3, len(result.Facts))] {
			memoryID := fact.ID

			docRefs, err := env.memory.GetDocumentReferences(env.ctx, memoryID)
			require.NoError(t, err)
			assert.NotEmpty(t, docRefs, "should have document references")

			docRef := docRefs[0]
			assert.NotEmpty(t, docRef.ID, "document reference should have ID")
			assert.NotEmpty(t, docRef.Content, "document reference should have content")
			assert.NotEmpty(t, docRef.Type, "document reference should have type")
		}
	})

	t.Run("SourceFiltering", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t, env.config.Source, env.config.InputPath)
			env.storeDocuments(t)
		}

		limit := 100

		invalidSource := "invalid-source"
		filter := memory.Filter{
			Source:   &invalidSource,
			Distance: 0.7,
			Limit:    &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do facts from %s say about the user?", env.config.Source), &filter)
		require.NoError(t, err)
		assert.Empty(t, result.Facts, "should not find memories for invalid source")
	})

	t.Run("DistanceFiltering", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t, env.config.Source, env.config.InputPath)
			env.storeDocuments(t)
		}

		limit := 100
		filter := memory.Filter{
			Source:   &env.config.Source,
			Distance: 0.001, // Very restrictive threshold
			Limit:    &limit,
		}

		result, err := env.memory.Query(env.ctx, "What do I know about gluon fields ?", &filter)
		require.NoError(t, err)
		assert.Empty(t, result.Facts, "should filter out documents for highly specific query")
	})
}

func TestStructuredFactFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := setupTestEnvironment(t)
	defer env.cleanup(t)

	env.loadDocuments(t, env.config.Source, env.config.InputPath)
	env.storeDocuments(t)

	limit := 100

	testCases := []struct {
		name        string
		filter      memory.Filter
		query       string
		expectEmpty bool
	}{
		{
			name: "FactCategoryFiltering",
			filter: memory.Filter{
				FactCategory: stringPtr("preference"),
				Source:       &env.config.Source,
				Limit:        &limit,
			},
			query: "user preferences",
		},
		{
			name: "FactSubjectFiltering",
			filter: memory.Filter{
				FactSubject: stringPtr("user"),
				Source:      &env.config.Source,
				Limit:       &limit,
			},
			query: "facts about user",
		},
		{
			name: "FactImportanceFiltering",
			filter: memory.Filter{
				FactImportance: intPtr(3),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query: "important facts",
		},
		{
			name: "FactImportanceRangeFiltering",
			filter: memory.Filter{
				FactImportanceMin: intPtr(2),
				FactImportanceMax: intPtr(3),
				Source:            &env.config.Source,
				Limit:             &limit,
			},
			query: "medium to high importance facts",
		},
		{
			name: "FactSensitivityFiltering",
			filter: memory.Filter{
				FactSensitivity: stringPtr("low"),
				Source:          &env.config.Source,
				Limit:           &limit,
			},
			query: "public information",
		},
		{
			name: "CombinedStructuredFiltering",
			filter: memory.Filter{
				FactCategory:   stringPtr("preference"),
				FactSubject:    stringPtr("user"),
				FactImportance: intPtr(2),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query: "user preferences with medium importance",
		},
		{
			name: "FactValuePartialMatching",
			filter: memory.Filter{
				FactValue: stringPtr("coffee"),
				Source:    &env.config.Source,
				Limit:     &limit,
			},
			query: "coffee related facts",
		},
		{
			name: "FactTemporalContextFiltering",
			filter: memory.Filter{
				FactTemporalContext: stringPtr("2024"),
				Source:              &env.config.Source,
				Limit:               &limit,
			},
			query: "facts from 2024",
		},
		{
			name: "MixedLegacyAndStructuredFiltering",
			filter: memory.Filter{
				Source:         &env.config.Source,
				Distance:       0.8,
				Limit:          &limit,
				FactCategory:   stringPtr("preference"),
				FactImportance: intPtr(3),
			},
			query: "high priority preferences",
		},
		{
			name: "ComplexRealisticFiltering",
			filter: memory.Filter{
				FactCategory:        stringPtr("goal_plan"),
				FactSubject:         stringPtr("user"),
				FactSensitivity:     stringPtr("medium"),
				FactImportanceMin:   intPtr(2),
				FactTemporalContext: stringPtr("Q1"),
				Source:              &env.config.Source,
				Limit:               &limit,
			},
			query: "user goals for Q1 with medium sensitivity and high importance",
		},
		{
			name: "FactAttributeFiltering",
			filter: memory.Filter{
				FactAttribute: stringPtr("health_metric"),
				Source:        &env.config.Source,
				Limit:         &limit,
			},
			query: "health metrics",
		},
		{
			name: "NonExistentCategoryFiltering",
			filter: memory.Filter{
				FactCategory: stringPtr("nonexistent_category"),
				Source:       &env.config.Source,
				Limit:        &limit,
			},
			query:       "facts from non-existent category",
			expectEmpty: true,
		},
		{
			name: "WrongImportanceFiltering",
			filter: memory.Filter{
				FactImportanceMin: intPtr(10),
				Source:            &env.config.Source,
				Limit:             &limit,
			},
			query:       "What do you know about me?",
			expectEmpty: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := env.memory.Query(env.ctx, tc.query, &tc.filter)
			require.NoError(t, err)

			if tc.expectEmpty {
				assert.Empty(t, result.Facts, "expected no results for test case: %s", tc.name)
			} else {
				env.logger.Info("Test completed", "name", tc.name, "results_count", len(result.Facts))
			}
		})
	}
}

func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }

func TestMemoryIntegrationSimple(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := setupTestEnvironment(t)
	defer env.cleanup(t)

	env.loadDocuments(t, env.config.Source, env.config.InputPath)
	assert.NotEmpty(t, env.documents)
	env.logger.Info("Documents loaded successfully", "count", len(env.documents))

	env.storeDocuments(t)
	env.logger.Info("Documents stored successfully")

	limit := 10
	filter := memory.Filter{
		Source:   &env.config.Source,
		Distance: 0.9, // Very permissive distance
		Limit:    &limit,
	}

	result, err := env.memory.Query(env.ctx, "LLM agent system implementation", &filter)
	require.NoError(t, err)
	env.logger.Info("Query completed", "query", "LLM agent system implementation", "results_count", len(result.Facts))

	invalidSource := "invalid-source"
	invalidFilter := memory.Filter{
		Source:   &invalidSource,
		Distance: 0.9,
		Limit:    &limit,
	}

	result, err = env.memory.Query(env.ctx, "anything", &invalidFilter)
	require.NoError(t, err)
	assert.Empty(t, result.Facts, "should not find memories for invalid source")
}

func TestBatchProcessingEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := setupTestEnvironment(t)
	defer env.cleanup(t)

	source := "misc"

	t.Run("EmptyFile", func(t *testing.T) {
		emptyFile := filepath.Join(env.tempDir, "empty.jsonl")
		err := os.WriteFile(emptyFile, []byte(""), 0o644)
		require.NoError(t, err)

		count, err := helpers.CountJSONLLines(emptyFile)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		records, err := helpers.ReadJSONLBatch(emptyFile, 0, 10)
		require.NoError(t, err)
		assert.Empty(t, records)

		documents, err := env.dataprocessing.ToDocuments(env.ctx, source, records)
		require.NoError(t, err)
		assert.Empty(t, documents)
	})

	t.Run("SingleRecord", func(t *testing.T) {
		singleRecordFile := filepath.Join(env.tempDir, "single.jsonl")
		singleRecordData := `{"data":{"content":"Single test record"},"timestamp":"2024-01-01T12:00:00Z","source":"misc"}`
		err := os.WriteFile(singleRecordFile, []byte(singleRecordData), 0o644)
		require.NoError(t, err)

		count, err := helpers.CountJSONLLines(singleRecordFile)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		records, err := helpers.ReadJSONLBatch(singleRecordFile, 0, 1)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "Single test record", records[0].Data["content"])

		records, err = helpers.ReadJSONLBatch(singleRecordFile, 0, 10)
		require.NoError(t, err)
		assert.Len(t, records, 1)

		records, err = helpers.ReadJSONLBatch(singleRecordFile, 1, 10)
		require.NoError(t, err)
		assert.Empty(t, records)

		documents, err := env.dataprocessing.ToDocuments(env.ctx, source, records)
		require.NoError(t, err)
		assert.Empty(t, documents)
	})

	t.Run("ExactBatchBoundaries", func(t *testing.T) {
		batchBoundaryFile := filepath.Join(env.tempDir, "batch_boundary.jsonl")
		var lines []string
		for i := 1; i <= 6; i++ {
			line := fmt.Sprintf(`{"data":{"content":"Record %d","id":"%d"},"timestamp":"2024-01-01T%02d:00:00Z","source":"misc"}`, i, i, i+10)
			lines = append(lines, line)
		}
		testData := strings.Join(lines, "\n")
		err := os.WriteFile(batchBoundaryFile, []byte(testData), 0o644)
		require.NoError(t, err)

		count, err := helpers.CountJSONLLines(batchBoundaryFile)
		require.NoError(t, err)
		assert.Equal(t, 6, count)

		batchSize := 2
		expectedBatches := 3

		for batchIndex := 0; batchIndex < expectedBatches; batchIndex++ {
			records, err := helpers.ReadJSONLBatch(batchBoundaryFile, batchIndex*batchSize, batchSize)
			require.NoError(t, err)

			if batchIndex < expectedBatches-1 {
				assert.Len(t, records, batchSize, "Batch %d should have %d records", batchIndex, batchSize)
			} else {
				assert.Len(t, records, 2, "Last batch should have remaining records")
			}

			for j, record := range records {
				expectedID := fmt.Sprintf("%d", batchIndex*batchSize+j+1)
				assert.Equal(t, expectedID, record.Data["id"], "Record ID mismatch in batch %d, record %d", batchIndex, j)
			}
		}

		records, err := helpers.ReadJSONLBatch(batchBoundaryFile, expectedBatches*batchSize, batchSize)
		require.NoError(t, err)
		assert.Empty(t, records, "Should get empty results when reading beyond file end")
	})

	t.Run("SmallBatchProcessing", func(t *testing.T) {
		smallBatchFile := filepath.Join(env.tempDir, "small_batch.jsonl")
		var lines []string
		for i := 1; i <= 10; i++ {
			line := fmt.Sprintf(`{"data":{"content":"Small batch record %d","category":"batch-%d"},"timestamp":"2024-01-01T%02d:00:00Z","source":"misc"}`, i, (i-1)%3+1, i+10)
			lines = append(lines, line)
		}
		testData := strings.Join(lines, "\n")
		err := os.WriteFile(smallBatchFile, []byte(testData), 0o644)
		require.NoError(t, err)

		count, err := helpers.CountJSONLLines(smallBatchFile)
		require.NoError(t, err)
		assert.Equal(t, 10, count)

		testCases := []struct {
			name            string
			batchSize       int
			expectedBatches int
		}{
			{"BatchSize1", 1, 10},
			{"BatchSize3", 3, 4},
			{"BatchSize4", 4, 3},
			{"BatchSize5", 5, 2},
			{"BatchSize7", 7, 2},
			{"BatchSize10", 10, 1},
			{"BatchSize15", 15, 1},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				totalProcessed := 0
				batchIndex := 0

				for {
					records, err := helpers.ReadJSONLBatch(smallBatchFile, batchIndex*tc.batchSize, tc.batchSize)
					require.NoError(t, err)

					if len(records) == 0 {
						break
					}

					totalProcessed += len(records)
					batchIndex++

					if batchIndex == tc.expectedBatches {
						expectedInLastBatch := count - (tc.expectedBatches-1)*tc.batchSize
						assert.Len(t, records, expectedInLastBatch, "Last batch should have correct number of records")
					} else if batchIndex < tc.expectedBatches {
						assert.Len(t, records, tc.batchSize, "Non-final batch should be full")
					}

					for j, record := range records {
						expectedContent := fmt.Sprintf("Small batch record %d", batchIndex*tc.batchSize+j-tc.batchSize+1)
						assert.Equal(t, expectedContent, record.Data["content"])
					}
				}

				assert.Equal(t, tc.expectedBatches, batchIndex, "Should process expected number of batches")
				assert.Equal(t, count, totalProcessed, "Should process all records across batches")
			})
		}
	})

	t.Run("PartialBatchConversion", func(t *testing.T) {
		partialBatchFile := filepath.Join(env.tempDir, "partial_batch.jsonl")
		var lines []string
		for i := 1; i <= 7; i++ {
			line := fmt.Sprintf(`{"data":{"content":"Partial batch content %d","type":"document"},"timestamp":"2024-01-01T%02d:00:00Z","source":"misc"}`, i, i+10)
			lines = append(lines, line)
		}
		testData := strings.Join(lines, "\n")
		err := os.WriteFile(partialBatchFile, []byte(testData), 0o644)
		require.NoError(t, err)

		batchSize := 3
		batchIndex := 0

		for {
			records, err := helpers.ReadJSONLBatch(partialBatchFile, batchIndex*batchSize, batchSize)
			require.NoError(t, err)

			if len(records) == 0 {
				break
			}

			documents, err := env.dataprocessing.ToDocuments(env.ctx, source, records)
			require.NoError(t, err)
			assert.Len(t, documents, len(records), "Should convert all records in batch to documents")

			for i, doc := range documents {
				expectedContent := fmt.Sprintf("Partial batch content %d", batchIndex*batchSize+i+1)
				assert.Contains(t, doc.Content(), expectedContent, "Document content should match record content")
				assert.Equal(t, source, doc.Source(), "Document source should match")
			}

			batchIndex++
		}

		assert.Equal(t, 3, batchIndex, "Should process 3 batches (3, 3, 1)")
	})

	t.Run("BatchSizeValidation", func(t *testing.T) {
		testFile := filepath.Join(env.tempDir, "validation_test.jsonl")
		testData := `{"data":{"content":"Test record"},"timestamp":"2024-01-01T12:00:00Z","source":"misc"}`
		err := os.WriteFile(testFile, []byte(testData), 0o644)
		require.NoError(t, err)

		_, err = helpers.ReadJSONLBatch(testFile, 0, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batchSize must be positive")

		_, err = helpers.ReadJSONLBatch(testFile, 0, -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batchSize must be positive")

		_, err = helpers.ReadJSONLBatch(testFile, -1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "startIndex cannot be negative")

		_, err = helpers.ReadJSONLBatch("", 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "filePath cannot be empty")
	})

	t.Run("LargeBatchSize", func(t *testing.T) {
		largeBatchFile := filepath.Join(env.tempDir, "large_batch.jsonl")
		var lines []string
		for i := 1; i <= 5; i++ {
			line := fmt.Sprintf(`{"data":{"content":"Large batch record %d"},"timestamp":"2024-01-01T%02d:00:00Z","source":"misc"}`, i, i+10)
			lines = append(lines, line)
		}
		testData := strings.Join(lines, "\n")
		err := os.WriteFile(largeBatchFile, []byte(testData), 0o644)
		require.NoError(t, err)

		records, err := helpers.ReadJSONLBatch(largeBatchFile, 0, 1000)
		require.NoError(t, err)
		assert.Len(t, records, 5, "Should return all records when batch size exceeds file size")

		for i, record := range records {
			expectedContent := fmt.Sprintf("Large batch record %d", i+1)
			assert.Equal(t, expectedContent, record.Data["content"])
		}
	})
}

func TestMain(m *testing.M) {
	setupSharedInfrastructure()

	code := m.Run()

	teardownSharedInfrastructure()

	os.Exit(code)
}
