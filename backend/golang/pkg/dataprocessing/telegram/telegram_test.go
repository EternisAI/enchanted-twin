package telegram

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestToDocuments(t *testing.T) {
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

	testData := `{"data":{"chatId":1601587058,"chatType":"saved_messages","forwardedFrom":"Eternal22","from":"Eternal22","messageId":59318,"messageType":"message","myMessage":false,"savedFrom":"Mahamat New","text":"I want to believe","to":"xxx","type":"message"},"timestamp":"2022-12-25T04:38:18Z","source":"telegram"}
{"data":{"type":"contact","firstName":"John","lastName":"Doe","phoneNumber":"+1234567890"},"timestamp":"2022-12-25T04:38:18Z","source":"telegram"}`

	if _, err := tempFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	err = tempFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	dbFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()                 //nolint:errcheck
	defer os.Remove(dbFile.Name()) //nolint:errcheck

	ctx := context.Background()
	store, err := db.NewStore(ctx, dbFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close() //nolint:errcheck

	count, err := helpers.CountJSONLLines(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to count JSONL lines: %v", err)
	}
	records, err := helpers.ReadJSONLBatch(tempFile.Name(), 0, count)
	if err != nil {
		t.Fatalf("ReadJSONL failed: %v", err)
	}

	telegramProcessor := NewTelegramProcessor(store)
	docs, err := telegramProcessor.ToDocuments(context.Background(), records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	assert.Equal(t, 2, len(docs), "Expected 2 documents")

	var conversationDoc, contactDoc memory.Document
	for _, doc := range docs {
		tags := doc.Tags()
		if len(tags) >= 3 && tags[2] == "chat" {
			conversationDoc = doc
		} else if len(tags) >= 3 && tags[2] == "contact" {
			contactDoc = doc
		}
	}

	expectedTimestamp, _ := time.Parse(time.RFC3339, "2022-12-25T04:38:18Z")
	assert.NotNil(t, conversationDoc, "Expected conversation document")
	assert.Contains(t, conversationDoc.Content(), "I want to believe", "Conversation should contain the message")
	assert.Equal(t, []string{"social", "telegram", "chat"}, conversationDoc.Tags())

	assert.NotNil(t, contactDoc, "Expected contact document")
	assert.Equal(t, "John Doe", contactDoc.Content())
	assert.Equal(t, &expectedTimestamp, contactDoc.Timestamp())
	assert.Equal(t, []string{"social", "telegram", "contact"}, contactDoc.Tags())
	assert.Equal(t, map[string]string{
		"type":        "contact",
		"firstName":   "John",
		"lastName":    "Doe",
		"phoneNumber": "+1234567890",
	}, contactDoc.Metadata())
}

func TestProcessDirectoryInput(t *testing.T) {
	testData := `{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe",
    "username": "@JohnDoe"
  },
  "contacts": {
    "about": "Contact list",
    "list": []
  },
  "chats": {
    "about": "Chat list",
    "list": [
      {
        "type": "personal_chat",
        "id": 123456,
        "name": "Test Chat",
        "messages": [
          {
            "id": 1,
            "type": "message",
            "date": "2023-01-15T10:30:00",
            "date_unixtime": "1673776200",
            "from": "JohnDoe",
            "from_id": "user1601587058",
            "text_entities": [
              {
                "type": "plain",
                "text": "Test message"
              }
            ]
          }
        ]
      }
    ]
  }
}`

	tempDir, err := os.MkdirTemp("", "telegram_test_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck

	resultJsonPath := fmt.Sprintf("%s/result.json", tempDir)
	if err := os.WriteFile(resultJsonPath, []byte(testData), 0o644); err != nil {
		t.Fatalf("Failed to write result.json: %v", err)
	}

	dbFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()                 //nolint:errcheck
	defer os.Remove(dbFile.Name()) //nolint:errcheck

	ctx := context.Background()
	store, err := db.NewStore(ctx, dbFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close() //nolint:errcheck

	processor := NewTelegramProcessor(store)
	records, err := processor.ProcessFile(ctx, tempDir)
	if err != nil {
		t.Fatalf("ProcessFile with directory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		record := records[0]
		if record.Data["type"] != "message" {
			t.Errorf("Expected message type, got %v", record.Data["type"])
		}
		if record.Data["text"] != "Test message" {
			t.Errorf("Expected 'Test message', got %v", record.Data["text"])
		}
	}

	t.Log("Successfully processed directory input and found result.json")
}

func TestProcessDirectoryInputCustomJsonName(t *testing.T) {
	testData := `{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe",
    "username": "@JohnDoe"
  },
  "contacts": {
    "about": "Contact list",
    "list": []
  },
  "chats": {
    "about": "Chat list",
    "list": [
      {
        "type": "personal_chat",
        "id": 123456,
        "name": "Test Chat",
        "messages": [
          {
            "id": 1,
            "type": "message",
            "date": "2023-01-15T10:30:00",
            "date_unixtime": "1673776200",
            "from": "JohnDoe",
            "from_id": "user1601587058",
            "text_entities": [
              {
                "type": "plain",
                "text": "Custom JSON test message"
              }
            ]
          }
        ]
      }
    ]
  }
}`

	tempDir, err := os.MkdirTemp("", "telegram_test_custom_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck

	customJsonPath := fmt.Sprintf("%s/telegram_export.json", tempDir)
	if err := os.WriteFile(customJsonPath, []byte(testData), 0o644); err != nil {
		t.Fatalf("Failed to write telegram_export.json: %v", err)
	}

	dbFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()                 //nolint:errcheck
	defer os.Remove(dbFile.Name()) //nolint:errcheck

	ctx := context.Background()
	store, err := db.NewStore(ctx, dbFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close() //nolint:errcheck

	processor := NewTelegramProcessor(store)
	records, err := processor.ProcessFile(ctx, tempDir)
	if err != nil {
		t.Fatalf("ProcessFile with directory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		record := records[0]
		if record.Data["type"] != "message" {
			t.Errorf("Expected message type, got %v", record.Data["type"])
		}
		if record.Data["text"] != "Custom JSON test message" {
			t.Errorf("Expected 'Custom JSON test message', got %v", record.Data["text"])
		}
	}

	t.Log("Successfully processed directory input with custom JSON file name")
}

func TestProcessDirectoryNoJsonFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "telegram_test_empty_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck

	txtFilePath := fmt.Sprintf("%s/readme.txt", tempDir)
	if err := os.WriteFile(txtFilePath, []byte("This is not a JSON file"), 0o644); err != nil {
		t.Fatalf("Failed to write text file: %v", err)
	}

	dbFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()                 //nolint:errcheck
	defer os.Remove(dbFile.Name()) //nolint:errcheck

	ctx := context.Background()
	store, err := db.NewStore(ctx, dbFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close() //nolint:errcheck

	processor := NewTelegramProcessor(store)
	records, err := processor.ProcessFile(ctx, tempDir)

	if err == nil {
		t.Errorf("Expected error when no JSON files found, but got none")
	}

	if records != nil {
		t.Errorf("Expected nil records when error occurs, got %v", records)
	}

	expectedError := "no JSON files found in directory"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got '%v'", expectedError, err)
	}

	t.Log("Successfully handled directory with no JSON files")
}
