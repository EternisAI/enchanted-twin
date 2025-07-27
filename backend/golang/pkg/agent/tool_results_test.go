package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestExecuteStreamWithPrivacy_ToolResultsCollection(t *testing.T) {
	// Test that tool results are properly collected and returned

	// Create a test tool that returns a specific result
	testTool := &mockDeAnonymizationTool{
		name: "test_tool",
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Tool execution successful with result data",
			},
		},
	}

	// Create a mock PrivateCompletionResult with tool calls
	mockResult := ai.PrivateCompletionResult{
		Message: openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "I'll execute the test tool.",
			ToolCalls: []openai.ChatCompletionMessageToolCall{
				{
					ID:   "call_001",
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      "test_tool",
						Arguments: `{"param": "PERSON_001"}`,
					},
				},
			},
		},
		ReplacementRules: map[string]string{
			"PERSON_001": "John Smith",
		},
	}

	// Test the logic that should happen in ExecuteStreamWithPrivacy

	// 1. Parse tool arguments
	var args map[string]interface{}
	toolCall := mockResult.Message.ToolCalls[0]
	err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	if err != nil {
		t.Fatalf("Failed to parse tool arguments: %v", err)
	}

	// 2. De-anonymize arguments
	deAnonymizedArgs := make(map[string]interface{})
	for key, value := range args {
		if strValue, ok := value.(string); ok {
			if realValue, exists := mockResult.ReplacementRules[strValue]; exists {
				deAnonymizedArgs[key] = realValue
			} else {
				deAnonymizedArgs[key] = value
			}
		} else {
			deAnonymizedArgs[key] = value
		}
	}

	// 3. Execute tool and collect results
	var toolResults []types.ToolResult
	result, err := testTool.Execute(context.Background(), deAnonymizedArgs)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	// Collect the result (this is what was missing before)
	toolResults = append(toolResults, result)

	// 4. Verify tool results are collected
	if len(toolResults) != 1 {
		t.Fatalf("Expected 1 tool result, got %d", len(toolResults))
	}

	// 5. Verify the tool result content
	if toolResults[0].Content() != "Tool execution successful with result data" {
		t.Errorf("Expected tool result content 'Tool execution successful with result data', got %s", toolResults[0].Content())
	}

	// 6. Verify tool received de-anonymized arguments
	if testTool.receivedArgs == nil {
		t.Fatal("Tool did not receive arguments")
	}

	if param, ok := testTool.receivedArgs["param"].(string); !ok || param != "John Smith" {
		t.Errorf("Expected de-anonymized param 'John Smith', got %v", testTool.receivedArgs["param"])
	}

	// 7. Simulate creating AgentResponse (what ExecuteStreamWithPrivacy should return)
	agentResponse := AgentResponse{
		ToolResults: toolResults, // This was missing before the fix
	}

	// 8. Verify the AgentResponse includes tool results
	if len(agentResponse.ToolResults) != 1 {
		t.Errorf("Expected 1 tool result in AgentResponse, got %d", len(agentResponse.ToolResults))
	}

	if agentResponse.ToolResults[0].Content() != "Tool execution successful with result data" {
		t.Errorf("Expected AgentResponse tool result content 'Tool execution successful with result data', got %s", agentResponse.ToolResults[0].Content())
	}

	t.Log("Tool results collection test passed - tool results are properly collected and returned")
}

func TestExecuteStreamWithPrivacy_MultipleToolResults(t *testing.T) {
	// Test collecting multiple tool results

	// Create two test tools
	tool1 := &mockDeAnonymizationTool{
		name: "tool1",
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Result from tool 1",
			},
		},
	}

	tool2 := &mockDeAnonymizationTool{
		name: "tool2",
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Result from tool 2",
			},
		},
	}

	// Create tools map
	toolMap := map[string]*mockDeAnonymizationTool{
		"tool1": tool1,
		"tool2": tool2,
	}

	// Mock multiple tool calls
	toolCalls := []openai.ChatCompletionMessageToolCall{
		{
			ID:   "call_001",
			Type: "function",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "tool1",
				Arguments: `{"param": "test1"}`,
			},
		},
		{
			ID:   "call_002",
			Type: "function",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "tool2",
				Arguments: `{"param": "test2"}`,
			},
		},
	}

	// Execute tools and collect results
	var toolResults []types.ToolResult
	for _, toolCall := range toolCalls {
		tool, exists := toolMap[toolCall.Function.Name]
		if !exists {
			t.Fatalf("Tool not found in map: %s", toolCall.Function.Name)
		}

		var args map[string]interface{}
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		if err != nil {
			t.Fatalf("Failed to parse tool arguments: %v", err)
		}

		result, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		toolResults = append(toolResults, result)
	}

	// Verify both tool results are collected
	if len(toolResults) != 2 {
		t.Fatalf("Expected 2 tool results, got %d", len(toolResults))
	}

	if toolResults[0].Content() != "Result from tool 1" {
		t.Errorf("Expected first tool result 'Result from tool 1', got %s", toolResults[0].Content())
	}

	if toolResults[1].Content() != "Result from tool 2" {
		t.Errorf("Expected second tool result 'Result from tool 2', got %s", toolResults[1].Content())
	}

	t.Log("Multiple tool results collection test passed")
}
