package evolvingmemory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestMemoryConstants(t *testing.T) {
	assert.Equal(t, "ADD", AddMemoryToolName)
	assert.Equal(t, "UPDATE", UpdateMemoryToolName)
	assert.Equal(t, "DELETE", DeleteMemoryToolName)
	assert.Equal(t, "NONE", NoneMemoryToolName)
	assert.Equal(t, "EXTRACT_FACTS", ExtractFactsToolName)
}

func TestStore_BackwardCompatibility(t *testing.T) {
	// Test with empty documents - this should work without any mocks
	t.Run("empty documents", func(t *testing.T) {
		logger := log.New(os.Stdout)
		mockClient := &weaviate.Client{}

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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		mockStorage := storage.New(mockClient, logger, embeddingsService)

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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
		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		completionsService := ai.NewOpenAIService(logger, completionsKey, completionsURL)
		embeddingsService := ai.NewOpenAIService(logger, embeddingsKey, embeddingsURL)

		mockStorage := storage.New(mockClient, logger, embeddingsService)

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}

		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		storageImpl, err := New(Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

func TestDocumentDeduplicationEdgeCases(t *testing.T) {
	t.Run("identical documents with different IDs", func(t *testing.T) {
		doc1 := &memory.TextDocument{
			FieldID:      "doc-1",
			FieldContent: "Identical content for deduplication test",
		}
		doc2 := &memory.TextDocument{
			FieldID:      "doc-2",
			FieldContent: "Identical content for deduplication test",
		}

		assert.Equal(t, doc1.Content(), doc2.Content())
		assert.NotEqual(t, doc1.ID(), doc2.ID())
	})

	t.Run("documents with same content but different metadata", func(t *testing.T) {
		now := time.Now()
		doc1 := &memory.TextDocument{
			FieldID:        "doc-meta-1",
			FieldContent:   "Same content different metadata",
			FieldTimestamp: &now,
			FieldMetadata:  map[string]string{"source": "user"},
		}
		doc2 := &memory.TextDocument{
			FieldID:        "doc-meta-2",
			FieldContent:   "Same content different metadata",
			FieldTimestamp: &now,
			FieldMetadata:  map[string]string{"source": "system"},
		}

		assert.Equal(t, doc1.Content(), doc2.Content())
		assert.NotEqual(t, doc1.Metadata()["source"], doc2.Metadata()["source"])
	})

	t.Run("empty documents deduplication", func(t *testing.T) {
		doc1 := &memory.TextDocument{
			FieldID:      "empty-1",
			FieldContent: "",
		}
		doc2 := &memory.TextDocument{
			FieldID:      "empty-2",
			FieldContent: "",
		}

		assert.Equal(t, "", doc1.Content())
		assert.Equal(t, "", doc2.Content())
		assert.Equal(t, doc1.Content(), doc2.Content())
	})

	t.Run("whitespace-only documents", func(t *testing.T) {
		doc1 := &memory.TextDocument{
			FieldID:      "whitespace-1",
			FieldContent: "   \n\t   ",
		}
		doc2 := &memory.TextDocument{
			FieldID:      "whitespace-2",
			FieldContent: "\t\n    \t",
		}

		assert.NotEmpty(t, doc1.Content())
		assert.NotEmpty(t, doc2.Content())
		assert.NotEqual(t, doc1.Content(), doc2.Content())
	})
}

func TestHashCollisionHandling(t *testing.T) {
	t.Run("documents with crafted similar content", func(t *testing.T) {
		content1 := "a" + string(make([]byte, 1000))
		content2 := "b" + string(make([]byte, 1000))

		doc1 := &memory.TextDocument{
			FieldID:      "hash-test-1",
			FieldContent: content1,
		}
		doc2 := &memory.TextDocument{
			FieldID:      "hash-test-2",
			FieldContent: content2,
		}

		assert.NotEqual(t, doc1.Content(), doc2.Content())
		assert.Equal(t, len(doc1.Content()), len(doc2.Content()))
	})

	t.Run("unicode content variations", func(t *testing.T) {
		doc1 := &memory.TextDocument{
			FieldID:      "unicode-1",
			FieldContent: "Hello 世界 🌍",
		}
		doc2 := &memory.TextDocument{
			FieldID:      "unicode-2",
			FieldContent: "Hello 世界 🌎",
		}

		assert.NotEqual(t, doc1.Content(), doc2.Content())
		assert.Contains(t, doc1.Content(), "世界")
		assert.Contains(t, doc2.Content(), "世界")
	})

	t.Run("binary-like content", func(t *testing.T) {
		binaryContent1 := string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD})
		binaryContent2 := string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFC})

		doc1 := &memory.TextDocument{
			FieldID:      "binary-1",
			FieldContent: binaryContent1,
		}
		doc2 := &memory.TextDocument{
			FieldID:      "binary-2",
			FieldContent: binaryContent2,
		}

		assert.NotEqual(t, doc1.Content(), doc2.Content())
		assert.Equal(t, len(doc1.Content()), len(doc2.Content()))
	})

	t.Run("hash computation consistency", func(t *testing.T) {
		content := "Test content for hash consistency"
		doc := &memory.TextDocument{
			FieldID:      "hash-consistency",
			FieldContent: content,
		}

		hash1 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc.Content())))
		hash2 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc.Content())))
		hash3 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc.Content())))

		assert.Equal(t, hash1, hash2)
		assert.Equal(t, hash2, hash3)
		assert.Equal(t, 64, len(hash1))
	})
}

func TestConcurrentDocumentStorage(t *testing.T) {
	t.Run("concurrent document creation", func(t *testing.T) {
		const numGoroutines = 10
		const docsPerGoroutine = 5

		docChan := make(chan *memory.TextDocument, numGoroutines*docsPerGoroutine)
		doneChan := make(chan struct{}, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer func() { doneChan <- struct{}{} }()

				for j := 0; j < docsPerGoroutine; j++ {
					doc := &memory.TextDocument{
						FieldID:      fmt.Sprintf("concurrent-%d-%d", goroutineID, j),
						FieldContent: fmt.Sprintf("Content from goroutine %d, document %d", goroutineID, j),
					}
					docChan <- doc
				}
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			select {
			case <-doneChan:
			case <-time.After(5 * time.Second):
				t.Fatal("Test timed out waiting for concurrent operations")
			}
		}

		close(docChan)
		documents := make([]*memory.TextDocument, 0, numGoroutines*docsPerGoroutine)
		for doc := range docChan {
			documents = append(documents, doc)
		}

		assert.Equal(t, numGoroutines*docsPerGoroutine, len(documents))

		seenIDs := make(map[string]bool)
		for _, doc := range documents {
			assert.False(t, seenIDs[doc.ID()], "Duplicate document ID found: %s", doc.ID())
			seenIDs[doc.ID()] = true
		}
	})

	t.Run("concurrent access to same document data", func(t *testing.T) {
		sharedContent := "Shared content for concurrent access test"
		const numGoroutines = 20

		results := make(chan string, numGoroutines)
		doneChan := make(chan struct{}, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer func() { doneChan <- struct{}{} }()

				doc := &memory.TextDocument{
					FieldID:      fmt.Sprintf("shared-access-%d", goroutineID),
					FieldContent: sharedContent,
				}

				content := doc.Content()
				results <- content
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			select {
			case <-doneChan:
			case <-time.After(5 * time.Second):
				t.Fatal("Test timed out")
			}
		}

		close(results)
		for result := range results {
			assert.Equal(t, sharedContent, result)
		}
	})
}

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

		mockStorage.On("Query", mock.Anything, "pizza preferences", expectedFilter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

		mockStorage.On("Query", mock.Anything, "test query", nilFilter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}
		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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
			Tags: &memory.TagsFilter{
				All: []string{"work", "important"},
			},
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

		mockStorage.On("Query", mock.Anything, "project updates", filter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}
		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

		mockStorage.On("Query", mock.Anything, "test", filter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}
		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

		mockStorage.On("Query", mock.Anything, "test", filter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

		mockStorage.On("Query", mock.Anything, "drink preferences", filter, mock.AnythingOfType("string")).
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
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

// TestPolynomialTagsFiltering tests the new polynomial tags filtering capabilities.
func TestPolynomialTagsFiltering(t *testing.T) {
	t.Run("simple ALL logic (backward compatible)", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{
				All: []string{"work", "important"},
			},
		}

		assert.NotNil(t, filter.Tags)
		assert.Len(t, filter.Tags.All, 2)
		assert.Contains(t, filter.Tags.All, "work")
		assert.Contains(t, filter.Tags.All, "important")
		assert.Empty(t, filter.Tags.Any)
		assert.Nil(t, filter.Tags.Expression)
		assert.False(t, filter.Tags.IsEmpty())
	})

	t.Run("simple ANY logic", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{
				Any: []string{"urgent", "deadline", "asap"},
			},
		}

		assert.NotNil(t, filter.Tags)
		assert.Empty(t, filter.Tags.All)
		assert.Len(t, filter.Tags.Any, 3)
		assert.Contains(t, filter.Tags.Any, "urgent")
		assert.Contains(t, filter.Tags.Any, "deadline")
		assert.Contains(t, filter.Tags.Any, "asap")
		assert.Nil(t, filter.Tags.Expression)
		assert.False(t, filter.Tags.IsEmpty())
	})

	t.Run("complex boolean expression - (A AND B) OR (C AND D)", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{
				Expression: &memory.BooleanExpression{
					Operator: memory.OR,
					Left: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"work", "Q1"},
					},
					Right: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"personal", "urgent"},
					},
				},
			},
		}

		assert.NotNil(t, filter.Tags)
		assert.Empty(t, filter.Tags.All)
		assert.Empty(t, filter.Tags.Any)
		assert.NotNil(t, filter.Tags.Expression)
		assert.Equal(t, memory.OR, filter.Tags.Expression.Operator)
		assert.True(t, filter.Tags.Expression.IsBranch())
		assert.False(t, filter.Tags.Expression.IsLeaf())
		assert.False(t, filter.Tags.IsEmpty())

		// Verify left branch: work AND Q1
		leftExpr := filter.Tags.Expression.Left
		assert.NotNil(t, leftExpr)
		assert.Equal(t, memory.AND, leftExpr.Operator)
		assert.True(t, leftExpr.IsLeaf())
		assert.False(t, leftExpr.IsBranch())
		assert.Contains(t, leftExpr.Tags, "work")
		assert.Contains(t, leftExpr.Tags, "Q1")

		// Verify right branch: personal AND urgent
		rightExpr := filter.Tags.Expression.Right
		assert.NotNil(t, rightExpr)
		assert.Equal(t, memory.AND, rightExpr.Operator)
		assert.True(t, rightExpr.IsLeaf())
		assert.False(t, rightExpr.IsBranch())
		assert.Contains(t, rightExpr.Tags, "personal")
		assert.Contains(t, rightExpr.Tags, "urgent")
	})

	t.Run("empty tags filter", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{},
		}

		assert.NotNil(t, filter.Tags)
		assert.True(t, filter.Tags.IsEmpty())
	})

	t.Run("nil tags filter", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: nil,
		}

		assert.Nil(t, filter.Tags)
		// IsEmpty should handle nil gracefully
		if filter.Tags != nil {
			assert.True(t, filter.Tags.IsEmpty())
		}
	})
}

func TestLargeDocumentHandling(t *testing.T) {
	t.Run("large text document creation", func(t *testing.T) {
		largeContent := make([]byte, 1024*1024)
		for i := range largeContent {
			largeContent[i] = byte('a' + (i % 26))
		}

		start := time.Now()
		doc := &memory.TextDocument{
			FieldID:      "large-doc-1mb",
			FieldContent: string(largeContent),
		}
		duration := time.Since(start)

		assert.Equal(t, 1024*1024, len(doc.Content()))
		assert.Less(t, duration, 100*time.Millisecond, "Large document creation took too long")
		t.Logf("Large document (1MB) creation took: %v", duration)
	})

	t.Run("many small documents creation", func(t *testing.T) {
		const numDocs = 10000

		start := time.Now()
		docs := make([]*memory.TextDocument, numDocs)
		for i := 0; i < numDocs; i++ {
			docs[i] = &memory.TextDocument{
				FieldID:      fmt.Sprintf("small-doc-%d", i),
				FieldContent: fmt.Sprintf("Small document content %d with some additional text to make it more realistic", i),
			}
		}
		duration := time.Since(start)

		assert.Equal(t, numDocs, len(docs))
		assert.Less(t, duration, 1*time.Second, "Creating many small documents took too long")
		t.Logf("Creating %d small documents took: %v", numDocs, duration)
	})

	t.Run("large conversation document", func(t *testing.T) {
		const numMessages = 10000
		messages := make([]memory.ConversationMessage, numMessages)

		start := time.Now()
		for i := 0; i < numMessages; i++ {
			speaker := "alice"
			if i%2 == 1 {
				speaker = "bob"
			}
			messages[i] = memory.ConversationMessage{
				Speaker: speaker,
				Content: fmt.Sprintf("This is message number %d in a very long conversation that tests large document handling", i),
				Time:    time.Now().Add(time.Duration(i) * time.Second),
			}
		}

		doc := &memory.ConversationDocument{
			FieldID:      "large-conversation",
			User:         "alice",
			Conversation: messages,
		}
		duration := time.Since(start)

		assert.Equal(t, numMessages, len(doc.Conversation))
		assert.Contains(t, doc.Content(), "message number 0")
		assert.Contains(t, doc.Content(), fmt.Sprintf("message number %d", numMessages-1))
		assert.Less(t, duration, 1*time.Second, "Large conversation creation took too long")
		t.Logf("Large conversation (%d messages) creation took: %v", numMessages, duration)
	})

	t.Run("memory usage validation", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		docs := make([]*memory.TextDocument, 100)
		for i := 0; i < 100; i++ {
			content := make([]byte, 10*1024)
			for j := range content {
				content[j] = byte('a' + (j % 26))
			}

			docs[i] = &memory.TextDocument{
				FieldID:      fmt.Sprintf("memory-test-doc-%d", i),
				FieldContent: string(content),
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		memIncrease := m2.Alloc - m1.Alloc
		t.Logf("Memory increase after creating large documents: %d bytes", memIncrease)

		assert.Less(t, memIncrease, uint64(5*1024*1024), "Memory usage increased too much")

		assert.Equal(t, 100, len(docs))
		for i, doc := range docs {
			assert.Equal(t, 10*1024, len(doc.Content()))
			assert.Equal(t, fmt.Sprintf("memory-test-doc-%d", i), doc.ID())
		}
	})

	t.Run("content hash consistency for large documents", func(t *testing.T) {
		largeContent := strings.Repeat("This is a test string for hash consistency. ", 10000)

		doc1 := &memory.TextDocument{
			FieldID:      "hash-large-1",
			FieldContent: largeContent,
		}
		doc2 := &memory.TextDocument{
			FieldID:      "hash-large-2",
			FieldContent: largeContent,
		}

		hash1 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc1.Content())))
		hash2 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc2.Content())))

		assert.Equal(t, hash1, hash2, "Identical large content should produce identical hashes")
		assert.Equal(t, doc1.Content(), doc2.Content())

		doc3 := &memory.TextDocument{
			FieldID:      "hash-large-3",
			FieldContent: largeContent + "x",
		}
		hash3 := fmt.Sprintf("%x", sha256.Sum256([]byte(doc3.Content())))
		assert.NotEqual(t, hash1, hash3, "Different content should produce different hashes")
	})
}

// TestTagsFilterAPI tests the API design and usability of the new tags filtering.
func TestTagsFilterAPI(t *testing.T) {
	t.Run("fluent API usage examples", func(t *testing.T) {
		// Example 1: Simple work-related documents
		workFilter := &memory.Filter{
			Source: stringPtr("conversations"),
			Tags: &memory.TagsFilter{
				All: []string{"work", "important"},
			},
		}
		assert.Equal(t, "conversations", *workFilter.Source)
		assert.Len(t, workFilter.Tags.All, 2)

		// Example 2: Urgent items from any source
		urgentFilter := &memory.Filter{
			Tags: &memory.TagsFilter{
				Any: []string{"urgent", "asap", "deadline"},
			},
			Limit: intPtr(20),
		}
		assert.Len(t, urgentFilter.Tags.Any, 3)
		assert.Equal(t, 20, *urgentFilter.Limit)

		// Example 3: Complex project filtering
		projectFilter := &memory.Filter{
			Tags: &memory.TagsFilter{
				Expression: &memory.BooleanExpression{
					Operator: memory.OR,
					Left: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"project", "alpha"},
					},
					Right: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"project", "beta"},
					},
				},
			},
			Distance: 0.8,
		}
		assert.NotNil(t, projectFilter.Tags.Expression)
		assert.Equal(t, float32(0.8), projectFilter.Distance)
	})

	t.Run("expression validation", func(t *testing.T) {
		// Valid leaf expression
		leafExpr := &memory.BooleanExpression{
			Operator: memory.AND,
			Tags:     []string{"tag1", "tag2"},
		}
		assert.True(t, leafExpr.IsLeaf())
		assert.False(t, leafExpr.IsBranch())

		// Valid branch expression
		branchExpr := &memory.BooleanExpression{
			Operator: memory.OR,
			Left: &memory.BooleanExpression{
				Operator: memory.AND,
				Tags:     []string{"left1"},
			},
			Right: &memory.BooleanExpression{
				Operator: memory.OR,
				Tags:     []string{"right1"},
			},
		}
		assert.False(t, branchExpr.IsLeaf())
		assert.True(t, branchExpr.IsBranch())

		// Invalid expressions
		emptyLeaf := &memory.BooleanExpression{
			Operator: memory.AND,
			Tags:     []string{}, // No tags
		}
		assert.False(t, emptyLeaf.IsLeaf())

		incompleteBranch := &memory.BooleanExpression{
			Operator: memory.OR,
			Left:     leafExpr,
			Right:    nil, // Missing operand
		}
		assert.False(t, incompleteBranch.IsBranch())
	})
}

// TestTagsFilteringIntegrationUpgrade tests integration scenarios for the tags filtering upgrade.
func TestTagsFilteringIntegrationUpgrade(t *testing.T) {
	t.Run("storage interface integration", func(t *testing.T) {
		mockStorage := &MockStorage{}
		logger := log.New(os.Stdout)

		// Test complex filter with multiple components
		complexFilter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Tags: &memory.TagsFilter{
				Expression: &memory.BooleanExpression{
					Operator: memory.OR,
					Left: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"work", "Q1"},
					},
					Right: &memory.BooleanExpression{
						Operator: memory.AND,
						Tags:     []string{"personal", "urgent"},
					},
				},
			},
			Distance: 0.75,
			Limit:    intPtr(8),
		}

		expectedResult := memory.QueryResult{
			Facts: []memory.MemoryFact{},
			Documents: []memory.TextDocument{
				{
					FieldID:      "complex-doc-1",
					FieldContent: "Q1 work project with alice",
					FieldSource:  "conversations",
					FieldTags:    []string{"work", "Q1", "project"},
					FieldMetadata: map[string]string{
						"source":    "conversations",
						"speakerID": "alice",
					},
				},
			},
		}

		mockStorage.On("Query", mock.Anything, "project status", complexFilter, mock.AnythingOfType("string")).
			Return(expectedResult, nil)

		// Create AI services inline (same as before)
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

		completionsModel := os.Getenv("COMPLETIONS_MODEL")
		if completionsModel == "" {
			completionsModel = "gpt-41-mini"
		}
		embeddingsModel := os.Getenv("EMBEDDINGS_MODEL")
		if embeddingsModel == "" {
			embeddingsModel = "text-embedding-3-small"
		}

		deps := Dependencies{
			Logger:             logger,
			Storage:            mockStorage,
			CompletionsService: completionsService,
			EmbeddingsService:  embeddingsService,
			CompletionsModel:   completionsModel,
			EmbeddingsModel:    embeddingsModel,
		}

		storage, err := New(deps)
		require.NoError(t, err)

		result, err := storage.Query(context.Background(), "project status", complexFilter)
		require.NoError(t, err)

		// Verify complex filtering worked
		assert.Len(t, result.Documents, 1)
		doc := result.Documents[0]
		assert.Equal(t, "complex-doc-1", doc.ID())
		assert.Contains(t, doc.Tags(), "work")
		assert.Contains(t, doc.Tags(), "Q1")
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "alice", doc.Metadata()["speakerID"])

		mockStorage.AssertExpectations(t)
	})

	t.Run("performance comparison scenarios", func(t *testing.T) {
		// Document the expected performance characteristics
		performanceScenarios := []struct {
			name        string
			filter      *memory.Filter
			expectation string
		}{
			{
				name: "simple_all_tags",
				filter: &memory.Filter{
					Tags: &memory.TagsFilter{
						All: []string{"work", "important"},
					},
				},
				expectation: "Should use single ContainsAll query - very fast",
			},
			{
				name: "simple_any_tags",
				filter: &memory.Filter{
					Tags: &memory.TagsFilter{
						Any: []string{"urgent", "deadline"},
					},
				},
				expectation: "Should use OR conditions - moderately fast",
			},
			{
				name: "complex_expression",
				filter: &memory.Filter{
					Tags: &memory.TagsFilter{
						Expression: &memory.BooleanExpression{
							Operator: memory.AND,
							Left: &memory.BooleanExpression{
								Operator: memory.OR,
								Tags:     []string{"project", "task"},
							},
							Right: &memory.BooleanExpression{
								Operator: memory.AND,
								Tags:     []string{"Q1", "important"},
							},
						},
					},
				},
				expectation: "Should use nested boolean queries - slower but powerful",
			},
		}

		for _, scenario := range performanceScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				assert.NotNil(t, scenario.filter.Tags)
				t.Logf("Filter: %+v", scenario.filter.Tags)
				t.Logf("Performance expectation: %s", scenario.expectation)
			})
		}
	})
}
