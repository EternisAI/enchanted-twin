package evolvingmemory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// TestConversationDocumentBasics tests basic ConversationDocument functionality
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

// TestTextDocumentBasics tests basic TextDocument functionality
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

// TestHelperFunctions tests utility functions
func TestHelperFunctions(t *testing.T) {
	// Test firstNChars
	assert.Equal(t, "Hello", firstNChars("Hello", 10))
	assert.Equal(t, "Hello...", firstNChars("Hello World", 5))

	// Test getCurrentDateForPrompt - it returns date in YYYY-MM-DD format
	dateStr := getCurrentDateForPrompt()
	assert.NotEmpty(t, dateStr)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, dateStr) // Matches YYYY-MM-DD format
}

// TestValidationRules documents the speaker validation rules
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

// TestMemoryConstants verifies constants are defined
func TestMemoryConstants(t *testing.T) {
	assert.Equal(t, "ADD", AddMemoryToolName)
	assert.Equal(t, "UPDATE", UpdateMemoryToolName)
	assert.Equal(t, "DELETE", DeleteMemoryToolName)
	assert.Equal(t, "NONE", NoneMemoryToolName)
	assert.Equal(t, "EXTRACT_FACTS", ExtractFactsToolName)
}

// Note: Full integration tests with real Weaviate/AI services
// should be run separately using the existing test-memory command
