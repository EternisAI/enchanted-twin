package plannedv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/workflow"
)

// SpecialToolHandler is a function type for handling special tools directly in the workflow
type SpecialToolHandler func(ctx workflow.Context, params map[string]any, toolCall ToolCall) (*types.ToolResult, error)

// ToolExecutor handles execution of tools in workflow context
type ToolExecutor struct {
	registry tools.ToolRegistry
	// workflowImmediateHandlers maps tool names to handlers for tools that execute directly in the workflow
	workflowImmediateHandlers map[string]SpecialToolHandler
}

// NewToolExecutor creates a new tool executor with the given registry
func NewToolExecutor(registry tools.ToolRegistry, logger *log.Logger) *ToolExecutor {
	executor := &ToolExecutor{
		registry:                  registry,
		workflowImmediateHandlers: make(map[string]SpecialToolHandler),
	}

	// Register built-in workflow immediate handlers
	executor.RegisterWorkflowImmediateHandler("final_response", executor.handleFinalResponse)
	executor.RegisterWorkflowImmediateHandler("sleep", executor.handleSleep)
	executor.RegisterWorkflowImmediateHandler("sleep_until", executor.handleSleepUntil)

	return executor
}

// RegisterWorkflowImmediateHandler registers a handler for a tool that executes directly in the workflow
func (e *ToolExecutor) RegisterWorkflowImmediateHandler(toolName string, handler SpecialToolHandler) {
	e.workflowImmediateHandlers[toolName] = handler
}

// Execute runs a tool call and returns the result
func (e *ToolExecutor) Execute(ctx workflow.Context, toolCall ToolCall, state *PlanState) (*types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing tool call", "id", toolCall.ID, "tool", toolCall.Function.Name)

	// Parse arguments
	var params map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	toolName := toolCall.Function.Name

	// Check for workflow immediate handlers
	if handler, exists := e.workflowImmediateHandlers[toolName]; exists {
		return handler(ctx, params, toolCall)
	}

	// Check if tool exists in registry
	var found bool
	if state.Registry != nil {
		_, found = state.Registry.Get(toolName)
	}
	if !found {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Execute the tool activity
	var result types.ToolResult
	err := workflow.ExecuteActivity(ctx, e.executeToolActivity, toolName, params).Get(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
	}

	// Store the result in the tool call
	toolCall.Result = &result

	return &result, nil
}

// executeToolActivity is a wrapper around the registry's Execute method
func (e *ToolExecutor) executeToolActivity(ctx context.Context, toolName string, params map[string]any) (types.ToolResult, error) {
	// Use the registry from the executor
	// In a real implementation, we'd pass the state's registry to the activity
	registry := e.registry
	if registry == nil {
		return types.ToolResult{
			Tool:   toolName,
			Params: params,
			Error:  "no registry available",
		}, fmt.Errorf("no registry available")
	}

	toolResult, err := registry.Execute(ctx, toolName, params)
	if err != nil {
		return types.ToolResult{
			Tool:   toolName,
			Params: params,
			Error:  err.Error(),
		}, nil
	}

	// Convert tools.ToolResult to types.ToolResult
	return types.ToolResult{
		Tool:      toolName,
		Params:    params,
		Content:   toolResult.Content,
		ImageURLs: toolResult.ImageURLs,
	}, nil
}

// ExecuteBatch executes multiple tool calls in parallel
func (e *ToolExecutor) ExecuteBatch(ctx workflow.Context, toolCalls []ToolCall, state *PlanState) ([]*types.ToolResult, error) {
	// Create a future for each tool call
	futures := make([]workflow.Future, len(toolCalls))
	results := make([]*types.ToolResult, len(toolCalls))

	// Start all tool calls in parallel
	for i, toolCall := range toolCalls {
		toolCallCopy := toolCall // Create a copy to avoid closure capturing the loop variable
		// Use local execution instead of activity for now
		// This will be expanded in the future to support parallel activity execution
		future := workflow.ExecuteLocalActivity(ctx, func() (*types.ToolResult, error) {
			return e.Execute(ctx, toolCallCopy, state)
		})
		futures[i] = future
	}

	// Wait for all futures to complete
	for i, future := range futures {
		var result *types.ToolResult
		if err := future.Get(ctx, &result); err != nil {
			results[i] = &types.ToolResult{
				Tool:  toolCalls[i].Function.Name,
				Error: err.Error(),
			}
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// Special tool handlers

// handleFinalResponse handles the special final_response tool
func (e *ToolExecutor) handleFinalResponse(ctx workflow.Context, params map[string]any, toolCall ToolCall) (*types.ToolResult, error) {
	output, _ := params["output"].(string)
	result := &types.ToolResult{
		Tool:    "final_response",
		Params:  params,
		Content: output,
		Data:    output,
	}

	// Store the result in the tool call
	toolCall.Result = result

	return result, nil
}

// handleSleep handles the sleep tool
func (e *ToolExecutor) handleSleep(ctx workflow.Context, params map[string]any, toolCall ToolCall) (*types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)

	// Extract duration parameter
	var durationSec float64
	var ok bool

	// Try to extract duration as float64
	durationSec, ok = params["duration"].(float64)
	if !ok {
		// Try to parse from other types
		if durInt, isInt := params["duration"].(int); isInt {
			durationSec = float64(durInt)
			ok = true
		} else if durStr, isStr := params["duration"].(string); isStr {
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
	reason := e.extractReason(params)

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
	result := &types.ToolResult{
		Tool:    "sleep",
		Params:  params,
		Content: message,
		Data:    message,
	}

	// Store the result in the tool call
	toolCall.Result = result

	return result, nil
}

// handleSleepUntil handles the sleep_until tool
func (e *ToolExecutor) handleSleepUntil(ctx workflow.Context, params map[string]any, toolCall ToolCall) (*types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)

	// Extract timestamp parameter
	timestampStr, ok := params["timestamp"].(string)
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
		result := &types.ToolResult{
			Tool:   "sleep_until",
			Params: params,
			Content: fmt.Sprintf(
				"Target time %s is in the past. No sleep performed.",
				targetTime.Format(time.RFC3339),
			),
			Data: "Target time is in the past",
		}

		// Store the result in the tool call
		toolCall.Result = result

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
	reason := e.extractReason(params)

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
	result := &types.ToolResult{
		Tool:    "sleep_until",
		Params:  params,
		Content: message,
		Data:    message,
	}

	// Store the result in the tool call
	toolCall.Result = result

	return result, nil
}

// extractReason extracts the optional reason parameter from the params map
func (e *ToolExecutor) extractReason(params map[string]any) string {
	reason := "No reason specified"
	if reasonParam, hasReason := params["reason"].(string); hasReason && reasonParam != "" {
		reason = reasonParam
	}
	return reason
}
