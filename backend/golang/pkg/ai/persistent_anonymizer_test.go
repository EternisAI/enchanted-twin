package ai

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTempDatabase(t *testing.T) (*sql.DB, string) {
	tempFile, err := os.CreateTemp("", "test_anonymizer_*.db")
	require.NoError(t, err)
	_ = tempFile.Close()

	db, err := sql.Open("sqlite3", tempFile.Name())
	require.NoError(t, err)

	// Run migrations
	err = runAnonymizationMigrations(db)
	require.NoError(t, err)

	return db, tempFile.Name()
}

func runAnonymizationMigrations(db *sql.DB) error {
	createTables := `
		CREATE TABLE conversation_dicts (
			conversation_id TEXT PRIMARY KEY NOT NULL,
			dict_data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE anonymized_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			message_hash TEXT NOT NULL,
			anonymized_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(conversation_id, message_hash),
			FOREIGN KEY (conversation_id) REFERENCES conversation_dicts(conversation_id) ON DELETE CASCADE
		);

		CREATE INDEX idx_conversation_messages ON anonymized_messages(conversation_id);
		CREATE INDEX idx_message_hash ON anonymized_messages(message_hash);
		CREATE INDEX idx_conversation_updated ON conversation_dicts(updated_at);
	`

	_, err := db.Exec(createTables)
	return err
}

func TestPersistentAnonymizationAcrossRestarts(t *testing.T) {
	t.Run("BasicPersistenceAcrossRestart", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		conversationID := "test-conversation-001"
		logger := log.New(nil)

		// === FIRST SERVICE INSTANCE ===
		anonymizer1 := NewPersistentAnonymizer(db, logger)
		defer anonymizer1.Shutdown()

		// First batch of messages
		messages1 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John, how are you?"),
			openai.AssistantMessage("Hi there! I'm doing well at OpenAI."),
		}

		_, dict1, _, err := anonymizer1.AnonymizeMessages(
			context.Background(), conversationID, messages1, nil, make(chan struct{}))
		require.NoError(t, err)

		// Verify anonymization occurred
		assert.Contains(t, dict1, "PERSON_001")
		assert.Contains(t, dict1, "COMPANY_001")
		assert.Equal(t, "John", dict1["PERSON_001"])
		assert.Equal(t, "OpenAI", dict1["COMPANY_001"])

		// Shutdown first instance
		anonymizer1.Shutdown()

		// === SECOND SERVICE INSTANCE (RESTART) ===
		anonymizer2 := NewPersistentAnonymizer(db, logger)
		defer anonymizer2.Shutdown()

		// Second batch of messages with new and existing entities
		messages2 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("John, please introduce me to Alice from OpenAI."),
			openai.AssistantMessage("Sure! Alice is a great colleague at OpenAI."),
		}

		_, dict2, rules2, err := anonymizer2.AnonymizeMessages(
			context.Background(), conversationID, messages2, nil, make(chan struct{}))
		require.NoError(t, err)

		// Verify persistence: John and OpenAI should have same tokens
		assert.Equal(t, "John", dict2["PERSON_001"])
		assert.Equal(t, "OpenAI", dict2["COMPANY_001"])

		// Verify new entity: Alice should get new token
		assert.Contains(t, dict2, "PERSON_003")
		assert.Equal(t, "Alice", dict2["PERSON_003"])

		// Verify only new rules were returned
		assert.NotContains(t, rules2, "PERSON_001")  // John was not newly anonymized
		assert.NotContains(t, rules2, "COMPANY_001") // OpenAI was not newly anonymized
		assert.Contains(t, rules2, "PERSON_003")     // Alice was newly anonymized

		t.Logf("Dict after restart: %v", dict2)
		t.Logf("New rules after restart: %v", rules2)
	})

	t.Run("MultipleRestartsWithIncrementalMessages", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		conversationID := "test-conversation-002"
		logger := log.New(nil)

		// Simulate 3 service restarts with message batches
		entities := []string{"John", "Alice", "Bob", "OpenAI", "Microsoft", "Google"}
		expectedTokens := make(map[string]string)

		for restart := 0; restart < 3; restart++ {
			t.Logf("=== SERVICE RESTART %d ===", restart+1)

			anonymizer := NewPersistentAnonymizer(db, logger)

			// Add 2 new entities per restart
			startIdx := restart * 2
			newEntities := entities[startIdx : startIdx+2]

			messages := []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello " + newEntities[0] + " and " + newEntities[1] + "!"),
			}

			_, dict, rules, err := anonymizer.AnonymizeMessages(
				context.Background(), conversationID, messages, nil, make(chan struct{}))
			require.NoError(t, err)

			// Verify all previous entities are still in dictionary
			for expectedToken, expectedEntity := range expectedTokens {
				assert.Equal(t, expectedEntity, dict[expectedToken],
					"Entity %s should still map to %s after restart %d",
					expectedEntity, expectedToken, restart+1)
			}

			// Track new tokens
			for token, entity := range rules {
				expectedTokens[token] = entity
			}

			// Verify only new entities are in rules
			assert.Len(t, rules, 2, "Should have exactly 2 new rules per restart")

			anonymizer.Shutdown()

			t.Logf("Dictionary after restart %d: %v", restart+1, dict)
			t.Logf("New rules after restart %d: %v", restart+1, rules)
		}
	})

	t.Run("ConversationIsolationAcrossRestarts", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)

		// === FIRST SERVICE INSTANCE ===
		anonymizer1 := NewPersistentAnonymizer(db, logger)

		// Process same entity in different conversations
		conv1Messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hi John from OpenAI!"),
		}
		conv2Messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John from Microsoft!"),
		}

		_, dict1, _, err := anonymizer1.AnonymizeMessages(
			context.Background(), "conv-1", conv1Messages, nil, make(chan struct{}))
		require.NoError(t, err)

		_, dict2, _, err := anonymizer1.AnonymizeMessages(
			context.Background(), "conv-2", conv2Messages, nil, make(chan struct{}))
		require.NoError(t, err)

		// John should have same tokens in both conversations (predefined mapping)
		johnToken1 := findTokenForEntity(dict1, "John")
		johnToken2 := findTokenForEntity(dict2, "John")
		assert.Equal(t, johnToken1, johnToken2, "John should have same predefined token across conversations")

		// But OpenAI and Microsoft should have different tokens
		openaiToken1 := findTokenForEntity(dict1, "OpenAI")
		microsoftToken2 := findTokenForEntity(dict2, "Microsoft")
		assert.NotEqual(t, openaiToken1, microsoftToken2, "Different companies should have different tokens")

		anonymizer1.Shutdown()

		// === SECOND SERVICE INSTANCE (RESTART) ===
		anonymizer2 := NewPersistentAnonymizer(db, logger)

		// Add more messages to both conversations
		conv1NewMessages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("John, how is work at OpenAI?"),
		}
		conv2NewMessages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("John, how is work at Microsoft?"),
		}

		_, dict1After, rules1After, err := anonymizer2.AnonymizeMessages(
			context.Background(), "conv-1", conv1NewMessages, nil, make(chan struct{}))
		require.NoError(t, err)

		_, dict2After, rules2After, err := anonymizer2.AnonymizeMessages(
			context.Background(), "conv-2", conv2NewMessages, nil, make(chan struct{}))
		require.NoError(t, err)

		// Verify conversation isolation maintained after restart
		johnToken1After := findTokenForEntity(dict1After, "John")
		johnToken2After := findTokenForEntity(dict2After, "John")

		assert.Equal(t, johnToken1, johnToken1After, "John's token in conv-1 should persist across restart")
		assert.Equal(t, johnToken2, johnToken2After, "John's token in conv-2 should persist across restart")

		// No new rules should be generated (all entities already anonymized)
		assert.Empty(t, rules1After, "No new rules should be generated for conv-1")
		assert.Empty(t, rules2After, "No new rules should be generated for conv-2")

		anonymizer2.Shutdown()
	})
}

// Test memory-only mode (no persistence).
func TestMemoryOnlyAnonymization(t *testing.T) {
	t.Run("MemoryOnlyModeWithoutPersistence", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)
		defer anonymizer.Shutdown()

		// First call with empty conversation ID (memory-only)
		messages1 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John from OpenAI!"),
		}

		_, dict1, rules1, err := anonymizer.AnonymizeMessages(
			context.Background(), "", messages1, nil, make(chan struct{}))
		require.NoError(t, err)

		assert.Contains(t, dict1, "PERSON_001")
		assert.Contains(t, dict1, "COMPANY_001")

		// Second call with same messages but different dictionary
		messages2 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John from OpenAI!"),
		}

		_, dict2, rules2, err := anonymizer.AnonymizeMessages(
			context.Background(), "", messages2, nil, make(chan struct{}))
		require.NoError(t, err)

		// Should get fresh anonymization (no persistence)
		assert.Equal(t, dict1, dict2, "Same input should produce same output in memory-only mode")
		assert.Equal(t, rules1, rules2, "Same input should produce same rules in memory-only mode")

		// Third call with existing dictionary
		existingDict := map[string]string{
			"PERSON_001":  "John",
			"COMPANY_001": "OpenAI",
		}

		messages3 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John from OpenAI and Alice from Microsoft!"),
		}

		_, dict3, rules3, err := anonymizer.AnonymizeMessages(
			context.Background(), "", messages3, existingDict, make(chan struct{}))
		require.NoError(t, err)

		// Should reuse existing dictionary and only add new entities
		assert.Equal(t, "John", dict3["PERSON_001"])
		assert.Equal(t, "OpenAI", dict3["COMPANY_001"])
		assert.Contains(t, dict3, "PERSON_003")  // Alice
		assert.Contains(t, dict3, "COMPANY_002") // Microsoft

		// Only new entities should be in rules
		assert.NotContains(t, rules3, "PERSON_001")  // John was provided
		assert.NotContains(t, rules3, "COMPANY_001") // OpenAI was provided
		assert.Contains(t, rules3, "PERSON_003")     // Alice is new
		assert.Contains(t, rules3, "COMPANY_002")    // Microsoft is new
	})
}

// Test backward compatibility.
func TestBackwardCompatibilityWithExistingInterface(t *testing.T) {
	t.Run("ExistingCompletionsMethodStillWorks", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)
		defer anonymizer.Shutdown()

		mockLLM := &mockCompletionsService{
			response: openai.ChatCompletionMessage{
				Content: "Hello PERSON_001!",
			},
		}

		service, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
			CompletionsService: mockLLM,
			AnonymizerManager:  NewPersistentAnonymizerManager(db, logger),
			ExecutorWorkers:    1,
			Logger:             logger,
		})
		require.NoError(t, err)
		defer service.Shutdown()

		// Use existing method (should work in memory-only mode)
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John!"),
		}

		result, err := service.Completions(context.Background(), messages, nil, "test-model", Background)
		require.NoError(t, err)

		// Should work as before (memory-only, no persistence)
		assert.Contains(t, result.ReplacementRules, "PERSON_001")
		assert.Equal(t, "John", result.ReplacementRules["PERSON_001"])
		assert.Equal(t, "Hello John!", result.Message.Content)
	})
}

// Test database corruption recovery.
func TestDatabaseCorruptionRecovery(t *testing.T) {
	t.Run("RecoveryFromCorruptedDatabase", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)

		// Create some conversation data
		conversationID := "test-conversation"
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John!"),
		}

		_, _, _, err := anonymizer.AnonymizeMessages(
			context.Background(), conversationID, messages, nil, make(chan struct{}))
		require.NoError(t, err)

		anonymizer.Shutdown()
		_ = db.Close()

		// Corrupt the database
		err = os.WriteFile(tempDB, []byte("corrupted data"), 0o644)
		require.NoError(t, err)

		// Create new database connection - should handle corruption gracefully
		db2, err := sql.Open("sqlite3", tempDB)
		require.NoError(t, err)
		defer func() { _ = db2.Close() }()

		// This should not crash even with corrupted database
		anonymizer2 := NewPersistentAnonymizer(db2, logger)
		defer anonymizer2.Shutdown()

		// Should fall back to memory-only mode or recreate database
		_, dict2, _, err := anonymizer2.AnonymizeMessages(
			context.Background(), conversationID, messages, nil, make(chan struct{}))

		// Should not crash and should provide some result
		assert.NoError(t, err)
		assert.NotEmpty(t, dict2)
	})
}

// Test concurrent access across restarts.
func TestConcurrentAccessAcrossRestarts(t *testing.T) {
	t.Run("ConcurrentConversationsWithRestarts", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)

		// Start multiple goroutines processing different conversations
		var wg sync.WaitGroup
		results := make(chan map[string]string, 6)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(convID string) {
				defer wg.Done()

				// Each goroutine creates its own anonymizer instance (simulating restart)
				anonymizer := NewPersistentAnonymizer(db, logger)
				defer anonymizer.Shutdown()

				messages := []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage("Hello John in conversation " + convID + "!"),
				}

				_, dict, _, err := anonymizer.AnonymizeMessages(
					context.Background(), convID, messages, nil, make(chan struct{}))
				require.NoError(t, err)

				results <- dict
			}("conv-" + string(rune('0'+i)))
		}

		wg.Wait()
		close(results)

		// Verify each conversation has isolated anonymization
		dicts := make([]map[string]string, 0, 3)
		for dict := range results {
			dicts = append(dicts, dict)
		}

		// All should have anonymized John with the same predefined token
		for _, dict := range dicts {
			johnToken := findTokenForEntity(dict, "John")
			assert.Equal(t, "PERSON_001", johnToken, "John should have predefined token PERSON_001")
		}
	})
}

// Test performance with large conversation history.
func TestPerformanceWithLargeHistory(t *testing.T) {
	t.Run("PerformanceWithManyMessages", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)
		defer anonymizer.Shutdown()

		conversationID := "large-conversation"

		// Create a large batch of messages
		var allMessages []openai.ChatCompletionMessageParamUnion
		for i := 0; i < 100; i++ {
			allMessages = append(allMessages, openai.UserMessage("Message "+string(rune('0'+i%10))+" from John"))
		}

		// First anonymization should process all messages
		start := time.Now()
		_, dict1, rules1, err := anonymizer.AnonymizeMessages(
			context.Background(), conversationID, allMessages, nil, make(chan struct{}))
		elapsed1 := time.Since(start)
		require.NoError(t, err)

		t.Logf("First anonymization (100 messages): %v", elapsed1)
		assert.NotEmpty(t, rules1)

		// Second anonymization should be faster (messages already processed)
		start = time.Now()
		_, dict2, rules2, err := anonymizer.AnonymizeMessages(
			context.Background(), conversationID, allMessages, nil, make(chan struct{}))
		elapsed2 := time.Since(start)
		require.NoError(t, err)

		t.Logf("Second anonymization (100 messages, cached): %v", elapsed2)
		assert.Empty(t, rules2, "No new rules should be generated for already processed messages")
		assert.Equal(t, dict1, dict2, "Dictionary should be consistent")

		// Adding one new message should be very fast
		newMessages := append(allMessages, openai.UserMessage("New message from Alice"))
		start = time.Now()
		_, dict3, rules3, err := anonymizer.AnonymizeMessages(
			context.Background(), conversationID, newMessages, nil, make(chan struct{}))
		elapsed3 := time.Since(start)
		require.NoError(t, err)

		t.Logf("Third anonymization (101 messages, 1 new): %v", elapsed3)
		assert.Len(t, rules3, 1, "Should have exactly 1 new rule")
		assert.Contains(t, rules3, "PERSON_003") // Alice
		assert.Equal(t, "Alice", dict3["PERSON_003"])

		// Performance should improve significantly
		assert.True(t, elapsed2 < elapsed1, "Second run should be faster than first")
		assert.True(t, elapsed3 < elapsed1, "Third run should be faster than first")
	})
}

// Helper functions.
func findTokenForEntity(dict map[string]string, entity string) string {
	for token, originalEntity := range dict {
		if originalEntity == entity {
			return token
		}
	}
	return ""
}
