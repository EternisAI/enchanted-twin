package telegram

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
)

func TestUsernameExtraction(t *testing.T) {
	testData := `{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe",
    "last_name": "",
    "phone_number": "+33 6 16 87 45 98",
    "username": "@JohnDoe",
    "bio": "ⵥ"
  },
  "contacts": {
    "about": "This is your contact list.",
    "list": [
      {
        "first_name": "Alice",
        "last_name": "Smith", 
        "phone_number": "+1 555 0123",
        "date": "2023-01-15T10:30:00",
        "date_unixtime": "1673776200"
      }
    ]
  },
  "chats": {
    "about": "This is your chat list.",
    "list": [
      {
        "type": "personal_chat",
        "id": 123456,
        "name": "Alice Smith",
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
                "text": "Hello Alice!"
              }
            ]
          },
          {
            "id": 2,
            "type": "message", 
            "date": "2023-01-15T10:31:00",
            "date_unixtime": "1673776260",
            "from": "Alice Smith",
            "from_id": "user987654321",
            "text_entities": [
              {
                "type": "plain",
                "text": "Hi John! How are you?"
              }
            ]
          }
        ]
      }
    ]
  }
}`

	tmpFile, err := os.CreateTemp("", "telegram_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close() //nolint:errcheck

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

	logger := log.New(os.Stdout)
	source := NewTelegramProcessor(store, logger)
	records, err := source.ProcessFile(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("ProcessFileWithStore failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}

	sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
	if err != nil {
		t.Fatalf("Failed to get source username: %v", err)
	}

	if sourceUsername == nil {
		t.Fatal("Expected username to be extracted, but got nil")
	}

	if sourceUsername.Username != "@JohnDoe" {
		t.Errorf("Expected username '@JohnDoe', got '%s'", sourceUsername.Username)
	}

	if sourceUsername.UserID == nil || *sourceUsername.UserID != "1601587058" {
		t.Errorf("Expected user ID '1601587058', got %v", sourceUsername.UserID)
	}

	if sourceUsername.FirstName == nil || *sourceUsername.FirstName != "JohnDoe" {
		t.Errorf("Expected first name 'JohnDoe', got %v", sourceUsername.FirstName)
	}

	if sourceUsername.PhoneNumber == nil || *sourceUsername.PhoneNumber != "+33 6 16 87 45 98" {
		t.Errorf("Expected phone number '+33 6 16 87 45 98', got %v", sourceUsername.PhoneNumber)
	}

	if sourceUsername.Bio == nil || *sourceUsername.Bio != "ⵥ" {
		t.Errorf("Expected bio 'ⵥ', got %v", sourceUsername.Bio)
	}

	// Verify message attribution
	var userMessage, otherMessage map[string]interface{}
	for _, record := range records {
		if record.Data["type"] == "message" {
			from, _ := record.Data["from"].(string)
			switch from {
			case "JohnDoe":
				userMessage = record.Data
			case "Alice Smith":
				otherMessage = record.Data
			}
		}
	}

	if userMessage == nil {
		t.Fatal("Expected to find user message")
	}
	if myMessage, ok := userMessage["myMessage"].(bool); !ok || !myMessage {
		t.Error("Expected user message to be marked as myMessage=true")
	}

	if otherMessage == nil {
		t.Fatal("Expected to find other message")
	}
	if myMessage, ok := otherMessage["myMessage"].(bool); !ok || myMessage {
		t.Error("Expected other message to be marked as myMessage=false")
	}
}

func TestUsernameExtractionFallback(t *testing.T) {
	// Test data without username in personal_information
	testData := `{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe"
  },
  "contacts": {
    "about": "This is your contact list.",
    "list": []
  },
  "chats": {
    "about": "This is your chat list.",
    "list": []
  }
}`

	tmpFile, err := os.CreateTemp("", "telegram_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close() //nolint:errcheck

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

	logger := log.New(os.Stdout)
	source := NewTelegramProcessor(store, logger)
	_, err = source.ProcessFile(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("ProcessFileWithStore failed: %v", err)
	}

	// Verify no username was stored (since extraction failed)
	sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
	if err != nil {
		t.Fatalf("Failed to get source username: %v", err)
	}

	if sourceUsername != nil {
		t.Error("Expected no username to be stored when extraction fails")
	}
}

func TestProcessFileWithStoreExample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping example test in short mode")
	}

	// This is an example of how to use the username extraction functionality
	t.Log("Example: Using ProcessFileWithStore with username extraction")

	// Create test data
	testData := map[string]interface{}{
		"personal_information": map[string]interface{}{
			"user_id":      1601587058,
			"first_name":   "JohnDoe",
			"username":     "@JohnDoe",
			"phone_number": "+33 6 16 87 45 98",
		},
		"contacts": map[string]interface{}{
			"about": "Contact list",
			"list":  []interface{}{},
		},
		"chats": map[string]interface{}{
			"about": "Chat list",
			"list":  []interface{}{},
		},
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "example_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	encoder := json.NewEncoder(tmpFile)
	if err := encoder.Encode(testData); err != nil {
		t.Fatalf("Failed to encode test data: %v", err)
	}
	tmpFile.Close() //nolint:errcheck

	dbFile, err := os.CreateTemp("", "example_*.db")
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

	logger := log.New(os.Stdout)
	source := NewTelegramProcessor(store, logger)
	records, err := source.ProcessFile(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("ProcessFileWithStore failed: %v", err)
	}

	t.Logf("Processed %d records", len(records))

	sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
	if err != nil {
		t.Fatalf("Failed to get source username: %v", err)
	}

	if sourceUsername != nil {
		t.Logf("Extracted username: %s", sourceUsername.Username)
		if sourceUsername.FirstName != nil {
			t.Logf("First name: %s", *sourceUsername.FirstName)
		}
		if sourceUsername.PhoneNumber != nil {
			t.Logf("Phone: %s", *sourceUsername.PhoneNumber)
		}
	} else {
		t.Log("No username found in export")
	}

	allUsernames, err := store.GetAllSourceUsernames(ctx)
	if err != nil {
		t.Fatalf("Failed to get all usernames: %v", err)
	}

	t.Logf("Total stored usernames: %d", len(allUsernames))
	for _, username := range allUsernames {
		t.Logf("- %s: %s", username.Source, username.Username)
	}
}
