package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
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

	testData := `{"data":{"type":"conversation","chatId":"1601587058","chatType":"saved_messages","chatName":"Test Chat","messages":[{"id":59318,"messageType":"message","from":"Eternal22","to":"xxx","text":"I want to believe","timestamp":"2022-12-25T04:38:18Z","forwardedFrom":"","savedFrom":"Mahamat New","myMessage":false}],"people":["Eternal22","xxx"],"user":"xxx"},"timestamp":"2022-12-25T04:38:18Z","source":"telegram"}
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

	logger := log.New(os.Stdout)
	telegramProcessor := NewTelegramProcessor(store, logger)
	docs, err := telegramProcessor.ToDocuments(context.Background(), records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	assert.Equal(t, 2, len(docs), "Expected 2 documents")

	var conversationDoc, contactDoc memory.Document
	for _, doc := range docs {
		docType := doc.Metadata()["type"]
		switch docType {
		case "conversation":
			conversationDoc = doc
		case "contact":
			contactDoc = doc
		}
	}

	expectedTimestamp, _ := time.Parse(time.RFC3339, "2022-12-25T04:38:18Z")
	assert.NotNil(t, conversationDoc, "Expected conversation document")
	assert.Contains(t, conversationDoc.Content(), "I want to believe", "Conversation should contain the message")
	assert.Equal(t, []string{"social", "chat"}, conversationDoc.Tags())

	assert.NotNil(t, contactDoc, "Expected contact document")
	assert.Equal(t, "John Doe", contactDoc.Content())
	assert.Equal(t, &expectedTimestamp, contactDoc.Timestamp())
	assert.Equal(t, []string{"social", "contact"}, contactDoc.Tags())
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

	logger := log.New(os.Stdout)
	processor := NewTelegramProcessor(store, logger)
	records, err := processor.ProcessFile(ctx, tempDir)
	if err != nil {
		t.Fatalf("ProcessFile with directory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		record := records[0]
		if record.Data["type"] != "conversation" {
			t.Errorf("Expected conversation type, got %v", record.Data["type"])
		}

		messages, ok := record.Data["messages"].([]messageData)
		if !ok {
			t.Errorf("Expected messages to be []messageData, got %T", record.Data["messages"])
		} else if len(messages) != 1 {
			t.Errorf("Expected 1 message in conversation, got %d", len(messages))
		} else if messages[0].Text != "Test message" {
			t.Errorf("Expected 'Test message', got %v", messages[0].Text)
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

	logger := log.New(os.Stdout)
	processor := NewTelegramProcessor(store, logger)
	records, err := processor.ProcessFile(ctx, tempDir)
	if err != nil {
		t.Fatalf("ProcessFile with directory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		record := records[0]
		if record.Data["type"] != "conversation" {
			t.Errorf("Expected conversation type, got %v", record.Data["type"])
		}

		messages, ok := record.Data["messages"].([]messageData)
		if !ok {
			t.Errorf("Expected messages to be []messageData, got %T", record.Data["messages"])
		} else if len(messages) != 1 {
			t.Errorf("Expected 1 message in conversation, got %d", len(messages))
		} else if messages[0].Text != "Custom JSON test message" {
			t.Errorf("Expected 'Custom JSON test message', got %v", messages[0].Text)
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

	logger := log.New(os.Stdout)
	processor := NewTelegramProcessor(store, logger)
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

func TestProcessRealTelegramExport(t *testing.T) {
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
	processor := NewTelegramProcessor(store, logger)

	// Use the real telegram export sample file
	records, err := processor.ProcessFile(ctx, "telegram_export_sample.json")
	if err != nil {
		t.Fatalf("ProcessFile with real sample failed: %v", err)
	}

	t.Logf("Processed %d records from real telegram export", len(records))

	// Should have contacts + conversations
	// From the sample: 5 contacts + multiple chat conversations
	if len(records) == 0 {
		t.Error("Expected to process some records, but got 0")
	}

	// Check that we have contact records
	contactCount := 0
	conversationCount := 0

	for _, record := range records {
		switch record.Data["type"] {
		case "contact":
			contactCount++
			t.Logf("Contact: %s %s", record.Data["firstName"], record.Data["lastName"])
		case "conversation":
			conversationCount++
			chatName, _ := record.Data["chatName"].(string)
			messages, _ := record.Data["messages"].([]messageData)
			t.Logf("Conversation: %s with %d messages", chatName, len(messages))

			// Verify messages have text content
			for i, msg := range messages {
				if msg.Text == "" {
					t.Errorf("Message %d in conversation %s has empty text", i, chatName)
				} else {
					t.Logf("  Message from %s: %s", msg.From, msg.Text[:min(50, len(msg.Text))])
				}
			}
		}
	}

	t.Logf("Found %d contacts and %d conversations", contactCount, conversationCount)

	if contactCount == 0 {
		t.Error("Expected to find contacts, but found none")
	}

	// private_group chats are processed, so we expect to find conversations
	if conversationCount == 0 {
		t.Error("Expected to find conversations, but found none")
	}

	// Check username extraction
	sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
	if err != nil {
		t.Fatalf("Failed to get source username: %v", err)
	}

	if sourceUsername == nil {
		t.Error("Expected username to be extracted from real sample")
	} else {
		t.Logf("Extracted username: %s", sourceUsername.Username)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestToDocumentsEndToEnd(t *testing.T) {
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

	// Set up source username
	sourceUsername := db.SourceUsername{
		Source:   "telegram",
		Username: "@JohnDoe",
	}
	err = store.SetSourceUsername(ctx, sourceUsername)
	if err != nil {
		t.Fatalf("Failed to set source username: %v", err)
	}

	logger := log.New(os.Stdout)
	processor := NewTelegramProcessor(store, logger)

	// Create realistic test records with proper message structure
	timestamp1, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")
	timestamp2, _ := time.Parse(time.RFC3339, "2023-01-15T10:31:00Z")

	msg1 := messageData{
		ID:          1,
		MessageType: "message",
		From:        "@JohnDoe",
		To:          "Alice",
		Text:        "Hello Alice, how are you?",
		Timestamp:   timestamp1,
		MyMessage:   true,
	}

	msg2 := messageData{
		ID:          2,
		MessageType: "message",
		From:        "Alice",
		To:          "@JohnDoe",
		Text:        "Hi John! I'm doing great, thanks!",
		Timestamp:   timestamp2,
		MyMessage:   false,
	}

	conversationRecord := types.Record{
		Data: map[string]interface{}{
			"type":     "conversation",
			"chatId":   "12345",
			"chatType": "personal_chat",
			"chatName": "Chat with Alice",
			"messages": []messageData{msg1, msg2},
			"people":   []string{"@JohnDoe", "Alice"},
			"user":     "@JohnDoe",
		},
		Timestamp: timestamp1,
		Source:    "telegram",
	}

	contactRecord := types.Record{
		Data: map[string]interface{}{
			"type":        "contact",
			"firstName":   "Alice",
			"lastName":    "Smith",
			"phoneNumber": "+1234567890",
		},
		Timestamp: timestamp1,
		Source:    "telegram",
	}

	records := []types.Record{conversationRecord, contactRecord}

	// Test ToDocuments
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	assert.Equal(t, 2, len(documents), "Expected 2 documents")

	var conversationDoc *memory.ConversationDocument
	var contactDoc *memory.TextDocument

	for _, doc := range documents {
		if doc.Metadata()["type"] == "conversation" {
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				conversationDoc = convDoc
			}
		} else if doc.Metadata()["type"] == "contact" {
			if textDoc, ok := doc.(*memory.TextDocument); ok {
				contactDoc = textDoc
			}
		}
	}

	// Verify conversation document
	assert.NotNil(t, conversationDoc, "Expected conversation document")
	assert.Equal(t, "12345", conversationDoc.FieldID)
	assert.Equal(t, "telegram", conversationDoc.FieldSource)
	assert.Equal(t, []string{"social", "chat"}, conversationDoc.FieldTags)
	assert.Equal(t, []string{"@JohnDoe", "Alice"}, conversationDoc.People)
	assert.Equal(t, "JohnDoe", conversationDoc.User)

	// Verify messages in conversation
	assert.Equal(t, 2, len(conversationDoc.Conversation), "Expected 2 messages in conversation")

	msg1Doc := conversationDoc.Conversation[0]
	assert.Equal(t, "@JohnDoe", msg1Doc.Speaker)
	assert.Equal(t, "Hello Alice, how are you?", msg1Doc.Content)
	assert.Equal(t, timestamp1, msg1Doc.Time)

	msg2Doc := conversationDoc.Conversation[1]
	assert.Equal(t, "Alice", msg2Doc.Speaker)
	assert.Equal(t, "Hi John! I'm doing great, thanks!", msg2Doc.Content)
	assert.Equal(t, timestamp2, msg2Doc.Time)

	// Verify contact document
	assert.NotNil(t, contactDoc, "Expected contact document")
	assert.Equal(t, "Alice Smith", contactDoc.FieldContent)
	assert.Equal(t, []string{"social", "contact"}, contactDoc.FieldTags)

	t.Log("✅ End-to-end ToDocuments test passed - messages are properly processed")
}

func TestJSONLMarshallingSimple(t *testing.T) {
	// This test verifies that messageData structs can be properly marshaled to JSON
	// It would FAIL before the fix (when fields were unexported) and PASS after the fix

	timestamp1, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")
	timestamp2, _ := time.Parse(time.RFC3339, "2023-01-15T10:31:00Z")

	// Create messageData instances as they would be created by the processor
	msg1 := messageData{
		ID:            12345,
		MessageType:   "message",
		From:          "JohnDoe",
		To:            "Alice",
		Text:          "Hello Alice, how are you doing today?",
		Timestamp:     timestamp1,
		ForwardedFrom: "",
		SavedFrom:     "",
		MyMessage:     true,
	}

	msg2 := messageData{
		ID:            12346,
		MessageType:   "message",
		From:          "Alice",
		To:            "JohnDoe",
		Text:          "Hi John! I'm doing great, thanks for asking!",
		Timestamp:     timestamp2,
		ForwardedFrom: "",
		SavedFrom:     "",
		MyMessage:     false,
	}

	// Create a record as it would be created by the processor
	record := types.Record{
		Data: map[string]interface{}{
			"type":     "conversation",
			"chatId":   "98765",
			"chatType": "personal_chat",
			"chatName": "Chat with Alice",
			"messages": []messageData{msg1, msg2}, // This is the critical part
			"people":   []string{"JohnDoe", "Alice"},
			"user":     "JohnDoe",
		},
	}

	// Test JSON marshaling directly
	jsonData, err := json.Marshal(record.Data)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshalled map[string]interface{}
	err = json.Unmarshal(jsonData, &unmarshalled)
	if err != nil {
		t.Fatalf("JSON unmarshalling failed: %v", err)
	}

	// Verify messages are properly serialized
	messagesInterface, ok := unmarshalled["messages"]
	if !ok {
		t.Fatal("Expected 'messages' field in unmarshalled data")
	}

	messagesSlice, ok := messagesInterface.([]interface{})
	if !ok {
		t.Fatalf("Expected messages to be []interface{}, got %T", messagesInterface)
	}

	if len(messagesSlice) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messagesSlice))
	}

	// Verify first message has all fields (this would be empty {} before the fix)
	msg1Map, ok := messagesSlice[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected first message to be map[string]interface{}, got %T", messagesSlice[0])
	}

	// These assertions would FAIL before the fix because msg1Map would be empty {}
	if msg1Map["id"] != float64(12345) {
		t.Errorf("Expected message ID 12345, got %v", msg1Map["id"])
	}

	if msg1Map["from"] != "JohnDoe" {
		t.Errorf("Expected message from 'JohnDoe', got %v", msg1Map["from"])
	}

	if msg1Map["text"] != "Hello Alice, how are you doing today?" {
		t.Errorf("Expected message text 'Hello Alice, how are you doing today?', got %v", msg1Map["text"])
	}

	if msg1Map["myMessage"] != true {
		t.Errorf("Expected myMessage true, got %v", msg1Map["myMessage"])
	}

	// Verify second message
	msg2Map, ok := messagesSlice[1].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected second message to be map[string]interface{}, got %T", messagesSlice[1])
	}

	if msg2Map["from"] != "Alice" {
		t.Errorf("Expected message from 'Alice', got %v", msg2Map["from"])
	}

	if msg2Map["text"] != "Hi John! I'm doing great, thanks for asking!" {
		t.Errorf("Expected message text 'Hi John! I'm doing great, thanks for asking!', got %v", msg2Map["text"])
	}

	if msg2Map["myMessage"] != false {
		t.Errorf("Expected myMessage false, got %v", msg2Map["myMessage"])
	}

	t.Log("✅ CRITICAL JSON MARSHALING TEST PASSED!")
	t.Log("✅ Messages are properly serialized with all fields (id, from, text, myMessage, etc.)")
	t.Log("✅ This test would have FAILED before making messageData fields exported")
	t.Log("✅ Before the fix: messages would appear as empty objects {} in JSON")
	t.Log("✅ After the fix: messages contain all the expected data")
}

func TestContactDocumentWithClearContext(t *testing.T) {
	// Test that contact documents now have clear context to prevent confusion with primary user
	dbFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()
	defer os.Remove(dbFile.Name())

	ctx := context.Background()
	store, err := db.NewStore(ctx, dbFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	timestamp1, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")

	// Create a contact record like Guillaume Uguen from the test data
	contactRecord := types.Record{
		Data: map[string]interface{}{
			"type":        "contact",
			"firstName":   "Guillaume",
			"lastName":    "Uguen",
			"phoneNumber": "0033632728113",
		},
		Timestamp: timestamp1,
		Source:    "telegram",
	}

	records := []types.Record{contactRecord}

	logger := log.New(os.Stdout)
	processor := NewTelegramProcessor(store, logger)

	// Test ToDocuments
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	assert.Equal(t, 1, len(documents), "Expected 1 document")

	contactDoc := documents[0]

	// Verify the content now clearly indicates this is a contact entry
	assert.Contains(t, contactDoc.Content(), "CONTACT ENTRY:", "Content should start with CONTACT ENTRY:")
	assert.Contains(t, contactDoc.Content(), "Guillaume Uguen", "Content should contain the contact name")
	assert.Contains(t, contactDoc.Content(), "Phone: 0033632728113", "Content should contain phone number")
	assert.Contains(t, contactDoc.Content(), "not information about the primary user", "Content should clarify this is not about primary user")

	// Verify enhanced metadata
	metadata := contactDoc.Metadata()
	assert.Equal(t, "contact_entry", metadata["document_type"], "Should have document_type as contact_entry")
	assert.Equal(t, "contact_list", metadata["data_category"], "Should have data_category as contact_list")
	assert.Equal(t, "false", metadata["is_primary_user"], "Should explicitly mark is_primary_user as false")
	assert.Equal(t, "telegram_contacts", metadata["contact_source"], "Should specify contact source")
	assert.Contains(t, metadata["extraction_guidance"], "extract relationship facts only", "Should provide extraction guidance")

	// Verify enhanced tags
	tags := contactDoc.Tags()
	assert.Contains(t, tags, "contact", "Should have contact tag")
	assert.Contains(t, tags, "contact_list", "Should have contact_list tag")
	assert.Contains(t, tags, "social", "Should have social tag")

	t.Log("✅ Contact document now has clear context to prevent confusion with primary user information")
	t.Logf("✅ Content: %s", contactDoc.Content())
	t.Logf("✅ Metadata: %+v", metadata)
}
