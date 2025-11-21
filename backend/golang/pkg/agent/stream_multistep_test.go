package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestExecuteStreamWithPrivacy_MultiStepToolExecution(t *testing.T) {
	logger := log.New(nil)

	// Create a mock tool that returns a result
	mockTool := &mockTool{
		name: "test_tool",
		definition: openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name: "test_tool",
			},
		},
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Tool executed successfully",
			},
		},
	}

	// Create a mock AI service that first returns tool calls, then a final response
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			// First call - returns tool call
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
						{
							ID:   "call_001",
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "test_tool",
								Arguments: `{"param": "PERSON_001"}`,
							},
						},
					},
				},
				ReplacementRules: map[string]string{
					"PERSON_001": "John Doe",
				},
			},
			// Second call - returns final response after tool execution
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Task completed! The tool has been executed successfully.",
				},
				ReplacementRules: map[string]string{
					"PERSON_001": "John Doe",
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
		openai.UserMessage("Execute the test tool"),
	}

	currentTools := []tools.Tool{mockTool}
	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, currentTools, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify the final response content
	if response.Content != "Task completed! The tool has been executed successfully." {
		t.Errorf("Expected final content 'Task completed! The tool has been executed successfully.', got %s", response.Content)
	}

	// Verify tool was executed
	if !mockTool.executed {
		t.Error("Tool was not executed")
	}

	// Verify tool received de-anonymized arguments
	if param, ok := mockTool.receivedArgs["param"].(string); !ok || param != "John Doe" {
		t.Errorf("Expected de-anonymized param 'John Doe', got %v", mockTool.receivedArgs["param"])
	}

	// Verify tool results are collected
	if len(response.ToolResults) != 1 {
		t.Errorf("Expected 1 tool result, got %d", len(response.ToolResults))
	}

	if response.ToolResults[0].Content() != "Tool executed successfully" {
		t.Errorf("Expected tool result content 'Tool executed successfully', got %s", response.ToolResults[0].Content())
	}

	// Verify tool calls are collected
	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}

	// Verify no errors for successful execution
	if len(response.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(response.Errors), response.Errors)
	}

	// Verify replacement rules are preserved
	if len(response.ReplacementRules) != 1 {
		t.Errorf("Expected 1 replacement rule, got %d", len(response.ReplacementRules))
	}

	if response.ReplacementRules["PERSON_001"] != "John Doe" {
		t.Errorf("Expected replacement rule for PERSON_001 to be 'John Doe', got %s", response.ReplacementRules["PERSON_001"])
	}

	// Verify AI service was called twice (once for tool call, once for final response)
	if mockAI.callCount != 2 {
		t.Errorf("Expected AI service to be called 2 times, got %d", mockAI.callCount)
	}
}

func TestExecuteStreamWithPrivacy_NoToolCallsMultiStep(t *testing.T) {
	logger := log.New(nil)

	// Create mock AI service with no tool calls
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "This is a regular response without tool calls.",
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
		openai.UserMessage("Hello"),
	}

	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, []tools.Tool{}, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify response content
	if response.Content != "This is a regular response without tool calls." {
		t.Errorf("Expected response content 'This is a regular response without tool calls.', got %s", response.Content)
	}

	// Verify no tool calls or results
	if len(response.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(response.ToolCalls))
	}

	if len(response.ToolResults) != 0 {
		t.Errorf("Expected 0 tool results, got %d", len(response.ToolResults))
	}

	// Verify no errors for successful execution
	if len(response.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(response.Errors), response.Errors)
	}

	// Verify AI service was called only once
	if mockAI.callCount != 1 {
		t.Errorf("Expected AI service to be called 1 time, got %d", mockAI.callCount)
	}
}

func TestExecuteStreamWithPrivacy_MaxStepsLimit(t *testing.T) {
	logger := log.New(nil)

	// Create a mock tool
	mockTool := &mockTool{
		name: "test_tool",
		definition: openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name: "test_tool",
			},
		},
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Tool executed",
			},
		},
	}

	// Create a mock AI service that always returns tool calls (to test max steps)
	responses := make([]ai.PrivateCompletionResult, MAX_STEPS+1)
	for i := 0; i < MAX_STEPS; i++ {
		responses[i] = ai.PrivateCompletionResult{
			Message: openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "",
				ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
					{
						ID:   "call_001",
						Type: "function",
						Function: openai.ChatCompletionMessageFunctionToolCallFunction{
							Name:      "test_tool",
							Arguments: `{"param": "test"}`,
						},
					},
				},
			},
			ReplacementRules: map[string]string{},
		}
	}

	mockAI := &mockAIService{
		responses: responses,
	}

	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Execute tool repeatedly"),
	}

	currentTools := []tools.Tool{mockTool}
	onDelta := func(delta ai.StreamDelta) {}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, "", messages, currentTools, onDelta, false)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify max steps limit was respected
	if mockAI.callCount != MAX_STEPS {
		t.Errorf("Expected AI service to be called %d times (max steps), got %d", MAX_STEPS, mockAI.callCount)
	}

	// Verify all tool calls were collected
	if len(response.ToolCalls) != MAX_STEPS {
		t.Errorf("Expected %d tool calls, got %d", MAX_STEPS, len(response.ToolCalls))
	}

	// Verify all tool results were collected
	if len(response.ToolResults) != MAX_STEPS {
		t.Errorf("Expected %d tool results, got %d", MAX_STEPS, len(response.ToolResults))
	}

	// Verify no errors for successful execution
	if len(response.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(response.Errors), response.Errors)
	}
}
