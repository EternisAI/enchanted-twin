package telegram

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
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

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
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

	// Verify message attribution - messages are now in conversation format
	var conversationRecord map[string]interface{}
	for _, record := range records {
		if record.Data["type"] == "conversation" {
			conversationRecord = record.Data
			break
		}
	}

	if conversationRecord == nil {
		t.Fatal("Expected to find conversation record")
	}

	messages, ok := conversationRecord["messages"].([]messageData)
	if !ok {
		t.Fatal("Expected messages to be []messageData")
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages in conversation, got %d", len(messages))
	}

	var userMessage, otherMessage *messageData
	for i := range messages {
		if messages[i].From == "JohnDoe" {
			userMessage = &messages[i]
		} else if messages[i].From == "Alice Smith" {
			otherMessage = &messages[i]
		}
	}

	if userMessage == nil {
		t.Fatal("Expected to find user message")
	}
	if !userMessage.MyMessage {
		t.Error("Expected user message to be marked as myMessage=true")
	}

	if otherMessage == nil {
		t.Fatal("Expected to find other message")
	}
	if otherMessage.MyMessage {
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

func TestUsernameExtractionAndDocumentGeneration(t *testing.T) {
	// Comprehensive test: raw data → ProcessFile → ToDocuments → verify trimming
	testData := `{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe",
    "username": "@JohnDoe",
    "phone_number": "+33 6 16 87 45 98"
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
        "id": 12345,
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
                "text": "Hi John!"
              }
            ]
          }
        ]
      }
    ]
  }
}`

	tmpFile, err := os.CreateTemp("", "telegram_full_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close() //nolint:errcheck

	dbFile, err := os.CreateTemp("", "test_full_*.db")
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
	processor := NewTelegramProcessor(store, logger)

	// Step 1: Process the file (this should extract and store the username)
	records, err := processor.ProcessFile(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Step 2: Verify username was extracted and stored correctly (with @ prefix)
	sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
	if err != nil {
		t.Fatalf("Failed to get source username: %v", err)
	}

	if sourceUsername == nil {
		t.Fatal("Expected username to be extracted and stored")
	}

	if sourceUsername.Username != "@JohnDoe" {
		t.Errorf("Expected stored username '@JohnDoe', got '%s'", sourceUsername.Username)
	}

	// Step 3: Convert records to documents (this should trim the @ prefix)
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	// Step 4: Find and verify the conversation document
	var conversationDoc *memory.ConversationDocument
	for _, doc := range documents {
		if doc.Metadata()["type"] == "conversation" {
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				conversationDoc = convDoc
				break
			}
		}
	}

	if conversationDoc == nil {
		t.Fatal("Expected to find conversation document")
	}

	// Step 5: Verify the username is trimmed in the document (no @ prefix)
	if conversationDoc.User != "JohnDoe" {
		t.Errorf("Expected document user 'JohnDoe' (trimmed), got '%s'", conversationDoc.User)
	}

	// Step 6: Verify message attribution works correctly with trimmed username
	if len(conversationDoc.Conversation) != 2 {
		t.Errorf("Expected 2 messages in conversation, got %d", len(conversationDoc.Conversation))
	}

	// Verify people list contains the participants (including the extracted username)
	// Note: The people list includes both the "from" speakers and "to" recipients
	actualPeople := conversationDoc.People
	t.Logf("People in conversation: %v", actualPeople)

	// Should contain at least the main participants
	expectedMinPeople := 3 // JohnDoe, Alice Smith, and @JohnDoe (as recipient)
	if len(actualPeople) < expectedMinPeople {
		t.Errorf("Expected at least %d people, got %d: %v", expectedMinPeople, len(actualPeople), actualPeople)
	}

	// Verify key participants are present
	hasJohnDoe := false
	hasAliceSmith := false
	for _, person := range actualPeople {
		if person == "JohnDoe" || person == "@JohnDoe" {
			hasJohnDoe = true
		}
		if person == "Alice Smith" {
			hasAliceSmith = true
		}
	}

	if !hasJohnDoe {
		t.Error("Expected JohnDoe to be in people list")
	}
	if !hasAliceSmith {
		t.Error("Expected Alice Smith to be in people list")
	}

	t.Log("✅ COMPREHENSIVE USERNAME TEST PASSED!")
	t.Logf("✅ Raw data username: @JohnDoe")
	t.Logf("✅ Database stored username: %s", sourceUsername.Username)
	t.Logf("✅ Document user field: %s (trimmed)", conversationDoc.User)
	t.Log("✅ Complete extraction → storage → trimming → document generation pipeline works correctly")
}
