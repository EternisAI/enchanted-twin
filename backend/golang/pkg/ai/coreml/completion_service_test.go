package coreml

import (
	"context"
	"testing"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestCompletionServiceInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewCompletionServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	response, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Content != "Mock completion response" {
		t.Errorf("Expected 'Mock completion response', got: %s", response.Content)
	}

	if response.Role != "assistant" {
		t.Errorf("Expected assistant role, got: %v", response.Role)
	}
}

func TestCompletionServiceNonInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewCompletionServiceWithProcess("./mock-binary", "./mock-model", false, mockProcess)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	response, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Content != "Mock completion response" {
		t.Errorf("Expected 'Mock completion response', got: %s", response.Content)
	}
}

func TestCompletionServiceFailure(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockFailingProcess("model failed to load")
	service := NewCompletionServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	_, err := service.Completions(ctx, messages, nil, "test-model")
	if err == nil {
		t.Fatal("Expected error, got none")
	}

	if err.Error() != "completion failed: model failed to load" {
		t.Errorf("Expected 'completion failed: model failed to load', got: %s", err.Error())
	}
}

func TestCompletionServiceImplementsInterface(t *testing.T) {
	mockProcess := NewMockSuccessfulProcess()
	service := NewCompletionServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	var _ ai.Completions = service
}
