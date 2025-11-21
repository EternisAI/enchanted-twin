package ai

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
)

func TestAnonymizerManager_MockAnonymizer(t *testing.T) {
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:    MockAnonymizerType,
		Enabled: true,
		Delay:   10 * time.Millisecond,
		Logger:  logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Test that the same instance is returned on multiple calls
	anonymizer2 := manager.GetAnonymizer()
	assert.Equal(t, anonymizer, anonymizer2)
}

func TestAnonymizerManager_MockAnonymizerDisabled(t *testing.T) {
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:    MockAnonymizerType,
		Enabled: false,
		Logger:  logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return NoOpAnonymizer when disabled
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_PersistentAnonymizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: db,
		Logger:   logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return PersistentAnonymizer
	_, ok := anonymizer.(*PersistentAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_PersistentAnonymizerNoDB(t *testing.T) {
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: nil, // No database provided
		Logger:   logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should fallback to NoOpAnonymizer when no database
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_LocalAnonymizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	// Create a mock that implements the required interface
	mockLlama := &MockLlamaAnonymizer{}
	mockLlama.On("Close").Return(nil)
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:            LocalAnonymizerType,
		LlamaAnonymizer: mockLlama,
		Database:        db,
		Logger:          logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return LocalAnonymizer
	_, ok := anonymizer.(*LocalAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_LocalAnonymizerNoLlama(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:            LocalAnonymizerType,
		LlamaAnonymizer: nil, // No Llama service provided
		Database:        db,
		Logger:          logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should fallback to NoOpAnonymizer when no Llama service
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_LocalAnonymizerNoDB(t *testing.T) {
	mockLlama := &MockLlamaAnonymizer{}
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:            LocalAnonymizerType,
		LlamaAnonymizer: mockLlama,
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
}

func TestAnonymizerManager_LLMAnonymizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockAIService := &MockCompletionsService{}
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:      LLMAnonymizerType,
		AIService: mockAIService,
		Model:     "test-model",
		Database:  db,
		Logger:    logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return LLMAnonymizer
	llmAnonymizer, ok := anonymizer.(*LLMAnonymizer)
	assert.True(t, ok)
	assert.Equal(t, mockAIService, llmAnonymizer.aiService)
	assert.Equal(t, "test-model", llmAnonymizer.model)
}

func TestAnonymizerManager_LLMAnonymizerDefaultModel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	mockAIService := &MockCompletionsService{}
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:      LLMAnonymizerType,
		AIService: mockAIService,
		Model:     "", // Empty model should use default
		Database:  db,
		Logger:    logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return LLMAnonymizer with default model
	llmAnonymizer, ok := anonymizer.(*LLMAnonymizer)
	assert.True(t, ok)
	assert.Equal(t, "openai/gpt-4o-mini", llmAnonymizer.model)
}

func TestAnonymizerManager_LLMAnonymizerNoAIService(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:      LLMAnonymizerType,
		AIService: nil, // No AI service provided
		Database:  db,
		Logger:    logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should fallback to NoOpAnonymizer when no AI service
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_LLMAnonymizerNoDB(t *testing.T) {
	mockAIService := &MockCompletionsService{}
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:      LLMAnonymizerType,
		AIService: mockAIService,
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
}

func TestAnonymizerManager_NoOpAnonymizer(t *testing.T) {
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:   NoOpAnonymizerType,
		Logger: logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return NoOpAnonymizer
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_DefaultType(t *testing.T) {
	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:   999, // Invalid type should default to NoOp
		Logger: logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should return NoOpAnonymizer for invalid type
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_NoLogger(t *testing.T) {
	config := AnonymizerConfig{
		Type:   NoOpAnonymizerType,
		Logger: nil, // No logger provided
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Should still work without logger
	_, ok := anonymizer.(*NoOpAnonymizer)
	assert.True(t, ok)
}

func TestAnonymizerManager_Shutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: db,
		Logger:   logger,
	}

	manager := NewAnonymizerManager(config)

	// Get anonymizer to trigger initialization
	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Test shutdown doesn't panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})

	// Test multiple shutdowns don't panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})
}

func TestAnonymizerManager_ShutdownWithMock(t *testing.T) {
	mockLlama := &MockLlamaAnonymizer{}
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:            LocalAnonymizerType,
		LlamaAnonymizer: mockLlama,
		Database:        db,
		Logger:          logger,
	}

	manager := NewAnonymizerManager(config)

	// Get anonymizer to trigger initialization
	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Mock the Close method for shutdown
	mockLlama.On("Close").Return(nil)

	// Test shutdown calls Close on underlying service
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})

	mockLlama.AssertExpectations(t)
}

func TestAnonymizerManager_FactoryFunctions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	t.Run("NewMockAnonymizerManager", func(t *testing.T) {
		manager := NewMockAnonymizerManager(10*time.Millisecond, true, logger)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)
		_, ok := anonymizer.(*MockAnonymizer)
		assert.True(t, ok)
	})

	t.Run("NewPersistentAnonymizerManager", func(t *testing.T) {
		manager := NewPersistentAnonymizerManager(db, logger)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)
		_, ok := anonymizer.(*PersistentAnonymizer)
		assert.True(t, ok)
	})

	t.Run("NewLocalAnonymizerManager", func(t *testing.T) {
		mockLlama := &MockLlamaAnonymizer{}
		mockLlama.On("Close").Return(nil)
		manager := NewLocalAnonymizerManager(mockLlama, db, logger)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)
		_, ok := anonymizer.(*LocalAnonymizer)
		assert.True(t, ok)
	})

	t.Run("NewLLMAnonymizerManager", func(t *testing.T) {
		mockAIService := &MockCompletionsService{}
		manager := NewLLMAnonymizerManager(mockAIService, "test-model", db, logger)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)
		_, ok := anonymizer.(*LLMAnonymizer)
		assert.True(t, ok)
	})

	t.Run("NewNoOpAnonymizerManager", func(t *testing.T) {
		manager := NewNoOpAnonymizerManager(logger)
		defer manager.Shutdown()

		anonymizer := manager.GetAnonymizer()
		assert.NotNil(t, anonymizer)
		_, ok := anonymizer.(*NoOpAnonymizer)
		assert.True(t, ok)
	})
}

func TestAnonymizerManager_GetDefaultReplacements(t *testing.T) {
	replacements := getDefaultReplacements()

	assert.NotEmpty(t, replacements)

	// Test some expected mappings
	assert.Equal(t, "PERSON_001", replacements["John"])
	assert.Equal(t, "PERSON_002", replacements["Jane"])
	assert.Equal(t, "COMPANY_001", replacements["OpenAI"])
	assert.Equal(t, "LOCATION_001", replacements["New York"])
}

func TestAnonymizerManager_PredefinedReplacements(t *testing.T) {
	logger := log.New(nil)

	customReplacements := map[string]string{
		"Alice": "CUSTOM_001",
		"Bob":   "CUSTOM_002",
	}

	config := AnonymizerConfig{
		Type:                   MockAnonymizerType,
		Enabled:                true,
		Delay:                  10 * time.Millisecond,
		PredefinedReplacements: customReplacements,
		Logger:                 logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Test that custom replacements are used
	mockAnonymizer, ok := anonymizer.(*MockAnonymizer)
	assert.True(t, ok)

	// Test anonymization through the interface
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello Alice and Bob"),
	}

	anonymizedMessages, dict, newRules, err := mockAnonymizer.AnonymizeMessages(
		context.Background(),
		"test-conv",
		messages,
		map[string]string{},
		make(chan struct{}),
	)

	assert.NoError(t, err)
	assert.Len(t, anonymizedMessages, 1)
	assert.NotEmpty(t, dict)
	assert.NotEmpty(t, newRules)
}

func TestAnonymizerManager_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: db,
		Logger:   logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	// Test concurrent access to GetAnonymizer
	const numGoroutines = 10
	results := make(chan Anonymizer, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			anonymizer := manager.GetAnonymizer()
			results <- anonymizer
		}()
	}

	// Collect all results
	var anonymizers []Anonymizer
	for i := 0; i < numGoroutines; i++ {
		anonymizers = append(anonymizers, <-results)
	}

	// All should be the same instance
	firstAnonymizer := anonymizers[0]
	for i := 1; i < len(anonymizers); i++ {
		assert.Equal(t, firstAnonymizer, anonymizers[i])
	}
}

// Integration test to verify the full workflow.
func TestAnonymizerManager_IntegrationTest(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close() //nolint:errcheck

	logger := log.New(nil)

	config := AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: db,
		Logger:   logger,
	}

	manager := NewAnonymizerManager(config)
	defer manager.Shutdown()

	anonymizer := manager.GetAnonymizer()
	assert.NotNil(t, anonymizer)

	// Test the anonymizer works end-to-end
	conversationID := "integration-test"
	testDict := map[string]string{
		"PERSON_001": "John",
		"PERSON_002": "Jane",
	}

	// Save dictionary
	err := anonymizer.SaveConversationDict(conversationID, testDict)
	assert.NoError(t, err)

	// Load dictionary
	loadedDict, err := anonymizer.LoadConversationDict(conversationID)
	assert.NoError(t, err)
	assert.Equal(t, testDict, loadedDict)

	// Test message hash
	message := openai.UserMessage("Hello John!")
	hash := anonymizer.GetMessageHash(message)
	assert.NotEmpty(t, hash)

	// Test message anonymization status
	isAnonymized, err := anonymizer.IsMessageAnonymized(conversationID, hash)
	assert.NoError(t, err)
	assert.False(t, isAnonymized)

	// Test de-anonymization
	anonymizedText := "Hello PERSON_001!"
	deanonymized := anonymizer.DeAnonymize(anonymizedText, testDict)
	assert.Equal(t, "Hello John!", deanonymized)
}
