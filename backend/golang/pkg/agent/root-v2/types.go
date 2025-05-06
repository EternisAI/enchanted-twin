package root

import "time"

// Root workflow constants
const (
	RootWorkflowID        = "root"    // New specific ID
	RootTaskQueue         = "default" // Specific queue
	ChildTaskQueue = "default" // Task queue for the agents themselves

	// Name of the child workflow type we are launching
	// PlannedAgentWorkflowName = "PlannedAgentWorkflow"
)

// Signal names
const (
	SignalCommand = "command"
)

// Query names
const (
	QueryListActiveRuns = "list_active_runs"
	QueryCommandStatus  = "get_command_status"
)

// Command names
const (
	CmdStartChildWorkflow = "start_child_workflow" // Start a child workflow"
)

// Command argument keys
const (
	ArgWorkflowName = "child_name" // Workflow to launch
	ArgWorkflowArgs = "child_args" // JSON string of plannedv2.PlanInput
	ArgTaskID       = "task_id"    // Optional user-friendly ID for the task
)

// Command structure for signals
type Command struct {
	Cmd   string         `json:"cmd"`
	Args  map[string]any `json:"args"`
	CmdID string         `json:"cmd_id"` // Unique ID for idempotency
}

// CommandStatus tracks the outcome of a command
type CommandStatus struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`           // "PROCESSING", "COMPLETED", "FAILED"
	RunID     string    `json:"run_id,omitempty"` // Store the RunID if successful
	Error     string    `json:"error,omitempty"`
}

// ChildRunInfo stores basic info about an active child workflow
type ChildRunInfo struct {
	RunID     string    `json:"run_id"`  // Temporal's Run ID
	TaskID    string    `json:"task_id"` // User-provided ID (optional)
	StartTime time.Time `json:"start_time"`
	// Status field can be added later when lifecycle management is implemented
}

// LauncherState holds the persistent workflow state
type LauncherState struct {
	// Tracks active planned agent runs (key: RunID)
	ActiveRuns map[string]*ChildRunInfo `json:"active_runs"`
	// Tracks processed command IDs for idempotency
	ProcessedCommands map[string]CommandStatus `json:"processed_commands"`
}

// Helper to create initial state
func NewLauncherState() *LauncherState {
	return &LauncherState{
		ActiveRuns:        make(map[string]*ChildRunInfo),
		ProcessedCommands: make(map[string]CommandStatus),
	}
}
