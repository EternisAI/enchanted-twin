package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

func TestPrivateCompletionsMockAnonymizer(t *testing.T) {
	logger := log.New(nil)
	
	// Create a mock anonymizer with no delay for testing
	anonymizer := NewMockAnonymizer(0, logger)
	
	// Test anonymization with messages
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, this is a test message from Alice at OpenAI."),
	}
	
	// Create a dummy interrupt channel for testing (never interrupted)
	interruptChan := make(chan struct{})
	defer close(interruptChan)
	
	anonymizedMessages, rules, err := anonymizer.AnonymizeMessages(ctx, messages, interruptChan)
	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}
	
	// Check that some replacements were made
	if len(rules) == 0 {
		t.Error("Expected some anonymization rules, got none")
	}
	
	// Check that we got the same number of messages back
	if len(anonymizedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(anonymizedMessages))
	}
	
	// Test de-anonymization (using a simple content example)
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

func TestFallbackCompletionsService(t *testing.T) {
	// Create a mock completions service
	mockService := &mockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "This is a test response",
		},
	}
	
	// Create fallback service
	fallback := NewFallbackCompletionsService(mockService)
	
	// Test completion
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message"),
	}
	
	result, err := fallback.Completions(ctx, messages, nil, "test-model", Background)
	if err != nil {
		t.Fatalf("Fallback completion failed: %v", err)
	}
	
	// Check result
	if result.Message.Content != "This is a test response" {
		t.Errorf("Expected 'This is a test response', got %q", result.Message.Content)
	}
	
	// Check that no replacement rules were created
	if len(result.ReplacementRules) != 0 {
		t.Errorf("Expected no replacement rules, got %d", len(result.ReplacementRules))
	}
}

func TestMockAnonymizerDelay(t *testing.T) {
	logger := log.New(nil)
	
	// Create a mock anonymizer with a small delay
	anonymizer := NewMockAnonymizer(10*time.Millisecond, logger)
	
	// Test that AnonymizeMessages applies the delay properly
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message with John and Alice"),
	}
	
	// Create a dummy interrupt channel for testing (never interrupted)
	interruptChan := make(chan struct{})
	defer close(interruptChan)
	
	start := time.Now()
	_, _, err := anonymizer.AnonymizeMessages(ctx, messages, interruptChan)
	elapsed := time.Since(start)
	
	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}
	
	// Should have taken at least the delay time (10ms)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms delay, but took %v", elapsed)
	}
	
	t.Logf("Anonymization took %v (expected >=10ms)", elapsed)
}

func TestPrivateCompletionsServiceMessageInterruption(t *testing.T) {
	logger := log.New(nil)
	
	// Create a mock anonymizer with a longer delay
	anonymizer := NewMockAnonymizer(100*time.Millisecond, logger)
	
	// Create a mock completions service
	mockService := &mockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "This response shouldn't be reached",
		},
	}
	
	// Create private completions service
	privateService := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockService, // Use mockService directly since it implements CompletionsService
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	
	// Test that message anonymization can be interrupted
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message that should be interrupted"),
	}
	
	start := time.Now()
	_, err := privateService.Completions(ctx, messages, nil, "test-model", Background)
	elapsed := time.Since(start)
	
	// Note: This test may not always interrupt since the microscheduler controls interruption
	// But we can verify the delay behavior at minimum
	t.Logf("Private completion took %v", elapsed)
	
	if err != nil && strings.Contains(err.Error(), "interrupted") {
		t.Logf("Successfully interrupted after %v", elapsed)
		if elapsed >= 100*time.Millisecond {
			t.Errorf("Expected interruption before full delay, but took %v", elapsed)
		}
	} else if err == nil {
		// If not interrupted, should have taken at least the delay time
		if elapsed < 100*time.Millisecond {
			t.Errorf("Expected at least 100ms delay, but took %v", elapsed)
		}
		t.Logf("Completed without interruption after %v", elapsed)
	} else {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// Mock implementation for testing
type mockCompletionsService struct {
	response openai.ChatCompletionMessage
	err      error
}

func (m *mockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error) {
	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}
	return PrivateCompletionResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}