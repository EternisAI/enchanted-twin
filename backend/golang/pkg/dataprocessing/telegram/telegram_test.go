package telegram

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToDocuments(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test-telegram-*.jsonl")
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
	testData := `{"data":{"chatId":1601587058,"chatType":"saved_messages","forwardedFrom":"Eternal22","from":"Eternal22","messageId":59318,"messageType":"message","myMessage":false,"savedFrom":"Mahamat New","text":"I want to believe","to":"xxx","type":"message"},"timestamp":"2022-12-25T04:38:18Z","source":"telegram"}
{"data":{"type":"contact","firstName":"John","lastName":"Doe","phoneNumber":"+1234567890"},"timestamp":"2022-12-25T04:38:18Z","source":"telegram"}`

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

	// Check message document
	expectedTimestamp, _ := time.Parse(time.RFC3339, "2022-12-25T04:38:18Z")
	assert.Equal(t, "I want to believe", docs[0].Content)
	assert.Equal(t, &expectedTimestamp, docs[0].Timestamp)
	assert.Equal(t, []string{"social", "telegram", "chat"}, docs[0].Tags)

	// Check contact document
	assert.Equal(t, "John Doe", docs[1].Content)
	assert.Equal(t, &expectedTimestamp, docs[1].Timestamp)
	assert.Equal(t, []string{"social", "telegram", "contact"}, docs[1].Tags)
	assert.Equal(t, map[string]string{
		"type":        "contact",
		"firstName":   "John",
		"lastName":    "Doe",
		"phoneNumber": "+1234567890",
	}, docs[1].Metadata)
}
