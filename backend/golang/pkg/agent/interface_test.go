package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestAIServiceInterface_Compatibility(t *testing.T) {
	// Test that ai.Service implements AIService interface
	var _ AIService = (*ai.Service)(nil)

	// Test that mockAIService implements AIService interface
	var _ AIService = (*mockAIService)(nil)
}

func TestAgentWithAIServiceInterface(t *testing.T) {
	logger := log.New(nil)

	// Test that Agent can be created with AIService interface
	mockAI := &mockAIService{
		response: ai.PrivateCompletionResult{
			Message: openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "Test response",
			},
			ReplacementRules: map[string]string{},
		},
	}

	agent := NewAgent(
		logger,
		nil, // nc
		mockAI,
		"gpt-4",
		"gpt-4",
		nil, // preToolCallback
		nil, // postToolCallback
	)

	// Check that the AI service is properly set (interface comparison)
	if agent.aiService == nil {
		t.Error("Agent should accept AIService interface")
	}

	// Test that the agent can use the interface
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message"),
	}

	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, nil, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	if response.Content != "Test response" {
		t.Errorf("Expected response content 'Test response', got %s", response.Content)
	}

	// Verify the mock was called
	if mockAI.callCount != 1 {
		t.Errorf("Expected AI service to be called once, got %d", mockAI.callCount)
	}
}

func TestAIServiceInterface_Methods(t *testing.T) {
	// Test that AIService interface has the required methods
	mockAI := &mockAIService{
		response: ai.PrivateCompletionResult{
			Message: openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "Test response",
			},
			ReplacementRules: map[string]string{},
		},
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test"),
	}
	tools := []openai.ChatCompletionToolUnionParam{}

	// Test CompletionsStreamWithPrivacy method
	result, err := mockAI.CompletionsStreamWithPrivacy(ctx, "", messages, tools, "gpt-4", func(ai.StreamDelta) {})
	if err != nil {
		t.Fatalf("CompletionsStreamWithPrivacy failed: %v", err)
	}

	if result.Message.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got %s", result.Message.Content)
	}

	// Test Completions method
	result2, err := mockAI.Completions(ctx, messages, tools, "gpt-4", ai.UI)
	if err != nil {
		t.Fatalf("Completions failed: %v", err)
	}

	if result2.Message.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got %s", result2.Message.Content)
	}
}

func TestNewAgent_AcceptsInterface(t *testing.T) {
	logger := log.New(nil)

	// Test with different implementations of AIService
	implementations := []AIService{
		&mockAIService{
			response: ai.PrivateCompletionResult{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Mock response",
				},
			},
		},
		// Could add more implementations here
	}

	for i, impl := range implementations {
		agent := NewAgent(
			logger,
			nil,
			impl,
			"gpt-4",
			"gpt-4",
			nil,
			nil,
		)

		if agent.aiService != impl {
			t.Errorf("Implementation %d: Agent should store the AIService interface", i)
		}
	}
}

func TestAIServiceInterface_ErrorHandling(t *testing.T) {
	logger := log.New(nil)

	testError := errors.New("test error")

	// Test error handling through interface
	mockAI := &mockAIService{
		err: testError,
	}

	agent := NewAgent(
		logger,
		nil,
		mockAI,
		"gpt-4",
		"gpt-4",
		nil,
		nil,
	)

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test"),
	}

	onDelta := func(delta ai.StreamDelta) {}

	_, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, nil, onDelta, false)
	if err == nil {
		t.Error("Expected error to be propagated through interface")
	}

	if err != testError {
		t.Errorf("Expected test error, got %v", err)
	}
}
