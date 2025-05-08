package plannedv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// Constants for workflow operations.
const (
	DefaultMaxSteps                    = 500
	DefaultActivityStartToCloseTimeout = 5 * time.Minute
	DefaultSystemPrompt                = "You are a helpful digital twin assistant that follows a plan. " +
		"Your task is to execute this plan step by step, use tools when needed, " +
		"and provide a clear final answer."
)

// PlannedAgentWorkflow is the main workflow for executing an agent plan.
func PlannedAgentWorkflow(ctx workflow.Context, input PlanInput) (string, error) {
	// Configure workflow
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: DefaultActivityStartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
			// TODO: add NonRetryableErrorTypes
		},
	})

	if input.Plan == "" {
		return "", fmt.Errorf("plan is required but was empty")
	}

	// Set default model if not provided
	if input.Model == "" {
		input.Model = "gpt-4o" // Default model
	}

	// Set default max steps if not provided
	if input.MaxSteps <= 0 {
		input.MaxSteps = DefaultMaxSteps
	}

	// Create initial state
	state := PlanState{
		Name:        input.Name,
		Plan:        input.Plan,
		CurrentStep: 0,
		// Schedule:      input.Schedule, // Schedule is handled by the parent
		Messages:      []ai.Message{},
		SelectedTools: input.ToolNames,
		ToolCalls:     []ToolCall{},
		ToolResults:   []types.ToolResult{},
		History:       []HistoryEntry{},
		Output:        "",
		ImageURLs:     []string{},
		StartedAt:     workflow.Now(ctx),
	}

	// Add system prompt
	systemPrompt := input.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	originStr, _ := json.Marshal(input.Origin)
	systemPrompt += fmt.Sprintf("\n\nTask Origin: %s\n", originStr)
	state.Messages = append(state.Messages, ai.NewSystemMessage(systemPrompt))

	// Add initial thought to history
	state.History = append(state.History, HistoryEntry{
		Type:      "thought",
		Content:   "I'm starting to execute the plan: " + state.Plan,
		Timestamp: workflow.Now(ctx),
	})

	// Register queries
	if err := registerQueries(ctx, &state); err != nil {
		state.Error = fmt.Sprintf("failed to register queries: %v", err)
		return "", fmt.Errorf("failed to register queries: %w", err)
	}

	// Register signals
	if err := registerSignals(ctx, &state); err != nil {
		state.Error = fmt.Sprintf("failed to register signals: %v", err)
		return "", fmt.Errorf("failed to register signals: %w", err)
	}

	// Execute the plan
	err := executeReActLoop(ctx, &state, input.Model, input.MaxSteps)
	if err != nil {
		if state.Error == "" {
			state.Error = fmt.Sprintf("execution failed: %v", err)
		}
		if state.Output == "" {
			state.Output = state.Error
		}
		return state.Output, fmt.Errorf("execution failed: %w", err)
	}

	return state.Output, nil
}

// executeReActLoop implements the ReAct loop for executing the plan.
func executeReActLoop(ctx workflow.Context, state *PlanState, model string, maxSteps int) error {
	logger := workflow.GetLogger(ctx)

	userMessage := fmt.Sprintf("# Faithfully complete the task **%s** by following the plan\n", state.Name)
	// if state.Schedule != "" {
	// 	userMessage += fmt.Sprintf("## The plan is scheduled for: %s\n\n", state.Schedule)
	// }
	userMessage += fmt.Sprintf("## Plan: %s\n\n", state.Plan)
	// Prompt the agent to start executing the plan
	state.Messages = append(
		state.Messages,
		ai.NewUserMessage(userMessage),
	)

	// Main ReAct loop
	for state.CurrentStep < maxSteps && state.CompletedAt.IsZero() {
		// Update the system time in the first message
		if err := updateSystemTime(ctx, state); err != nil {
			logger.Warn("Failed to update system time", "error", err)
		}

		// Generate the next actions using LLM
		toolCalls, err := generateNextAction(ctx, state, model)
		if err != nil {
			logger.Error("Failed to generate next actions", "error", err)
			state.History = append(state.History, HistoryEntry{
				Type:      "error",
				Content:   fmt.Sprintf("Failed to generate next actions: %v", err),
				Timestamp: workflow.Now(ctx),
			})
			// Add an error message to help the LLM recover
			errorMsg := fmt.Sprintf("Error: %v. Please try a different approach.", err)
			state.Messages = append(
				state.Messages,
				ai.NewToolMessage(errorMsg, "error_"+workflow.Now(ctx).Format(time.RFC3339)),
			)
			continue // continue instead of returning, to let the LLM try again
		}

		// Record the tool calls in history
		toolCallsJson, _ := json.Marshal(toolCalls)
		state.History = append(state.History, HistoryEntry{
			Type:      "actions",
			Content:   string(toolCallsJson),
			Timestamp: workflow.Now(ctx),
		})

		// Process each tool call
		for _, toolCall := range toolCalls {
			// Check if this is a final response
			toolFnName := toolCall.Function.Name
			if toolFnName == "final_response" || toolFnName == "complete_workflow" {
				// Parse arguments
				var params map[string]any
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					logger.Error("Failed to parse final response arguments", "error", err)
					errorMsg := fmt.Sprintf("Error parsing final response: %v", err)
					state.Messages = append(state.Messages, ai.NewToolMessage(errorMsg, toolCall.ID))
					continue
				}

				output, _ := params["output"].(string)
				logger.Info("Plan execution complete with final response", "output", output)
				state.Output = output
				state.CompletedAt = workflow.Now(ctx)
				break
			}

			// Execute the tool call
			result, err := executeAction(ctx, toolCall, state)

			// Always add a tool message, either with result or error
			if err != nil {
				logger.Error("Failed to execute tool call", "tool_call", toolCall, "error", err)

				// Record the error in history
				state.History = append(state.History, HistoryEntry{
					Type:      "error",
					Content:   fmt.Sprintf("Failed to execute tool call: %v", err),
					Timestamp: workflow.Now(ctx),
				})

				// Create an error result
				errorMsg := fmt.Sprintf("Error executing %s: %v", toolCall.Function.Name, err)

				// Add error message as a tool result
				errorResult := &types.StructuredToolResult{
					ToolName:   toolCall.Function.Name,
					ToolParams: make(map[string]any),
					Output:     map[string]any{"content": errorMsg},
					ToolError:  err.Error(),
				}

				// Add the error result to our collection
				state.ToolResults = append(state.ToolResults, errorResult)

				// Add tool message to message history with error
				state.Messages = append(state.Messages, ai.NewToolMessage(errorResult.Content(), toolCall.ID))
			} else {
				// Add the successful tool result to our collection
				state.ToolResults = append(state.ToolResults, result)

				// Add tool message to message history with result
				state.Messages = append(state.Messages, ai.NewToolMessage(result.Content(), toolCall.ID))

				// Record the observation in history
				state.History = append(state.History, HistoryEntry{
					Type:      "observation",
					Content:   result.Content(),
					Timestamp: workflow.Now(ctx),
				})

				// Collect image URLs if any
				imageURLs := result.ImageURLs()
				if len(imageURLs) > 0 {
					state.ImageURLs = append(state.ImageURLs, imageURLs...)
				}
			}
		}

		// If we completed the plan, break out of the loop
		if !state.CompletedAt.IsZero() {
			break
		}

		logger.Info("Step completed", "step", state.CurrentStep, "of max", maxSteps)

		state.CurrentStep++
	}

	// Check if we hit the max steps without completing
	if state.CurrentStep >= maxSteps && state.CompletedAt.IsZero() {
		logger.Warn("Reached maximum number of steps without completing the plan")
		state.History = append(state.History, HistoryEntry{
			Type:      "system",
			Content:   "Reached maximum number of steps without completing the plan",
			Timestamp: workflow.Now(ctx),
		})

		// Ask the LLM for a summary
		state.Messages = append(
			state.Messages,
			ai.NewUserMessage(
				"You've reached the maximum number of steps. Please provide a summary of what you've accomplished so far and what remains to be done.",
			),
		)

		// For the final completion, we need an AgentActivities instance,
		// which would typically be provided by the caller
		var activities *AgentActivities
		var finalCompletion openai.ChatCompletionMessage
		err := workflow.ExecuteActivity(ctx, activities.LLMCompletionActivity, model, state.Messages, []openai.ChatCompletionToolParam{}).
			Get(ctx, &finalCompletion)
		if err != nil {
			logger.Error("Failed to get final summary", "error", err)
			state.Output = "Reached maximum number of steps without completing. Unable to get summary."
		} else {
			state.Output = finalCompletion.Content
			state.History = append(state.History, HistoryEntry{
				Type:      "thought",
				Content:   finalCompletion.Content,
				Timestamp: workflow.Now(ctx),
			})
		}

		state.CompletedAt = workflow.Now(ctx)
	}

	return nil
}

func updateSystemTime(ctx workflow.Context, state *PlanState) error {
	// TODO: should use a template for/to update the system message
	// Update the system time in the state
	if state == nil || len(state.Messages) == 0 {
		return fmt.Errorf("state is nil or messages empty")
	}
	if state.Messages[0].Role != "system" {
		return fmt.Errorf("first message is not a system message")
	}

	now := workflow.Now(ctx).Format(time.RFC3339)
	timePattern := "Current System Time: "
	timeStr := fmt.Sprintf("%s%s\n", timePattern, now)

	// Check if the message already contains a time pattern
	currentContent := state.Messages[0].Content

	if timeIndex := strings.Index(currentContent, timePattern); timeIndex != -1 {
		// Find the end of the existing timestamp (look for newline)
		endOfLine := strings.Index(currentContent[timeIndex:], "\n")
		if endOfLine == -1 {
			// If no newline, append one to the new time string
			newContent := currentContent[:timeIndex] + timeStr
			state.Messages[0].Content = newContent
		} else {
			// Replace just the line with the timestamp
			newContent := currentContent[:timeIndex] + timeStr + currentContent[timeIndex+endOfLine+1:]
			state.Messages[0].Content = newContent
		}
	} else {
		// No existing timestamp, add to the end with appropriate newlines
		if !strings.HasSuffix(currentContent, "\n") {
			currentContent += "\n\n"
		} else if !strings.HasSuffix(currentContent, "\n\n") {
			currentContent += "\n"
		}
		state.Messages[0].Content = currentContent + timeStr
	}

	return nil
}
