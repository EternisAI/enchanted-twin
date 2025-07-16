package ai

import (
	"context"
	"database/sql"
	"testing"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicPersistence tests that the basic persistence functionality works.
func TestBasicPersistence(t *testing.T) {
	// Set up test database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE conversation_dicts (
			conversation_id TEXT PRIMARY KEY,
			dict_data TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE anonymized_messages (
			conversation_id TEXT NOT NULL,
			message_hash TEXT NOT NULL,
			anonymized_at DATETIME NOT NULL,
			PRIMARY KEY (conversation_id, message_hash)
		)
	`)
	require.NoError(t, err)

	logger := log.New(nil)

	// Test PersistentAnonymizer
	t.Run("PersistentAnonymizer", func(t *testing.T) {
		persistentAnonymizer := NewPersistentAnonymizer(db, logger)
		defer persistentAnonymizer.Shutdown()

		// Test dictionary persistence
		conversationID := "test-conversation"
		testDict := map[string]string{
			"PERSON_001":   "Alice",
			"PERSON_002":   "Bob",
			"LOCATION_001": "New York",
		}

		// Save dictionary
		err := persistentAnonymizer.SaveConversationDict(conversationID, testDict)
		assert.NoError(t, err)

		// Load dictionary
		loadedDict, err := persistentAnonymizer.LoadConversationDict(conversationID)
		assert.NoError(t, err)
		assert.Equal(t, testDict, loadedDict)

		// Test message hash
		message := openai.UserMessage("Hello Alice!")
		messageHash := persistentAnonymizer.GetMessageHash(message)
		assert.NotEmpty(t, messageHash)

		// Test message anonymization tracking
		isAnonymized, err := persistentAnonymizer.IsMessageAnonymized(conversationID, messageHash)
		assert.NoError(t, err)
		assert.False(t, isAnonymized)

		// Test anonymization with persistence
		messages := []openai.ChatCompletionMessageParamUnion{message}
		interruptChan := make(chan struct{})

		anonymizedMessages, updatedDict, _, err := persistentAnonymizer.AnonymizeMessages(
			context.Background(),
			conversationID,
			messages,
			map[string]string{},
			interruptChan,
		)

		assert.NoError(t, err)
		assert.Len(t, anonymizedMessages, 1)
		assert.Contains(t, updatedDict, "PERSON_001")
		assert.Equal(t, "Alice", updatedDict["PERSON_001"])

		// Verify message was marked as anonymized
		isAnonymized, err = persistentAnonymizer.IsMessageAnonymized(conversationID, messageHash)
		assert.NoError(t, err)
		assert.True(t, isAnonymized)

		// Test de-anonymization
		anonymizedText := "Hello PERSON_001!"
		deanonymized := persistentAnonymizer.DeAnonymize(anonymizedText, testDict)
		assert.Equal(t, "Hello Alice!", deanonymized)
	})

	// Test ConversationStore directly
	t.Run("ConversationStore", func(t *testing.T) {
		store := NewSQLiteConversationStore(db, logger)
		defer store.Close() //nolint:errcheck

		conversationID := "test-store-conversation"
		testDict := map[string]string{
			"PERSON_001": "Charlie",
			"PERSON_002": "David",
		}

		// Test save and load
		err := store.SaveConversationDict(conversationID, testDict)
		assert.NoError(t, err)

		loadedDict, err := store.GetConversationDict(conversationID)
		assert.NoError(t, err)
		assert.Equal(t, testDict, loadedDict)

		// Test message anonymization tracking
		messageHash := "test-hash-123"
		isAnonymized, err := store.IsMessageAnonymized(conversationID, messageHash)
		assert.NoError(t, err)
		assert.False(t, isAnonymized)

		// Mark as anonymized
		err = store.MarkMessageAnonymized(conversationID, messageHash)
		assert.NoError(t, err)

		// Check again
		isAnonymized, err = store.IsMessageAnonymized(conversationID, messageHash)
		assert.NoError(t, err)
		assert.True(t, isAnonymized)
	})

	// Test AnonymizerManager
	t.Run("AnonymizerManager", func(t *testing.T) {
		logger := log.New(nil)

		// Test MockAnonymizer creation
		mockConfig := AnonymizerConfig{
			Type:    MockAnonymizerType,
			Enabled: true,
			Logger:  logger,
		}
		mockManager := NewAnonymizerManager(mockConfig)
		defer mockManager.Shutdown()

		mockAnonymizer := mockManager.GetAnonymizer()
		assert.NotNil(t, mockAnonymizer)
		_, ok := mockAnonymizer.(*MockAnonymizer)
		assert.True(t, ok)

		// Test PersistentAnonymizer creation
		persistentConfig := AnonymizerConfig{
			Type:     PersistentAnonymizerType,
			Database: db,
			Logger:   logger,
		}
		persistentManager := NewAnonymizerManager(persistentConfig)
		defer persistentManager.Shutdown()

		persistentAnonymizer := persistentManager.GetAnonymizer()
		assert.NotNil(t, persistentAnonymizer)
		_, ok = persistentAnonymizer.(*PersistentAnonymizer)
		assert.True(t, ok)

		// Test NoOpAnonymizer creation
		noOpConfig := AnonymizerConfig{
			Type:   NoOpAnonymizerType,
			Logger: logger,
		}
		noOpManager := NewAnonymizerManager(noOpConfig)
		defer noOpManager.Shutdown()

		noOpAnonymizer := noOpManager.GetAnonymizer()
		assert.NotNil(t, noOpAnonymizer)
		_, ok = noOpAnonymizer.(*NoOpAnonymizer)
		assert.True(t, ok)
	})
}

// Test memory-only mode.
func TestMemoryOnlyMode(t *testing.T) {
	// Set up test database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE conversation_dicts (
			conversation_id TEXT PRIMARY KEY,
			dict_data TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE anonymized_messages (
			conversation_id TEXT NOT NULL,
			message_hash TEXT NOT NULL,
			anonymized_at DATETIME NOT NULL,
			PRIMARY KEY (conversation_id, message_hash)
		)
	`)
	require.NoError(t, err)

	logger := log.New(nil)

	// Test PersistentAnonymizer memory-only mode
	t.Run("PersistentAnonymizer Memory-Only", func(t *testing.T) {
		persistentAnonymizer := NewPersistentAnonymizer(db, logger)
		defer persistentAnonymizer.Shutdown()

		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello Alice!"),
		}

		// Test memory-only mode
		anonymizedMessages, updatedDict, _, err := persistentAnonymizer.AnonymizeMessages(
			context.Background(),
			"", // Empty conversationID triggers memory-only mode
			messages,
			map[string]string{},
			make(chan struct{}),
		)

		assert.NoError(t, err)
		assert.Len(t, anonymizedMessages, 1)
		assert.NotNil(t, updatedDict)

		// Verify nothing was persisted
		loadedDict, err := persistentAnonymizer.LoadConversationDict("any-conversation")
		assert.NoError(t, err)
		assert.Empty(t, loadedDict)
	})

	// Test MockAnonymizer functionality
	t.Run("MockAnonymizer", func(t *testing.T) {
		replacements := map[string]string{
			"Alice": "PERSON_001",
			"Bob":   "PERSON_002",
		}

		mockAnonymizer := NewMockAnonymizer(0, replacements, logger)
		defer mockAnonymizer.Shutdown()

		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello Alice and Bob!"),
		}

		// Test anonymization
		anonymizedMessages, updatedDict, newRules, err := mockAnonymizer.AnonymizeMessages(
			context.Background(),
			"test-conversation",
			messages,
			map[string]string{},
			make(chan struct{}),
		)

		assert.NoError(t, err)
		assert.Len(t, anonymizedMessages, 1)
		assert.NotEmpty(t, updatedDict)
		assert.NotEmpty(t, newRules)

		// Test de-anonymization
		anonymizedText := "Hello PERSON_001 and PERSON_002!"
		deanonymized := mockAnonymizer.DeAnonymize(anonymizedText, updatedDict)
		assert.Equal(t, "Hello Alice and Bob!", deanonymized)
	})
}

// Test integration with database requirements.
func TestDatabaseRequirements(t *testing.T) {
	logger := log.New(nil)

	// Test LLMAnonymizer requires database
	t.Run("LLMAnonymizer requires database", func(t *testing.T) {
		mockCompletionsService := &MockCompletionsService{}
		config := AnonymizerConfig{
			Type:      LLMAnonymizerType,
			AIService: mockCompletionsService,
			Database:  nil, // No database provided
			Logger:    logger,
		}

		manager := NewAnonymizerManager(config)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)

		// Should fallback to NoOpAnonymizer when no database
		_, ok := anonymizer.(*NoOpAnonymizer)
		assert.True(t, ok)
	})

	// Test LocalAnonymizer requires database
	t.Run("LocalAnonymizer requires database", func(t *testing.T) {
		config := AnonymizerConfig{
			Type:            LocalAnonymizerType,
			LlamaAnonymizer: nil, // No LlamaAnonymizer provided
			Database:        nil, // No database provided
			Logger:          logger,
		}

		manager := NewAnonymizerManager(config)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)

		// Should fallback to NoOpAnonymizer when no database
		_, ok := anonymizer.(*NoOpAnonymizer)
		assert.True(t, ok)
	})
}
