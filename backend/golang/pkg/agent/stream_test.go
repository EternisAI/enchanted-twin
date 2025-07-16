package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// Mock tool for testing.
type mockTool struct {
	name         string
	definition   openai.ChatCompletionToolParam
	executed     bool
	receivedArgs map[string]interface{}
	result       types.ToolResult
}

func (m *mockTool) Definition() openai.ChatCompletionToolParam {
	return m.definition
}

func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (types.ToolResult, error) {
	m.executed = true
	m.receivedArgs = args
	return m.result, nil
}

// Mock AI service for testing.
type mockAIService struct {
	response  ai.PrivateCompletionResult
	responses []ai.PrivateCompletionResult
	callCount int
	err       error
}

func (m *mockAIService) CompletionsStreamWithPrivacy(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	tools []openai.ChatCompletionToolParam,
	model string,
	onDelta func(ai.StreamDelta),
) (ai.PrivateCompletionResult, error) {
	if m.err != nil {
		return ai.PrivateCompletionResult{}, m.err
	}

	// Handle multiple responses if available
	if len(m.responses) > 0 {
		if m.callCount >= len(m.responses) {
			// Return the last response if we've exceeded the number of responses
			return m.responses[len(m.responses)-1], nil
		}
		result := m.responses[m.callCount]
		m.callCount++
		return result, nil
	}

	// Fallback to single response
	m.callCount++
	return m.response, nil
}

// Add other required methods to satisfy interface.
func (m *mockAIService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority ai.Priority) (ai.PrivateCompletionResult, error) {
	return m.response, m.err
}

// Ensure mockAIService implements AIService.
var _ AIService = (*mockAIService)(nil)

func (m *mockAIService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) ai.Stream {
	return ai.Stream{}
}

func TestExecuteStreamWithPrivacy_ToolExecutionWithDeAnonymization(t *testing.T) {
	logger := log.New(nil)

	// Create a mock tool
	mockTestTool := &mockTool{
		name: "test_search",
		definition: openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        "test_search",
				Description: param.Opt[string]{Value: "Search for information about a person"},
			},
		},
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Found information about John Smith at OpenAI",
			},
		},
	}

	// Create mock AI service with de-anonymized tool call response
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			// First call - return tool call
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "I'll search for information about John Smith at OpenAI.",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "call_test_001",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "test_search",
								Arguments: `{"name": "John Smith", "company": "OpenAI"}`, // De-anonymized arguments
							},
						},
					},
				},
				ReplacementRules: map[string]string{
					"PERSON_001":  "John Smith",
					"COMPANY_001": "OpenAI",
				},
			},
			// Second call - return final response
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Search completed successfully.",
				},
				ReplacementRules: map[string]string{
					"PERSON_001":  "John Smith",
					"COMPANY_001": "OpenAI",
				},
			},
		},
	}

	// Track tool callback calls
	var preToolCallbackCalled bool
	var postToolCallbackCalled bool

	// Create agent with mock AI service
	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
		PreToolCallback: func(toolCall openai.ChatCompletionMessageToolCall) {
			preToolCallbackCalled = true
		},
		PostToolCallback: func(toolCall openai.ChatCompletionMessageToolCall, result types.ToolResult) {
			postToolCallbackCalled = true
		},
	}

	// Execute with privacy
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Search for John Smith at OpenAI"),
	}

	currentTools := []tools.Tool{mockTestTool}

	onDelta := func(delta ai.StreamDelta) {
		// Mock delta handler
	}

	// Execute the stream with privacy
	response, err := agent.ExecuteStreamWithPrivacy(ctx, messages, currentTools, onDelta, false)
	// Verify no error
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify tool was executed
	if !mockTestTool.executed {
		t.Error("Tool was not executed")
	}

	// Verify tool received de-anonymized arguments
	if mockTestTool.receivedArgs == nil {
		t.Fatal("Tool did not receive arguments")
	}

	// Check de-anonymized arguments
	if name, ok := mockTestTool.receivedArgs["name"].(string); !ok || name != "John Smith" {
		t.Errorf("Expected de-anonymized name 'John Smith', got %v", mockTestTool.receivedArgs["name"])
	}

	if company, ok := mockTestTool.receivedArgs["company"].(string); !ok || company != "OpenAI" {
		t.Errorf("Expected de-anonymized company 'OpenAI', got %v", mockTestTool.receivedArgs["company"])
	}

	// Verify callbacks were called
	if !preToolCallbackCalled {
		t.Error("PreToolCallback was not called")
	}

	if !postToolCallbackCalled {
		t.Error("PostToolCallback was not called")
	}

	// Verify response contains tool calls
	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in response, got %d", len(response.ToolCalls))
	}

	// Verify final content is from the last response
	if response.Content != "Search completed successfully." {
		t.Errorf("Expected final content 'Search completed successfully.', got '%s'", response.Content)
	}

	// Verify no errors for successful execution
	if len(response.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(response.Errors), response.Errors)
	}

	// Verify replacement rules are preserved
	if len(response.ReplacementRules) != 2 {
		t.Errorf("Expected 2 replacement rules, got %d", len(response.ReplacementRules))
	}

	if response.ReplacementRules["PERSON_001"] != "John Smith" {
		t.Errorf("Expected replacement rule for PERSON_001 to be 'John Smith', got %s", response.ReplacementRules["PERSON_001"])
	}

	if response.ReplacementRules["COMPANY_001"] != "OpenAI" {
		t.Errorf("Expected replacement rule for COMPANY_001 to be 'OpenAI', got %s", response.ReplacementRules["COMPANY_001"])
	}

	t.Logf("Test passed - tool executed with de-anonymized arguments: %v", mockTestTool.receivedArgs)
}

func TestExecuteStreamWithPrivacy_NoToolCalls(t *testing.T) {
	logger := log.New(nil)

	// Create mock AI service with no tool calls
	mockAI := &mockAIService{
		response: ai.PrivateCompletionResult{
			Message: openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "This is a regular response without tool calls.",
			},
			ReplacementRules: map[string]string{
				"PERSON_001": "John Smith",
			},
		},
	}

	// Create agent with mock AI service
	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	// Execute with privacy
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello"),
	}

	onDelta := func(delta ai.StreamDelta) {
		// Mock delta handler
	}

	// Execute the stream with privacy
	response, err := agent.ExecuteStreamWithPrivacy(ctx, messages, []tools.Tool{}, onDelta, false)
	// Verify no error
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify response content
	if response.Content != "This is a regular response without tool calls." {
		t.Errorf("Expected response content to be preserved, got %s", response.Content)
	}

	// Verify no tool calls
	if len(response.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(response.ToolCalls))
	}

	// Verify no errors for successful execution
	if len(response.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(response.Errors), response.Errors)
	}

	// Verify replacement rules are preserved
	if len(response.ReplacementRules) != 1 {
		t.Errorf("Expected 1 replacement rule, got %d", len(response.ReplacementRules))
	}

	if response.ReplacementRules["PERSON_001"] != "John Smith" {
		t.Errorf("Expected replacement rule for PERSON_001 to be 'John Smith', got %s", response.ReplacementRules["PERSON_001"])
	}

	t.Logf("Test passed - no tool calls handled correctly")
}

func TestExecuteStreamWithPrivacy_ToolNotFound(t *testing.T) {
	logger := log.New(nil)

	// Create mock AI service with tool call for non-existent tool
	mockAI := &mockAIService{
		responses: []ai.PrivateCompletionResult{
			// First call - return tool call
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "I'll use a tool that doesn't exist.",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "call_nonexistent",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "nonexistent_tool",
								Arguments: `{"query": "test"}`,
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
					Content: "Tool execution completed with errors.",
				},
				ReplacementRules: map[string]string{},
			},
		},
	}

	// Create agent with mock AI service
	agent := &Agent{
		logger:           logger,
		aiService:        mockAI,
		CompletionsModel: "gpt-4",
	}

	// Execute with privacy
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Use nonexistent tool"),
	}

	onDelta := func(delta ai.StreamDelta) {
		// Mock delta handler
	}

	// Execute the stream with privacy
	response, err := agent.ExecuteStreamWithPrivacy(ctx, messages, []tools.Tool{}, onDelta, false)
	// Verify no error (should handle gracefully)
	if err != nil {
		t.Fatalf("ExecuteStreamWithPrivacy failed: %v", err)
	}

	// Verify response still has tool calls (even though tool wasn't found)
	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in response, got %d", len(response.ToolCalls))
	}

	// Verify that error was recorded in response
	if len(response.Errors) == 0 {
		t.Error("Expected tool error to be recorded in response.Errors")
	}

	// Verify error message contains tool name
	if len(response.Errors) > 0 && !strings.Contains(response.Errors[0], "nonexistent_tool") {
		t.Errorf("Expected error message to contain 'nonexistent_tool', got: %s", response.Errors[0])
	}

	t.Logf("Test passed - tool not found handled gracefully with errors recorded: %v", response.Errors)
}
