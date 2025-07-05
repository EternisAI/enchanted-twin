package ai

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

func TestPrivateCompletionsMockAnonymizer(t *testing.T) {
	logger := log.New(nil)
	
	// Create a mock anonymizer with no delay for testing
	anonymizer := NewMockAnonymizer(0, logger)
	
	// Test anonymization
	ctx := context.Background()
	content := "Hello John, this is a test message from Alice at OpenAI."
	
	anonymized, rules, err := anonymizer.Anonymize(ctx, content)
	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}
	
	// Check that some replacements were made
	if len(rules) == 0 {
		t.Error("Expected some anonymization rules, got none")
	}
	
	// Check that original content was modified
	if anonymized == content {
		t.Error("Expected content to be anonymized, but it's unchanged")
	}
	
	// Test de-anonymization
	restored := anonymizer.DeAnonymize(anonymized, rules)
	if restored != content {
		t.Errorf("De-anonymization failed. Expected: %q, got: %q", content, restored)
	}
	
	t.Logf("Original: %s", content)
	t.Logf("Anonymized: %s", anonymized)
	t.Logf("Rules: %v", rules)
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

func TestPrivateCompletionsServiceDelayedAnonymizer(t *testing.T) {
	logger := log.New(nil)
	
	// Create a mock anonymizer with a small delay
	anonymizer := NewMockAnonymizer(10*time.Millisecond, logger)
	
	// Test that anonymization works with delay
	ctx := context.Background()
	content := "Test message with John and Alice"
	
	start := time.Now()
	_, _, err := anonymizer.Anonymize(ctx, content)
	elapsed := time.Since(start)
	
	if err != nil {
		t.Fatalf("Anonymization with delay failed: %v", err)
	}
	
	// Check that the delay was respected (should be at least 10ms)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected delay of at least 10ms, got %v", elapsed)
	}
}

// Mock implementation for testing
type mockCompletionsService struct {
	response openai.ChatCompletionMessage
	err      error
}

func (m *mockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateResult, error) {
	if m.err != nil {
		return PrivateResult{}, m.err
	}
	return PrivateResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}