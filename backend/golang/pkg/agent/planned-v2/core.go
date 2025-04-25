package plannedv2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/openai/openai-go"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DefaultMaxSteps is the default number of iterations for ReAct loop
const DefaultMaxSteps = 15

// Constants for workflow operations
const (
	DefaultExecutionTimeout = 30 * time.Second // Reduced for tests
	HeartbeatInterval       = 100 * time.Millisecond
)

// PlannedAgentWorkflow is the main workflow for executing an agent plan
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
		planInput.Model = "claude-3-sonnet-20240229" // Default model
	}

	// Set default max steps if not provided
	if planInput.MaxSteps <= 0 {
		planInput.MaxSteps = DefaultMaxSteps
	}

	// Create initial state
	state := PlanState{
		Plan:        planInput.Plan,
		CurrentStep: 0,
		Complete:    false,
		Messages:    []Message{},
		ToolCalls:   []openai.ChatCompletionMessageToolCall{},
		ToolResults: []ToolResult{},
		History:     []HistoryEntry{},
		Tools:       []ToolDefinition{},
		Output:      "",
		ImageURLs:   []string{},
		StartTime:   workflow.Now(ctx),
	}

	// Fetch and register tools
	if err := fetchAndRegisterTools(ctx, &state, planInput.ToolNames); err != nil {
		state.Error = fmt.Sprintf("failed to fetch tools: %v", err)
		return fmt.Errorf("failed to fetch tools: %w", err)
	}

	// Add system prompt
	systemPrompt := planInput.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = fmt.Sprintf("You are a helpful assistant that follows a plan. Your task is to execute this plan step by step:\n\n%s\n\nAs you work through the plan, think step-by-step, use tools when needed, and provide a clear final answer.", planInput.Plan)
	}
	state.Messages = append(state.Messages, SystemMessage(systemPrompt))

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

// executeReActLoop implements the ReAct loop for executing the plan
func executeReActLoop(ctx workflow.Context, state *PlanState, model string, maxSteps int) error {
	logger := workflow.GetLogger(ctx)

	// Convert our tools to OpenAI format for the API
	apiToolDefinitions := make([]openai.ChatCompletionToolParam, 0, len(state.Tools))
	for _, tool := range state.Tools {
		apiToolDefinitions = append(apiToolDefinitions, tool.ToOpenAITool())
	}

	// Prompt the agent to start executing the plan
	state.Messages = append(state.Messages, UserMessage(fmt.Sprintf("Please start executing this plan: %s", state.Plan)))

	// Main ReAct loop
	for state.CurrentStep < maxSteps && !state.Complete {
		logger.Info("Executing step", "step", state.CurrentStep+1, "of", maxSteps)

		logger.Info("[XXX] messages", "messages", state.Messages)

		// Generate the next action using LLM
		action, err := generateNextAction(ctx, state, apiToolDefinitions, model)
		if err != nil {
			logger.Error("Failed to generate next action", "error", err)
			state.History = append(state.History, HistoryEntry{
				Type:      "error",
				Content:   fmt.Sprintf("Failed to generate next action: %v", err),
				Timestamp: workflow.Now(ctx),
			})
			return err
		}

		// Record the action in history
		actionJson, _ := json.Marshal(action)
		state.History = append(state.History, HistoryEntry{
			Type:      "action",
			Content:   string(actionJson),
			Timestamp: workflow.Now(ctx),
		})

		// If this is a final response with no tool calls, we're done
		if action.Tool == "final_response" {
			logger.Info("Plan execution complete with final response", "output", action.Params["output"].(string))
			state.Output = action.Params["output"].(string)
			state.Complete = true
			break
		}

		// Execute the action
		result, err := executeAction(ctx, action, state)
		if err != nil {
			logger.Error("Failed to execute action", "action", action, "error", err)
			state.History = append(state.History, HistoryEntry{
				Type:      "error",
				Content:   fmt.Sprintf("Failed to execute action: %v", err),
				Timestamp: workflow.Now(ctx),
			})
			return err
		}

		// Add the tool result
		state.ToolResults = append(state.ToolResults, *result)

		// Add to message history
		// For LLM API compatibility, we need to add the tool message
		if len(state.ToolCalls) > 0 && len(state.ToolCalls) > len(state.ToolResults) {
			latestToolCall := state.ToolCalls[len(state.ToolCalls)-1]
			state.Messages = append(state.Messages, ToolMessage(result.Content, latestToolCall.ID))
		}

		// Record the observation in history
		state.History = append(state.History, HistoryEntry{
			Type:      "observation",
			Content:   result.Content,
			Timestamp: workflow.Now(ctx),
		})

		// Collect image URLs if any
		if len(result.ImageURLs) > 0 {
			state.ImageURLs = append(state.ImageURLs, result.ImageURLs...)
		}

		logger.Info("Step completed", "step", state.CurrentStep, "of max", maxSteps)

		state.CurrentStep++

		if err := workflow.Sleep(ctx, HeartbeatInterval); err != nil {
			logger.Error("Error during heartbeat sleep", "error", err)
		}
	}

	// Check if we hit the max steps without completing
	if state.CurrentStep >= maxSteps && !state.Complete {
		logger.Warn("Reached maximum number of steps without completing the plan")
		state.History = append(state.History, HistoryEntry{
			Type:      "system",
			Content:   "Reached maximum number of steps without completing the plan",
			Timestamp: workflow.Now(ctx),
		})
		state.Messages = append(state.Messages, SystemMessage("Reached maximum number of steps without completing the plan"))

		// Ask the LLM for a summary
		state.Messages = append(state.Messages, UserMessage("You've reached the maximum number of steps. Please provide a summary of what you've accomplished so far and what remains to be done."))

		var finalCompletion LLMCompletion
		err := workflow.ExecuteActivity(ctx, LLMCompletionActivity, model, state.Messages, []openai.ChatCompletionToolParam{}).Get(ctx, &finalCompletion)
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

// fetchAndRegisterTools fetches available tools and registers them
func fetchAndRegisterTools(ctx workflow.Context, state *PlanState, toolNames []string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Fetching and registering tools", "requested_tools", toolNames)

	// TODO: This would query tools from the Root workflow or a tool registry
	// For now, we'll just add some default tools for testing

	// Add echo tool
	state.Tools = append(state.Tools, ToolDefinition{
		Name:        "echo",
		Description: "Echoes back the input text",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"text"},
		},
		Entrypoint: types.ToolDefEntrypoint{
			Type: types.ToolDefEntrypointTypeWorkflow,
		},
	})

	// Add math tool
	state.Tools = append(state.Tools, ToolDefinition{
		Name:        "math",
		Description: "Performs basic math operations",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type": "string",
					"enum": []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]any{
					"type": "number",
				},
				"b": map[string]any{
					"type": "number",
				},
			},
			"required": []string{"operation", "a", "b"},
		},
		Entrypoint: types.ToolDefEntrypoint{
			Type: types.ToolDefEntrypointTypeWorkflow,
		},
	})

	// Add sleep tool
	state.Tools = append(state.Tools, ToolDefinition{
		Name:        "sleep",
		Description: "Pauses execution for a specified duration in seconds",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"duration": map[string]any{
					"type":        "number",
					"description": "Duration to sleep in seconds",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Optional reason for the sleep",
				},
			},
			"required": []string{"duration"},
		},
		Entrypoint: types.ToolDefEntrypoint{
			Type: types.ToolDefEntrypointTypeWorkflow,
		},
	})

	// Add sleep_until tool
	state.Tools = append(state.Tools, ToolDefinition{
		Name:        "sleep_until",
		Description: "Pauses execution until a specific time (ISO8601 format)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"timestamp": map[string]any{
					"type":        "string",
					"description": "ISO8601 timestamp to sleep until (e.g. 2023-06-15T14:30:00Z)",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Optional reason for the sleep",
				},
			},
			"required": []string{"timestamp"},
		},
		Entrypoint: types.ToolDefEntrypoint{
			Type: types.ToolDefEntrypointTypeWorkflow,
		},
	})

	// If specific tools were requested, we would query them here
	// For now, we'll keep the default tools

	logger.Info("Registered tools", "count", len(state.Tools))
	return nil
}
