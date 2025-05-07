package root

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log" // Or your preferred logger
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func RegisterWorkflowsAndActivities(w *worker.Worker, logger *log.Logger) {
	logger.Info("Registering root workflow")

	// Register the workflows
	(*w).RegisterWorkflow(RootWorkflow)
}

// EnsureRunRootWorkflow checks if the root workflow is running and starts it if not.
// It's designed to be idempotent.
func EnsureRunRootWorkflow(ctx context.Context, c client.Client, logger *log.Logger) error {
	workflowID := RootWorkflowID
	taskQueue := RootTaskQueue

	logger = logger.With("WorkflowID", workflowID, "TaskQueue", taskQueue)
	logger.Info("Ensuring Root workflow is running...")

	// 1. Check if the workflow already exists and is running
	desc, err := c.DescribeWorkflowExecution(ctx, workflowID, "") // "" for runID means the latest run

	if err == nil {
		// Workflow exists, check its status
		status := desc.GetWorkflowExecutionInfo().GetStatus()
		if status == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
			logger.Info("Root workflow is already running.")
			return nil // Already running, nothing more to do
		}
		// If it exists but is not running (e.g., completed, failed, timed out)
		logger.Warn("Root workflow exists but is not in RUNNING state.", "Status", status.String())
		// We'll proceed to try and start it, relying on the reuse policy
	} else if _, ok := err.(*serviceerror.NotFound); ok {
		// Workflow not found, we need to start it
		logger.Info("Root workflow not found. Attempting to start.")
	} else {
		// Some other error occurred describing the workflow
		logger.Error("Failed to describe root workflow", "error", err)
		return fmt.Errorf("failed to describe root workflow %s: %w", workflowID, err)
	}

	// 2. Attempt to start the workflow
	// Use a WorkflowIDReusePolicy that allows starting if the previous run finished
	// AllowDuplicate is suitable for singletons that might complete or fail and need restarting.
	workflowOptions := client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: 0, // Runs indefinitely until ContinueAsNew or termination
		WorkflowRunTimeout:       0, // Runs indefinitely until ContinueAsNew or termination
		// This policy allows starting a new run if the previous one with the same ID
		// is NOT currently running (e.g., completed, failed, timedout, canceled, terminated).
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}

	logger.Info("Executing RootWorkflow...")
	// Pass nil for the initial state
	_, err = c.ExecuteWorkflow(ctx, workflowOptions, RootWorkflow, (*RootState)(nil))
	if err != nil {
		// Check if the error is because it's already running (due to race condition or policy)
		if _, ok := err.(*serviceerror.WorkflowExecutionAlreadyStarted); ok {
			logger.Warn("Root workflow already started (detected during ExecuteWorkflow call).")
			// This is considered success in the context of "ensuring" it's running.
			return nil
		}
		// Any other error during start is a problem
		logger.Error("Failed to start root workflow", "error", err)
		return fmt.Errorf("failed to start root workflow %s: %w", workflowID, err)
	}

	logger.Info("Root workflow started successfully or was already running.")
	return nil
}
