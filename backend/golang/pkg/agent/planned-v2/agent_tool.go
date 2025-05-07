package plannedv2

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/agent/root-v2"
	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// ExecutePlanTool implements a tool for planned agent execution, optionally scheduled.
type ExecutePlanTool struct {
	logger         *log.Logger
	temporalClient client.Client
	model          string
	maxSteps       int // Keep internal default
}

// NewExecutePlanTool creates a new ExecutePlanTool.
func NewExecutePlanTool(
	logger *log.Logger,
	temporalClient client.Client,
	model string,
) *ExecutePlanTool {
	return &ExecutePlanTool{
		logger:         logger,
		temporalClient: temporalClient,
		model:          model,
		maxSteps:       1000,
	}
}

// Definition returns the tool definition for OpenAI.
func (t *ExecutePlanTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "execute_plan",
			Description: param.NewOpt(
				"Submits a multi-step plan to the system for autonomous execution. Optionally schedule the plan for repeated execution.",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					// --- Required ---
					"name": map[string]any{ // User-provided name
						"type":        "string",
						"description": "A descriptive name for the task the plan accomplishes (e.g., 'Daily Weather Check').",
					},
					"plan": map[string]any{
						"type":        "string",
						"description": "A detailed step-by-step plan for the agent to execute.",
					},
					// --- Optional ---
					"schedule": map[string]any{
						"type":        "string",
						"description": "Optional iCalendar RRULE formatted schedule string (e.g., 'FREQ=DAILY;INTERVAL=1;BYHOUR=9'). If provided, the plan will be scheduled.",
					},
					"tools": map[string]any{
						"type":        "array",
						"description": "Optional list of tool names the agent executing the plan should have access to.",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"name", "plan"},
			},
		},
	}
}

// Execute signals the RootWorkflow to start a planned agent workflow.
func (t *ExecutePlanTool) Execute(
	ctx context.Context,
	args map[string]any,
) (agenttypes.ToolResult, error) {
	// --- 1. Extract and Validate Arguments ---
	plan, ok := args["plan"].(string)
	if !ok || plan == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "execute_plan",
			ToolParams: args,
			ToolError:  "plan (string) is required",
		}, fmt.Errorf("plan is required")
	}

	// Extract the user-provided 'name'
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "execute_plan",
			ToolParams: args,
			ToolError:  "name (string) is required",
		}, fmt.Errorf("name is required")
	}

	toolNames := []string{}
	if toolsArg, ok := args["tools"].([]any); ok {
		for _, toolRaw := range toolsArg {
			if toolName, ok := toolRaw.(string); ok {
				toolNames = append(toolNames, toolName)
			}
		}
	}

	schedule := ""
	if scheduleArg, ok := args["schedule"].(string); ok {
		schedule = scheduleArg
	}

	// --- 2. Prepare Input for the Child Workflow (PlannedAgentWorkflow) ---
	planInput := PlanInput{
		Name:      name,
		Schedule:  schedule,
		Plan:      plan,
		ToolNames: toolNames,
		Model:     t.model,
		MaxSteps:  t.maxSteps,
		Origin:    args, // Pass original tool args for context within the child
	}

	// // Marshal PlanInput to JSON []byte for passing as a single arg
	// planInputBytes, err := json.Marshal(planInput)
	// if err != nil {
	// 	return &agenttypes.StructuredToolResult{
	// 		ToolName:   "execute_plan",
	// 		ToolParams: args,
	// 		ToolError:  fmt.Sprintf("failed to marshal plan input: %v", err),
	// 	}, fmt.Errorf("failed to marshal plan input: %w", err)
	// }

	// --- 3. Prepare the Command for the Root Workflow ---
	cmdID := uuid.NewString()  // Generate unique command ID for idempotency
	taskID := uuid.NewString() // Generate unique task ID for Temporal workflow ID

	// Root workflow expects arguments for the child workflow as []any.
	// Here, PlannedAgentWorkflow expects a single PlanInput argument.
	// We marshal it to JSON bytes above.
	childWorkflowArgs := []any{planInput}

	commandArgs := map[string]any{
		root.ArgWorkflowName: WorkflowName,      // Name of the child workflow *type*
		root.ArgTaskID:       taskID,            // Use the generated UUID for Temporal tracking
		root.ArgWorkflowArgs: childWorkflowArgs, // Pass the marshaled PlanInput bytes
	}

	command := root.Command{
		Cmd:   root.CmdStartChildWorkflow,
		Args:  commandArgs,
		CmdID: cmdID,
	}

	// --- 4. Signal the Root Workflow ---
	t.logger.Info("Signaling RootWorkflow to start child workflow",
		"RootWorkflowID", root.RootWorkflowID,
		"Command", command.Cmd,
		"CmdID", command.CmdID,
		"ChildWorkflowName", WorkflowName,
		"TaskID", taskID, // Log the generated Temporal task ID
		"UserName", name, // Log the user-provided name
	)

	if err := t.temporalClient.SignalWorkflow(
		ctx,
		root.RootWorkflowID, // Target the static Root Workflow ID
		"",                  // Target the latest run
		root.SignalCommand,  // The signal channel name
		command,             // The command payload
	); err != nil {
		t.logger.Error("Failed to signal RootWorkflow", "error", err, "CmdID", cmdID)
		return &agenttypes.StructuredToolResult{
			ToolName:   "execute_plan",
			ToolParams: args,
			ToolError:  fmt.Sprintf("failed to signal root workflow: %v", err),
		}, fmt.Errorf("failed to signal root workflow: %w", err)
	}

	// --- 5. Return Success (Asynchronous) ---
	successMsg := fmt.Sprintf("Successfully submitted task '%s' for execution. Check status with Command ID: %s. Internal Task ID: %s", name, cmdID, taskID)
	t.logger.Info(successMsg)

	return &agenttypes.StructuredToolResult{
		ToolName:   "execute_plan",
		ToolParams: args,
		Output: map[string]any{
			"content":    successMsg,
			"command_id": cmdID,  // For querying command status
			"task_id":    taskID, // The internal Temporal task ID
			"name":       name,   // The user-provided name
		},
	}, nil
}
