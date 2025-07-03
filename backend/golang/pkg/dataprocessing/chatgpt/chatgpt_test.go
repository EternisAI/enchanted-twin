package chatgpt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestSimpleConversationProcessing(t *testing.T) {
	sampleJSON := `[
  {
    "title": "Test Conversation",
    "create_time": 1742179529.637027,
    "update_time": 1742184311.774424,
    "mapping": {
      "msg1": {
        "id": "msg1",
        "message": {
          "id": "msg1",
          "author": { "role": "user", "name": null, "metadata": {} },
          "create_time": 1742179528.989,
          "content": {
            "content_type": "text",
            "parts": ["Hello, how are you?"]
          }
        },
        "parent": null,
        "children": ["msg2"]
      },
      "msg2": {
        "id": "msg2",
        "message": {
          "id": "msg2",
          "author": { "role": "assistant", "name": null, "metadata": {} },
          "create_time": 1742179541.698968,
          "content": {
            "content_type": "text",
            "parts": ["I am doing well, thank you! How can I help you today?"]
          }
        },
        "parent": "msg1",
        "children": []
      }
    }
  }
]`

	tempDir, err := os.MkdirTemp("", "chatgpt-test")
	require.NoError(t, err)

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}
	}()

	tempFilePath := filepath.Join(tempDir, "conversations.json")
	err = os.WriteFile(tempFilePath, []byte(sampleJSON), 0o644)
	require.NoError(t, err)

	logger := log.New(os.Stdout)
	store, err := db.NewStore(context.Background(), "test")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	dataSource, err := NewChatGPTProcessor(store, logger)
	if err != nil {
		t.Fatalf("Failed to create chatgpt processor: %v", err)
	}

	ctx := context.Background()
	records, err := dataSource.ProcessFile(ctx, tempFilePath)

	require.NoError(t, err)
	require.Len(t, records, 1, "Expected 1 conversation record")

	doc := records[0]
	assert.Equal(t, "chatgpt", doc.FieldSource)

	// Verify conversation structure
	assert.Equal(t, "Test Conversation", doc.FieldMetadata["title"])
	assert.Contains(t, doc.FieldTags, "chat")
	assert.Contains(t, doc.FieldTags, "conversation")
	assert.Equal(t, []string{"user", "assistant"}, doc.People)
	assert.Equal(t, "user", doc.User)

	// Verify messages exist and are properly structured
	assert.Equal(t, 2, len(doc.Conversation), "Expected exactly 2 messages in conversation")

	// Verify message content (order may vary since we iterate over a map)
	userFound := false
	assistantFound := false
	for _, msg := range doc.Conversation {
		if msg.Speaker == "user" && msg.Content == "Hello, how are you?" {
			userFound = true
		}
		if msg.Speaker == "assistant" && msg.Content == "I am doing well, thank you! How can I help you today?" {
			assistantFound = true
		}
	}
	assert.True(t, userFound, "Expected to find user message")
	assert.True(t, assistantFound, "Expected to find assistant message")
}
