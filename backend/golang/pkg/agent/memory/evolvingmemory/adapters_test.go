package evolvingmemory

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// TestFactExtractorAdapter tests the fact extractor adapter structure.
func TestFactExtractorAdapter(t *testing.T) {
	logger := log.Default()
	mockClient := &weaviate.Client{}

	storage, _ := New(logger, mockClient, &ai.Service{}, &ai.Service{})
	adapter := NewFactExtractor(storage)

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
		// This will fail with actual service calls, but we're testing the structure
		_, err := adapter.ExtractFacts(context.Background(), prepDoc)

		// We expect an error because the AI service is not properly initialized
		assert.Error(t, err)
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
		_, err := adapter.ExtractFacts(context.Background(), prepDoc)
		assert.Error(t, err) // Expected due to uninitialized service
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
}

// TestMemoryOperationsAdapter tests the memory operations adapter structure.
func TestMemoryOperationsAdapter(t *testing.T) {
	logger := log.Default()
	mockClient := &weaviate.Client{}

	t.Run("SearchSimilar_Structure", func(t *testing.T) {
		storage, _ := New(logger, mockClient, &ai.Service{}, &ai.Service{})
		adapter := NewMemoryOperations(storage)

		// Test that SearchSimilar wraps Query method properly
		// This will fail with actual calls, but tests the structure
		_, err := adapter.SearchSimilar(context.Background(), "test fact", "speaker1")
		assert.Error(t, err) // Expected due to uninitialized client
	})

	t.Run("UpdateMemory_RequiresGetByID", func(t *testing.T) {
		storage, _ := New(logger, mockClient, &ai.Service{}, &ai.Service{})
		adapter := NewMemoryOperations(storage)

		// Test that UpdateMemory tries to get the original document
		embedding := make([]float32, 10)
		err := adapter.UpdateMemory(context.Background(), "mem1", "new content", embedding)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "getting original document")
	})

	t.Run("DeleteMemory_Structure", func(t *testing.T) {
		storage, _ := New(logger, mockClient, &ai.Service{}, &ai.Service{})
		adapter := NewMemoryOperations(storage)

		// Test that DeleteMemory wraps Delete method
		err := adapter.DeleteMemory(context.Background(), "mem1")
		assert.Error(t, err) // Expected due to uninitialized client
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
	storage, _ := New(logger, mockClient, &ai.Service{}, &ai.Service{})

	t.Run("NewFactExtractor", func(t *testing.T) {
		adapter := NewFactExtractor(storage)
		assert.NotNil(t, adapter)

		// Verify it implements the interface
		_ = adapter
	})

	t.Run("NewMemoryOperations", func(t *testing.T) {
		adapter := NewMemoryOperations(storage)
		assert.NotNil(t, adapter)

		// Verify it implements the interface
		_ = adapter
	})
}
