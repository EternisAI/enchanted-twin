package evolvingmemory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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
	assert.Contains(t, conversation.Content(), "I love pizza")
	assert.Contains(t, conversation.Content(), "I prefer sushi")
	assert.NotNil(t, conversation.Timestamp())
	assert.Equal(t, "alice", conversation.Metadata()["user"])
}

func TestTextDocumentBasics(t *testing.T) {
	now := time.Now()
	// Test data
	textDoc := &memory.TextDocument{
		FieldID:        "text-456",
		FieldContent:   "This is a sample text document for testing purposes.",
		FieldTimestamp: &now,
		FieldTags:      []string{"sample", "test"},
		FieldMetadata:  map[string]string{"type": "test", "author": "alice"},
	}

	// Test Document interface methods
	assert.Equal(t, "text-456", textDoc.ID())
	assert.Equal(t, "This is a sample text document for testing purposes.", textDoc.Content())
	assert.NotNil(t, textDoc.Timestamp())
	assert.Equal(t, "test", textDoc.Metadata()["type"])
	assert.Equal(t, "alice", textDoc.Metadata()["author"])
	assert.Contains(t, textDoc.Tags(), "sample")
	assert.Contains(t, textDoc.Tags(), "test")
}
