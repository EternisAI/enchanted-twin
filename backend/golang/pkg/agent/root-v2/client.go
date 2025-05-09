package root

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log" // Or your logger
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// RootClient provides a convenient way to interact with the running RootWorkflow.
type RootClient struct {
	temporalClient client.Client
	logger         *log.Logger
}

// NewRootClient creates a new client for interacting with the RootWorkflow.
func NewRootClient(c client.Client, logger *log.Logger) *RootClient {
	return &RootClient{
		temporalClient: c,
		logger:         logger.With("component", "root-client", "RootWorkflowID", RootWorkflowID),
	}
}

// ListWorkflows queries the RootWorkflow for a list of currently tracked active child runs.
// Note: In V0, this list isn't actively pruned based on actual child completion status.
func (rc *RootClient) ListWorkflows(ctx context.Context) (map[string]*ChildRunInfo, error) {
	rc.logger.Debug("Querying for active runs")

	resp, err := rc.temporalClient.QueryWorkflow(
		ctx,
		RootWorkflowID, // Target the specific Root Workflow ID
		"",             // Target the latest run
		QueryListWorkflows,
		// No arguments for this query
	)
	if err != nil {
		rc.logger.Error("Failed to query for active runs", "error", err)
		return nil, fmt.Errorf("failed to query %s: %w", QueryListWorkflows, err)
	}

	var runs map[string]*ChildRunInfo
	if err := resp.Get(&runs); err != nil {
		rc.logger.Error("Failed to decode active runs from query result", "error", err)
		return nil, fmt.Errorf("failed to decode query result for %s: %w", QueryListWorkflows, err)
	}

	rc.logger.Debug("Successfully retrieved active runs", "count", len(runs))
	return runs, nil
}

// StartChildWorkflow signals the RootWorkflow to start a new child workflow.
// It returns the command ID used for the signal, which can be used with GetCommandStatus.
//
// Parameters:
//   - ctx: Context for the operation.
//   - workflowName: The registered type name of the child workflow to start (e.g., "PlannedAgentWorkflow").
//   - taskID: A unique identifier for this specific task instance (used for the child's Workflow ID).
//   - workflowArgs: A slice of arguments to pass to the child workflow's execution function.
func (rc *RootClient) StartChildWorkflow(ctx context.Context, workflowName string, taskID string, workflowArgs ...any) (string, error) {
	if workflowName == "" {
		return "", fmt.Errorf("workflowName cannot be empty")
	}
	if taskID == "" {
		// Or generate a UUID if taskID shouldn't be mandatory from caller
		return "", fmt.Errorf("taskID cannot be empty")
	}

	cmdID := uuid.NewString() // Unique ID for this command

	commandArgs := map[string]any{
		ArgWorkflowName: workflowName,
		ArgTaskID:       taskID,
		ArgWorkflowArgs: workflowArgs, // Pass the arguments as provided
	}

	command := Command{
		Cmd:   CmdStartChildWorkflow,
		Args:  commandArgs,
		CmdID: cmdID,
	}

	rc.logger.Info("Signaling RootWorkflow to start child",
		"Command", command.Cmd,
		"CmdID", cmdID,
		"ChildWorkflowName", workflowName,
		"TaskID", taskID,
	)

	err := rc.temporalClient.SignalWorkflow(
		ctx,
		RootWorkflowID, // Target the specific Root Workflow ID
		"",             // Target the latest run
		SignalCommand,  // The signal channel name
		command,        // The command payload
	)
	if err != nil {
		rc.logger.Error("Failed to signal RootWorkflow", "error", err, "CmdID", cmdID)
		return "", fmt.Errorf("failed to signal root workflow command %s: %w", CmdStartChildWorkflow, err)
	}

	rc.logger.Info("Successfully signaled RootWorkflow", "CmdID", cmdID)
	return cmdID, nil // Return the command ID for status tracking
}

// GetCommandStatus queries the RootWorkflow for the status of a previously sent command.
func (rc *RootClient) GetCommandStatus(ctx context.Context, commandID string) (CommandStatus, error) {
	if commandID == "" {
		return CommandStatus{}, fmt.Errorf("commandID cannot be empty")
	}

	rc.logger.Debug("Querying for command status", "CmdID", commandID)

	resp, err := rc.temporalClient.QueryWorkflow(
		ctx,
		RootWorkflowID, // Target the specific Root Workflow ID
		"",             // Target the latest run
		QueryCommandStatus,
		commandID, // Pass the command ID as the query argument
	)
	if err != nil {
		rc.logger.Error("Failed to query for command status", "CmdID", commandID, "error", err)
		return CommandStatus{}, fmt.Errorf("failed to query %s for CmdID %s: %w", QueryCommandStatus, commandID, err)
	}

	var status CommandStatus
	if err := resp.Get(&status); err != nil {
		rc.logger.Error("Failed to decode command status from query result", "CmdID", commandID, "error", err)
		return CommandStatus{}, fmt.Errorf("failed to decode query result for %s for CmdID %s: %w", QueryCommandStatus, commandID, err)
	}

	rc.logger.Debug("Successfully retrieved command status", "CmdID", commandID, "Status", status.Status)
	return status, nil
}

// TerminateChildWorkflow signals the RootWorkflow to terminate a running child workflow.
// It returns the command ID used for the signal, which can be used with GetCommandStatus.
func (rc *RootClient) TerminateChildWorkflow(ctx context.Context, runID string, reason string) (string, error) {
	cmdID := uuid.NewString() // Unique ID for this command

	commandArgs := map[string]any{
		ArgRunID:  runID,
		ArgReason: reason,
	}

	command := Command{
		Cmd:   CmdTerminateChildWorkflow,
		Args:  commandArgs,
		CmdID: cmdID,
	}

	rc.logger.Info("Signaling RootWorkflow terminate child",
		"Command", command.Cmd,
		"CmdID", cmdID,
		"RunID", runID,
	)

	err := rc.temporalClient.SignalWorkflow(
		ctx,
		RootWorkflowID, // Target the specific Root Workflow ID
		"",             // Target the latest run
		SignalCommand,  // The signal channel name
		command,        // The command payload
	)
	if err != nil {
		rc.logger.Error("Failed to signal RootWorkflow for termination",
			"error", err,
			"CmdID", cmdID,
			"RunID", runID)
		return "", fmt.Errorf("failed to signal RootWorkflow for termination of %s: %w",
			runID, err)
	}

	return cmdID, nil
}
