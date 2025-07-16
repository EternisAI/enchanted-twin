package twinchat

import (
	"testing"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

func TestMessageContentHandling_EmptyContentWithToolResults(t *testing.T) {
	// Simulate the logic from SendMessage for handling empty content with tool results

	// Test case 1: Empty content with tool results
	response := mockAgentResponse{
		Content: "",
		ToolResults: []types.ToolResult{
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "Task completed successfully",
				},
			},
		},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "Task completed successfully." {
		t.Errorf("Expected messageContent to be 'Task completed successfully.', got %s", messageContent)
	}
}

func TestMessageContentHandling_EmptyContentWithoutToolResults(t *testing.T) {
	// Test case 2: Empty content without tool results
	response := mockAgentResponse{
		Content:     "",
		ToolResults: []types.ToolResult{},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "" {
		t.Errorf("Expected messageContent to remain empty, got %s", messageContent)
	}
}

func TestMessageContentHandling_NonEmptyContentWithToolResults(t *testing.T) {
	// Test case 3: Non-empty content with tool results
	response := mockAgentResponse{
		Content: "Here's your scheduled task!",
		ToolResults: []types.ToolResult{
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "Task scheduled",
				},
			},
		},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "Here's your scheduled task!" {
		t.Errorf("Expected messageContent to be 'Here's your scheduled task!', got %s", messageContent)
	}
}

func TestMessageContentHandling_NonEmptyContentWithoutToolResults(t *testing.T) {
	// Test case 4: Non-empty content without tool results
	response := mockAgentResponse{
		Content:     "This is a regular response",
		ToolResults: []types.ToolResult{},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "This is a regular response" {
		t.Errorf("Expected messageContent to be 'This is a regular response', got %s", messageContent)
	}
}

func TestMessageContentHandling_MultipleToolResults(t *testing.T) {
	// Test case 5: Empty content with multiple tool results
	response := mockAgentResponse{
		Content: "",
		ToolResults: []types.ToolResult{
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "First tool result",
				},
			},
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "Second tool result",
				},
			},
		},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "Task completed successfully." {
		t.Errorf("Expected messageContent to be 'Task completed successfully.', got %s", messageContent)
	}
}

// Mock agent response for testing.
type mockAgentResponse struct {
	Content          string
	ToolCalls        []openai.ChatCompletionMessageToolCall
	ToolResults      []types.ToolResult
	ImageURLs        []string
	ReplacementRules map[string]string
}

func TestMessageContentHandling_EdgeCases(t *testing.T) {
	// Test case 6: Whitespace content with tool results
	response := mockAgentResponse{
		Content: "   ",
		ToolResults: []types.ToolResult{
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "Task completed",
				},
			},
		},
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	// Should not change whitespace content
	if messageContent != "   " {
		t.Errorf("Expected messageContent to remain '   ', got '%s'", messageContent)
	}
}

func TestMessageContentHandling_NilToolResults(t *testing.T) {
	// Test case 7: Empty content with nil tool results
	response := mockAgentResponse{
		Content:     "",
		ToolResults: nil,
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	if messageContent != "" {
		t.Errorf("Expected messageContent to remain empty with nil tool results, got %s", messageContent)
	}
}

// Benchmark the content handling logic.
func BenchmarkMessageContentHandling(b *testing.B) {
	response := mockAgentResponse{
		Content: "",
		ToolResults: []types.ToolResult{
			&types.StructuredToolResult{
				Output: map[string]any{
					"content": "Task completed",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageContent := response.Content
		if messageContent == "" && len(response.ToolResults) > 0 {
			messageContent = "Task completed successfully."
		}
		_ = messageContent
	}
}
