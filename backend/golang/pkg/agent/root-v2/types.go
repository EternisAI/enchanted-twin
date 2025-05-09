package root

import "time"

// Root workflow constants.
const (
	RootWorkflowID = "root"    // New specific ID
	RootTaskQueue  = "default" // Specific queue
	ChildTaskQueue = "default" // Task queue for the agents themselves
)

// Signal names.
const (
	SignalCommand = "command"
)

// Query names.
const (
	QueryListWorkflows = "list_workflows"
	QueryCommandStatus = "get_command_status"
)

// Command names.
const (
	CmdStartChildWorkflow     = "start_child_workflow"     // Start a child workflow
	CmdTerminateChildWorkflow = "terminate_child_workflow" // Terminate a running child workflow
)

// Command argument keys.
const (
	ArgWorkflowName = "child_name" // Workflow to launch
	ArgWorkflowArgs = "child_args" // JSON string of plannedv2.PlanInput
	ArgTaskID       = "task_id"    // Optional user-friendly ID for the task
	ArgRunID        = "run_id"     // Temporal run ID to terminate
	ArgReason       = "reason"     // Reason for termination
)

// Command structure for signals.
type Command struct {
	Cmd   string         `json:"cmd"`
	Args  map[string]any `json:"args"`
	CmdID string         `json:"cmd_id"` // Unique ID for idempotency
}

// CommandStatus tracks the outcome of a command.
type CommandStatus struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`           // "PROCESSING", "COMPLETED", "FAILED"
	RunID     string    `json:"run_id,omitempty"` // Store the RunID if successful
	Error     string    `json:"error,omitempty"`
}

// ChildRunInfo stores basic info about an active child workflow.
type ChildRunInfo struct {
	RunID        string    `json:"run_id"`      // Temporal's Run ID
	WorkflowID   string    `json:"workflow_id"` // Child workflow ID
	TaskID       string    `json:"task_id"`     // User-provided ID (optional)
	CreatedAt    time.Time `json:"start_time"`
	CompletedAt      time.Time `json:"ended_at,omitempty"`      // Optional end time
	TerminatedAt time.Time `json:"terminated_at,omitempty"` // Optional termination time`
}

// RootState holds the persistent workflow state.
type RootState struct {
	// Tracks active planned agent runs (key: RunID)
	Tasks map[string]*ChildRunInfo `json:"active_runs"`
	// Tracks processed command IDs for idempotency (key: CmdID)
	ProcessedCommands map[string]CommandStatus `json:"processed_commands"`
}

// Helper to create initial state.
func NewRootState() *RootState {
	return &RootState{
		Tasks:             make(map[string]*ChildRunInfo),
		ProcessedCommands: make(map[string]CommandStatus),
	}
}
