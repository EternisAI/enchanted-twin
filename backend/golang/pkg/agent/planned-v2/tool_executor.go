package plannedv2

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/workflow"
)

// ToolExecutor handles execution of tools in workflow context
type ToolExecutor struct {
	registry tools.ToolRegistry
}

// NewToolExecutor creates a new tool executor with the given registry
func NewToolExecutor(registry tools.ToolRegistry, logger *log.Logger) *ToolExecutor {
	executor := &ToolExecutor{
		registry: registry,
	}

	return executor
}

// Execute runs a tool call and returns the result
func (e *ToolExecutor) Execute(ctx workflow.Context, toolCall ToolCall, state *PlanState) (types.ToolResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing tool call", "id", toolCall.ID, "tool", toolCall.Function.Name)

	// Parse arguments
	var params map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	toolName := toolCall.Function.Name

	// Check if this is a workflow immediate tool first
	if tools.IsWorkflowImmediateTool(toolName) {
		immediateTool, _ := tools.GetWorkflowImmediateTool(toolName)
		result, err := immediateTool.ExecuteInWorkflow(ctx, params)
		if err != nil {
			return nil, err
		}
		
		// Store the result in the tool call
		toolCall.Result = result
		
		return result, nil
	}

	// Execute the tool activity
	var result types.ToolResult
	err := workflow.ExecuteActivity(ctx, e.executeToolActivity, toolName, params).Get(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
	}

	// Store the result in the tool call
	toolCall.Result = result

	return result, nil
}

// executeToolActivity is a wrapper around the registry's Execute method
func (e *ToolExecutor) executeToolActivity(ctx context.Context, toolName string, params map[string]any) (types.ToolResult, error) {
	// Use the registry from the executor
	// In a real implementation, we'd pass the state's registry to the activity
	registry := e.registry
	if registry == nil {
		return &types.StructuredToolResult{
			ToolName:   toolName,
			ToolParams: params,
			ToolError:  "no registry available",
		}, fmt.Errorf("no registry available")
	}

	toolResult, err := registry.Execute(ctx, toolName, params)
	if err != nil {
		return &types.StructuredToolResult{
			ToolName:   toolName,
			ToolParams: params,
			ToolError:  err.Error(),
		}, nil
	}

	// Tool results are already in the correct format
	return toolResult, nil
}

// ExecuteBatch executes multiple tool calls in parallel
func (e *ToolExecutor) ExecuteBatch(ctx workflow.Context, toolCalls []ToolCall, state *PlanState) ([]types.ToolResult, error) {
	// Create a future for each tool call
	futures := make([]workflow.Future, len(toolCalls))
	results := make([]types.ToolResult, len(toolCalls))

	// Start all tool calls in parallel
	for i, toolCall := range toolCalls {
		toolCallCopy := toolCall // Create a copy to avoid closure capturing the loop variable
		// Use local execution instead of activity for now
		// This will be expanded in the future to support parallel activity execution
		future := workflow.ExecuteLocalActivity(ctx, func() (types.ToolResult, error) {
			return e.Execute(ctx, toolCallCopy, state)
		})
		futures[i] = future
	}

	// Wait for all futures to complete
	for i, future := range futures {
		var result types.ToolResult
		if err := future.Get(ctx, &result); err != nil {
			// Create an error result
			results[i] = &types.StructuredToolResult{
				ToolName:   toolCalls[i].Function.Name,
				ToolError:  err.Error(),
				ToolParams: make(map[string]any),
			}
		} else {
			results[i] = result
		}
	}

	return results, nil
}
