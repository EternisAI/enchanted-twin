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
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// createMockStorage creates a WeaviateStorage instance with mocked services for testing.
func createMockStorage(t *testing.T) *WeaviateStorage {
	t.Helper()

	// Create a mock logger
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level: log.WarnLevel,
	})

	// Create storage with mocks
	storage := &WeaviateStorage{
		logger:             logger,
		completionsService: &ai.Service{},      // Would need proper mocking in real tests
		embeddingsService:  &ai.Service{},      // Would need proper mocking in real tests
		client:             &weaviate.Client{}, // Mock client
	}

	return storage
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 4, config.Workers)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 30*time.Second, config.FactExtractionTimeout)
	assert.True(t, config.StreamingProgress)
}

func TestStoreV2_EmptyDocuments(t *testing.T) {
	storage := createMockStorage(t)
	ctx := context.Background()
	config := DefaultConfig()

	progressCh, errorCh := storage.StoreV2(ctx, []memory.Document{}, config)

	// Should receive progress with 0 documents
	progress := <-progressCh
	assert.Equal(t, 0, progress.Processed)
	assert.Equal(t, 0, progress.Total)
	assert.Equal(t, "preparation", progress.Stage)

	// Channels should close
	_, ok := <-progressCh
	assert.False(t, ok)
	_, ok = <-errorCh
	assert.False(t, ok)
}

func TestPipelineIntegration_BasicFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the basic pipeline flow with mocked dependencies
	storage := createMockStorage(t)
	ctx := context.Background()
	config := DefaultConfig()
	config.Workers = 2
	config.BatchSize = 2

	// Create test documents
	docs := []memory.Document{
		&memory.ConversationDocument{
			FieldID: "conv1",
			User:    "user1",
			Conversation: []memory.ConversationMessage{
				{
					Speaker: "user1",
					Content: "Hello, how are you?",
					Time:    time.Now(),
				},
				{
					Speaker: "assistant",
					Content: "I'm doing well, thank you!",
					Time:    time.Now(),
				},
			},
		},
		&memory.TextDocument{
			FieldID:      "text1",
			FieldContent: "Test text content",
		},
	}

	progressCh, errorCh := storage.StoreV2(ctx, docs, config)

	// Collect results
	var progressUpdates []Progress
	var errors []error

	done := make(chan bool)
	go func() {
		for p := range progressCh {
			progressUpdates = append(progressUpdates, p)
		}
		done <- true
	}()

	go func() {
		for e := range errorCh {
			errors = append(errors, e)
		}
		done <- true
	}()

	// Wait for completion
	<-done
	<-done

	// Verify no critical errors (fact extraction will fail without real LLM)
	// but the pipeline should handle errors gracefully
	require.NotNil(t, progressUpdates)
}

func TestFindMemoryByID(t *testing.T) {
	memories := []ExistingMemory{
		{ID: "mem1", Content: "First memory"},
		{ID: "mem2", Content: "Second memory"},
		{ID: "mem3", Content: "Third memory"},
	}

	// Test finding existing memory
	found := findMemoryByID(memories, "mem2")
	require.NotNil(t, found)
	assert.Equal(t, "mem2", found.ID)
	assert.Equal(t, "Second memory", found.Content)

	// Test finding non-existent memory
	notFound := findMemoryByID(memories, "mem4")
	assert.Nil(t, notFound)
}

func TestToFloat32(t *testing.T) {
	input := []float64{1.5, 2.7, 3.9, 4.1}
	expected := []float32{1.5, 2.7, 3.9, 4.1}

	result := toFloat32(input)

	require.Len(t, result, len(input))
	for i := range result {
		assert.Equal(t, expected[i], result[i])
	}
}
