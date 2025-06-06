package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
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

// Shared test infrastructure.
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

		// Temporarily clear os.Args to avoid flag parsing conflicts
		originalArgs := os.Args
		os.Args = []string{"test"}

		sharedWeaviateServer, err = bootstrap.BootstrapWeaviateServer(ctx, sharedLogger, weaviatePort, weaviateDataPath)
		if err != nil {
			panic(fmt.Sprintf("failed to start weaviate server: %v", err))
		}

		// Restore os.Args
		os.Args = originalArgs

		sharedWeaviateClient, err = weaviate.NewClient(weaviate.Config{
			Host:   fmt.Sprintf("localhost:%s", weaviatePort),
			Scheme: "http",
		})
		if err != nil {
			panic(fmt.Sprintf("failed to create weaviate client: %v", err))
		}

		// Initialize schema once
		aiEmbeddingsService := ai.NewOpenAIService(sharedLogger, os.Getenv("EMBEDDINGS_API_KEY"), "https://api.openai.com/v1")
		err = bootstrap.InitSchema(sharedWeaviateClient, sharedLogger, aiEmbeddingsService)
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

	// Simple approach: try to delete objects by getting all and then deleting
	// This is good enough for test cleanup
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

	// Ensure shared infrastructure is ready
	setupSharedInfrastructure()

	// Clear data from previous tests
	clearWeaviateData(t)

	config := getTestConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	// Create test-specific temp directory for database
	tempDir, err := os.MkdirTemp("", "memory-test-*")
	require.NoError(t, err)

	storePath := filepath.Join(tempDir, "test.db")
	store, err := db.NewStore(ctx, storePath)
	require.NoError(t, err)

	completionsModel := config.CompletionsModel
	if completionsModel == "" {
		completionsModel = "gpt-4o-mini"
	}

	openAiService := ai.NewOpenAIService(sharedLogger, config.CompletionsApiKey, config.CompletionsApiUrl)
	aiEmbeddingsService := ai.NewOpenAIService(sharedLogger, config.EmbeddingsApiKey, config.EmbeddingsApiUrl)

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService, completionsModel, store)

	storageInterface := storage.New(sharedWeaviateClient, sharedLogger, aiEmbeddingsService)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             sharedLogger,
		Storage:            storageInterface,
		CompletionsService: openAiService,
		EmbeddingsService:  aiEmbeddingsService,
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

func (env *testEnvironment) loadDocuments(t *testing.T) {
	t.Helper()

	_, err := env.dataprocessing.ProcessSource(env.ctx, env.config.Source, env.config.InputPath, env.config.OutputPath)
	require.NoError(t, err)

	records, err := helpers.ReadJSONL[types.Record](env.config.OutputPath)
	require.NoError(t, err)

	documents, err := env.dataprocessing.ToDocuments(env.ctx, env.config.Source, records)
	require.NoError(t, err)

	require.NotEmpty(t, documents, "no documents to test with")
	env.documents = documents
}

func (env *testEnvironment) storeDocuments(t *testing.T) {
	env.logger.Info("Documents loaded successfully", "count", len(env.documents))

	env.logger.Info("Waiting for memory processing to complete...")

	config := evolvingmemory.DefaultConfig()
	progressCh, errorCh := env.memory.StoreV2(env.ctx, env.documents, config)

	// Properly wait for completion by consuming channels until they close
	var errors []error

	// Use a timeout to prevent hanging if something goes wrong
	timeout := time.After(2 * time.Minute)

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
			t.Fatal("Memory processing timed out after 2 minutes")

		case <-env.ctx.Done():
			t.Fatal("Context canceled during memory processing")
		}
	}

	// Check for errors
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

	completionsApiKey := getEnvOrDefault("TEST_COMPLETIONS_API_KEY", os.Getenv("COMPLETIONS_API_KEY"))
	embeddingsApiKey := getEnvOrDefault("TEST_EMBEDDINGS_API_KEY", os.Getenv("EMBEDDINGS_API_KEY"))

	if completionsApiKey == "" {
		t.Error("No completions API key found (set COMPLETIONS_API_KEY or TEST_COMPLETIONS_API_KEY)")
	}
	if embeddingsApiKey == "" {
		t.Error("No embeddings API key found (set EMBEDDINGS_API_KEY or TEST_EMBEDDINGS_API_KEY)")
	}

	return testConfig{
		Source:            source,
		InputPath:         inputPath,
		OutputPath:        outputPath,
		CompletionsModel:  getEnvOrDefault("TEST_COMPLETIONS_MODEL", "gpt-4o-mini"),
		CompletionsApiKey: completionsApiKey,
		CompletionsApiUrl: getEnvOrDefault("TEST_COMPLETIONS_API_URL", "https://openrouter.ai/api/v1"),
		EmbeddingsModel:   getEnvOrDefault("TEST_EMBEDDINGS_MODEL", "text-embedding-3-small"),
		EmbeddingsApiKey:  embeddingsApiKey,
		EmbeddingsApiUrl:  getEnvOrDefault("TEST_EMBEDDINGS_API_URL", "https://api.openai.com/v1"),
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
		env.loadDocuments(t)
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
			env.loadDocuments(t)
			env.storeDocuments(t)
		}

		// Memory processing is now complete since storeDocuments() waits for completion

		limit := 100
		filter := memory.Filter{
			Source:   &env.config.Source,
			Distance: 0.8,
			Limit:    &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do facts from %s say about the user?", env.config.Source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Documents, "should find memories with valid source")
	})

	t.Run("DocumentReferences", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t)
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
		require.NotEmpty(t, result.Documents)

		for _, doc := range result.Documents[:min(3, len(result.Documents))] {
			memoryID := doc.ID()

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
			env.loadDocuments(t)
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
		assert.Empty(t, result.Documents, "should not find memories for invalid source")
	})

	t.Run("DistanceFiltering", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.loadDocuments(t)
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
		assert.Empty(t, result.Documents, "should filter out documents for highly specific query")
	})
}

func TestStructuredFactFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := setupTestEnvironment(t)
	defer env.cleanup(t)

	// Setup data
	env.loadDocuments(t)
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
				assert.Empty(t, result.Documents, "expected no results for test case: %s", tc.name)
			} else {
				env.logger.Info("Test completed", "name", tc.name, "results_count", len(result.Documents))
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

	// Test 1: Data processing and storage
	env.loadDocuments(t)
	assert.NotEmpty(t, env.documents)
	env.logger.Info("Documents loaded successfully", "count", len(env.documents))

	env.storeDocuments(t)
	env.logger.Info("Documents stored successfully")

	// Test 2: Basic functionality test - just verify the system can query
	// (without requiring specific content matches)
	limit := 10
	filter := memory.Filter{
		Source:   &env.config.Source,
		Distance: 0.9, // Very permissive distance
		Limit:    &limit,
	}

	// Try a broad query that should have some chance of matching
	result, err := env.memory.Query(env.ctx, "LLM agent system implementation", &filter)
	require.NoError(t, err)
	env.logger.Info("Query completed", "query", "LLM agent system implementation", "results_count", len(result.Documents))

	// Test 3: Verify source filtering works (should return empty for invalid source)
	invalidSource := "invalid-source"
	invalidFilter := memory.Filter{
		Source:   &invalidSource,
		Distance: 0.9,
		Limit:    &limit,
	}

	result, err = env.memory.Query(env.ctx, "anything", &invalidFilter)
	require.NoError(t, err)
	assert.Empty(t, result.Documents, "should not find memories for invalid source")
}

func TestMain(m *testing.M) {
	// Setup shared infrastructure
	setupSharedInfrastructure()

	// Run tests
	code := m.Run()

	// Cleanup
	teardownSharedInfrastructure()

	os.Exit(code)
}
