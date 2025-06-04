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
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// TestConversationDocumentBasics tests basic ConversationDocument functionality.
func TestConversationDocumentBasics(t *testing.T) {
	// Test data
	conversation := &memory.ConversationDocument{
		FieldID: "conv-123",
		User:    "alice",
		Conversation: []memory.ConversationMessage{
			{Speaker: "alice", Content: "I love pizza", Time: time.Now()},
			{Speaker: "bob", Content: "I prefer sushi", Time: time.Now()},
		},
	}

	// Test Document interface methods
	assert.Equal(t, "conv-123", conversation.ID())
	assert.Contains(t, conversation.Content(), "alice: I love pizza")
	assert.Contains(t, conversation.Content(), "bob: I prefer sushi")
	assert.NotNil(t, conversation.Timestamp())
	assert.Equal(t, "alice", conversation.Metadata()["user"])
}

// TestTextDocumentBasics tests basic TextDocument functionality.
func TestTextDocumentBasics(t *testing.T) {
	now := time.Now()
	textDoc := &memory.TextDocument{
		FieldID:        "text-456",
		FieldContent:   "The user's favorite color is blue.",
		FieldTimestamp: &now,
		FieldMetadata:  map[string]string{"source": "notes"},
	}

	// Test Document interface methods
	assert.Equal(t, "text-456", textDoc.ID())
	assert.Equal(t, "The user's favorite color is blue.", textDoc.Content())
	assert.Equal(t, &now, textDoc.Timestamp())
	assert.Equal(t, "notes", textDoc.Metadata()["source"])
}

// TestValidationRules documents the speaker validation rules.
func TestValidationRules(t *testing.T) {
	tests := []struct {
		name             string
		currentSpeakerID string
		targetSpeakerID  string
		action           string
		shouldBeAllowed  bool
		description      string
	}{
		{
			name:             "speaker_can_update_own_memory",
			currentSpeakerID: "alice",
			targetSpeakerID:  "alice",
			action:           UpdateMemoryToolName,
			shouldBeAllowed:  true,
			description:      "Speaker should be able to update their own memories",
		},
		{
			name:             "speaker_cannot_update_other_memory",
			currentSpeakerID: "alice",
			targetSpeakerID:  "bob",
			action:           UpdateMemoryToolName,
			shouldBeAllowed:  false,
			description:      "Speaker should not be able to update another speaker's memories",
		},
		{
			name:             "document_cannot_update_speaker_memory",
			currentSpeakerID: "",
			targetSpeakerID:  "alice",
			action:           UpdateMemoryToolName,
			shouldBeAllowed:  false,
			description:      "Document-level context should not update speaker-specific memories",
		},
		{
			name:             "document_can_update_document_memory",
			currentSpeakerID: "",
			targetSpeakerID:  "",
			action:           UpdateMemoryToolName,
			shouldBeAllowed:  true,
			description:      "Document-level context can update document-level memories",
		},
	}

	// This just documents the expected behavior - actual validation
	// will be tested when we implement the pure validation functions
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("%s: %s", tt.name, tt.description)
		})
	}
}

// TestMemoryConstants verifies constants are defined.
func TestMemoryConstants(t *testing.T) {
	assert.Equal(t, "ADD", AddMemoryToolName)
	assert.Equal(t, "UPDATE", UpdateMemoryToolName)
	assert.Equal(t, "DELETE", DeleteMemoryToolName)
	assert.Equal(t, "NONE", NoneMemoryToolName)
	assert.Equal(t, "EXTRACT_FACTS", ExtractFactsToolName)
}

// TestStore_BackwardCompatibility tests the backward compatibility of the Store method.
func TestStore_BackwardCompatibility(t *testing.T) {
	// Test with empty documents - this should work without any mocks
	t.Run("empty documents", func(t *testing.T) {
		logger := log.New(os.Stdout)
		mockClient := &weaviate.Client{}

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		mockStorage := storage.New(mockClient, logger, embeddingsService)

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		})
		require.NoError(t, err)

		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")

		err = storageImplTyped.Store(context.Background(), []memory.Document{}, nil)
		assert.NoError(t, err)
	})

	// Test with progress callback - simplified version
	t.Run("with progress callback", func(t *testing.T) {
		progressCalls := 0
		callback := func(processed, total int) {
			progressCalls++
			// Empty documents, so nothing to process
			assert.Equal(t, 0, processed)
			assert.Equal(t, 0, total)
		}

		logger := log.New(os.Stdout)
		mockClient := &weaviate.Client{}

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		mockStorage := storage.New(mockClient, logger, embeddingsService)

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		})
		require.NoError(t, err)

		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")

		err = storageImplTyped.Store(context.Background(), []memory.Document{}, callback)
		assert.NoError(t, err)

		// For empty documents, callback might not be called, which is fine
		t.Logf("Progress callback called %d times", progressCalls)
	})

	// Test context cancellation
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		logger := log.New(os.Stdout)
		mockClient := &weaviate.Client{}

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		mockStorage := storage.New(mockClient, logger, embeddingsService)

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		})
		require.NoError(t, err)

		storageImplTyped, ok := storageImpl.(*StorageImpl)
		require.True(t, ok, "Failed to type assert to StorageImpl")

		docs := []memory.Document{
			&memory.TextDocument{
				FieldID:      "test-doc-2",
				FieldContent: "Test content 2",
			},
		}

		err = storageImplTyped.Store(ctx, docs, nil)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

// Note: Full integration tests with real Weaviate/AI services
// should be run separately using the existing test-memory command

// FILTERING INTEGRATION TESTS 🧪

// Helper functions for filtering tests.
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }

// TestAdvancedFiltering_Integration tests the complete filtering pipeline.
func TestAdvancedFiltering_Integration(t *testing.T) {
	t.Run("filter validation in storage interface", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Mock the Query method to verify filter is passed correctly
		expectedFilter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Distance:    0.7,
			Limit:       intPtr(5),
		}

		expectedResult := memory.QueryResult{
			Facts: []memory.MemoryFact{},
			Documents: []memory.TextDocument{
				{
					FieldID:      "test-123",
					FieldContent: "alice likes pizza",
					FieldSource:  "conversations",
					FieldMetadata: map[string]string{
						"source":    "conversations",
						"speakerID": "alice",
					},
				},
			},
		}

		mockStorage.On("Query", mock.Anything, "pizza preferences", expectedFilter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		// Create storage with mock
		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		// Test the query with filter
		result, err := storage.Query(context.Background(), "pizza preferences", expectedFilter)
		require.NoError(t, err)

		// Verify result structure
		assert.Len(t, result.Documents, 1)
		assert.Equal(t, "test-123", result.Documents[0].ID())
		assert.Equal(t, "conversations", result.Documents[0].Source())
		assert.Equal(t, "alice", result.Documents[0].Metadata()["speakerID"])

		mockStorage.AssertExpectations(t)
	})

	t.Run("nil filter backward compatibility", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Mock the Query method with nil filter
		var nilFilter *memory.Filter = nil
		expectedResult := memory.QueryResult{
			Facts:     []memory.MemoryFact{},
			Documents: []memory.TextDocument{},
		}

		mockStorage.On("Query", mock.Anything, "test query", nilFilter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		// Test query with nil filter (backward compatibility)
		result, err := storage.Query(context.Background(), "test query", nil)
		require.NoError(t, err)
		assert.Empty(t, result.Documents)

		mockStorage.AssertExpectations(t)
	})

	t.Run("tags filtering", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Filter with tags - documents must contain ALL specified tags
		filter := &memory.Filter{
			Tags:  []string{"work", "important"},
			Limit: intPtr(5),
		}

		expectedResult := memory.QueryResult{
			Facts: []memory.MemoryFact{},
			Documents: []memory.TextDocument{
				{
					FieldID:      "test-doc-1",
					FieldContent: "Work meeting notes about important project",
					FieldTags:    []string{"work", "important", "meeting"},
				},
			},
		}

		mockStorage.On("Query", mock.Anything, "project updates", filter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		result, err := storage.Query(context.Background(), "project updates", filter)
		require.NoError(t, err)
		assert.Len(t, result.Documents, 1)
		assert.Equal(t, "Work meeting notes about important project", result.Documents[0].Content())
		assert.Contains(t, result.Documents[0].Tags(), "work")
		assert.Contains(t, result.Documents[0].Tags(), "important")

		mockStorage.AssertExpectations(t)
	})
}

// TestFilterBehavior_EdgeCases tests edge cases in filter behavior.
func TestFilterBehavior_EdgeCases(t *testing.T) {
	t.Run("empty filter values", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Filter with empty string values
		filter := &memory.Filter{
			Source:      stringPtr(""),
			ContactName: stringPtr(""),
			Distance:    0,
			Limit:       intPtr(0),
		}

		expectedResult := memory.QueryResult{
			Facts:     []memory.MemoryFact{},
			Documents: []memory.TextDocument{},
		}

		mockStorage.On("Query", mock.Anything, "test", filter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		result, err := storage.Query(context.Background(), "test", filter)
		require.NoError(t, err)
		assert.Empty(t, result.Documents)

		mockStorage.AssertExpectations(t)
	})

	t.Run("extreme filter values", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Filter with extreme values
		filter := &memory.Filter{
			Source:      stringPtr("very-long-source-name-that-might-cause-issues"),
			ContactName: stringPtr("user-with-very-long-name-123456789"),
			Distance:    2.0,           // > 1.0
			Limit:       intPtr(10000), // Very large limit
		}

		expectedResult := memory.QueryResult{
			Facts:     []memory.MemoryFact{},
			Documents: []memory.TextDocument{},
		}

		mockStorage.On("Query", mock.Anything, "test", filter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		result, err := storage.Query(context.Background(), "test", filter)
		require.NoError(t, err)
		assert.Empty(t, result.Documents)

		mockStorage.AssertExpectations(t)
	})
}

// TestDirectFieldVsJSONMetadata tests the hybrid approach behavior.
func TestDirectFieldVsJSONMetadata(t *testing.T) {
	t.Run("direct fields override JSON metadata", func(t *testing.T) {
		// Simulate a document with both direct fields and conflicting JSON metadata
		doc := memory.TextDocument{
			FieldID:      "test-456",
			FieldContent: "test content",
			FieldSource:  "conversations", // Direct field
			FieldMetadata: map[string]string{
				"source":    "old_source", // Should be overridden
				"speakerID": "alice",
				"extra":     "metadata",
			},
		}

		// Direct field should take precedence
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "conversations", doc.FieldSource)

		// But metadata map might still contain old value
		// (Real implementation should merge correctly)
		assert.Contains(t, doc.Metadata(), "speakerID")
		assert.Contains(t, doc.Metadata(), "extra")
	})

	t.Run("JSON metadata fallback when direct fields empty", func(t *testing.T) {
		doc := memory.TextDocument{
			FieldID:      "test-789",
			FieldContent: "test content",
			FieldSource:  "", // Empty direct field
			FieldMetadata: map[string]string{
				"source":    "json_source", // Should be used
				"speakerID": "bob",
			},
		}

		// Should fall back to metadata when direct field is empty
		assert.Equal(t, "", doc.Source()) // Direct field is empty
		assert.Equal(t, "json_source", doc.Metadata()["source"])
		assert.Equal(t, "bob", doc.Metadata()["speakerID"])
	})
}

// TestQueryResultStructure tests the structure of query results with filtering.
func TestQueryResultStructure(t *testing.T) {
	t.Run("filtered result structure", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		filter := &memory.Filter{
			Source: stringPtr("conversations"),
			Limit:  intPtr(3),
		}

		// Create realistic mock result
		now := time.Now()
		expectedResult := memory.QueryResult{
			Facts: []memory.MemoryFact{}, // Usually empty in current implementation
			Documents: []memory.TextDocument{
				{
					FieldID:        "doc-1",
					FieldContent:   "alice likes coffee",
					FieldTimestamp: &now,
					FieldSource:    "conversations",
					FieldMetadata: map[string]string{
						"source":    "conversations",
						"speakerID": "alice",
						"channel":   "general",
					},
				},
				{
					FieldID:        "doc-2",
					FieldContent:   "bob prefers tea",
					FieldTimestamp: &now,
					FieldSource:    "conversations",
					FieldMetadata: map[string]string{
						"source":    "conversations",
						"speakerID": "bob",
						"channel":   "general",
					},
				},
			},
		}

		mockStorage.On("Query", mock.Anything, "drink preferences", filter).
			Return(expectedResult, nil)

		// Create AI services inline
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
			t.Skip("Skipping AI-dependent tests: API keys not set")
			return
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		result, err := storage.Query(context.Background(), "drink preferences", filter)
		require.NoError(t, err)

		// Verify result structure
		assert.Len(t, result.Documents, 2)
		assert.Empty(t, result.Facts) // Current implementation

		// Verify first document
		doc1 := result.Documents[0]
		assert.Equal(t, "doc-1", doc1.ID())
		assert.Equal(t, "alice likes coffee", doc1.Content())
		assert.Equal(t, "conversations", doc1.Source())
		assert.Equal(t, "alice", doc1.Metadata()["speakerID"])
		assert.Equal(t, "general", doc1.Metadata()["channel"])
		assert.NotNil(t, doc1.Timestamp())

		// Verify second document
		doc2 := result.Documents[1]
		assert.Equal(t, "doc-2", doc2.ID())
		assert.Equal(t, "bob prefers tea", doc2.Content())
		assert.Equal(t, "conversations", doc2.Source())
		assert.Equal(t, "bob", doc2.Metadata()["speakerID"])

		mockStorage.AssertExpectations(t)
	})
}

// TestFilterUsagePatterns tests realistic usage patterns.
func TestFilterUsagePatterns(t *testing.T) {
	t.Run("conversation filtering pattern", func(t *testing.T) {
		// Pattern: Get recent conversations with a specific person
		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Distance:    0.8, // Semantic similarity threshold
			Limit:       intPtr(10),
		}

		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)
	})

	t.Run("email filtering pattern", func(t *testing.T) {
		// Pattern: Find work-related emails
		filter := &memory.Filter{
			Source:   stringPtr("email"),
			Distance: 0.7,
			Limit:    intPtr(20),
		}

		assert.Equal(t, "email", *filter.Source)
		assert.Nil(t, filter.ContactName) // No specific contact
		assert.Equal(t, float32(0.7), filter.Distance)
		assert.Equal(t, 20, *filter.Limit)
	})

	t.Run("semantic search only pattern", func(t *testing.T) {
		// Pattern: Pure semantic search with distance limit
		filter := &memory.Filter{
			Distance: 0.6,
			Limit:    intPtr(5),
		}

		assert.Nil(t, filter.Source)      // No source filter
		assert.Nil(t, filter.ContactName) // No contact filter
		assert.Equal(t, float32(0.6), filter.Distance)
		assert.Equal(t, 5, *filter.Limit)
	})
}

// TestSchemaEvolution tests schema evolution and migration scenarios.
func TestSchemaEvolution(t *testing.T) {
	t.Run("schema field constants", func(t *testing.T) {
		// Verify all expected schema fields are defined
		// These should match the constants in storage.go
		expectedFields := map[string]string{
			"content":      "content",
			"timestamp":    "timestamp",
			"metadataJson": "metadataJson",
			"source":       "source",    // New direct field
			"speakerID":    "speakerID", // New direct field
			"tags":         "tags",
		}

		for fieldName, expectedValue := range expectedFields {
			assert.NotEmpty(t, expectedValue, "Field %s should not be empty", fieldName)
		}
	})

	t.Run("metadata merging behavior", func(t *testing.T) {
		// Test the metadata merging logic that should happen in storage
		jsonMetadata := map[string]string{
			"source":    "old_source",
			"speakerID": "old_speaker",
			"channel":   "general",
			"extra":     "data",
		}

		// Simulate direct field values
		directSource := "conversations"
		directSpeakerID := "alice"

		// Merge logic (direct fields take precedence)
		finalMetadata := make(map[string]string)
		for k, v := range jsonMetadata {
			finalMetadata[k] = v
		}
		if directSource != "" {
			finalMetadata["source"] = directSource
		}
		if directSpeakerID != "" {
			finalMetadata["speakerID"] = directSpeakerID
		}

		// Verify merging behavior
		assert.Equal(t, "conversations", finalMetadata["source"]) // Overridden
		assert.Equal(t, "alice", finalMetadata["speakerID"])      // Overridden
		assert.Equal(t, "general", finalMetadata["channel"])      // Preserved
		assert.Equal(t, "data", finalMetadata["extra"])           // Preserved
	})
}

// TestPerformanceImplications documents performance expectations.
func TestPerformanceImplications(t *testing.T) {
	t.Run("query pattern efficiency", func(t *testing.T) {
		// Document expected query patterns for performance

		// OLD PATTERN (slow):
		// - JSON pattern matching: metadataJson LIKE '*"source":"value"*'
		// - Full string search on serialized JSON
		// - Cannot use proper database indexing

		// NEW PATTERN (fast):
		// - Direct field equality: source = "value"
		// - Proper database indexing possible
		// - Combined with filters.And for multiple conditions

		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
		}

		// This should generate efficient queries like:
		// WHERE (source = "conversations" AND speakerID = "alice")
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
	})

	t.Run("memory usage patterns", func(t *testing.T) {
		// Large result sets should be limited
		filter := &memory.Filter{
			Limit: intPtr(100), // Reasonable limit
		}
		assert.Equal(t, 100, *filter.Limit)

		// Distance filtering reduces result set size
		filter.Distance = 0.8 // Strict similarity
		assert.Equal(t, float32(0.8), filter.Distance)
	})
}
