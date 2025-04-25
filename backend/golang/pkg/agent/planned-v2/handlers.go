package plannedv2

import (
	"fmt"

	"go.temporal.io/sdk/workflow"
)

// Signal types for the planned agent workflow
const (
	SignalStop       = "stop_execution"
	SignalUpdatePlan = "update_plan"
)

// Query types for the planned agent workflow
const (
	QueryGetState       = "get_state"
	QueryGetHistory     = "get_history"
	QueryGetOutput      = "get_output"
	QueryGetToolCalls   = "get_tool_calls"
	QueryGetCurrentStep = "get_current_step"
)

// registerQueries registers all query handlers for the workflow
func registerQueries(ctx workflow.Context, state *PlanState) error {
	// Register query to get current state
	if err := workflow.SetQueryHandler(ctx, QueryGetState, func() (PlanState, error) {
		return *state, nil
	}); err != nil {
		return fmt.Errorf("failed to register get_state query handler: %w", err)
	}
	// Register query to get history
	if err := workflow.SetQueryHandler(ctx, QueryGetHistory, func() ([]HistoryEntry, error) {
		return state.History, nil
	}); err != nil {
		return fmt.Errorf("failed to register get_history query handler: %w", err)
	}
	// Register query to get output
	if err := workflow.SetQueryHandler(ctx, QueryGetOutput, func() (string, error) {
		return state.Output, nil
	}); err != nil {
		return fmt.Errorf("failed to register get_output query handler: %w", err)
	}
	// Register query to get tool calls
	if err := workflow.SetQueryHandler(ctx, QueryGetToolCalls, func() ([]map[string]interface{}, error) {
		// Convert to JSON-serializable format
		result := make([]map[string]interface{}, len(state.ToolCalls))
		for i, tc := range state.ToolCalls {
			result[i] = map[string]interface{}{
				"id":       tc.ID,
				"type":     tc.Type,
				"function": tc.Function,
			}
		}
		return result, nil
	}); err != nil {
		return fmt.Errorf("failed to register get_tool_calls query handler: %w", err)
	}
	// Register query to get current step
	if err := workflow.SetQueryHandler(ctx, QueryGetCurrentStep, func() (int, error) {
		return state.CurrentStep, nil
	}); err != nil {
		return fmt.Errorf("failed to register get_current_step query handler: %w", err)
	}
	return nil
}

// registerSignals registers all signal handlers for the workflow
func registerSignals(ctx workflow.Context, state *PlanState) error {
	// Register signal to stop execution
	stopChan := workflow.GetSignalChannel(ctx, SignalStop)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var signal interface{}
			stopChan.Receive(ctx, &signal)
			// Set the state to complete
			state.Complete = true
			state.Output = "Execution stopped by signal"
			// Add to history
			state.History = append(state.History, HistoryEntry{
				Type:      "system",
				Content:   "Execution stopped by signal",
				Timestamp: workflow.Now(ctx),
			})
		}
	})
	// Register signal to update plan
	updateChan := workflow.GetSignalChannel(ctx, SignalUpdatePlan)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var newPlan string
			updateChan.Receive(ctx, &newPlan)
			if newPlan != "" {
				// Add a system message to indicate plan update
				state.Messages = append(state.Messages, SystemMessage(fmt.Sprintf("The plan has been updated to: %s", newPlan)))
				// Add to history
				state.History = append(state.History, HistoryEntry{
					Type:      "system",
					Content:   "Plan updated",
					Timestamp: workflow.Now(ctx),
				})
				state.Plan = newPlan
			}
		}
	})
	return nil
}
