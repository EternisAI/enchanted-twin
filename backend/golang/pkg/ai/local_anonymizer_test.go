package ai

import (
	"context"
	"testing"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLocalAnonymizer_NewLocalAnonymizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)

	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	assert.NotNil(t, anonymizer)
	assert.Equal(t, mockLlama, anonymizer.llama)
	assert.NotNil(t, anonymizer.store)
	assert.NotNil(t, anonymizer.hasher)
	assert.Equal(t, logger, anonymizer.logger)
}

func TestLocalAnonymizer_AnonymizeMessages_MemoryOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Mock the Llama service response
	mockLlama.On("Anonymize", mock.Anything, "Hello John, how are you?").Return(
		map[string]string{"John": "PERSON_001"}, nil)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})

	// Test memory-only mode (empty conversationID)
	anonymizedMessages, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		"", // empty conversationID triggers memory-only mode
		messages,
		existingDict,
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)
	assert.Contains(t, updatedDict, "John")
	assert.Equal(t, "PERSON_001", updatedDict["John"])
	assert.Contains(t, newRules, "John")
	assert.Equal(t, "PERSON_001", newRules["John"])

	mockLlama.AssertExpectations(t)
}

func TestLocalAnonymizer_AnonymizeMessages_Persistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Mock the Llama service response
	mockLlama.On("Anonymize", mock.Anything, "Hello John, how are you?").Return(
		map[string]string{"John": "PERSON_001"}, nil)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})
	conversationID := "test-conversation-1"

	// Test persistent mode
	anonymizedMessages, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		conversationID,
		messages,
		existingDict,
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)
	assert.Contains(t, updatedDict, "John")
	assert.Equal(t, "PERSON_001", updatedDict["John"])
	assert.Contains(t, newRules, "John")

	// Verify data was persisted
	dict, err := anonymizer.LoadConversationDict(conversationID)
	assert.NoError(t, err)
	assert.Contains(t, dict, "John")
	assert.Equal(t, "PERSON_001", dict["John"])

	// Verify message was marked as anonymized
	messageHash := anonymizer.GetMessageHash(messages[0])
	isAnonymized, err := anonymizer.IsMessageAnonymized(conversationID, messageHash)
	assert.NoError(t, err)
	assert.True(t, isAnonymized)

	mockLlama.AssertExpectations(t)
}

func TestLocalAnonymizer_AnonymizeMessages_PersistentDeduplication(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{"John": "PERSON_001"}
	interruptChan := make(chan struct{})
	conversationID := "test-conversation-2"

	// Pre-populate the database with existing data
	err := anonymizer.SaveConversationDict(conversationID, existingDict)
	require.NoError(t, err)

	messageHash := anonymizer.GetMessageHash(messages[0])
	err = anonymizer.store.MarkMessageAnonymized(conversationID, messageHash)
	require.NoError(t, err)

	// Test that already anonymized messages are skipped
	anonymizedMessages, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		conversationID,
		messages,
		map[string]string{},
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)
	assert.Contains(t, updatedDict, "John")
	assert.Equal(t, "PERSON_001", updatedDict["John"])
	assert.Empty(t, newRules) // No new rules should be created

	// Mock service should not be called since message was already anonymized
	mockLlama.AssertNotCalled(t, "Anonymize")
}

func TestLocalAnonymizer_AnonymizeMessages_EmptyContent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Test message with empty content
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(""), // Empty system message
	}

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})

	anonymizedMessages, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		"test-conversation-3",
		messages,
		existingDict,
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)
	assert.Empty(t, updatedDict)
	assert.Empty(t, newRules)

	// Mock service should not be called for empty content
	mockLlama.AssertNotCalled(t, "Anonymize")
}

func TestLocalAnonymizer_AnonymizeMessages_ExistingDictMerge(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Pre-populate conversation dictionary
	conversationID := "test-conversation-4"
	storedDict := map[string]string{"Alice": "PERSON_001"}
	err := anonymizer.SaveConversationDict(conversationID, storedDict)
	require.NoError(t, err)

	// Mock the Llama service response for new entity
	mockLlama.On("Anonymize", mock.Anything, "Hello John, meet Alice").Return(
		map[string]string{"John": "PERSON_002"}, nil)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, meet Alice"),
	}

	// Provide existing dictionary that should merge with stored one
	existingDict := map[string]string{"Bob": "PERSON_003"}
	interruptChan := make(chan struct{})

	anonymizedMessages, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		conversationID,
		messages,
		existingDict,
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)

	// Should contain all three mappings
	assert.Contains(t, updatedDict, "Alice") // From stored dict
	assert.Contains(t, updatedDict, "Bob")   // From existing dict
	assert.Contains(t, updatedDict, "John")  // From new discovery

	assert.Equal(t, "PERSON_001", updatedDict["Alice"])
	assert.Equal(t, "PERSON_003", updatedDict["Bob"])
	assert.Equal(t, "PERSON_002", updatedDict["John"])

	// Only new discovery should be in newRules
	assert.Len(t, newRules, 1)
	assert.Contains(t, newRules, "John")
	assert.Equal(t, "PERSON_002", newRules["John"])

	mockLlama.AssertExpectations(t)
}

func TestLocalAnonymizer_LoadSaveConversationDict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	conversationID := "test-conversation-5"
	testDict := map[string]string{
		"John":     "PERSON_001",
		"Jane":     "PERSON_002",
		"New York": "LOCATION_001",
	}

	// Test saving dictionary
	err := anonymizer.SaveConversationDict(conversationID, testDict)
	assert.NoError(t, err)

	// Test loading dictionary
	loadedDict, err := anonymizer.LoadConversationDict(conversationID)
	assert.NoError(t, err)
	assert.Equal(t, testDict, loadedDict)

	// Test loading non-existent dictionary
	emptyDict, err := anonymizer.LoadConversationDict("non-existent")
	assert.NoError(t, err)
	assert.Empty(t, emptyDict)
}

func TestLocalAnonymizer_GetMessageHash(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	message1 := openai.UserMessage("Hello John!")
	message2 := openai.UserMessage("Hello Jane!")
	message3 := openai.UserMessage("Hello John!")

	hash1 := anonymizer.GetMessageHash(message1)
	hash2 := anonymizer.GetMessageHash(message2)
	hash3 := anonymizer.GetMessageHash(message3)

	assert.NotEmpty(t, hash1)
	assert.NotEmpty(t, hash2)
	assert.NotEqual(t, hash1, hash2) // Different messages should have different hashes
	assert.Equal(t, hash1, hash3)    // Same messages should have same hash
}

func TestLocalAnonymizer_IsMessageAnonymized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	conversationID := "test-conversation-6"
	messageHash := "test-hash-123"

	// Test message not anonymized initially
	isAnonymized, err := anonymizer.IsMessageAnonymized(conversationID, messageHash)
	assert.NoError(t, err)
	assert.False(t, isAnonymized)

	// Mark message as anonymized
	err = anonymizer.store.MarkMessageAnonymized(conversationID, messageHash)
	assert.NoError(t, err)

	// Test message is now anonymized
	isAnonymized, err = anonymizer.IsMessageAnonymized(conversationID, messageHash)
	assert.NoError(t, err)
	assert.True(t, isAnonymized)
}

func TestLocalAnonymizer_DeAnonymize(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	rules := map[string]string{
		"John":     "PERSON_001",
		"Jane":     "PERSON_002",
		"New York": "LOCATION_001",
	}

	anonymizedText := "Hello PERSON_001, are you visiting LOCATION_001 with PERSON_002?"
	expectedText := "Hello John, are you visiting New York with Jane?"

	deanonymizedText := anonymizer.DeAnonymize(anonymizedText, rules)
	assert.Equal(t, expectedText, deanonymizedText)
}

func TestLocalAnonymizer_Shutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Mock the Close method
	mockLlama.On("Close").Return(nil)

	// Test shutdown doesn't panic
	assert.NotPanics(t, func() {
		anonymizer.Shutdown()
	})

	mockLlama.AssertExpectations(t)
}

func TestLocalAnonymizer_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})

	// Test with canceled context
	_, _, _, err := anonymizer.AnonymizeMessages(
		ctx,
		"test-conversation-7",
		messages,
		existingDict,
		interruptChan,
	)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestLocalAnonymizer_InterruptChannel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})

	// Close interrupt channel immediately
	close(interruptChan)

	// Test with closed interrupt channel
	_, _, _, err := anonymizer.AnonymizeMessages(
		context.Background(),
		"test-conversation-8",
		messages,
		existingDict,
		interruptChan,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "anonymization interrupted")
}

func TestLocalAnonymizer_ApplyReplacements(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	rules := map[string]string{
		"John":     "PERSON_001",
		"Jane":     "PERSON_002",
		"New York": "LOCATION_001",
	}

	originalText := "Hello John, are you visiting New York with Jane?"
	expectedAnonymized := "Hello PERSON_001, are you visiting LOCATION_001 with PERSON_002?"

	anonymizedText := anonymizer.applyReplacements(originalText, rules)
	assert.Equal(t, expectedAnonymized, anonymizedText)
}

func TestLocalAnonymizer_ApplyReplacementsSortedByLength(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Test that longer strings are replaced first to avoid partial matches
	rules := map[string]string{
		"John Smith": "PERSON_001",
		"John":       "PERSON_002", // Shorter string that's a substring of the longer one
	}

	originalText := "Hello John Smith and John"
	expectedAnonymized := "Hello PERSON_001 and PERSON_002"

	anonymizedText := anonymizer.applyReplacements(originalText, rules)
	assert.Equal(t, expectedAnonymized, anonymizedText)
}

func TestLocalAnonymizer_ExtractMessageContent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Test with user message
	userMessage := openai.UserMessage("Hello John!")
	content := anonymizer.extractMessageContent(userMessage)
	assert.Equal(t, "Hello John!", content)

	// Test with system message
	systemMessage := openai.SystemMessage("You are a helpful assistant.")
	content = anonymizer.extractMessageContent(systemMessage)
	assert.Equal(t, "You are a helpful assistant.", content)

	// Test with assistant message
	assistantMessage := openai.AssistantMessage("How can I help you?")
	content = anonymizer.extractMessageContent(assistantMessage)
	assert.Equal(t, "How can I help you?", content)
}

func TestLocalAnonymizer_ReplaceMessageContent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	originalMessage := openai.UserMessage("Hello John!")
	newContent := "Hello PERSON_001!"

	replacedMessage := anonymizer.replaceMessageContent(originalMessage, newContent)

	// Extract content from replaced message to verify it was changed
	extractedContent := anonymizer.extractMessageContent(replacedMessage)
	assert.Equal(t, newContent, extractedContent)
}

func TestLocalAnonymizer_DuplicateRuleHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Mock the Llama service to return a rule that already exists
	mockLlama.On("Anonymize", mock.Anything, "Hello John, how are you?").Return(
		map[string]string{"John": "PERSON_001"}, nil)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	// Provide existing dictionary with the same rule
	existingDict := map[string]string{"John": "PERSON_001"}
	interruptChan := make(chan struct{})

	// Test memory-only mode
	_, updatedDict, newRules, err := anonymizer.AnonymizeMessages(
		context.Background(),
		"", // memory-only mode
		messages,
		existingDict,
		interruptChan,
	)

	assert.NoError(t, err)
	assert.Contains(t, updatedDict, "John")
	assert.Equal(t, "PERSON_001", updatedDict["John"])
	assert.Empty(t, newRules) // No new rules should be created for existing mappings

	mockLlama.AssertExpectations(t)
}

func TestLocalAnonymizer_LlamaServiceError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)
	anonymizer := NewLocalAnonymizer(mockLlama, db, logger)

	// Mock the Llama service to return an error
	mockLlama.On("Anonymize", mock.Anything, "Hello John, how are you?").Return(
		map[string]string{}, assert.AnError)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{}
	interruptChan := make(chan struct{})

	// Test error handling
	_, _, _, err := anonymizer.AnonymizeMessages(
		context.Background(),
		"test-conversation-9",
		messages,
		existingDict,
		interruptChan,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to anonymize message content")

	mockLlama.AssertExpectations(t)
}
