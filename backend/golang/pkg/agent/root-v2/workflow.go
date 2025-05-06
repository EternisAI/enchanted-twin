package root

import (
	"fmt"
	"maps"
	"time"

	"go.temporal.io/sdk/workflow"
)

// History size/event count threshold to trigger ContinueAsNew.
const (
	historyThreshold  = 10000              // Adjust as needed
	commandPruningAge = 7 * 24 * time.Hour // Prune command statuses older than 1 week
)

// RootWorkflow launches and manages child agent workflows.
func RootWorkflow(ctx workflow.Context, prevState *LauncherState) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("PlannedAgentLauncherWorkflow starting or continuing", "WorkflowID", RootWorkflowID)

	// Initialize state
	state := prevState
	if state == nil {
		logger.Info("Initializing new LauncherState")
		state = NewLauncherState()
	}

	// --- Setup Query Handlers ---
	err := workflow.SetQueryHandler(ctx, QueryListActiveRuns, func() (map[string]*ChildRunInfo, error) {
		// Return a copy
		runsCopy := make(map[string]*ChildRunInfo, len(state.ActiveRuns))
		maps.Copy(runsCopy, state.ActiveRuns)
		return runsCopy, nil
	})
	if err != nil {
		logger.Error("Failed to register list_active_runs query handler", "error", err)
		return fmt.Errorf("failed to register list_active_runs query handler: %w", err)
	}

	err = workflow.SetQueryHandler(ctx, QueryCommandStatus, func(cmdID string) (CommandStatus, error) {
		status, exists := state.ProcessedCommands[cmdID]
		if !exists {
			return CommandStatus{}, fmt.Errorf("command status for '%s' not found", cmdID)
		}
		return status, nil
	})
	if err != nil {
		logger.Error("Failed to register get_command_status query handler", "error", err)
		return fmt.Errorf("failed to register get_command_status query handler: %w", err)
	}

	// --- Main Workflow Loop ---
	commandChan := workflow.GetSignalChannel(ctx, SignalCommand)

	for {
		selector := workflow.NewSelector(ctx)

		// Listen for new commands
		selector.AddReceive(commandChan, func(c workflow.ReceiveChannel, more bool) {
			if !more {
				logger.Info("Command channel closed.")
				return
			}
			var cmd Command
			c.Receive(ctx, &cmd)
			logger.Info("Received command", "Command", cmd.Cmd, "CmdID", cmd.CmdID)

			// --- Idempotency Check ---
			if status, exists := state.ProcessedCommands[cmd.CmdID]; exists && status.Status != "PROCESSING" {
				logger.Info("Skipping already processed command", "CmdID", cmd.CmdID, "Status", status.Status)
				return // Already processed
			}
			state.ProcessedCommands[cmd.CmdID] = CommandStatus{
				Timestamp: workflow.Now(ctx),
				Status:    "PROCESSING",
			}

			// --- Dispatch Command ---
			var cmdErr error
			var runID string // To store RunID for successful starts

			switch cmd.Cmd {
			case CmdStartChildWorkflow:
				runID, cmdErr = handleStartChildWorkflow(ctx, state, cmd.Args)
			default:
				cmdErr = fmt.Errorf("unknown command type: %s", cmd.Cmd)
			}

			// --- Update Command Status ---
			finalStatus := CommandStatus{Timestamp: workflow.Now(ctx)}
			if cmdErr != nil {
				finalStatus.Status = "FAILED"
				finalStatus.Error = cmdErr.Error()
				logger.Error("Command processing failed", "CmdID", cmd.CmdID, "Command", cmd.Cmd, "Error", cmdErr)
			} else {
				finalStatus.Status = "COMPLETED"
				finalStatus.RunID = runID // Store the RunID on success
				logger.Info("Command processing completed", "CmdID", cmd.CmdID, "Command", cmd.Cmd, "RunID", runID)
			}
			state.ProcessedCommands[cmd.CmdID] = finalStatus
		})

		// Wait for a signal
		selector.Select(ctx)

		// --- ContinueAsNew Check ---
		currentHistoryLength := workflow.GetInfo(ctx).GetCurrentHistoryLength()
		if currentHistoryLength > historyThreshold {
			logger.Info("History length threshold reached, continuing as new.", "HistoryLength", currentHistoryLength)
			pruneProcessedCommands(ctx, state, workflow.Now(ctx))
			// TODO: In a future version, prune state.ActiveRuns based on querying Temporal for their actual status
			// For V0, we just carry over all runs listed as "active".
			// TODO: Should drain the signal and command queues here
			return workflow.NewContinueAsNewError(ctx, RootWorkflow, state)
		}
	}
}

// --- Command Handler ---

func handleStartChildWorkflow(ctx workflow.Context, state *LauncherState, args map[string]any) (string, error) {
	logger := workflow.GetLogger(ctx)

	// Child Workflow args
	workflowName, ok := args[ArgWorkflowName]
	if !ok || workflowName == "" {
		return "", fmt.Errorf("missing or invalid argument %s (json string)", ArgWorkflowName)
	}

	taskID, okTaskID := args[ArgTaskID].(string)
	if !okTaskID || taskID == "" {
		return "", fmt.Errorf("missing or invalid argument %s (json string)", ArgTaskID)
	}

	childArgs, okChildArgs := args[ArgWorkflowArgs].([]any)
	if !okChildArgs {
		return "", fmt.Errorf("invalid argument %s (json array)", ArgWorkflowArgs)
	}

	// --- Start Child Workflow ---
	// Use a child workflow ID that includes the task ID if provided for easier identification
	// Or use a UUID if taskID is empty
	childWorkflowID := fmt.Sprintf("%s_%s", workflowName, taskID)

	// Set options for the child workflow
	// Note: You might want configurable timeouts passed in via args later
	childOpts := workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID, // Assign a meaningful or unique ID
		TaskQueue:  ChildTaskQueue,
		// WorkflowExecutionTimeout: time.Hour * 1, // Example timeout
		// WorkflowRunTimeout:       time.Hour * 1, // Example timeout
	}
	childCtx := workflow.WithChildOptions(ctx, childOpts)

	logger.Info("Starting child", "Workflow", workflowName, "ChildWorkflowID", childOpts.WorkflowID)

	// Execute the child, passing the PlanInput JSON as []byte
	childFuture := workflow.ExecuteChildWorkflow(childCtx, workflowName, childArgs...)

	// Get the execution info (which contains RunID)
	var childExecution workflow.Execution
	if err := childFuture.GetChildWorkflowExecution().Get(ctx, &childExecution); err != nil {
		logger.Error("Failed to get child workflow execution info", "error", err)
		return "", fmt.Errorf("failed to start child workflow: %w", err)
	}
	runID := childExecution.RunID
	logger.Info("Child workflow started", "ChildWorkflowID", childExecution.ID, "RunID", runID)

	// --- Update State ---
	state.ActiveRuns[runID] = &ChildRunInfo{
		RunID:     runID,
		TaskID:    taskID, // Store the user-provided task ID
		StartTime: workflow.Now(ctx),
	}

	// --- V0: No Lifecycle Management ---
	// In this simple version, we don't wait for the child to complete or update its status.
	// The run will remain in ActiveRuns until the Launcher workflow itself ContinuesAsNew or terminates.
	// A future version would launch a goroutine here to wait on childFuture.Get(ctx, &result)
	// and then update the status in ActiveRuns.

	return runID, nil // Return the RunID
}

// --- Helper Functions ---

// pruneProcessedCommands removes old command statuses to prevent unbounded map growth.
func pruneProcessedCommands(ctx workflow.Context, state *LauncherState, now time.Time) {
	logger := workflow.GetLogger(ctx) // Use context from where it's called if possible
	count := 0
	for cmdID, status := range state.ProcessedCommands {
		if now.Sub(status.Timestamp) > commandPruningAge {
			delete(state.ProcessedCommands, cmdID)
			count++
		}
	}
	if count > 0 {
		logger.Info("Pruned old command statuses", "Count", count)
	}
}
