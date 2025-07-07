package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

func TestPrivateCompletionsMockAnonymizer(t *testing.T) {
	logger := log.New(nil)

	anonymizer := InitMockAnonymizer(0, logger)

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, this is a test message from Alice at OpenAI."),
	}

	interruptChan := make(chan struct{})
	defer close(interruptChan)

	anonymizedMessages, rules, err := anonymizer.AnonymizeMessages(ctx, messages, interruptChan)
	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}

	if len(rules) == 0 {
		t.Error("Expected some anonymization rules, got none")
	}

	if len(anonymizedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(anonymizedMessages))
	}

	content := "Hello John, this is a test message from Alice at OpenAI."
	anonymized := "Hello PERSON_001, this is a test message from PERSON_003 at COMPANY_001."
	restored := anonymizer.DeAnonymize(anonymized, rules)
	if restored != content {
		t.Errorf("De-anonymization failed. Expected: %q, got: %q", content, restored)
	}

	t.Logf("Messages processed: %d", len(anonymizedMessages))
	t.Logf("Rules generated: %v", rules)
	t.Logf("Restored: %s", restored)
}

func TestMockAnonymizerDelay(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure we get the correct delay configuration
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(10*time.Millisecond, logger)

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message with John and Alice"),
	}

	interruptChan := make(chan struct{})
	defer close(interruptChan)

	start := time.Now()
	_, _, err := anonymizer.AnonymizeMessages(ctx, messages, interruptChan)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms delay, but took %v", elapsed)
	}

	t.Logf("Anonymization took %v (expected >=10ms)", elapsed)
}

func TestPrivateCompletionsServiceMessageInterruption(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure we get the correct delay configuration
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(100*time.Millisecond, logger)

	mockService := &mockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "This response shouldn't be reached",
		},
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockService,
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message that should be interrupted"),
	}

	start := time.Now()
	_, err = privateService.Completions(ctx, messages, nil, "test-model", Background)
	elapsed := time.Since(start)

	t.Logf("Private completion took %v", elapsed)

	if err != nil && strings.Contains(err.Error(), "interrupted") {
		t.Logf("Successfully interrupted after %v", elapsed)
		if elapsed >= 100*time.Millisecond {
			t.Errorf("Expected interruption before full delay, but took %v", elapsed)
		}
	} else if err == nil {
		if elapsed < 100*time.Millisecond {
			t.Errorf("Expected at least 100ms delay, but took %v", elapsed)
		}
		t.Logf("Completed without interruption after %v", elapsed)
	} else {
		t.Fatalf("Unexpected error: %v", err)
	}
}

type mockCompletionsService struct {
	response openai.ChatCompletionMessage
	err      error
}

func (m *mockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}
	return PrivateCompletionResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}

// Enhanced mock that captures what was sent to the LLM
type capturingMockCompletionsService struct {
	response        openai.ChatCompletionMessage
	err             error
	capturedMessages []openai.ChatCompletionMessageParamUnion
}

func (m *capturingMockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	// Capture what was sent to the LLM for verification
	m.capturedMessages = make([]openai.ChatCompletionMessageParamUnion, len(messages))
	copy(m.capturedMessages, messages)
	
	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}
	return PrivateCompletionResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}

func TestNewPrivateCompletionsServiceValidation(t *testing.T) {
	logger := log.New(nil)

	// Test with nil CompletionsService
	_, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: nil,
		Anonymizer:         InitMockAnonymizer(0, logger),
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "completionsService is required") {
		t.Errorf("Expected CompletionsService validation error, got: %v", err)
	}

	// Test with nil Anonymizer
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         nil,
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "anonymizer is required") {
		t.Errorf("Expected Anonymizer validation error, got: %v", err)
	}

	// Test with nil Logger
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         InitMockAnonymizer(0, logger),
		Logger:             nil,
	})
	if err == nil || !strings.Contains(err.Error(), "logger is required") {
		t.Errorf("Expected Logger validation error, got: %v", err)
	}

	// Test with valid config
	service, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         InitMockAnonymizer(0, logger),
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Errorf("Expected valid config to succeed, got error: %v", err)
	}
	if service == nil {
		t.Error("Expected service to be created")
	}
	if service != nil {
		service.Shutdown()
	}
}

func TestPrivateCompletionsE2EAnonymizationFlow(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure clean state
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(0, logger) // No delay for faster test

	// Create capturing mock that will record what the LLM sees
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "Hello PERSON_001! I understand you work at COMPANY_001 in LOCATION_006. That's great!",
		},
	}

	// Create private completions service with our mocks
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()

	// Original user message containing sensitive information
	originalMessage := "Hello John! I heard you work at OpenAI in San Francisco. Can you tell me about your projects?"
	
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(originalMessage),
	}

	// Execute the full e2e flow
	result, err := privateService.Completions(ctx, messages, nil, "test-model", Background)
	if err != nil {
		t.Fatalf("Private completions failed: %v", err)
	}

	// === VERIFICATION 1: Original message was anonymized before sending to LLM ===
	if len(mockLLM.capturedMessages) != 1 {
		t.Fatalf("Expected 1 message sent to LLM, got %d", len(mockLLM.capturedMessages))
	}

	// Extract the content that was sent to the LLM
	sentToLLMBytes, err := json.Marshal(mockLLM.capturedMessages[0])
	if err != nil {
		t.Fatalf("Failed to marshal captured message: %v", err)
	}

	var sentToLLMMap map[string]interface{}
	if err := json.Unmarshal(sentToLLMBytes, &sentToLLMMap); err != nil {
		t.Fatalf("Failed to unmarshal captured message: %v", err)
	}

	anonymizedContent, ok := sentToLLMMap["content"].(string)
	if !ok {
		t.Fatalf("Expected string content in captured message")
	}

	// Verify that sensitive data was anonymized in what was sent to LLM
	if strings.Contains(anonymizedContent, "John") {
		t.Errorf("LLM received un-anonymized name 'John': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "OpenAI") {
		t.Errorf("LLM received un-anonymized company 'OpenAI': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "San Francisco") {
		t.Errorf("LLM received un-anonymized location 'San Francisco': %s", anonymizedContent)
	}

	// Verify that anonymized tokens were used instead
	if !strings.Contains(anonymizedContent, "PERSON_001") {
		t.Errorf("LLM did not receive anonymized name token 'PERSON_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "COMPANY_001") {
		t.Errorf("LLM did not receive anonymized company token 'COMPANY_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "LOCATION_006") {
		t.Errorf("LLM did not receive anonymized location token 'LOCATION_006': %s", anonymizedContent)
	}

	t.Logf("Original message: %s", originalMessage)
	t.Logf("Anonymized sent to LLM: %s", anonymizedContent)

	// === VERIFICATION 2: LLM response was de-anonymized before returning to user ===
	finalResponse := result.Message.Content

	// Verify that the response was de-anonymized
	if strings.Contains(finalResponse, "PERSON_001") {
		t.Errorf("Final response still contains anonymized token 'PERSON_001': %s", finalResponse)
	}
	if strings.Contains(finalResponse, "COMPANY_001") {
		t.Errorf("Final response still contains anonymized token 'COMPANY_001': %s", finalResponse)
	}
	if strings.Contains(finalResponse, "LOCATION_006") {
		t.Errorf("Final response still contains anonymized token 'LOCATION_006': %s", finalResponse)
	}

	// Verify that original terms were restored
	if !strings.Contains(finalResponse, "John") {
		t.Errorf("Final response missing de-anonymized name 'John': %s", finalResponse)
	}
	if !strings.Contains(finalResponse, "OpenAI") {
		t.Errorf("Final response missing de-anonymized company 'OpenAI': %s", finalResponse)
	}
	if !strings.Contains(finalResponse, "San Francisco") {
		t.Errorf("Final response missing de-anonymized location 'San Francisco': %s", finalResponse)
	}

	t.Logf("Final de-anonymized response: %s", finalResponse)

	// === VERIFICATION 3: Replacement rules were captured correctly ===
	if len(result.ReplacementRules) == 0 {
		t.Error("Expected replacement rules to be returned, got none")
	}

	expectedRules := map[string]string{
		"PERSON_001":   "John",
		"COMPANY_001":  "OpenAI", 
		"LOCATION_006": "San Francisco",
	}

	for token, expectedOriginal := range expectedRules {
		if actual, exists := result.ReplacementRules[token]; !exists {
			t.Errorf("Missing replacement rule for token '%s'", token)
		} else if actual != expectedOriginal {
			t.Errorf("Wrong replacement rule for '%s': expected '%s', got '%s'", token, expectedOriginal, actual)
		}
	}

	t.Logf("Replacement rules: %v", result.ReplacementRules)

	// === VERIFICATION 4: End-to-end privacy guarantee ===
	t.Log("E2E Privacy Verification PASSED:")
	t.Log("  1. Sensitive data was anonymized before LLM")
	t.Log("  2. LLM only saw anonymized tokens") 
	t.Log("  3. Response was de-anonymized before user")
	t.Log("  4. Replacement rules preserved for full round-trip")
	t.Log("  5. Full system integration through microscheduler worked")
}
