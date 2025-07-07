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
