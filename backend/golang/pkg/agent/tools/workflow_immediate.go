package tools

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// FinalResponseTool is a special tool that represents a final response from the agent
type FinalResponseTool struct{}

// Definition returns the tool definition
func (t *FinalResponseTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "final_response",
			Description: param.NewOpt("Provide a final response to the user's query"),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"output": map[string]any{
						"type":        "string",
						"description": "The final response content to provide to the user",
					},
				},
				"required": []string{"output"},
			},
		},
	}
}

// Execute processes the final response
func (t *FinalResponseTool) Execute(ctx context.Context, inputs map[string]any) (ToolResult, error) {
	output, ok := inputs["output"].(string)
	if !ok {
		return ToolResult{}, fmt.Errorf("final_response tool requires an 'output' parameter of type string")
	}

	return ToolResult{
		Content: output,
	}, nil
}

// SleepTool is a special tool that pauses execution for a specified duration
type SleepTool struct{}

// Definition returns the tool definition
func (t *SleepTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "sleep",
			Description: param.NewOpt("Pause execution for a specified duration in seconds"),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"duration": map[string]any{
						"type":        "number",
						"description": "Duration to sleep in seconds",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Reason for sleeping (optional)",
					},
				},
				"required": []string{"duration"},
			},
		},
	}
}

// Execute processes the sleep request
// Note: This cannot be executed directly in an activity, but needs special workflow handling
func (t *SleepTool) Execute(ctx context.Context, inputs map[string]any) (ToolResult, error) {
	// This implementation is a placeholder - the actual sleep happens in the workflow
	// via the workflow immediate execution
	return ToolResult{
		Content: "Sleep must be executed within a workflow context",
	}, fmt.Errorf("sleep tool can only be executed within a workflow context")
}

// SleepUntilTool is a special tool that pauses execution until a specified time
type SleepUntilTool struct{}

// Definition returns the tool definition
func (t *SleepUntilTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "sleep_until",
			Description: param.NewOpt("Pause execution until a specified time (ISO8601 format)"),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timestamp": map[string]any{
						"type":        "string",
						"description": "Target time to sleep until in ISO8601 format (e.g. 2023-04-01T10:30:00Z)",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Reason for sleeping (optional)",
					},
				},
				"required": []string{"timestamp"},
			},
		},
	}
}

// Execute processes the sleep_until request
// Note: This cannot be executed directly in an activity, but needs special workflow handling
func (t *SleepUntilTool) Execute(ctx context.Context, inputs map[string]any) (ToolResult, error) {
	// This implementation is a placeholder - the actual sleep happens in the workflow
	// via the workflow immediate execution
	return ToolResult{
		Content: "Sleep_until must be executed within a workflow context",
	}, fmt.Errorf("sleep_until tool can only be executed within a workflow context")
}

// ExtractReason extracts the optional reason parameter from tool inputs
func ExtractReason(inputs map[string]any) string {
	reason := "No reason specified"
	if reasonParam, hasReason := inputs["reason"].(string); hasReason && reasonParam != "" {
		reason = reasonParam
	}
	return reason
}

// WorkflowImmediateTools returns all tools that are executed directly within a workflow
func WorkflowImmediateTools() []Tool {
	return []Tool{
		&FinalResponseTool{},
		&SleepTool{},
		&SleepUntilTool{},
	}
}