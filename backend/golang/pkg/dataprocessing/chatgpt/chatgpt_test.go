package chatgpt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
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

	dataSource := NewChatGPTProcessor(tempDir)

	ctx := context.Background()
	records, err := dataSource.ProcessFile(ctx, tempFilePath, nil)

	require.NoError(t, err)
	require.Len(t, records, 1, "Expected 1 conversation record")

	expectedTime := time.Unix(1742179529, 0)

	record := records[0]
	assert.Equal(t, "chatgpt", record.Source)
	assert.True(t, expectedTime.Equal(record.Timestamp))

	// Verify conversation structure
	assert.Equal(t, "Test Conversation", record.Data["title"])

	// Verify messages exist and are properly structured
	messagesInterface, ok := record.Data["messages"]
	assert.True(t, ok, "Expected messages field in conversation data")

	messages, ok := messagesInterface.([]ConversationMessage)
	assert.True(t, ok, "Expected messages to be of type []ConversationMessage")
	assert.Equal(t, 2, len(messages), "Expected exactly 2 messages in conversation")

	// Verify message content (order may vary since we iterate over a map)
	userFound := false
	assistantFound := false
	for _, msg := range messages {
		if msg.Role == "user" && msg.Text == "Hello, how are you?" {
			userFound = true
		}
		if msg.Role == "assistant" && msg.Text == "I am doing well, thank you! How can I help you today?" {
			assistantFound = true
		}
	}
	assert.True(t, userFound, "Expected to find user message")
	assert.True(t, assistantFound, "Expected to find assistant message")
}

func TestConversationToDocuments(t *testing.T) {
	messages := []ConversationMessage{
		{Role: "user", Text: "Hello, how are you?"},
		{Role: "assistant", Text: "I am doing well, thank you! How can I help you today?"},
		{Role: "user", Text: "Can you tell me about Puerto Vallarta?"},
		{Role: "assistant", Text: "Puerto Vallarta is a beautiful coastal city in Mexico..."},
	}

	records := []types.Record{
		{
			Data: map[string]any{
				"title":    "Test Conversation",
				"messages": messages,
			},
			Timestamp: time.Now(),
			Source:    "chatgpt",
		},
	}

	chatgptProcessor := NewChatGPTProcessor("")
	documents, err := chatgptProcessor.ToDocuments(records)
	require.NoError(t, err)
	require.Len(t, documents, 1)

	doc := documents[0]

	assert.Contains(t, doc.Tags(), "chat")
	assert.Contains(t, doc.Tags(), "chatgpt")
	assert.Contains(t, doc.Tags(), "conversation")
	assert.Equal(t, "conversation", doc.Metadata()["type"])
	assert.Equal(t, "Test Conversation", doc.Metadata()["title"])
	assert.Equal(t, "chatgpt", doc.Source())

	expectedContent := "user: Hello, how are you?\nassistant: I am doing well, thank you! How can I help you today?\nuser: Can you tell me about Puerto Vallarta?\nassistant: Puerto Vallarta is a beautiful coastal city in Mexico..."
	assert.Equal(t, expectedContent, doc.Content())
}

// writeRecordsToJSONL writes records to a JSONL file (simplified version for testing).
func writeRecordsToJSONL(records []types.Record, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			err = errors.Wrap(err, "failed to close file")
			fmt.Println("failed to close file", err)
		}
	}()

	for _, record := range records {
		jsonRecord := struct {
			Data      map[string]interface{} `json:"data"`
			Timestamp string                 `json:"timestamp"`
			Source    string                 `json:"source"`
		}{
			Data:      record.Data,
			Timestamp: record.Timestamp.Format(time.RFC3339),
			Source:    record.Source,
		}

		jsonData, err := json.Marshal(jsonRecord)
		if err != nil {
			return err
		}

		if _, err := file.Write(jsonData); err != nil {
			return err
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			return err
		}
	}

	return nil
}

func TestJSONLRoundTrip(t *testing.T) {
	// Create test records with conversation messages
	messages := []ConversationMessage{
		{Role: "user", Text: "What's the weather like?"},
		{Role: "assistant", Text: "I don't have access to real-time weather data, but I can help you find weather information."},
		{Role: "user", Text: "How do I check the weather?"},
		{Role: "assistant", Text: "You can check weather by visiting weather websites, using weather apps, or asking voice assistants."},
	}

	originalRecords := []types.Record{
		{
			Data: map[string]any{
				"title":    "Weather Conversation",
				"messages": messages,
			},
			Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Source:    "chatgpt",
		},
	}

	tempDir, err := os.MkdirTemp("", "chatgpt-jsonl-test")
	require.NoError(t, err)

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}
	}()

	jsonlPath := filepath.Join(tempDir, "test_records.jsonl")
	err = writeRecordsToJSONL(originalRecords, jsonlPath)
	require.NoError(t, err, "Failed to save records to JSONL")

	readRecords, err := helpers.ReadJSONL[types.Record](jsonlPath)
	require.NoError(t, err, "Failed to read records from JSONL")

	require.Len(t, readRecords, 1, "Expected 1 record after reading from JSONL")

	readRecord := readRecords[0]
	assert.Equal(t, "chatgpt", readRecord.Source)
	assert.Equal(t, "Weather Conversation", readRecord.Data["title"])

	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.True(t, expectedTime.Equal(readRecord.Timestamp),
		"Expected timestamp %v, got %v", expectedTime, readRecord.Timestamp)

	messagesInterface, ok := readRecord.Data["messages"]
	assert.True(t, ok, "Expected messages field in read record")

	messagesSlice, ok := messagesInterface.([]interface{})
	assert.True(t, ok, "Expected messages to be a slice after JSON unmarshaling")
	assert.Equal(t, 4, len(messagesSlice), "Expected 4 messages")

	firstMsg, ok := messagesSlice[0].(map[string]interface{})
	assert.True(t, ok, "Expected first message to be a map")
	assert.Equal(t, "user", firstMsg["Role"])
	assert.Equal(t, "What's the weather like?", firstMsg["Text"])

	secondMsg, ok := messagesSlice[1].(map[string]interface{})
	assert.True(t, ok, "Expected second message to be a map")
	assert.Equal(t, "assistant", secondMsg["Role"])
	assert.Equal(t, "I don't have access to real-time weather data, but I can help you find weather information.", secondMsg["Text"])

	// Test conversion to Document format
	chatgptProcessor := NewChatGPTProcessor("")
	docs, err := chatgptProcessor.ToDocuments(readRecords)
	require.NoError(t, err, "Error converting records to documents")
	require.Len(t, docs, 1, "Expected 1 document after conversion")

	convertedDoc := docs[0]
	assert.Contains(t, convertedDoc.Tags(), "chat")
	assert.Contains(t, convertedDoc.Tags(), "chatgpt")
	assert.Contains(t, convertedDoc.Tags(), "conversation")
	assert.Equal(t, "conversation", convertedDoc.Metadata()["type"])
	assert.Equal(t, "Weather Conversation", convertedDoc.Metadata()["title"])
}
