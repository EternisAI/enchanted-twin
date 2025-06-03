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
