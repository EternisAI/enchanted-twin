package plannedv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/openai/openai-go"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DefaultMaxSteps is the default number of iterations for ReAct loop.
const DefaultMaxSteps = 100

// Constants for workflow operations.
const (
	DefaultExecutionTimeout = 30 * time.Second
)

// PlannedAgentWorkflow is the main workflow for executing an agent plan.
func PlannedAgentWorkflow(ctx workflow.Context, input []byte) error {
	// Configure workflow
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: DefaultExecutionTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PlannedAgentWorkflow", "input_size", len(input))

	// Parse input
	var planInput PlanInput
	if err := json.Unmarshal(input, &planInput); err != nil {
		return fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if planInput.Plan == "" {
		return fmt.Errorf("plan is required but was empty")
	}

	// Set default model if not provided
	if planInput.Model == "" {
		planInput.Model = "gpt-4o" // Default model
	}

	// Set default max steps if not provided
	planInput.MaxSteps = DefaultMaxSteps

	// Create initial state
	state := PlanState{
		Plan:          planInput.Plan,
		CurrentStep:   0,
		Complete:      false,
		Schedule:      planInput.Schedule,
		Messages:      []ai.Message{},
		SelectedTools: planInput.ToolNames,
		ToolCalls:     []ToolCall{},
		ToolResults:   []types.ToolResult{},
		History:       []HistoryEntry{},
		Output:        "",
		ImageURLs:     []string{},
		StartTime:     workflow.Now(ctx),
	}

	// Add system prompt
	systemPrompt := planInput.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = fmt.Sprintf(
			"You are a helpful assistant that follows a plan.\nYour task is to execute this plan step by step:\n\n%s\n\nAs you work through the plan, think step-by-step, use tools when needed, and provide a clear final answer.",
			planInput.Plan,
		)
	}
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
		return fmt.Errorf("failed to register queries: %w", err)
	}

	// Register signals
	if err := registerSignals(ctx, &state); err != nil {
		state.Error = fmt.Sprintf("failed to register signals: %v", err)
		return fmt.Errorf("failed to register signals: %w", err)
	}

	// Execute the plan
	err := executeReActLoop(ctx, &state, planInput.Model, planInput.MaxSteps)
	if err != nil {
		state.Error = fmt.Sprintf("execution failed: %v", err)
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}

// executeReActLoop implements the ReAct loop for executing the plan.
func executeReActLoop(ctx workflow.Context, state *PlanState, model string, maxSteps int) error {
	logger := workflow.GetLogger(ctx)

	userMessage := "Please faithfully complete the following task\n"
	if state.Schedule != "" {
		userMessage += fmt.Sprintf("Schedule: %s\n\n", state.Schedule)
	}
	userMessage += fmt.Sprintf("Plan: %s\n\n", state.Plan)
	// Prompt the agent to start executing the plan
	state.Messages = append(
		state.Messages,
		ai.NewUserMessage(userMessage),
	)

	// Main ReAct loop
	for state.CurrentStep < maxSteps && !state.Complete {
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
			if toolCall.Function.Name == "final_response" {
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
				state.Complete = true
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
					ToolParams: make(map[string]interface{}),
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
		if state.Complete {
			break
		}

		logger.Info("Step completed", "step", state.CurrentStep, "of max", maxSteps)

		state.CurrentStep++
	}

	// Check if we hit the max steps without completing
	if state.CurrentStep >= maxSteps && !state.Complete {
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

		state.Complete = true
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

	now := time.Now().Format(time.RFC3339)
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

// // fetchAndRegisterTools fetches available tools and registers them.
// func fetchAndRegisterTools(ctx workflow.Context, state *PlanState, toolNames []string) error {
// 	logger := workflow.GetLogger(ctx)
// 	logger.Info("Fetching and registering tools", "requested_tools", toolNames)

// 	// Always add built-in workflow tools
// 	addBuiltInWorkflowTools(state, logger)

// 	registeredTools := []tools.Tool{}

// 	// If specific tools were requested, get only those
// 	if len(toolNames) > 0 {
// 		for _, name := range toolNames {
// 			// Skip built-in workflow tools as they're added separately
// 			if name == "sleep" || name == "sleep_until" || name == "final_response" {
// 				continue
// 			}

// 			if tool, exists := registry.Get(name); exists {
// 				registeredTools = append(registeredTools, tool)
// 			} else {
// 				logger.Warn("Requested tool not found in registry", "tool", name)
// 			}
// 		}
// 	} else {
// 		// No specific tools requested, get all tools from registry
// 		for _, name := range registry.List() {
// 			// Skip built-in workflow tools as they're added separately
// 			if name == "sleep" || name == "sleep_until" || name == "final_response" {
// 				continue
// 			}

// 			if tool, exists := registry.Get(name); exists {
// 				registeredTools = append(registeredTools, tool)
// 			}
// 		}
// 	}

// 	// Register all regular tools from the registry
// 	if len(registeredTools) > 0 {
// 		if err := registry.Register(registeredTools...); err != nil {
// 			logger.Error("Failed to register tools", "error", err)
// 		}
// 	}

// 	workflowToolCount := len(tools.WorkflowImmediateTools())
// 	logger.Info("Tools registered for workflow",
// 		"total_tools", len(registry.List()),
// 		"workflow_tools", workflowToolCount,
// 		"registry_tools", len(registeredTools))

// 	return nil
// }

// // addBuiltInWorkflowTools adds the built-in workflow tools to the state
// func addBuiltInWorkflowTools(state *PlanState, logger log.Logger) {
// 	// Register workflow immediate tools with the registry
// 	if state.Registry != nil {
// 		if err := state.Registry.Register(tools.WorkflowImmediateTools()...); err != nil {
// 			logger.Warn("Failed to register workflow immediate tools", "error", err)
// 			// Error is non-critical as it just means some workflow tools won't be available
// 		} else {
// 			logger.Debug("Registered workflow immediate tools", "count", len(tools.WorkflowImmediateTools()))
// 		}
// 	}
// }
