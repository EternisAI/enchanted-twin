package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestMultiStepToolExecution_Simple(t *testing.T) {
	logger := log.New(nil)

	// Create a simple mock tool
	mockTool := &mockTool{
		name: "test_tool",
		definition: openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name: "test_tool",
			},
		},
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Tool executed successfully",
			},
		},
	}

	// Create a mock AI service with multiple responses
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			// First call - return tool call
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "call_001",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "test_tool",
								Arguments: `{"param": "test"}`,
							},
						},
					},
				},
				ReplacementRules: map[string]string{},
			},
			// Second call - return final response
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Task completed successfully with tool execution!",
				},
				ReplacementRules: map[string]string{},
			},
		},
	}

	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Execute the test tool"),
	}

	currentTools := []tools.Tool{mockTool}
	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, currentTools, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify the final response content
	if response.Content != "Task completed successfully with tool execution!" {
		t.Errorf("Expected final content 'Task completed successfully with tool execution!', got '%s'", response.Content)
	}

	// Verify tool was executed
	if !mockTool.executed {
		t.Error("Tool was not executed")
	}

	// Verify tool results are collected
	if len(response.ToolResults) != 1 {
		t.Errorf("Expected 1 tool result, got %d", len(response.ToolResults))
	}

	// Verify tool calls are collected
	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}

	// Verify AI service was called twice
	if mockAI.callCount != 2 {
		t.Errorf("Expected AI service to be called 2 times, got %d", mockAI.callCount)
	}

	t.Log("Multi-step tool execution test passed!")
}

func TestMultiStepToolExecution_NoToolCalls(t *testing.T) {
	logger := log.New(nil)

	// Create mock AI service with no tool calls
	mockAI := &mockAIService{
		response: ai.PrivateCompletionResult{
			Message: openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "No tools needed for this response.",
			},
			ReplacementRules: map[string]string{},
		},
	}

	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello"),
	}

	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, []tools.Tool{}, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify response content
	if response.Content != "No tools needed for this response." {
		t.Errorf("Expected response content 'No tools needed for this response.', got '%s'", response.Content)
	}

	// Verify no tool calls or results
	if len(response.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(response.ToolCalls))
	}

	if len(response.ToolResults) != 0 {
		t.Errorf("Expected 0 tool results, got %d", len(response.ToolResults))
	}

	t.Log("No tool calls test passed!")
}

func TestMultiStepToolExecution_DeAnonymization(t *testing.T) {
	logger := log.New(nil)

	// Create a simple mock tool
	mockTool := &mockTool{
		name: "test_tool",
		definition: openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name: "test_tool",
			},
		},
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Tool executed with de-anonymized data",
			},
		},
	}

	// Create a mock AI service with multiple responses
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			// First call - return tool call with anonymized data
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "call_001",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "test_tool",
								Arguments: `{"name": "PERSON_001", "email": "EMAIL_001"}`,
							},
						},
					},
				},
				ReplacementRules: map[string]string{
					"PERSON_001": "John Smith",
					"EMAIL_001":  "john@example.com",
				},
			},
			// Second call - return final response
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Data processed successfully!",
				},
				ReplacementRules: map[string]string{
					"PERSON_001": "John Smith",
					"EMAIL_001":  "john@example.com",
				},
			},
		},
	}

	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Process the data"),
	}

	currentTools := []tools.Tool{mockTool}
	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, currentTools, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify tool was executed
	if !mockTool.executed {
		t.Error("Tool was not executed")
	}

	// Verify tool received de-anonymized arguments
	if mockTool.receivedArgs == nil {
		t.Fatal("Tool did not receive arguments")
	}

	// Check de-anonymized arguments
	if name, ok := mockTool.receivedArgs["name"].(string); !ok || name != "John Smith" {
		t.Errorf("Expected de-anonymized name 'John Smith', got %v", mockTool.receivedArgs["name"])
	}

	if email, ok := mockTool.receivedArgs["email"].(string); !ok || email != "john@example.com" {
		t.Errorf("Expected de-anonymized email 'john@example.com', got %v", mockTool.receivedArgs["email"])
	}

	// Verify replacement rules are preserved
	if response.ReplacementRules["PERSON_001"] != "John Smith" {
		t.Errorf("Expected replacement rule for PERSON_001 to be 'John Smith', got %s", response.ReplacementRules["PERSON_001"])
	}

	if response.ReplacementRules["EMAIL_001"] != "john@example.com" {
		t.Errorf("Expected replacement rule for EMAIL_001 to be 'john@example.com', got %s", response.ReplacementRules["EMAIL_001"])
	}

	t.Log("De-anonymization test passed!")
}
