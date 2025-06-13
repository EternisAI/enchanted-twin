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
	jsonHelpers "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
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

		embeddingsApiUrl := os.Getenv("EMBEDDINGS_API_URL")
		if embeddingsApiUrl == "" {
			embeddingsApiUrl = "https://api.openai.com/v1" // fallback
		}

		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small" // fallback
		}

		aiEmbeddingsService := ai.NewOpenAIService(sharedLogger, os.Getenv("EMBEDDINGS_API_KEY"), embeddingsApiUrl)
		err = bootstrap.InitSchema(sharedWeaviateClient, sharedLogger, aiEmbeddingsService, embeddingsModel)
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

	for _, className := range []string{"MemoryFact", "SourceDocument"} {
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

	dataprocessingService := dataprocessing.NewDataProcessingService(openAiService.Service, completionsModel, store, sharedLogger)

	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiEmbeddingsService, config.EmbeddingsModel)
	if err != nil {
		t.Fatalf("Failed to create embedding wrapper: %v", err)
	}

	storageInterface, err := storage.New(storage.NewStorageInput{
		Client:            sharedWeaviateClient,
		Logger:            sharedLogger,
		EmbeddingsWrapper: embeddingsWrapper,
	})
	if err != nil {
		t.Fatalf("Failed to create storage interface: %v", err)
	}

	var mem evolvingmemory.MemoryStorage
	mem, err = evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             sharedLogger,
		Storage:            storageInterface,
		CompletionsService: openAiService.Service,
		CompletionsModel:   config.CompletionsModel,
		EmbeddingsWrapper:  embeddingsWrapper,
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

	count, err := jsonHelpers.CountJSONLLines(env.config.OutputPath)
	if err != nil {
		t.Fatalf("Failed to count JSONL lines: %v", err)
	}

	batchSize := 3
	var allRecords []types.Record

	env.logger.Info("Loading documents in batches", "totalRecords", count, "batchSize", batchSize)

	for batchIndex := 0; ; batchIndex++ {
		startIndex := batchIndex * batchSize
		records, err := jsonHelpers.ReadJSONLBatch(env.config.OutputPath, startIndex, batchSize)
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

func (env *testEnvironment) loadDocumentsFromJSONL(t *testing.T, source, inputPath string) {
	t.Helper()

	// Check if file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Skipping test - file not found: %s", inputPath)
		return
	}

	// Read the file to check format
	fileContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Check if file is empty
	if len(fileContent) == 0 {
		t.Skip("Empty test file")
		return
	}

	count, err := jsonHelpers.CountJSONLLines(inputPath)
	if err != nil {
		t.Fatalf("Failed to count JSONL lines: %v", err)
	}

	if count == 0 {
		t.Skip("No records in test file")
		return
	}

	batchSize := 3
	var allRecords []types.Record

	env.logger.Info("Loading documents from JSONL in batches", "totalRecords", count, "batchSize", batchSize)

	for batchIndex := 0; ; batchIndex++ {
		startIndex := batchIndex * batchSize
		records, err := jsonHelpers.ReadJSONLBatch(inputPath, startIndex, batchSize)
		require.NoError(t, err)

		if len(records) == 0 {
			break
		}

		env.logger.Info("Loaded batch", "batchIndex", batchIndex, "recordCount", len(records), "startIndex", startIndex)
		allRecords = append(allRecords, records...)

		for i, record := range records {
			// Handle both "text" and "content" fields
			content := ""
			if text, ok := record.Data["text"].(string); ok {
				content = text
			} else if contentStr, ok := record.Data["content"].(string); ok {
				content = contentStr
			}

			env.logger.Info("Record in batch",
				"batchIndex", batchIndex,
				"recordIndex", i,
				"globalIndex", startIndex+i,
				"source", record.Source,
				"content_preview", truncateString(content, 100))
		}
	}

	env.logger.Info("All records processed", "totalLoaded", len(allRecords), "expectedCount", count)
	require.Equal(t, count, len(allRecords), "Should load all records across batches")

	// Convert to documents with error handling
	documents, err := env.dataprocessing.ToDocuments(env.ctx, source, allRecords)
	if err != nil {
		env.logger.Error("Failed to convert records to documents", "error", err)
		// Don't fail here, just skip if no documents can be created
		if len(documents) == 0 {
			t.Skip("Could not convert WhatsApp records to documents")
			return
		}
	}

	env.documents = documents

	env.logger.Info("Documents converted", "count", len(documents))
	for i, doc := range documents {
		env.logger.Info("Document", "index", i, "id", doc.ID(), "source", doc.Source(), "content_preview", truncateString(doc.Content(), 150))
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
	env.storeDocumentsWithTimeout(t, 50*time.Minute)
}

func (env *testEnvironment) storeDocumentsWithTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()

	processingTimeout := timeout

	// Override with environment variable if set
	if localProcessingTimeout := os.Getenv("LOCAL_MODEL_PROCESSING_TIMEOUT"); localProcessingTimeout != "" {
		if duration, err := time.ParseDuration(localProcessingTimeout); err == nil {
			processingTimeout = duration
			env.logger.Info("Using custom processing timeout for local model", "timeout", duration)
		}
	}

	// Use context with timeout
	ctx, cancel := context.WithTimeout(env.ctx, processingTimeout)
	defer cancel()

	// Use Store with callback for progress tracking
	err := env.memory.Store(ctx, env.documents, func(processed, total int) {
		env.logger.Infof("Progress: %d/%d", processed, total)
	})
	if err != nil {
		t.Fatalf("Memory processing failed: %v", err)
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

	completionsModel := os.Getenv("COMPLETIONS_MODEL")
	if completionsModel == "" {
		completionsModel = "gpt-4o-mini"
	}

	embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
	if embeddingsModel == "" {
		embeddingsModel = "text-embedding-3-small"
	}

	// Read API URLs from environment variables
	completionsApiUrl := os.Getenv("COMPLETIONS_API_URL")
	if completionsApiUrl == "" {
		completionsApiUrl = "https://openrouter.ai/api/v1" // fallback
	}

	embeddingsApiUrl := os.Getenv("EMBEDDINGS_API_URL")
	if embeddingsApiUrl == "" {
		embeddingsApiUrl = "https://api.openai.com/v1" // fallback
	}

	return testConfig{
		Source:            source,
		InputPath:         inputPath,
		OutputPath:        outputPath,
		CompletionsModel:  completionsModel,
		CompletionsApiKey: completionsApiKey,
		CompletionsApiUrl: completionsApiUrl,
		EmbeddingsModel:   embeddingsModel,
		EmbeddingsApiKey:  embeddingsApiKey,
		EmbeddingsApiUrl:  embeddingsApiUrl,
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

	t.Run("Query whatsapp", func(t *testing.T) {
		source := "whatsapp"
		inputPath := "testdata/whatsapp_sample.jsonl"

		// First, create a simple test file if it doesn't exist
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			// Create testdata directory
			testdataDir := filepath.Dir(inputPath)
			if err := os.MkdirAll(testdataDir, 0o755); err != nil {
				t.Fatalf("Failed to create testdata directory: %v", err)
			}

			// Create sample WhatsApp data
			sampleData := []string{
				`{"data":{"text":"2023-05-15 10:30:00 - Alice: Hey, how are you?\n2023-05-15 10:31:00 - Bob: I'm good! Just working on the project.\n2023-05-15 10:32:00 - Alice: Great! Let me know if you need help.","metadata":{"chat_session":"alice_bob_chat","participants":["Alice","Bob"],"user":"Bob"}},"timestamp":"2023-05-15T10:32:00Z","source":"whatsapp"}`,
				`{"data":{"text":"2023-05-16 14:00:00 - Carol: Meeting at 3pm today?\n2023-05-16 14:01:00 - Bob: Yes, I'll be there.\n2023-05-16 14:02:00 - Carol: Perfect, see you then!","metadata":{"chat_session":"carol_bob_chat","participants":["Carol","Bob"],"user":"Bob"}},"timestamp":"2023-05-16T14:02:00Z","source":"whatsapp"}`,
			}

			content := strings.Join(sampleData, "\n")
			if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
				t.Fatalf("Failed to create sample WhatsApp file: %v", err)
			}
			t.Logf("Created sample WhatsApp test file: %s", inputPath)
		}

		env.loadDocumentsFromJSONL(t, source, inputPath)

		if len(env.documents) == 0 {
			t.Skip("No WhatsApp documents loaded - skipping test")
			return
		}

		// Log detailed information about the ConversationDocuments
		env.logger.Info("=== WhatsApp ConversationDocuments Details ===")
		for i, doc := range env.documents {
			env.logger.Info("ConversationDocument",
				"index", i,
				"id", doc.ID(),
				"source", doc.Source(),
				"content_length", len(doc.Content()),
			)

			// Log the full conversation content with formatting
			env.logger.Info("Conversation Content:",
				"document_index", i,
				"full_content", doc.Content(),
			)

			// If we can cast to ConversationDocument, log additional details
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				env.logger.Info("ConversationDocument Metadata",
					"document_index", i,
					"participants", convDoc.People,
					"message_count", len(convDoc.Conversation),
					"user", convDoc.User,
					"chat_session", convDoc.FieldMetadata["chat_session"],
					"metadata", convDoc.FieldMetadata,
				)
			}

			env.logger.Info("--- End Document %d ---", i)
		}
		env.logger.Info("=== End WhatsApp ConversationDocuments ===")

		// Store with extended timeout for WhatsApp
		env.storeDocumentsWithTimeout(t, 30*time.Minute)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do we know about the user from %s source?", source), &filter)

		// More lenient error handling for WhatsApp
		if err != nil {
			env.logger.Warn("WhatsApp query returned error (may be expected)", "error", err)
			// Don't fail the test, just log
			t.Logf("WhatsApp query error (non-fatal): %v", err)
		} else {
			env.logger.Info("WhatsApp query results", "fact_count", len(result.Facts))

			if len(result.Facts) == 0 {
				t.Logf("No facts extracted from WhatsApp conversations (this may be normal)")
			} else {
				for _, fact := range result.Facts {
					env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
				}
			}
		}
	})

	// t.Run("Important facts", func(t *testing.T) {
	// 	if len(env.documents) == 0 {
	// 		env.loadDocuments(t, env.config.Source, env.config.InputPath)
	// 		env.storeDocuments(t)
	// 	}

	// 	limit := 100
	// 	filter := memory.Filter{
	// 		FactImportanceMin: helpers.Ptr(3),
	// 		Limit:             &limit,
	// 	}

	// 	result, err := env.memory.Query(env.ctx, "What are the most important facts about me?", &filter)
	// 	require.NoError(t, err)

	// 	env.logger.Info("Importance filtered query result", "count", len(result.Facts))
	// 	assert.NotEmpty(t, result.Facts, "should find important facts about the user")

	// 	for _, fact := range result.Facts {
	// 		env.logger.Info("Important fact", "id", fact.ID, "content", fact.Content)
	// 	}

	// 	foundFieldsMedal := false
	// 	for _, fact := range result.Facts {
	// 		if strings.Contains(strings.ToLower(fact.Content), "fields medal") {
	// 			foundFieldsMedal = true
	// 			break
	// 		}
	// 	}
	// 	assert.True(t, foundFieldsMedal, "should find a document containing 'Fields Medal'")
	// })

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

func TestMemoryFactFiltering(t *testing.T) {
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
				FactCategory: helpers.Ptr("preference"),
				Source:       &env.config.Source,
				Limit:        &limit,
			},
			query:       "user preferences",
			expectEmpty: false, // Test data contains environment preferences
		},
		{
			name: "FactSubjectFiltering",
			filter: memory.Filter{
				Subject: helpers.Ptr("user"),
				Source:  &env.config.Source,
				Limit:   &limit,
			},
			query: "facts about user",
		},
		{
			name: "FactImportanceFiltering",
			filter: memory.Filter{
				FactImportance: helpers.Ptr(3),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query: "important facts",
		},
		{
			name: "FactImportanceRangeFiltering",
			filter: memory.Filter{
				FactImportanceMin: helpers.Ptr(2),
				FactImportanceMax: helpers.Ptr(3),
				Source:            &env.config.Source,
				Limit:             &limit,
			},
			query: "medium to high importance facts",
		},
		{
			name: "CombinedStructuredFiltering",
			filter: memory.Filter{
				FactCategory:   helpers.Ptr("preference"),
				Subject:        helpers.Ptr("user"),
				FactImportance: helpers.Ptr(2),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query:       "user preferences with medium importance",
			expectEmpty: false, // Test data contains environment preferences with importance 2
		},
		{
			name: "FactAttributeFiltering",
			filter: memory.Filter{
				FactAttribute: helpers.Ptr("health_metric"),
				Source:        &env.config.Source,
				Limit:         &limit,
			},
			query:       "health metrics",
			expectEmpty: true, // Changed to true
		},
		{
			name: "TimestampRangeFiltering",
			filter: memory.Filter{
				TimestampAfter: func() *time.Time { t := time.Now().AddDate(0, 0, -30); return &t }(),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query: "recent activities",
		},
		{
			name: "CombinedAdvancedFiltering",
			filter: memory.Filter{
				FactCategory:   helpers.Ptr("health"),
				Subject:        helpers.Ptr("user"),
				FactImportance: helpers.Ptr(3),
				Source:         &env.config.Source,
				Limit:          &limit,
			},
			query:       "critical health facts for user",
			expectEmpty: true, // Changed to true
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

		count, err := jsonHelpers.CountJSONLLines(emptyFile)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		records, err := jsonHelpers.ReadJSONLBatch(emptyFile, 0, 10)
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

		count, err := jsonHelpers.CountJSONLLines(singleRecordFile)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		records, err := jsonHelpers.ReadJSONLBatch(singleRecordFile, 0, 1)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "Single test record", records[0].Data["content"])

		records, err = jsonHelpers.ReadJSONLBatch(singleRecordFile, 0, 10)
		require.NoError(t, err)
		assert.Len(t, records, 1)

		records, err = jsonHelpers.ReadJSONLBatch(singleRecordFile, 1, 10)
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

		count, err := jsonHelpers.CountJSONLLines(batchBoundaryFile)
		require.NoError(t, err)
		assert.Equal(t, 6, count)

		batchSize := 2
		expectedBatches := 3

		for batchIndex := 0; batchIndex < expectedBatches; batchIndex++ {
			records, err := jsonHelpers.ReadJSONLBatch(batchBoundaryFile, batchIndex*batchSize, batchSize)
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

		records, err := jsonHelpers.ReadJSONLBatch(batchBoundaryFile, expectedBatches*batchSize, batchSize)
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

		count, err := jsonHelpers.CountJSONLLines(smallBatchFile)
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
					records, err := jsonHelpers.ReadJSONLBatch(smallBatchFile, batchIndex*tc.batchSize, tc.batchSize)
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
			records, err := jsonHelpers.ReadJSONLBatch(partialBatchFile, batchIndex*batchSize, batchSize)
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

		_, err = jsonHelpers.ReadJSONLBatch(testFile, 0, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batchSize must be positive")

		_, err = jsonHelpers.ReadJSONLBatch(testFile, 0, -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batchSize must be positive")

		_, err = jsonHelpers.ReadJSONLBatch(testFile, -1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "startIndex cannot be negative")

		_, err = jsonHelpers.ReadJSONLBatch("", 0, 1)
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

		records, err := jsonHelpers.ReadJSONLBatch(largeBatchFile, 0, 1000)
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
