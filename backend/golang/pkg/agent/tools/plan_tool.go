package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/client"
)

// PlanTool implements a tool for planning and executing tasks with the planner.
type PlanTool struct {
	logger         *log.Logger
	temporalClient client.Client
	model          string
}

// NewPlanTool creates a new plan tool.
func NewPlanTool(logger *log.Logger, temporalClient client.Client, model string) *PlanTool {
	return &PlanTool{
		logger:         logger,
		temporalClient: temporalClient,
		model:          model,
	}
}

// Definition returns the tool definition.
func (t *PlanTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "execute_plan",
			Description: param.NewOpt(
				"Execute a plan to carry out a specific task using a sequence of steps",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"plan": map[string]any{
						"type":        "string",
						"description": "A detailed plan describing the task to execute, broken down into numbered steps",
					},
					"use_tools": map[string]any{
						"type":        "array",
						"description": "List of tool names that should be available to the plan execution",
						"items": map[string]any{
							"type": "string",
						},
					},
					"max_steps": map[string]any{
						"type":        "integer",
						"description": "Maximum number of steps (LLM calls) to execute for the plan (default: 15)",
					},
					"wait": map[string]any{
						"type":        "integer",
						"description": "Wait timeout in seconds for plan completion. If omitted, returns immediately with workflow ID",
					},
				},
				"required": []string{"plan"},
			},
		},
	}
}

// Execute executes the plan tool.
func (t *PlanTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	plan, ok := args["plan"].(string)
	if !ok || plan == "" {
		return ToolResult{}, errors.New("plan is required")
	}

	// Extract optional tool names
	toolNames := []string{}
	if toolNamesRaw, ok := args["use_tools"].([]any); ok {
		for _, toolNameRaw := range toolNamesRaw {
			if toolName, ok := toolNameRaw.(string); ok {
				toolNames = append(toolNames, toolName)
			}
		}
	}

	// Extract max steps
	maxSteps := 15 // Default to 15 steps
	if maxStepsRaw, ok := args["max_steps"].(float64); ok {
		steps := int(maxStepsRaw)
		if steps > 0 {
			maxSteps = steps
		}
	}

	// Start the workflow
	workflowID := fmt.Sprintf("plan_%d", time.Now().UnixNano())
	workflowOptions := client.StartWorkflowOptions{
		ID:                 workflowID,
		TaskQueue:          "default",
		WorkflowRunTimeout: 10 * time.Minute, // Fixed timeout, we'll use max_steps for control
	}

	// Create a list of tools to use
	availableTools := getAvailableTools(toolNames)
	toolsBytes, err := json.Marshal(availableTools)
	if err != nil {
		return ToolResult{}, errors.Wrap(err, "failed to marshal tools")
	}

	// Create the workflow input
	input := map[string]any{
		"plan":      plan,
		"tools":     toolsBytes,
		"model":     t.model,
		"max_steps": maxSteps,
		"system_prompt": fmt.Sprintf(
			"You are a helpful assistant that executes plans. Your current plan is: %s",
			plan,
		),
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return ToolResult{}, errors.Wrap(err, "failed to marshal input")
	}

	// Start the workflow
	run, err := t.temporalClient.ExecuteWorkflow(
		ctx,
		workflowOptions,
		"PlannerWorkflow",
		inputBytes,
	)
	if err != nil {
		return ToolResult{}, errors.Wrap(err, "failed to start workflow")
	}

	t.logger.Info("Started planner workflow", "workflowID", workflowID, "runID", run.GetRunID())

	// Check if we should wait for completion
	waitSeconds := 0
	if waitRaw, ok := args["wait"].(float64); ok && waitRaw > 0 {
		waitSeconds = int(waitRaw)
	}

	// If no wait parameter or wait is 0, return immediately
	if waitSeconds == 0 {
		return ToolResult{
			Content: fmt.Sprintf(
				"Plan execution started with workflow ID: %s. It will continue in the background.",
				workflowID,
			),
		}, nil
	}

	// Wait for workflow completion with the specified timeout
	waitTimeout := time.Duration(waitSeconds) * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	var result string
	err = run.GetWithOptions(waitCtx, &result, client.WorkflowRunGetOptions{})

	// Check if we timed out
	if errors.Is(err, context.DeadlineExceeded) {
		// Query for the current output
		resp, queryErr := t.temporalClient.QueryWorkflow(
			ctx,
			workflowID,
			run.GetRunID(),
			"get_output",
		)
		if queryErr != nil {
			return ToolResult{
				Content: fmt.Sprintf(
					"The plan is still executing (workflow ID: %s). It will continue in the background. The output so far could not be retrieved: %v",
					workflowID,
					queryErr,
				),
			}, nil
		}

		var output string
		if queryErr = resp.Get(&output); queryErr != nil {
			return ToolResult{
				Content: fmt.Sprintf(
					"The plan is still executing (workflow ID: %s). It will continue in the background. The output so far could not be retrieved: %v",
					workflowID,
					queryErr,
				),
			}, nil
		}

		if output == "" {
			output = "No output yet."
		}

		return ToolResult{
			Content: fmt.Sprintf(
				"The plan is still executing (workflow ID: %s). It will continue in the background. Output so far: %s",
				workflowID,
				output,
			),
		}, nil
	}

	if err != nil {
		return ToolResult{
			Content: fmt.Sprintf("Error executing plan: %v (workflow ID: %s)", err, workflowID),
		}, nil
	}

	// Query for the final output
	resp, err := t.temporalClient.QueryWorkflow(ctx, workflowID, run.GetRunID(), "get_output")
	if err != nil {
		return ToolResult{
			Content: fmt.Sprintf(
				"Plan executed, but couldn't retrieve output: %v (workflow ID: %s)",
				err,
				workflowID,
			),
		}, nil
	}

	var output string
	if err = resp.Get(&output); err != nil {
		return ToolResult{
			Content: fmt.Sprintf(
				"Plan executed, but couldn't retrieve output: %v (workflow ID: %s)",
				err,
				workflowID,
			),
		}, nil
	}

	// Check if we have image URLs
	var imageURLs []string
	stateResp, err := t.temporalClient.QueryWorkflow(ctx, workflowID, run.GetRunID(), "get_state")
	if err != nil {
		t.logger.Error("Failed to query workflow state", "error", err)
		return ToolResult{}, errors.Wrap(err, "failed to query workflow state")
	}

	var state map[string]any
	if err = stateResp.Get(&state); err == nil {
		if urls, ok := state["image_urls"].([]any); ok {
			for _, url := range urls {
				if urlStr, ok := url.(string); ok {
					imageURLs = append(imageURLs, urlStr)
				}
			}
		}
	}

	return ToolResult{
		Content:   output,
		ImageURLs: imageURLs,
	}, nil
}

// It creates instances of the requested tools.
func getAvailableTools(toolNames []string) []Tool {
	// Create a map of available tool creators
	toolCreators := map[string]func() Tool{
		"search": func() Tool { return &SearchTool{} },
		"image":  func() Tool { return &ImageTool{} },
		"echo": func() Tool {
			return &EchoTool{}
		},
	}

	// If no specific tools requested, return a reasonable default set
	if len(toolNames) == 0 {
		// Return basic set of tools - echo tool is always available for testing
		return []Tool{&EchoTool{}}
	}

	// Otherwise create the requested tools
	var tools []Tool
	for _, name := range toolNames {
		if creator, ok := toolCreators[name]; ok {
			tools = append(tools, creator())
		}
	}

	// Always include the echo tool for testing
	hasEcho := false
	for _, tool := range tools {
		if tool.Definition().Function.Name == "echo" {
			hasEcho = true
			break
		}
	}
	if !hasEcho {
		tools = append(tools, &EchoTool{})
	}

	return tools
}
