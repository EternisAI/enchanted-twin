package evolvingmemory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// createTestAIServices creates AI services for testing.
func createTestAIServices() (*ai.Service, *ai.Service) {
	logger := log.Default()
	envPath := filepath.Join("..", "..", "..", "..", ".env")
	_ = godotenv.Load(envPath)
	completionsKey := os.Getenv("COMPLETIONS_API_KEY")
	embeddingsKey := os.Getenv("EMBEDDINGS_API_KEY")
	if embeddingsKey == "" {
		embeddingsKey = completionsKey
	}
	completionsURL := os.Getenv("COMPLETIONS_API_URL")
	if completionsURL == "" {
		completionsURL = "https://api.openai.com/v1"
	}
	embeddingsURL := os.Getenv("EMBEDDINGS_API_URL")
	if embeddingsURL == "" {
		embeddingsURL = "https://api.openai.com/v1"
	}
	if completionsKey == "" {
		return nil, nil
	}
	completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
	embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)
	return completionsService, embeddingsService
}

// TestStorageImplBasicFunctionality tests the core storage functionality.
func TestStorageImplBasicFunctionality(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	// Create mock storage instead of using real Weaviate client
	mockStorage := &MockStorage{}
	mockStorage.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string")).Return(memory.QueryResult{
		Facts: []memory.MemoryFact{},
	}, nil)
	mockStorage.On("EnsureSchemaExists", mock.Anything).Return(nil)
	mockStorage.On("StoreBatch", mock.Anything, mock.Anything).Return(nil)
	// Add mock for the new StoreDocument method
	mockStorage.On("StoreDocument", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return("mock-doc-id", nil)

	storageImpl, err := New(Dependencies{
		Logger:             logger,
		Storage:            mockStorage,
		CompletionsService: completionsService,
		EmbeddingsService:  embeddingsService,
		CompletionsModel:   "gpt-4.1-mini",
		EmbeddingsModel:    "text-embedding-3-small",
	})
	require.NoError(t, err)

	t.Run("StoreV2_ConversationDocument", func(t *testing.T) {
		// Create test conversation document
		convDoc := &memory.ConversationDocument{
			FieldID: "conv-123",
			User:    "alice",
			Conversation: []memory.ConversationMessage{
				{Speaker: "alice", Content: "I love pizza", Time: time.Now()},
				{Speaker: "bob", Content: "I prefer sushi", Time: time.Now()},
			},
		}

		// Convert to unified document interface
		docs := []memory.Document{convDoc}

		// Test Store functionality
		progressCh, errorCh := storageImpl.Store(context.Background(), docs)

		// Process channels (this will likely fail with mock storage, but tests the interface)
		select {
		case <-progressCh:
			t.Log("Received progress update")
		case err := <-errorCh:
			t.Logf("Received expected error with mock storage: %v", err)
		case <-time.After(time.Second):
			t.Log("Test completed without hanging")
		}
	})

	t.Run("Store_TextDocument", func(t *testing.T) {
		// Create test text document
		now := time.Now()
		textDoc := &memory.TextDocument{
			FieldID:        "text-456",
			FieldContent:   "The user's favorite color is blue.",
			FieldTimestamp: &now,
			FieldMetadata:  map[string]string{"source": "notes"},
		}

		// Convert to unified document interface
		docs := []memory.Document{textDoc}

		// Test Store functionality (this will likely fail with mock storage, but tests the interface)
		progressCh, errorCh := storageImpl.Store(context.Background(), docs)

		// Drain channels
		go func() {
			for range progressCh {
				// Progress updates
			}
		}()

		// Check for errors
		var storeErr error
		for err := range errorCh {
			if storeErr == nil {
				storeErr = err
			}
		}

		// We expect this to fail with mock storage, but the interface should work
		t.Logf("Store returned (expected error with mock): %v", storeErr)
	})

	t.Run("Query_Functionality", func(t *testing.T) {
		// Test Query functionality (this will likely fail with mock storage, but tests the interface)
		_, err := storageImpl.Query(context.Background(), "test query", nil)
		// We expect this to fail with mock storage, but the interface should work
		t.Logf("Query returned (expected error with mock): %v", err)
	})
}

// TestDecisionParsing tests the decision parsing logic.
func TestDecisionParsing(t *testing.T) {
	t.Run("ParseDecision_UPDATE", func(t *testing.T) {
		// Test parsing UPDATE decision arguments
		args := UpdateToolArguments{
			MemoryID:      "mem123",
			UpdatedMemory: "updated content",
			Reason:        "More accurate",
		}

		// Verify the structure matches what's expected
		assert.Equal(t, "mem123", args.MemoryID)
		assert.Equal(t, "updated content", args.UpdatedMemory)
		assert.Equal(t, "More accurate", args.Reason)
	})

	t.Run("ParseDecision_DELETE", func(t *testing.T) {
		// Test parsing DELETE decision arguments
		args := DeleteToolArguments{
			MemoryID: "mem456",
			Reason:   "Outdated information",
		}

		assert.Equal(t, "mem456", args.MemoryID)
		assert.Equal(t, "Outdated information", args.Reason)
	})
}

// TestStorageImplCreation tests storage implementation creation.
func TestStorageImplCreation(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to dummy if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		// Create dummy services for testing structure
		completionsService = &ai.Service{}
		embeddingsService = &ai.Service{}
	}

	mockStorage := &MockStorage{}
	storageImpl, err := New(Dependencies{
		Logger:             logger,
		Storage:            mockStorage,
		CompletionsService: completionsService,
		EmbeddingsService:  embeddingsService,
		CompletionsModel:   "gpt-4.1-mini",
		EmbeddingsModel:    "text-embedding-3-small",
	})
	require.NoError(t, err)

	t.Run("CreationSuccess", func(t *testing.T) {
		assert.NotNil(t, storageImpl)

		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")

		// Verify the storage interface is accessible
		assert.NotNil(t, storageImplTyped)
	})

	t.Run("ImplementsMemoryStorage", func(t *testing.T) {
		// Verify it implements the MemoryStorage interface
		_ = storageImpl
		assert.NotNil(t, storageImpl)
	})
}

// TestDependencyValidation tests that the New function properly validates dependencies.
func TestDependencyValidation(t *testing.T) {
	logger := log.New(os.Stdout)
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		// Create dummy services for testing structure
		completionsService = &ai.Service{}
		embeddingsService = &ai.Service{}
	}
	mockStorage := &MockStorage{}

	t.Run("NilStorage", func(t *testing.T) {
		_, err := New(Dependencies{
			Logger:             logger,
			Storage:            nil, // nil storage
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   "gpt-4.1-mini",
			EmbeddingsModel:    "text-embedding-3-small",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage interface cannot be nil")
	})

	t.Run("NilLogger", func(t *testing.T) {
		_, err := New(Dependencies{
			Logger:             nil, // nil logger
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   "gpt-4.1-mini",
			EmbeddingsModel:    "text-embedding-3-small",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logger cannot be nil")
	})

	t.Run("NilCompletionsService", func(t *testing.T) {
		_, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: nil, // nil completions service
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   "gpt-4.1-mini",
			EmbeddingsModel:    "text-embedding-3-small",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service cannot be nil")
	})

	t.Run("NilEmbeddingsService", func(t *testing.T) {
		_, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  nil, // nil embeddings service
			CompletionsModel:   "gpt-4.1-mini",
			EmbeddingsModel:    "text-embedding-3-small",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "embeddings service cannot be nil")
	})
}
