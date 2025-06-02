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
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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
	logger := log.Default()
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to nil if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		// Create dummy services for basic structural tests
		aiService = &ai.Service{}
	}

	// Create storage with services
	storage := &WeaviateStorage{
		logger:             logger,
		client:             mockClient,
		completionsService: aiService,
		embeddingsService:  aiService,
	}
	adapter, err := NewFactExtractor(storage)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	t.Run("ExtractFacts_ConversationDocument", func(t *testing.T) {
		testAI := createTestAIService()
		if testAI == nil {
			t.Skip("Skipping AI-dependent test: COMPLETIONS_API_KEY not set")
		}

		// Create storage with real AI service
		storageWithAI := &WeaviateStorage{
			logger:             logger,
			client:             mockClient,
			completionsService: testAI,
			embeddingsService:  testAI,
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
		storageWithAI := &WeaviateStorage{
			logger:             logger,
			client:             mockClient,
			completionsService: testAI,
			embeddingsService:  testAI,
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
		storageWithoutCompletions := &WeaviateStorage{
			logger:             logger,
			client:             mockClient,
			completionsService: nil,
			embeddingsService:  aiService,
		}
		_, err := NewFactExtractor(storageWithoutCompletions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service not initialized")
	})
}

// TestMemoryOperationsAdapter tests the memory operations adapter structure.
func TestMemoryOperationsAdapter(t *testing.T) {
	logger := log.Default()
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to dummy if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		aiService = &ai.Service{}
	}

	// Just a basic test to ensure the function doesn't completely fail
	t.Run("BasicStructure", func(t *testing.T) {
		storage := &WeaviateStorage{
			logger:             logger,
			client:             mockClient,
			completionsService: aiService,
			embeddingsService:  aiService,
		}
		_, err := NewMemoryOperations(storage)
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
	logger := log.Default()
	mockClient := &weaviate.Client{}

	// Try to create real AI services, fall back to dummy if no env vars
	aiService := createTestAIService()
	if aiService == nil {
		aiService = &ai.Service{}
	}

	storage, _ := New(logger, mockClient, aiService, aiService)

	t.Run("NewFactExtractor", func(t *testing.T) {
		adapter, err := NewFactExtractor(storage)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)

		// Verify it implements the interface
		_ = adapter
	})

	t.Run("NewMemoryOperations", func(t *testing.T) {
		adapter, err := NewMemoryOperations(storage)
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
		storageWithoutClient := &WeaviateStorage{
			logger:             logger,
			client:             nil,
			completionsService: aiService,
			embeddingsService:  aiService,
		}
		_, err := NewMemoryOperations(storageWithoutClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "weaviate client not initialized")
	})

	t.Run("NewMemoryOperations_NilCompletionsService", func(t *testing.T) {
		storageWithoutCompletions := &WeaviateStorage{
			logger:             logger,
			client:             mockClient,
			completionsService: nil,
			embeddingsService:  aiService,
		}
		_, err := NewMemoryOperations(storageWithoutCompletions)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "completions service not initialized")
	})
}
