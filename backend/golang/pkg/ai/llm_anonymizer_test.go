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

func TestLLMAnonymizer_NewLLMAnonymizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)

	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	assert.NotNil(t, anonymizer)
	assert.Equal(t, mockService, anonymizer.aiService)
	assert.Equal(t, "test-model", anonymizer.model)
	assert.NotNil(t, anonymizer.store)
	assert.NotNil(t, anonymizer.hasher)
	assert.Equal(t, logger, anonymizer.logger)
}

func TestLLMAnonymizer_AnonymizeMessages_MemoryOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	// Mock the AI service response - it will call Completions (fallback) since mockService is not *Service
	mockResponse := PrivateCompletionResult{
		Message: openai.ChatCompletionMessage{
			Role: "assistant",
			ToolCalls: []openai.ChatCompletionMessageToolCall{
				{
					ID:   "test-call",
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      "replace_entities",
						Arguments: `{"replacements": [{"original": "John", "replacement": "PERSON_001"}]}`,
					},
				},
			},
		},
	}

	mockService.On("Completions", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockResponse, nil)

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
	assert.Contains(t, updatedDict, "PERSON_001")
	assert.Equal(t, "John", updatedDict["PERSON_001"])
	assert.Contains(t, newRules, "PERSON_001")
	assert.Equal(t, "John", newRules["PERSON_001"])

	mockService.AssertExpectations(t)
}

func TestLLMAnonymizer_AnonymizeMessages_Persistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	// Mock the AI service response
	mockResponse := PrivateCompletionResult{
		Message: openai.ChatCompletionMessage{
			Role: "assistant",
			ToolCalls: []openai.ChatCompletionMessageToolCall{
				{
					ID:   "test-call",
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      "replace_entities",
						Arguments: `{"replacements": [{"original": "John", "replacement": "PERSON_001"}]}`,
					},
				},
			},
		},
	}

	mockService.On("Completions", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockResponse, nil)

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
	assert.Contains(t, updatedDict, "PERSON_001")
	assert.Equal(t, "John", updatedDict["PERSON_001"])
	assert.Contains(t, newRules, "PERSON_001")

	// Verify data was persisted
	dict, err := anonymizer.LoadConversationDict(conversationID)
	assert.NoError(t, err)
	assert.Contains(t, dict, "PERSON_001")
	assert.Equal(t, "John", dict["PERSON_001"])

	// Verify message was marked as anonymized
	messageHash := anonymizer.GetMessageHash(messages[0])
	isAnonymized, err := anonymizer.IsMessageAnonymized(conversationID, messageHash)
	assert.NoError(t, err)
	assert.True(t, isAnonymized)

	mockService.AssertExpectations(t)
}

func TestLLMAnonymizer_AnonymizeMessages_PersistentDeduplication(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, how are you?"),
	}

	existingDict := map[string]string{"PERSON_001": "John"}
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
	assert.Contains(t, updatedDict, "PERSON_001")
	assert.Equal(t, "John", updatedDict["PERSON_001"])
	assert.Empty(t, newRules) // No new rules should be created

	// Mock service should not be called since message was already anonymized
	mockService.AssertNotCalled(t, "Completions")
}

func TestLLMAnonymizer_LoadSaveConversationDict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	conversationID := "test-conversation-3"
	testDict := map[string]string{
		"PERSON_001":   "John",
		"PERSON_002":   "Jane",
		"LOCATION_001": "New York",
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

func TestLLMAnonymizer_GetMessageHash(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

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

func TestLLMAnonymizer_IsMessageAnonymized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	conversationID := "test-conversation-4"
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

func TestLLMAnonymizer_DeAnonymize(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	rules := map[string]string{
		"PERSON_001":   "John",
		"PERSON_002":   "Jane",
		"LOCATION_001": "New York",
	}

	anonymizedText := "Hello PERSON_001, are you visiting LOCATION_001 with PERSON_002?"
	expectedText := "Hello John, are you visiting New York with Jane?"

	deanonymizedText := anonymizer.DeAnonymize(anonymizedText, rules)
	assert.Equal(t, expectedText, deanonymizedText)
}

func TestLLMAnonymizer_Shutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	// Test shutdown doesn't panic
	assert.NotPanics(t, func() {
		anonymizer.Shutdown()
	})
}

func TestLLMAnonymizer_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

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
		"test-conversation-5",
		messages,
		existingDict,
		interruptChan,
	)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestLLMAnonymizer_InterruptChannel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

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
		"test-conversation-6",
		messages,
		existingDict,
		interruptChan,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "anonymization interrupted")
}

func TestLLMAnonymizer_AnonymizeText(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	rules := map[string]string{
		"PERSON_001":   "John",
		"PERSON_002":   "Jane",
		"LOCATION_001": "New York",
	}

	originalText := "Hello John, are you visiting New York with Jane?"
	expectedAnonymized := "Hello PERSON_001, are you visiting LOCATION_001 with PERSON_002?"

	anonymizedText := anonymizer.anonymizeText(originalText, rules)
	assert.Equal(t, expectedAnonymized, anonymizedText)
}

func TestLLMAnonymizer_AnonymizeTextSortedByLength(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockService := &MockCompletionsService{}
	logger := log.New(nil)
	anonymizer := NewLLMAnonymizer(mockService, "test-model", db, logger)

	// Test that longer strings are replaced first to avoid partial matches
	rules := map[string]string{
		"PERSON_001": "John Smith",
		"PERSON_002": "John", // Shorter string that's a substring of the longer one
	}

	originalText := "Hello John Smith and John"
	expectedAnonymized := "Hello PERSON_001 and PERSON_002"

	anonymizedText := anonymizer.anonymizeText(originalText, rules)
	assert.Equal(t, expectedAnonymized, anonymizedText)
}
