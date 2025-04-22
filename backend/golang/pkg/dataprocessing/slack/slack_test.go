package slack

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToDocuments(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test-slack-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		err = os.Remove(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to remove temp file: %v", err)
		}
	}()

	// Write test data to the file
	testData := `{"data":{"text":"Hello world","username":"john_doe","channelName":"general","myMessage":true},"timestamp":"2022-12-25T04:38:18Z","source":"slack"}
{"data":{"text":"How are you?","username":"jane_doe","channelName":"general","myMessage":false},"timestamp":"2022-12-25T04:39:18Z","source":"slack"}`

	if _, err := tempFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	err = tempFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Test the function
	docs, err := ToDocuments(tempFile.Name())
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	// Verify results
	assert.Equal(t, 2, len(docs), "Expected 2 documents")

	// Check first message
	expectedTimestamp1, _ := time.Parse(time.RFC3339, "2022-12-25T04:38:18Z")
	assert.Equal(t, "From john_doe in channel general: Hello world", docs[0].Content)
	assert.Equal(t, &expectedTimestamp1, docs[0].Timestamp)
	assert.Equal(t, []string{"social", "slack", "chat"}, docs[0].Tags)
	assert.Equal(t, map[string]string{
		"type":           "message",
		"channelName":    "general",
		"authorUsername": "john_doe",
	}, docs[0].Metadata)

	// Check second message
	expectedTimestamp2, _ := time.Parse(time.RFC3339, "2022-12-25T04:39:18Z")
	assert.Equal(t, "From jane_doe in channel general: How are you?", docs[1].Content)
	assert.Equal(t, &expectedTimestamp2, docs[1].Timestamp)
	assert.Equal(t, []string{"social", "slack", "chat"}, docs[1].Tags)
	assert.Equal(t, map[string]string{
		"type":           "message",
		"channelName":    "general",
		"authorUsername": "jane_doe",
	}, docs[1].Metadata)
}
