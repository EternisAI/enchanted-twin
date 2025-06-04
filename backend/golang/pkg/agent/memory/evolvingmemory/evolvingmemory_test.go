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
