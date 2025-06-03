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
)

// createMockStorage creates a StorageImpl instance with mocked services for testing.
func createMockStorage(t *testing.T) *StorageImpl {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level: log.WarnLevel,
	})

	// Try to create separate AI services, fall back to skipping tests if no env vars
	completionsService, embeddingsService := createTestAIServices()
	if completionsService == nil || embeddingsService == nil {
		t.Skip("Skipping AI-dependent tests: API keys not set")
		return nil
	}

	// Create mock storage interface using embeddings service for vector operations
	mockStorage := storage.New(&weaviate.Client{}, logger, embeddingsService)

	storage := &StorageImpl{
		logger:             logger,
		completionsService: completionsService, // OpenRouter for LLM
		embeddingsService:  embeddingsService,  // OpenAI for embeddings
		storage:            mockStorage,
	}

	return storage
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 4, config.Workers)
	assert.Equal(t, 50, config.FactsPerWorker)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 30*time.Second, config.FlushInterval)
	assert.Equal(t, 30*time.Second, config.FactExtractionTimeout)
	assert.Equal(t, 30*time.Second, config.MemoryDecisionTimeout)
	assert.Equal(t, 30*time.Second, config.StorageTimeout)
	assert.True(t, config.EnableRichContext)
	assert.True(t, config.ParallelFactExtraction)
	assert.True(t, config.StreamingProgress)
}

func TestStoreV2BasicFlow(t *testing.T) {
	// Skip this test since it requires real Weaviate client and causes nil pointer dereference with mock
	t.Skip("Skipping test that requires real Weaviate client - causes nil pointer dereference with mock")

	ctx := context.Background()
	storage := createMockStorage(t)

	// Create test documents
	docs := []memory.Document{
		&memory.TextDocument{
			FieldID:      "test-1",
			FieldContent: "This is a test document",
		},
	}

	config := Config{
		Workers:               1,
		BatchSize:             10,
		FlushInterval:         100 * time.Millisecond,
		FactExtractionTimeout: 5 * time.Second,
		MemoryDecisionTimeout: 5 * time.Second,
		StorageTimeout:        5 * time.Second,
	}

	// Test that channels are created and closed properly
	progressCh, errorCh := storage.StoreV2(ctx, docs, config)

	require.NotNil(t, progressCh)
	require.NotNil(t, errorCh)

	// Consume channels until they close
	var progressUpdates []Progress
	var errors []error

	for progressCh != nil || errorCh != nil {
		select {
		case progress, ok := <-progressCh:
			if !ok {
				progressCh = nil
				continue
			}
			progressUpdates = append(progressUpdates, progress)

		case err, ok := <-errorCh:
			if !ok {
				errorCh = nil
				continue
			}
			errors = append(errors, err)

		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out waiting for channels to close")
		}
	}

	// We should have at least one progress update
	assert.NotEmpty(t, progressUpdates)

	// Errors are expected in this mock environment, so we don't assert on them
	t.Logf("Progress updates: %d, Errors: %d", len(progressUpdates), len(errors))
}

func TestStoreV2EmptyDocuments(t *testing.T) {
	ctx := context.Background()
	storage := createMockStorage(t)

	config := DefaultConfig()

	progressCh, errorCh := storage.StoreV2(ctx, []memory.Document{}, config)

	// Should get one progress update indicating completion
	select {
	case progress := <-progressCh:
		assert.Equal(t, 0, progress.Processed)
		assert.Equal(t, 0, progress.Total)
		assert.Equal(t, "preparation", progress.Stage)
	case <-time.After(5 * time.Second):
		t.Fatal("Expected progress update for empty documents")
	}

	// Channels should close
	select {
	case _, ok := <-progressCh:
		assert.False(t, ok, "Progress channel should be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("Progress channel should close")
	}

	select {
	case _, ok := <-errorCh:
		assert.False(t, ok, "Error channel should be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("Error channel should close")
	}
}

func TestPipelineIntegration_BasicFlow(t *testing.T) {
	t.Skip("Skipping integration test that requires real AI services - causes nil pointer dereference")

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
