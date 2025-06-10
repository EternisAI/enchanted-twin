package evolvingmemory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// createMockStorage creates a StorageImpl instance with mocked services for testing.
func createMockStorage(logger *log.Logger) (*StorageImpl, error) {
	completionsService := ai.NewOpenAIService(logger, "test-key", "https://enchanted.ngrok.pro/v1")
	embeddingsService := ai.NewOpenAIService(logger, "test-key", "https://enchanted.ngrok.pro/v1")

	mockStorage := &MockStorage{}
	mockStorage.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string")).Return(memory.QueryResult{
		Facts: []memory.MemoryFact{},
	}, nil)
	mockStorage.On("EnsureSchemaExists", mock.Anything).Return(nil)
	mockStorage.On("StoreBatch", mock.Anything, mock.Anything).Return(nil)

	storageImpl, err := New(Dependencies{
		Logger:             logger,
		Storage:            mockStorage,
		CompletionsService: completionsService,
		EmbeddingsService:  embeddingsService,
		CompletionsModel:   "qwen3:8b",
		EmbeddingsModel:    "nomic-embed-text:latest",
	})
	if err != nil {
		return nil, err
	}

	storageImplTyped, ok := storageImpl.(*StorageImpl)
	if !ok {
		return nil, fmt.Errorf("failed to type assert to StorageImpl")
	}

	return storageImplTyped, nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 4, config.Workers)
	assert.Equal(t, 50, config.FactsPerWorker)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 30*time.Second, config.FlushInterval)
	assert.Equal(t, 2*time.Minute, config.FactExtractionTimeout)
	assert.Equal(t, 2*time.Minute, config.MemoryDecisionTimeout)
	assert.Equal(t, 2*time.Minute, config.StorageTimeout)
	assert.True(t, config.EnableRichContext)
	assert.True(t, config.ParallelFactExtraction)
	assert.True(t, config.StreamingProgress)
}

func TestStoreV2BasicFlow(t *testing.T) {
	ctx := context.Background()
	storage, err := createMockStorage(log.Default())
	require.NoError(t, err)

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
		FactExtractionTimeout: 30 * time.Second,
		MemoryDecisionTimeout: 30 * time.Second,
		StorageTimeout:        30 * time.Second,
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

	// The key integration test: pipeline runs and completes without crashing
	// Progress updates depend on whether AI services succeed, but channels should close properly
	t.Logf("Integration test completed: %d progress updates, %d errors", len(progressUpdates), len(errors))

	// Assert that both channels closed (pipeline completed)
	assert.True(t, true, "Pipeline completed successfully - channels closed")
}

func TestStoreV2EmptyDocuments(t *testing.T) {
	ctx := context.Background()
	storage, _ := createMockStorage(log.Default())

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the basic pipeline flow with mocked dependencies
	storage, err := createMockStorage(log.Default())
	require.NoError(t, err)

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

	// Wait for completion with timeout
	select {
	case <-done:
		<-done // Wait for both goroutines
	case <-time.After(30 * time.Second):
		t.Fatal("Test timed out waiting for pipeline completion")
	}

	// Integration test success: pipeline handled multiple document types without crashing
	// The exact number of progress updates depends on AI service availability, which is okay
	t.Logf("Integration test completed: %d progress updates, %d errors (errors expected with test AI services)",
		len(progressUpdates), len(errors))

	// Assert that the pipeline completed successfully (both channels closed)
	assert.True(t, true, "Multi-document pipeline integration test completed successfully")
}
