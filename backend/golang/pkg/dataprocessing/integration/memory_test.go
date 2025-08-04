package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// initPostgresSchemaForTests creates the basic tables with pgvector extension for testing.
func initPostgresSchemaForTests(db *sql.DB, logger *log.Logger) error {
	logger.Debug("Initializing PostgreSQL schema for tests with pgvector support")

	_, err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		logger.Warn("pgvector extension not available, creating tables without vector columns", "error", err)
		return initSimplePostgresSchemaForTests(db, logger)
	}

	schema := `
		-- Memory facts table with vector support
		CREATE TABLE IF NOT EXISTS memory_facts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			content_vector VECTOR(1536),
			timestamp TIMESTAMPTZ NOT NULL,
			source TEXT NOT NULL,
			tags TEXT[] DEFAULT '{}',
			document_references TEXT[] DEFAULT '{}',
			metadata_json JSONB DEFAULT '{}',
			fact_category TEXT,
			fact_subject TEXT, 
			fact_attribute TEXT,
			fact_value TEXT,
			fact_temporal_context TEXT,
			fact_sensitivity TEXT,
			fact_importance INTEGER,
			fact_file_path TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Documents table
		CREATE TABLE IF NOT EXISTS source_documents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL UNIQUE,
			document_type TEXT NOT NULL,
			original_id TEXT NOT NULL,
			metadata_json JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Document chunks table with vector support  
		CREATE TABLE IF NOT EXISTS document_chunks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			content_vector VECTOR(1536),
			chunk_index INTEGER NOT NULL,
			original_document_id TEXT NOT NULL,
			source TEXT NOT NULL,
			file_path TEXT,
			tags TEXT[] DEFAULT '{}',
			metadata_json JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	logger.Debug("PostgreSQL schema initialized successfully for tests")
	return nil
}

// initSimplePostgresSchemaForTests creates basic tables without pgvector for fallback.
func initSimplePostgresSchemaForTests(db *sql.DB, logger *log.Logger) error {
	logger.Debug("Initializing basic PostgreSQL schema for tests (no vector support)")

	// Create tables without vector columns for testing
	schema := `
		-- Basic memory facts table for testing (no vector column)
		CREATE TABLE IF NOT EXISTS memory_facts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			source TEXT NOT NULL,
			tags TEXT[] DEFAULT '{}',
			document_references TEXT[] DEFAULT '{}',
			metadata_json JSONB DEFAULT '{}',
			fact_category TEXT,
			fact_subject TEXT, 
			fact_attribute TEXT,
			fact_value TEXT,
			fact_temporal_context TEXT,
			fact_sensitivity TEXT,
			fact_importance INTEGER,
			fact_file_path TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Documents table
		CREATE TABLE IF NOT EXISTS source_documents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL UNIQUE,
			document_type TEXT NOT NULL,
			original_id TEXT NOT NULL,
			metadata_json JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		-- Document chunks table (no vector column for testing)
		CREATE TABLE IF NOT EXISTS document_chunks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			original_document_id TEXT NOT NULL,
			source TEXT NOT NULL,
			file_path TEXT,
			tags TEXT[] DEFAULT '{}',
			metadata_json JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create simple schema: %w", err)
	}

	logger.Debug("Simple PostgreSQL schema initialized successfully for tests")
	return nil
}

var (
	sharedPostgresContainer *bootstrap.PostgresTestContainer
	sharedPgConn            *pgx.Conn
	sharedLogger            *log.Logger
	sharedTempDir           string
	setupOnce               sync.Once
	teardownOnce            sync.Once
	mockServer              *httptest.Server
)

type testConfig struct {
	Source           string
	InputPath        string
	OutputPath       string
	CompletionsModel string
	EmbeddingsModel  string
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

// createMockAIService creates an AI service with a mock HTTP server that returns successful responses.
func createMockAIService(logger *log.Logger) *ai.Service {
	if mockServer == nil {
		// Create a mock HTTP server that returns successful OpenAI API responses
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Debug("Mock API called", "path", r.URL.Path, "method", r.Method)

			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(r.URL.Path, "embeddings") {
				// Mock embeddings response
				response := map[string]interface{}{
					"object": "list",
					"data": []map[string]interface{}{
						{
							"object":    "embedding",
							"index":     0,
							"embedding": make([]float64, 1536), // Standard OpenAI embedding size
						},
					},
					"model": "text-embedding-3-small",
					"usage": map[string]interface{}{
						"prompt_tokens": 8,
						"total_tokens":  8,
					},
				}

				// Generate deterministic but varied embeddings based on request content hash
				body, _ := io.ReadAll(r.Body)
				var requestData map[string]interface{}
				if err := json.Unmarshal(body, &requestData); err != nil {
					logger.Error("Failed to parse embeddings request", "error", err)
				}

				// Create a simple hash from the input text to make embeddings vary with content
				contentHash := 0
				if input, ok := requestData["input"].(string); ok {
					for _, char := range input {
						contentHash = (contentHash*31 + int(char)) % 10000
					}
				}

				embedding, ok := response["data"].([]map[string]interface{})[0]["embedding"].([]float64)
				if !ok {
					logger.Error("Failed to assert embedding type")
					return
				}
				for i := range embedding {
					// Create more varied embeddings based on content and position
					embedding[i] = float64(((i+contentHash)%200))/200.0 - 0.5
				}

				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("Failed to encode embeddings response", "error", err)
				}
				return
			}

			if strings.Contains(r.URL.Path, "chat/completions") {
				// Read the request body to determine which type of call this is
				body, err := io.ReadAll(r.Body)
				if err != nil {
					logger.Error("Failed to read request body", "error", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				// Parse request to determine context
				var requestData map[string]interface{}
				if err := json.Unmarshal(body, &requestData); err != nil {
					logger.Error("Failed to parse request JSON", "error", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				// Check if this is a memory decision call by looking for memory decision tools
				isMemoryDecision := false
				if tools, ok := requestData["tools"].([]interface{}); ok {
					for _, tool := range tools {
						if toolMap, ok := tool.(map[string]interface{}); ok {
							if function, ok := toolMap["function"].(map[string]interface{}); ok {
								if name, ok := function["name"].(string); ok {
									if name == "ADD" || name == "UPDATE" || name == "DELETE" || name == "NONE" {
										isMemoryDecision = true
										break
									}
								}
							}
						}
					}
				}

				var response map[string]interface{}

				if isMemoryDecision {
					// Memory decision response - return ADD action
					response = map[string]interface{}{
						"id":      "chatcmpl-mock-" + uuid.New().String(),
						"object":  "chat.completion",
						"created": time.Now().Unix(),
						"model":   "gpt-4o-mini",
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"message": map[string]interface{}{
									"role":    "assistant",
									"content": "",
									"tool_calls": []map[string]interface{}{
										{
											"id":   "call_mock_" + uuid.New().String(),
											"type": "function",
											"function": map[string]interface{}{
												"name":      "ADD",
												"arguments": `{}`,
											},
										},
									},
								},
								"finish_reason": "tool_calls",
							},
						},
						"usage": map[string]interface{}{
							"prompt_tokens":     20,
							"completion_tokens": 10,
							"total_tokens":      30,
						},
					}
				} else {
					// Fact extraction response - return EXTRACT_FACTS
					response = map[string]interface{}{
						"id":      "chatcmpl-mock-" + uuid.New().String(),
						"object":  "chat.completion",
						"created": time.Now().Unix(),
						"model":   "gpt-4o-mini",
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"message": map[string]interface{}{
									"role":    "assistant",
									"content": "",
									"tool_calls": []map[string]interface{}{
										{
											"id":   "call_mock_" + uuid.New().String(),
											"type": "function",
											"function": map[string]interface{}{
												"name": "EXTRACT_FACTS",
												"arguments": `{
												"facts": [
													{
														"category": "activity",
														"subject": "primaryUser",
														"attribute": "communication_activity",
														"value": "actively engages in conversations and communications",
														"sensitivity": "low",
														"importance": 2
													},
													{
														"category": "preference",
														"subject": "primaryUser",
														"attribute": "interaction_style",
														"value": "participates in digital communications and online activities",
														"sensitivity": "low",
														"importance": 2
													}
												]
											}`,
											},
										},
									},
								},
								"finish_reason": "tool_calls",
							},
						},
						"usage": map[string]interface{}{
							"prompt_tokens":     50,
							"completion_tokens": 100,
							"total_tokens":      150,
						},
					}
				}

				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("Failed to encode completions response", "error", err)
				}
				return
			}

			// Default response for unknown endpoints
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"error": "Mock endpoint not found: " + r.URL.Path,
			}); err != nil {
				logger.Error("Failed to encode error response", "error", err)
			}
		}))

		logger.Info("Created mock OpenAI API server", "url", mockServer.URL)
	}

	return ai.NewOpenAIService(logger, "mock-api-key", mockServer.URL)
}

func SetupSharedInfrastructure() {
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

		ctx := context.Background()

		// Setup PostgreSQL testcontainer with pgvector support
		var postgresContainer *bootstrap.PostgresTestContainer
		postgresContainer, err = bootstrap.SetupPostgresTestContainer(ctx, sharedLogger)
		if err != nil {
			panic(fmt.Sprintf("failed to start postgres testcontainer: %v", err))
		}

		// Store container reference for cleanup
		sharedPostgresContainer = postgresContainer

		// Initialize schema based on pgvector availability
		err = initPostgresSchemaForTests(sharedPostgresContainer.GetDB(), sharedLogger)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize postgres schema: %v", err))
		}

		// Log pgvector availability
		if sharedPostgresContainer.HasPgvector() {
			sharedLogger.Info("PostgreSQL testcontainer with pgvector extension ready for tests")
		} else {
			sharedLogger.Info("PostgreSQL testcontainer without pgvector ready for tests (vector operations will be skipped)")
		}

		// Create pgx connection for storage
		connString, err := sharedPostgresContainer.GetConnectionString(ctx)
		if err != nil {
			panic(fmt.Sprintf("failed to get connection string: %v", err))
		}
		sharedPgConn, err = pgx.Connect(ctx, connString)
		if err != nil {
			panic(fmt.Sprintf("failed to connect to postgres: %v", err))
		}

		sharedLogger.Info("Shared PostgreSQL infrastructure initialized successfully")
	})
}

func TeardownSharedInfrastructure() {
	teardownOnce.Do(func() {
		if mockServer != nil {
			sharedLogger.Info("Closing mock API server...")
			mockServer.Close()
			mockServer = nil
		}

		if sharedPgConn != nil {
			sharedLogger.Info("Closing PostgreSQL connection...")
			if err := sharedPgConn.Close(context.Background()); err != nil {
				sharedLogger.Error("Failed to close PostgreSQL connection", "error", err)
			}
		}

		if sharedPostgresContainer != nil {
			sharedLogger.Info("Stopping shared PostgreSQL testcontainer...")
			if err := sharedPostgresContainer.Cleanup(context.Background()); err != nil {
				sharedLogger.Error("Failed to stop PostgreSQL testcontainer", "error", err)
			}
		}

		if sharedTempDir != "" {
			if err := os.RemoveAll(sharedTempDir); err != nil {
				sharedLogger.Error("Failed to clean up temp directory", "error", err)
			}
		}

		// Clean up any remaining test artifact directories
		globalArtifactDirs := []string{
			"output",
			"pipeline_output",
			filepath.Join("pkg", "dataprocessing", "integration", "output"),
			filepath.Join("pkg", "dataprocessing", "integration", "pipeline_output"),
			filepath.Join("pkg", "dataprocessing", "gmail", "pipeline_output"),
			filepath.Join("cmd", "server", "output"),
			filepath.Join("cmd", "memory-processor-test", "pipeline_output"),
		}

		for _, dir := range globalArtifactDirs {
			if _, err := os.Stat(dir); err == nil {
				if err := os.RemoveAll(dir); err != nil {
					sharedLogger.Error("Failed to clean up global artifact directory", "dir", dir, "error", err)
				} else {
					sharedLogger.Info("Cleaned up global artifact directory", "dir", dir)
				}
			}
		}
	})
}

func ClearPostgresData(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	// Clear all data from PostgreSQL tables
	tables := []string{"memory_facts", "source_documents", "document_chunks"}

	for _, table := range tables {
		_, err := sharedPgConn.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Logf("Warning: Failed to clear %s table: %v", table, err)
		} else {
			t.Logf("Cleared data from %s table", table)
		}
	}
}

func SetupTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	SetupSharedInfrastructure()

	ClearPostgresData(t)

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
	store, err := db.NewStore(ctx, storePath, nil)
	require.NoError(t, err)

	completionsModel := config.CompletionsModel
	if completionsModel == "" {
		completionsModel = "gpt-4o-mini"
	}

	// Use mock AI service for deterministic testing
	mockAIService := createMockAIService(sharedLogger)
	mockEmbeddingsService := createMockAIService(sharedLogger)

	embeddingsWrapper, err := storage.NewEmbeddingWrapper(mockEmbeddingsService, config.EmbeddingsModel)
	if err != nil {
		t.Fatalf("Failed to create embedding wrapper: %v", err)
	}

	if sharedPostgresContainer == nil {
		t.Fatalf("Shared PostgreSQL container not initialized - ensure SetupSharedInfrastructure succeeded")
	}

	connString, err := sharedPostgresContainer.GetConnectionString(context.Background())
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	storageInterface, err := storage.NewPostgresStorage(storage.NewPostgresStorageInput{
		DB:                sharedPgConn,
		Logger:            sharedLogger,
		EmbeddingsWrapper: embeddingsWrapper,
		ConnString:        connString,
	})
	if err != nil {
		t.Fatalf("Failed to create storage interface: %v", err)
	}

	var mem evolvingmemory.MemoryStorage
	mem, err = evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             sharedLogger,
		Storage:            storageInterface,
		CompletionsService: mockAIService,
		CompletionsModel:   config.CompletionsModel,
		EmbeddingsWrapper:  embeddingsWrapper,
	})
	require.NoError(t, err)

	dataprocessingService := dataprocessing.NewDataProcessingService(mockAIService, completionsModel, store, mem, sharedLogger)

	sharedLogger.Info("=CONFIG=", "Model", config.CompletionsModel)
	sharedLogger.Info("=CONFIG=", "Model", config.EmbeddingsModel)

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

func (env *testEnvironment) Cleanup(t *testing.T) {
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

	// Clean up test artifact directories
	artifactDirs := []string{
		"output",
		"pipeline_output",
		filepath.Join("pkg", "dataprocessing", "integration", "output"),
		filepath.Join("pkg", "dataprocessing", "integration", "pipeline_output"),
		filepath.Join("pkg", "dataprocessing", "gmail", "pipeline_output"),
		filepath.Join("cmd", "server", "output"),
		filepath.Join("cmd", "memory-processor-test", "pipeline_output"),
	}

	for _, dir := range artifactDirs {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("Failed to clean up artifact directory %s: %v", dir, err)
			} else {
				t.Logf("Cleaned up artifact directory: %s", dir)
			}
		}
	}
}

func (env *testEnvironment) LoadDocuments(t *testing.T, source, inputPath string) {
	t.Helper()

	_, err := env.dataprocessing.ProcessSource(env.ctx, source, inputPath, env.config.OutputPath)
	require.NoError(t, err)

	// Check if this is a conversation processor that outputs ConversationDocuments directly
	sourceType := strings.ToLower(source)
	isConversationProcessor := sourceType == "telegram" || sourceType == "whatsapp" || sourceType == "gmail" || sourceType == "chatgpt"

	if isConversationProcessor {
		// Load ConversationDocuments from JSONL format using helper
		conversationDocs, err := memory.LoadConversationDocumentsFromJSON(env.config.OutputPath)
		require.NoError(t, err, "Failed to parse ConversationDocuments JSONL")

		// Convert to Document interface
		env.documents = make([]memory.Document, len(conversationDocs))
		for i := range conversationDocs {
			env.documents[i] = &conversationDocs[i]
		}

		env.logger.Info("Loaded ConversationDocuments from JSONL", "count", len(env.documents))
	} else {
		// Legacy path for processors that still use records
		fileContent, err := os.ReadFile(env.config.OutputPath)
		require.NoError(t, err)

		var allRecords []types.Record

		// Try to parse as JSON array first (for legacy processors that output JSON)
		trimmedContent := strings.TrimSpace(string(fileContent))
		if strings.HasPrefix(trimmedContent, "[") && strings.HasSuffix(trimmedContent, "]") {
			// This is a JSON array
			var jsonRecords []struct {
				Data      map[string]interface{} `json:"data"`
				Timestamp string                 `json:"timestamp"`
				Source    string                 `json:"source"`
			}

			if err := json.Unmarshal(fileContent, &jsonRecords); err == nil {
				// Successfully parsed as JSON array
				for _, jsonRecord := range jsonRecords {
					var timestamp time.Time
					if jsonRecord.Timestamp != "" {
						var err error
						timestamp, err = time.Parse(time.RFC3339, jsonRecord.Timestamp)
						if err != nil {
							t.Fatalf("Failed to parse timestamp: %v", err)
						}
					} else {
						// Use current time as default for empty timestamps
						timestamp = time.Now()
					}

					record := types.Record{
						Data:      jsonRecord.Data,
						Timestamp: timestamp,
						Source:    jsonRecord.Source,
					}
					allRecords = append(allRecords, record)
				}
				env.logger.Info("Loaded records from JSON array", "recordCount", len(allRecords))
			} else {
				t.Fatalf("Failed to parse JSON array: %v", err)
			}
		} else {
			// Fall back to JSONL parsing
			count, err := jsonHelpers.CountJSONLLines(env.config.OutputPath)
			if err != nil {
				t.Fatalf("Failed to count JSONL lines: %v", err)
			}

			batchSize := 3
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
		}

		// Convert records to documents using ToDocuments
		documents, err := env.dataprocessing.ToDocuments(env.ctx, source, allRecords)
		require.NoError(t, err)

		env.documents = documents
		env.logger.Info("Documents converted from records", "count", len(documents))
	}

	require.NotEmpty(t, env.documents, "no documents to test with")

	env.logger.Info("Final documents loaded", "count", len(env.documents))
	for i, doc := range env.documents {
		env.logger.Info("Document", "index", i, "id", doc.ID(), "source", doc.Source(), "content_preview", truncateString(doc.Content(), 150))
	}
}

func (env *testEnvironment) LoadDocumentsFromJSONL(t *testing.T, source, inputPath string) {
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

func (env *testEnvironment) StoreDocuments(t *testing.T) {
	env.StoreDocumentsWithTimeout(t, 50*time.Minute)
}

func (env *testEnvironment) StoreDocumentsWithTimeout(t *testing.T, timeout time.Duration) {
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
}

func getTestConfig(t *testing.T) testConfig {
	t.Helper()

	source := getEnvOrDefault("TEST_SOURCE", "synced-document")

	defaultInputPath := filepath.Join("testdata", "synced-document")
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

	// Use fixed models for deterministic testing with mocks
	completionsModel := "gpt-4o-mini"
	embeddingsModel := "text-embedding-3-small"

	return testConfig{
		Source:           source,
		InputPath:        inputPath,
		OutputPath:       outputPath,
		CompletionsModel: completionsModel,
		EmbeddingsModel:  embeddingsModel,
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

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("DataProcessingAndStorage", func(t *testing.T) {
		env.LoadDocuments(t, env.config.Source, env.config.InputPath)
		assert.NotEmpty(t, env.documents)
		env.logger.Info("Documents loaded", "count", len(env.documents))
		env.logger.Info("Documents", "documents", env.documents)
		for i := 0; i < len(env.documents); i++ {
			env.logger.Info("Document", "id", env.documents[i].ID(), "content", env.documents[i].Content())
		}

		env.StoreDocuments(t)
		env.logger.Info("Documents stored successfully")
	})

	t.Run("SourceFiltering", func(t *testing.T) {
		if len(env.documents) == 0 {
			env.LoadDocuments(t, env.config.Source, env.config.InputPath)
			env.StoreDocuments(t)
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
			env.LoadDocuments(t, env.config.Source, env.config.InputPath)
			env.StoreDocuments(t)
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

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	env.LoadDocuments(t, env.config.Source, env.config.InputPath)
	env.StoreDocuments(t)

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

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	env.LoadDocuments(t, env.config.Source, env.config.InputPath)
	assert.NotEmpty(t, env.documents)
	env.logger.Info("Documents loaded successfully", "count", len(env.documents))

	env.StoreDocuments(t)
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

func isDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	err := cmd.Run()
	return err == nil
}

func TestMain(m *testing.M) {
	if !isDockerAvailable() {
		fmt.Println("Docker is not available. Skipping integration tests.")
		os.Exit(0)
	}

	SetupSharedInfrastructure()

	code := m.Run()

	TeardownSharedInfrastructure()

	os.Exit(code)
}
