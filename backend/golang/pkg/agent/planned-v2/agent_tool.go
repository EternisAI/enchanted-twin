package plannedv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"go.temporal.io/sdk/client"
)

// PlannedAgentTool implements a tool for planned agent execution.
type PlannedAgentTool struct {
	logger         *log.Logger
	temporalClient client.Client
	model          string
	maxSteps       int
}

// NewPlannedAgentTool creates a new PlannedAgentTool.
func NewPlannedAgentTool(
	logger *log.Logger,
	temporalClient client.Client,
	model string,
) *PlannedAgentTool {
	return &PlannedAgentTool{
		logger:         logger,
		temporalClient: temporalClient,
		model:          model,
		maxSteps:       15, // Default value
	}
}

// Definition returns the tool definition.
func (t *PlannedAgentTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "schedule_task",
			Description: param.NewOpt(
				"Schedule a task to be executed periodically by the agent with planning and reasoning",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"plan": map[string]any{
						"type":        "string",
						"description": "A detailed step-by-step plan to execute",
					},
					"tools": map[string]any{
						"type":        "array",
						"description": "Optional list of tool names to use (defaults to all available tools)",
						"items": map[string]any{
							"type": "string",
						},
					},
					"system_prompt": map[string]any{
						"type":        "string",
						"description": "Optional system prompt override",
					},
				},
				"required": []string{"plan"},
			},
		},
	}
}

// Execute starts the planned agent workflow and waits for completion.
func (t *PlannedAgentTool) Execute(
	ctx context.Context,
	args map[string]any,
) (agenttypes.ToolResult, error) {
	plan, ok := args["plan"].(string)
	if !ok || plan == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: args,
			ToolError:  "plan is required",
		}, fmt.Errorf("plan is required")
	}

	// Extract optional parameters
	toolNames := []string{}
	if toolsArg, ok := args["tools"].([]any); ok {
		for _, t := range toolsArg {
			if toolName, ok := t.(string); ok {
				toolNames = append(toolNames, toolName)
			}
		}
	}

	systemPrompt := ""
	if promptArg, ok := args["system_prompt"].(string); ok {
		systemPrompt = promptArg
	}

	// Create workflow input
	input := PlanInput{
		Plan:         plan,
		ToolNames:    toolNames,
		Model:        t.model,
		SystemPrompt: systemPrompt,
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: args,
			ToolError:  fmt.Sprintf("failed to marshal input: %v", err),
		}, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Generate a unique workflow ID using UUID
	workflowID := fmt.Sprintf("%s_%s", WorkflowName, uuid.New().String())

	// Set workflow options
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "default",
		// WorkflowExecutionTimeout: 30 * time.Minute,  // TODO: pass this as a plan parameter
	}

	// Start the workflow
	t.logger.Info("Starting planned agent workflow", "plan_length", len(plan), "tools", toolNames)
	run, err := t.temporalClient.ExecuteWorkflow(
		ctx,
		workflowOptions,
		WorkflowName,
		inputJSON,
	)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: args,
			ToolError:  fmt.Sprintf("failed to start workflow: %v", err),
		}, fmt.Errorf("failed to start workflow: %w", err)
	}

	t.logger.Info(
		"Workflow started",
		"workflow_id",
		run.GetID(),
		"run_id",
		run.GetRunID(),
	)

	// Wait for workflow completion with timeout
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result string
	err = run.Get(waitCtx, &result)
	if err != nil {
		// Try to query for output in case of timeout
		var output string
		resp, queryErr := t.temporalClient.QueryWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			QueryGetOutput,
			nil,
		)
		if queryErr == nil {
			queryErr = resp.Get(&output)
			if queryErr == nil && output != "" {
				return &agenttypes.StructuredToolResult{
					ToolName:   "schedule_task",
					ToolParams: args,
					Output: map[string]any{
						"content": fmt.Sprintf(
							"Plan execution in progress. Current output: %s",
							output,
						),
					},
				}, nil
			}
		}

		return &agenttypes.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: args,
			ToolError:  fmt.Sprintf("workflow execution failed: %v", err),
		}, fmt.Errorf("workflow execution failed: %w", err)
	}

	// Create the result
	return &agenttypes.StructuredToolResult{
		ToolName:   "schedule_task",
		ToolParams: args,
		Output: map[string]any{
			"content": result,
		},
	}, nil
}
