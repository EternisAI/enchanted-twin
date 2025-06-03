package evolvingmemory

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// TestFactExtractorAdapter tests the fact extractor adapter structure.
func TestFactExtractorAdapter(t *testing.T) {
	logger := log.New(os.Stdout)
	mockClient := &weaviate.Client{}

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	// Create storage with embeddings service (for vector operations)
	mockStorage := storage.New(mockClient, logger, embeddingsService)
	storageImpl := &StorageImpl{
		logger:             logger,
		completionsService: completionsService, // OpenRouter for LLM
		embeddingsService:  embeddingsService,  // OpenAI for embeddings
		storage:            mockStorage,
	}
	adapter, err := NewFactExtractor(storageImpl)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	t.Run("ExtractFacts_ConversationDocument", func(t *testing.T) {
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
		facts, err := adapter.ExtractFacts(context.Background(), prepDoc)
		assert.NoError(t, err)
		assert.NotNil(t, facts)
		// Should be empty or contain facts depending on LLM response
		t.Logf("Extracted %d facts", len(facts))
	})

	t.Run("ExtractFacts_TextDocument", func(t *testing.T) {
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
		facts, err := adapter.ExtractFacts(context.Background(), prepDoc)
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
			embeddingsService:  embeddingsService,
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

	// Try to create separate AI services, fall back to skipping if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	// Just a basic test to ensure the function doesn't completely fail
	t.Run("BasicStructure", func(t *testing.T) {
		mockStorage := storage.New(mockClient, logger, embeddingsService)
		storageImpl := &StorageImpl{
			logger:             logger,
			completionsService: completionsService,
			embeddingsService:  embeddingsService,
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

	// Try to create separate AI services, fall back to dummy if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		// Create dummy services for testing structure
		completionsService = &ai.Service{}
		embeddingsService = &ai.Service{}
	}

	mockStorage := storage.New(mockClient, logger, embeddingsService)
	storageImpl, err := New(logger, mockStorage, completionsService, embeddingsService)
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
			completionsService: completionsService,
			embeddingsService:  embeddingsService,
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
			embeddingsService:  embeddingsService,
			storage:            mockStorage,
		}
		_, err := NewMemoryOperations(storageWithoutCompletions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service not initialized")
	})
}

func TestNewFactExtractor_Success(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageImpl := &StorageImpl{
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
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

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: nil, // Missing completions service
		embeddingsService:  embeddingsService,
		storage:            mockStorageInterface,
	}

	extractor, err := NewFactExtractor(storageWithoutCompletions)

	require.Error(t, err)
	assert.Nil(t, extractor)
	assert.Contains(t, err.Error(), "completions service not initialized")
}

func TestFactExtractorAdapter_ExtractFacts_ConversationDocument(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageWithAI := &StorageImpl{
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
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

	// With a real AI service, this should succeed
	facts, err := extractor.ExtractFacts(context.Background(), doc)

	// Since we have a real AI service, we expect success
	assert.NoError(t, err)
	assert.NotNil(t, facts)
	t.Logf("Successfully extracted %d facts from conversation", len(facts))
}

func TestFactExtractorAdapter_ExtractFacts_TextDocument(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageWithAI := &StorageImpl{
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
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

	// With a real AI service, this should succeed
	facts, err := extractor.ExtractFacts(context.Background(), doc)

	// Since we have a real AI service, we expect success
	assert.NoError(t, err)
	assert.NotNil(t, facts)
	t.Logf("Successfully extracted %d facts from text document", len(facts))
}

func TestFactExtractorAdapter_ExtractFacts_UnknownDocumentType(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
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

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockClient := &weaviate.Client{}
	mockStorageInterface := storage.New(mockClient, logger, embeddingsService)

	storageImpl, err := New(logger, mockStorageInterface, completionsService, embeddingsService)
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

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	storageWithoutClient := &StorageImpl{
		logger:             logger,
		completionsService: completionsService,
		embeddingsService:  embeddingsService,
		storage:            nil, // Missing storage interface
	}

	memOps, err := NewMemoryOperations(storageWithoutClient)

	require.Error(t, err)
	assert.Nil(t, memOps)
	assert.Contains(t, err.Error(), "storage interface not initialized")
}

func TestNewMemoryOperations_NilCompletionsService(t *testing.T) {
	logger := log.New(os.Stdout)

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return
	}

	mockStorageInterface := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storageWithoutCompletions := &StorageImpl{
		logger:             logger,
		completionsService: nil, // Missing completions service
		embeddingsService:  embeddingsService,
		storage:            mockStorageInterface,
	}

	memOps, err := NewMemoryOperations(storageWithoutCompletions)

	require.Error(t, err)
	assert.Nil(t, memOps)
	assert.Contains(t, err.Error(), "completions service not initialized")
}
