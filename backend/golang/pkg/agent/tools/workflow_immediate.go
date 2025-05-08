package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// These tools have special handling in the workflow context and aren't executed as activities.
type WorkflowImmediateTool interface {
	Tool // Embed the base Tool interface

	// ExecuteInWorkflow executes the tool directly in a workflow context
	// This is used for tools that need to use workflow-specific functionality like Sleep
	ExecuteInWorkflow(ctx workflow.Context, inputs map[string]any) (types.ToolResult, error)
}

// FinalResponseTool is a special tool that represents a final response from the agent.
type FinalResponseTool struct{}

// Definition returns the tool definition.
func (t *FinalResponseTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "complete_workflow",
			Description: param.NewOpt("End the current workflow (plan) and provide an optional response. " +
				"Use this tool ONLY when the **entire multi-step plan** has been successfully completed. " +
				"Provide the final overall result of the whole plan in the 'output' parameter. " +
				"Do NOT use this for intermediate steps or partial results."),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"output": map[string]any{
						"type":        "string",
						"description": "Response content that the caller may retrieve",
					},
				},
				"required": []string{"output"},
			},
		},
	}
}

// Execute processes the final response.
func (t *FinalResponseTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	output, ok := inputs["output"].(string)
	if !ok {
		return nil, fmt.Errorf("final_response tool requires an 'output' parameter of type string")
	}

	return &types.StructuredToolResult{
		ToolName:   "final_response",
		ToolParams: inputs,
		Output: map[string]any{
			"content": output,
		},
	}, nil
}

// ExecuteInWorkflow executes the final_response tool in a workflow context.
func (t *FinalResponseTool) ExecuteInWorkflow(ctx workflow.Context, inputs map[string]any) (types.ToolResult, error) {
	output, _ := inputs["output"].(string)
	result := &types.StructuredToolResult{
		ToolName:   "final_response",
		ToolParams: inputs,
		Output: map[string]any{
			"content": output,
			"data":    output,
		},
	}
	return result, nil
}

// SleepTool is a special tool that pauses execution for a specified duration.
type SleepTool struct{}

// Definition returns the tool definition.
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

// Note: This cannot be executed directly in an activity, but needs special workflow handling.
func (t *SleepTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	// This implementation is a placeholder - the actual sleep happens in the workflow
	// via the workflow immediate execution
	errorMsg := "sleep tool can only be executed within a workflow context"
	return &types.StructuredToolResult{
		ToolName:   "sleep",
		ToolParams: inputs,
		Output: map[string]any{
			"content": "Sleep must be executed within a workflow context",
		},
		ToolError: errorMsg,
	}, errors.New(errorMsg)
}

// ExecuteInWorkflow executes the sleep tool in a workflow context.
func (t *SleepTool) ExecuteInWorkflow(ctx workflow.Context, inputs map[string]any) (types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)

	// Extract duration parameter
	var durationSec float64
	var ok bool

	// Try to extract duration as float64
	durationSec, ok = inputs["duration"].(float64)
	if !ok {
		// Try to parse from other types
		if durInt, isInt := inputs["duration"].(int); isInt {
			durationSec = float64(durInt)
			ok = true
		} else if durStr, isStr := inputs["duration"].(string); isStr {
			// Try parsing string as float
			var err error
			var durFloat float64
			if _, err = fmt.Sscanf(durStr, "%f", &durFloat); err == nil {
				durationSec = durFloat
				ok = true
			}
		}
	}

	// Check if we have a valid duration
	if !ok || durationSec <= 0 {
		return nil, fmt.Errorf("sleep tool requires a positive duration parameter (seconds)")
	}

	// Cap the maximum sleep duration for safety
	maxSleepSec := 24 * 60 * 60.0 // 24 hours in seconds
	if durationSec > maxSleepSec {
		logger.Warn(
			"Sleep duration capped",
			"requested_seconds",
			durationSec,
			"max_seconds",
			maxSleepSec,
		)
		durationSec = maxSleepSec
	}

	// Get optional reason parameter
	reason := ExtractReason(inputs)

	// Convert to duration
	duration := time.Duration(durationSec * float64(time.Second))

	// Sleep in the workflow
	if err := workflow.Sleep(ctx, duration); err != nil {
		logger.Error("Error during tool sleep", "error", err)
		return nil, fmt.Errorf("sleep error: %w", err)
	}

	// Create result message
	message := fmt.Sprintf(
		"Slept for %.2f seconds. Reason: %s",
		duration.Seconds(),
		reason,
	)

	// Return result
	result := &types.StructuredToolResult{
		ToolName:   "sleep",
		ToolParams: inputs,
		Output: map[string]any{
			"content": message,
			"data":    message,
		},
	}

	return result, nil
}

// SleepUntilTool is a special tool that pauses execution until a specified time.
type SleepUntilTool struct{}

// Definition returns the tool definition.
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

// Note: This cannot be executed directly in an activity, but needs special workflow handling.
func (t *SleepUntilTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	// This implementation is a placeholder - the actual sleep happens in the workflow
	// via the workflow immediate execution
	errorMsg := "sleep_until tool can only be executed within a workflow context"
	return &types.StructuredToolResult{
		ToolName:   "sleep_until",
		ToolParams: inputs,
		Output: map[string]any{
			"content": "Sleep_until must be executed within a workflow context",
		},
		ToolError: errorMsg,
	}, errors.New(errorMsg)
}

// ExecuteInWorkflow executes the sleep_until tool in a workflow context.
func (t *SleepUntilTool) ExecuteInWorkflow(ctx workflow.Context, inputs map[string]any) (types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)

	// Extract timestamp parameter
	timestampStr, ok := inputs["timestamp"].(string)
	if !ok || timestampStr == "" {
		return nil, fmt.Errorf("sleep_until tool requires a timestamp parameter (ISO8601 format)")
	}

	// Parse the timestamp
	targetTime, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp '%s': %w", timestampStr, err)
	}

	// Get current time
	now := workflow.Now(ctx)

	// Calculate duration to sleep
	sleepDuration := targetTime.Sub(now)

	// Check if time is in the past
	if sleepDuration <= 0 {
		message := fmt.Sprintf(
			"Target time %s is in the past. No sleep performed.",
			targetTime.Format(time.RFC3339),
		)

		result := &types.StructuredToolResult{
			ToolName:   "sleep_until",
			ToolParams: inputs,
			Output: map[string]any{
				"content": message,
				"data":    "Target time is in the past",
			},
		}

		return result, nil
	}

	// Cap the maximum sleep duration for safety
	maxSleepDuration := 24 * time.Hour // 24 hours
	if sleepDuration > maxSleepDuration {
		logger.Warn(
			"Sleep duration capped",
			"requested_duration",
			sleepDuration,
			"max_duration",
			maxSleepDuration,
		)
		sleepDuration = maxSleepDuration
	}

	// Get optional reason parameter
	reason := ExtractReason(inputs)

	// Sleep in the workflow
	if err := workflow.Sleep(ctx, sleepDuration); err != nil {
		logger.Error("Error during tool sleep", "error", err)
		return nil, fmt.Errorf("sleep error: %w", err)
	}

	// Get the actual time after sleep
	actualTime := workflow.Now(ctx)

	// Create result message
	message := fmt.Sprintf(
		"Slept until %s (sleep duration: %s). Reason: %s",
		actualTime.Format(time.RFC3339),
		sleepDuration,
		reason,
	)

	// Return result
	result := &types.StructuredToolResult{
		ToolName:   "sleep_until",
		ToolParams: inputs,
		Output: map[string]any{
			"content": message,
			"data":    message,
		},
	}

	return result, nil
}

// ExtractReason extracts the optional reason parameter from tool inputs.
func ExtractReason(inputs map[string]any) string {
	reason := "No reason specified"
	if reasonParam, hasReason := inputs["reason"].(string); hasReason && reasonParam != "" {
		reason = reasonParam
	}
	return reason
}

// GetWorkflowImmediateTool gets a workflow immediate tool by name.
func GetWorkflowImmediateTool(name string) (WorkflowImmediateTool, bool) {
	tools := map[string]WorkflowImmediateTool{
		"final_response": &FinalResponseTool{},
		"sleep":          &SleepTool{},
		"sleep_until":    &SleepUntilTool{},
	}

	tool, exists := tools[name]
	return tool, exists
}

// IsWorkflowImmediateTool checks if a tool is a workflow immediate tool.
func IsWorkflowImmediateTool(name string) bool {
	_, exists := GetWorkflowImmediateTool(name)
	return exists
}

// WorkflowImmediateTools returns all tools that are executed directly within a workflow.
func WorkflowImmediateTools() []Tool {
	return []Tool{
		&FinalResponseTool{},
		&SleepTool{},
		&SleepUntilTool{},
	}
}
