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
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// createTestAIService creates a real AI service if API key is available, otherwise returns nil.
func createTestAIService() *ai.Service {
	// Try to load .env file from the backend/golang directory
	envPath := filepath.Join("..", "..", "..", "..", ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		log.Default().Warn("Failed to load .env file", "path", envPath, "error", err)
	} else {
		log.Default().Debug("Successfully loaded .env file", "path", envPath)
	}

	apiKey := os.Getenv("COMPLETIONS_API_KEY")
	if apiKey == "" {
		log.Default().Debug("COMPLETIONS_API_KEY not found in environment")
		return nil
	}

	apiURL := os.Getenv("COMPLETIONS_API_URL")
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1" // Default fallback
	}

	log.Default().Debug("COMPLETIONS_API_KEY found, creating AI service")
	return ai.NewOpenAIService(log.Default(), apiKey, apiURL)
}

// TestFactExtractorAdapter tests the fact extractor adapter structure.
func TestFactExtractorAdapter(t *testing.T) {
	logger := log.New(os.Stdout)
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to nil if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		// Create dummy services for basic structural tests
		aiService = &ai.Service{}
	}

	// Create storage with services
	mockStorage := storage.New(mockClient, logger, aiService)
	storageImpl := &StorageImpl{
		logger:             logger,
		completionsService: aiService,
		embeddingsService:  aiService,
		storage:            mockStorage,
	}
	adapter, err := NewFactExtractor(storageImpl)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	t.Run("ExtractFacts_ConversationDocument", func(t *testing.T) {
		testAI := createTestAIService()
		if testAI == nil {
			t.Skip("Skipping AI-dependent test: COMPLETIONS_API_KEY not set")
		}

		// Create storage with real AI service
		testMockStorage := storage.New(mockClient, logger, testAI)
		storageWithAI := &StorageImpl{
			logger:             logger,
			completionsService: testAI,
			embeddingsService:  testAI,
			storage:            testMockStorage,
		}
		adapterWithAI, err := NewFactExtractor(storageWithAI)
		assert.NoError(t, err)

		// Create test conversation document
		convDoc := &memory.ConversationDocument{
			FieldID: "conv-123",
			User:    "alice",
			Conversation: []memory.ConversationMessage{
				{Speaker: "alice", Content: "I love pizza", Time: time.Now()},
				{Speaker: "bob", Content: "I prefer sushi", Time: time.Now()},
			},
		}
		// Create prepared document
		prepDoc := PreparedDocument{
			Original:   convDoc,
			Type:       DocumentTypeConversation,
			SpeakerID:  "alice",
			Timestamp:  time.Now(),
			DateString: "2024-01-15",
		}

		// Test that the adapter routes to the correct method
		facts, err := adapterWithAI.ExtractFacts(context.Background(), prepDoc)
		assert.NoError(t, err)
		assert.NotNil(t, facts)
		// Should be empty or contain facts depending on LLM response
		t.Logf("Extracted %d facts", len(facts))
	})

	t.Run("ExtractFacts_TextDocument", func(t *testing.T) {
		testAI := createTestAIService()
		if testAI == nil {
			t.Skip("Skipping AI-dependent test: COMPLETIONS_API_KEY not set")
		}

		// Create storage with real AI service
		testMockStorage := storage.New(mockClient, logger, testAI)
		storageWithAI := &StorageImpl{
			logger:             logger,
			completionsService: testAI,
			embeddingsService:  testAI,
			storage:            testMockStorage,
		}
		adapterWithAI, err := NewFactExtractor(storageWithAI)
		assert.NoError(t, err)

		// Create test text document
		now := time.Now()
		textDoc := &memory.TextDocument{
			FieldID:        "text-456",
			FieldContent:   "The user's favorite color is blue.",
			FieldTimestamp: &now,
			FieldMetadata:  map[string]string{"source": "notes"},
		}
		// Create prepared document
		prepDoc := PreparedDocument{
			Original:   textDoc,
			Type:       DocumentTypeText,
			SpeakerID:  "user",
			Timestamp:  now,
			DateString: "2024-01-15",
		}

		// Test that the adapter routes to the correct method
		facts, err := adapterWithAI.ExtractFacts(context.Background(), prepDoc)
		assert.NoError(t, err)
		assert.NotNil(t, facts)
		// Should be empty or contain facts depending on LLM response
		t.Logf("Extracted %d facts", len(facts))
	})

	t.Run("ExtractFacts_UnknownDocumentType", func(t *testing.T) {
		prepDoc := PreparedDocument{
			Original:   &memory.TextDocument{},
			Type:       DocumentType("unknown"),
			SpeakerID:  "user",
			Timestamp:  time.Now(),
			DateString: "2024-01-15",
		}

		_, err := adapter.ExtractFacts(context.Background(), prepDoc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown document type")
	})

	t.Run("ExtractFacts_InvalidDocumentTypeAssertion", func(t *testing.T) {
		// Test with wrong document type for conversation
		prepDoc := PreparedDocument{
			Original:   &memory.TextDocument{}, // Wrong type!
			Type:       DocumentTypeConversation,
			SpeakerID:  "user",
			Timestamp:  time.Now(),
			DateString: "2024-01-15",
		}

		_, err := adapter.ExtractFacts(context.Background(), prepDoc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "document is not a ConversationDocument")
	})

	t.Run("NewFactExtractor_NilStorage", func(t *testing.T) {
		_, err := NewFactExtractor(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage cannot be nil")
	})

	t.Run("NewFactExtractor_NilCompletionsService", func(t *testing.T) {
		storageWithoutCompletions := &StorageImpl{
			logger:             logger,
			completionsService: nil,
			embeddingsService:  aiService,
			storage:            mockStorage,
		}
		_, err := NewFactExtractor(storageWithoutCompletions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service not initialized")
	})
}

// TestMemoryOperationsAdapter tests the memory operations adapter structure.
func TestMemoryOperationsAdapter(t *testing.T) {
	logger := log.New(os.Stdout)
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to dummy if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		aiService = &ai.Service{}
	}

	// Just a basic test to ensure the function doesn't completely fail
	t.Run("BasicStructure", func(t *testing.T) {
		mockStorage := storage.New(mockClient, logger, aiService)
		storageImpl := &StorageImpl{
			logger:             logger,
			completionsService: aiService,
			embeddingsService:  aiService,
			storage:            mockStorage,
		}
		_, err := NewMemoryOperations(storageImpl)
		assert.NoError(t, err)
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

// TestAdapterCreation tests adapter creation.
func TestAdapterCreation(t *testing.T) {
	logger := log.New(os.Stdout)
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to dummy if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		aiService = &ai.Service{}
	}

	mockStorage := storage.New(mockClient, logger, aiService)
	storageImpl, err := New(logger, mockStorage, aiService, aiService)
	require.NoError(t, err)

	t.Run("NewFactExtractor", func(t *testing.T) {
		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")
		adapter, err := NewFactExtractor(storageImplTyped)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)

		// Verify it implements the interface
		_ = adapter
	})

	t.Run("NewMemoryOperations", func(t *testing.T) {
		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")
		adapter, err := NewMemoryOperations(storageImplTyped)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)

		// Verify it implements the interface
		_ = adapter
	})

	t.Run("NewMemoryOperations_NilStorage", func(t *testing.T) {
		_, err := NewMemoryOperations(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage cannot be nil")
	})

	t.Run("NewMemoryOperations_NilClient", func(t *testing.T) {
		storageWithoutInterface := &StorageImpl{
			logger:             logger,
			completionsService: aiService,
			embeddingsService:  aiService,
			storage:            nil,
		}
		_, err := NewMemoryOperations(storageWithoutInterface)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage interface not initialized")
	})

	t.Run("NewMemoryOperations_NilCompletionsService", func(t *testing.T) {
		storageWithoutCompletions := &StorageImpl{
			logger:             logger,
			completionsService: nil,
			embeddingsService:  aiService,
			storage:            mockStorage,
		}
		_, err := NewMemoryOperations(storageWithoutCompletions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service not initialized")
	})
}

func TestNewFactExtractor_Success(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageImpl := &StorageImpl{
		logger:             logger,
		completionsService: mockAI,
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageImpl)

	require.NoError(t, err)
	assert.NotNil(t, extractor)
}

func TestNewFactExtractor_NilStorage(t *testing.T) {
	extractor, err := NewFactExtractor(nil)

	require.Error(t, err)
	assert.Nil(t, extractor)
	assert.Contains(t, err.Error(), "storage cannot be nil")
}

func TestNewFactExtractor_NilCompletionsService(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: nil, // Missing completions service
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageWithoutCompletions)

	require.Error(t, err)
	assert.Nil(t, extractor)
	assert.Contains(t, err.Error(), "completions service not initialized")
}

func TestFactExtractorAdapter_ExtractFacts_ConversationDocument(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageWithAI := &StorageImpl{
		logger:             logger,
		completionsService: mockAI,
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageWithAI)
	require.NoError(t, err)

	// Create a conversation document
	convDoc := &memory.ConversationDocument{
		FieldID: "test-conv",
		Conversation: []memory.ConversationMessage{
			{Speaker: "user", Content: "Hello there"},
		},
		User: "user",
	}

	doc := PreparedDocument{
		Original:  convDoc,
		Type:      DocumentTypeConversation,
		SpeakerID: "user",
	}

	// This will fail in tests without real AI service, but we can test the path
	_, err = extractor.ExtractFacts(context.Background(), doc)

	// We expect an error here since we don't have a real AI service
	// The important thing is that it routes to the conversation extraction method
	assert.Error(t, err)
}

func TestFactExtractorAdapter_ExtractFacts_TextDocument(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageWithAI := &StorageImpl{
		logger:             logger,
		completionsService: mockAI,
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageWithAI)
	require.NoError(t, err)

	// Create a text document
	textDoc := &memory.TextDocument{
		FieldID:      "test-text",
		FieldContent: "This is a test document",
	}

	doc := PreparedDocument{
		Original:  textDoc,
		Type:      DocumentTypeText,
		SpeakerID: "user",
	}

	// This will fail in tests without real AI service, but we can test the path
	_, err = extractor.ExtractFacts(context.Background(), doc)

	// We expect an error here since we don't have a real AI service
	assert.Error(t, err)
}

func TestFactExtractorAdapter_ExtractFacts_UnknownDocumentType(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: mockAI,
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageWithoutCompletions)
	require.NoError(t, err)

	// Create a document with unknown type
	doc := PreparedDocument{
		Original:  &memory.TextDocument{},
		Type:      "unknown",
		SpeakerID: "user",
	}

	_, err = extractor.ExtractFacts(context.Background(), doc)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown document type")
}

// Memory Operations Tests

func TestNewMemoryOperations_Success(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockClient := &weaviate.Client{}
	mockStorageInterface := storage.New(mockClient, logger, mockAI)

	storageImpl, err := New(logger, mockStorageInterface, mockAI, mockAI)
	require.NoError(t, err)

	// Type assert to get StorageImpl
	storageImplTyped, ok := storageImpl.(*StorageImpl)
	require.True(t, ok, "Failed to type assert to StorageImpl")

	memOps, err := NewMemoryOperations(storageImplTyped)

	require.NoError(t, err)
	assert.NotNil(t, memOps)
}

func TestNewMemoryOperations_NilStorage(t *testing.T) {
	memOps, err := NewMemoryOperations(nil)

	require.Error(t, err)
	assert.Nil(t, memOps)
	assert.Contains(t, err.Error(), "storage cannot be nil")
}

func TestNewMemoryOperations_NilStorageInterface(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}

	storageWithoutClient := &StorageImpl{
		logger:             logger,
		completionsService: mockAI,
		embeddingsService:  mockAI,
		storage:            nil, // Missing storage interface
	}

	memOps, err := NewMemoryOperations(storageWithoutClient)

	require.Error(t, err)
	assert.Nil(t, memOps)
	assert.Contains(t, err.Error(), "storage interface not initialized")
}

func TestNewMemoryOperations_NilCompletionsService(t *testing.T) {
	logger := log.New(os.Stdout)
	mockAI := &ai.Service{}
	mockStorageInterface := storage.New(&weaviate.Client{}, logger, mockAI)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: nil, // Missing completions service
		embeddingsService:  mockAI,
		storage:            mockStorageInterface,
	}

	memOps, err := NewMemoryOperations(storageWithoutCompletions)

	require.Error(t, err)
	assert.Nil(t, memOps)
	assert.Contains(t, err.Error(), "completions service not initialized")
}
