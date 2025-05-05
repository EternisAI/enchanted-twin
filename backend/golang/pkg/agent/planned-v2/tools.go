package plannedv2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// generateNextAction uses the LLM to determine the next actions based on the plan and history.
func generateNextAction(
	ctx workflow.Context,
	state *PlanState,
	model string,
) ([]ToolCall, error) {
	logger := workflow.GetLogger(ctx)

	// Execute LLM completion to get next action
	var completion openai.ChatCompletionMessage

	// We need an AgentActivities instance for the activity call
	var activities *AgentActivities
	err := workflow.ExecuteActivity(ctx, activities.LLMCompletionActivity, model, state.Messages, state.SelectedTools).
		Get(ctx, &completion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate next actions: %w", err)
	}

	// Add the LLM's response to the message history
	aiToolCalls := make([]ai.ToolCall, 0, len(completion.ToolCalls))
	for _, tc := range completion.ToolCalls {
		aiToolCalls = append(aiToolCalls, ai.FromOpenAIToolCall(tc))
	}
	state.Messages = append(
		state.Messages,
		ai.NewAssistantMessage(completion.Content, aiToolCalls),
	)

	// Add as thought to history
	state.History = append(state.History, HistoryEntry{
		Type:      "thought",
		Content:   completion.Content,
		Timestamp: workflow.Now(ctx),
	})

	// If no tool calls, we treat this as a final response
	if len(completion.ToolCalls) == 0 {
		logger.Info("LLM provided final response with no tool calls")

		// Create a special "final_response" tool call
		finalResponseCall := ToolCall{
			ToolCall: ai.ToolCall{
				ID:   "final_response_" + workflow.Now(ctx).Format(time.RFC3339),
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      "final_response",
					Arguments: fmt.Sprintf(`{"output": %q}`, completion.Content),
				},
			},
		}

		return []ToolCall{finalResponseCall}, nil
	}

	// Convert OpenAI tool calls to our custom format
	toolCalls := ToolCallsFromOpenAI(completion.ToolCalls)
	state.ToolCalls = append(state.ToolCalls, toolCalls...)

	logger.Info("Generated next actions", "total_tool_calls", len(toolCalls))

	return toolCalls, nil
}

// executeAction executes a tool call and returns the result.
func executeAction(ctx workflow.Context, toolCall ToolCall, state *PlanState) (types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing tool call", "id", toolCall.ID, "tool", toolCall.Function.Name)

	// Parse arguments if not already parsed
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	toolName := toolCall.Function.Name

	// Special case for final response
	if toolName == "final_response" {
		output, _ := params["output"].(string)
		result := &types.StructuredToolResult{
			ToolName:   toolName,
			ToolParams: params,
			Output:     map[string]any{"content": output, "data": output},
		}

		// Store the result in the tool call
		toolCall.Result = result

		return result, nil
	}

	// Handle special tools that execute directly in the workflow
	if toolName == "sleep" {
		result, err := executeSleep(ctx, params)
		if err != nil {
			return nil, err
		}

		// Store the result in the tool call
		toolCall.Result = result

		return result, nil
	}

	if toolName == "sleep_until" {
		result, err := executeSleepUntil(ctx, params)
		if err != nil {
			return nil, err
		}

		// Store the result in the tool call
		toolCall.Result = result

		return result, nil
	}

	// Execute the tool activity
	var activities *AgentActivities
	var result types.ToolResult
	err := workflow.ExecuteActivity(ctx, activities.ExecuteToolActivity, toolName, params).Get(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
	}

	// Store the result in the tool call
	toolCall.Result = result

	return result, nil
}

// SleepConfig holds common configuration for sleep operations.
type SleepConfig struct {
	Duration time.Duration
	Reason   string
	ToolName string
	Params   map[string]interface{}
}

// executeSleepWithConfig performs the actual sleep operation and returns a result.
func executeSleepWithConfig(ctx workflow.Context, config SleepConfig) (types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)

	// Log the sleep
	logger.Info(
		"Workflow sleeping",
		"duration",
		config.Duration,
		"reason",
		config.Reason,
		"tool",
		config.ToolName,
	)

	// Sleep in the workflow
	if err := workflow.Sleep(ctx, config.Duration); err != nil {
		logger.Error("Error during tool sleep", "error", err)
		return nil, fmt.Errorf("sleep error: %w", err)
	}

	// Create result message
	var message string
	if config.ToolName == "sleep" {
		message = fmt.Sprintf(
			"Slept for %.2f seconds. Reason: %s",
			config.Duration.Seconds(),
			config.Reason,
		)
	} else {
		// For sleep_until
		actualTime := workflow.Now(ctx)
		message = fmt.Sprintf("Slept until %s (sleep duration: %s). Reason: %s",
			actualTime.Format(time.RFC3339), config.Duration, config.Reason)
	}

	// Return result
	return &types.StructuredToolResult{
		ToolName:   config.ToolName,
		ToolParams: config.Params,
		Output:     map[string]any{"content": message, "data": message},
	}, nil
}

// executeSleep pauses workflow execution for the specified duration.
func executeSleep(ctx workflow.Context, params map[string]interface{}) (types.ToolResult, error) {
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
	reason := extractReason(params)

	// Convert to duration
	duration := time.Duration(durationSec * float64(time.Second))

	// Create sleep config and execute
	config := SleepConfig{
		Duration: duration,
		Reason:   reason,
		ToolName: "sleep",
		Params:   params,
	}

	return executeSleepWithConfig(ctx, config)
}

// executeSleepUntil pauses workflow execution until the specified time.
func executeSleepUntil(ctx workflow.Context, params map[string]interface{}) (types.ToolResult, error) {
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
		message := fmt.Sprintf(
			"Target time %s is in the past. No sleep performed.",
			targetTime.Format(time.RFC3339),
		)
		return &types.StructuredToolResult{
			ToolName:   "sleep_until",
			ToolParams: params,
			Output: map[string]any{
				"content": message,
				"data":    "Target time is in the past",
			},
		}, nil
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
		// Max duration applied
	}

	// Get optional reason parameter
	reason := extractReason(params)

	// Create sleep config and execute
	config := SleepConfig{
		Duration: sleepDuration,
		Reason:   reason,
		ToolName: "sleep_until",
		Params:   params,
	}

	return executeSleepWithConfig(ctx, config)
}

// extractReason extracts the optional reason parameter from the params map.
func extractReason(params map[string]interface{}) string {
	reason := "No reason specified"
	if reasonParam, hasReason := params["reason"].(string); hasReason && reasonParam != "" {
		reason = reasonParam
	}
	return reason
}
