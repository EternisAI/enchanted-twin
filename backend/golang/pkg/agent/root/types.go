package root

// Root workflow constants
const (
	// RootWorkflowID is the ID of the root workflow
	RootWorkflowID = "enchanted_twin_root"

	// WorkflowTaskQueue is the task queue for all agent workflows
	WorkflowTaskQueue = "default"

	// Default heartbeat interval (5 minutes)
	DefaultHeartbeat = 5 * 60 * 1000 // milliseconds

	// Default history cutoff threshold for ContinueAsNew (~1MB in bytes)
	DefaultHistoryCutoff = 1024 * 1024 // 1MB
)

// Signal names
const (
	// SignalCommand is the signal name for sending commands to the root workflow
	SignalCommand = "command"
)

// Query names
const (
	// QueryGetState is the query name for getting the full root state
	QueryGetState = "get_state"

	// QueryListAgents is the query name for listing all registered agents
	QueryListAgents = "list_agents"

	// QueryGetAgent is the query name for getting a specific agent by ID
	QueryGetAgent = "get_agent"

	// QueryListRuns is the query name for listing all active runs
	QueryListRuns = "list_runs"

	// QueryListTools is the query name for listing all registered tools
	QueryListTools = "list_tools"
)

// Command names
const (
	// CmdCreateAgent is the command name for creating a new agent
	CmdCreateAgent = "create_agent"

	// CmdDeleteAgent is the command name for deleting an agent
	CmdDeleteAgent = "delete_agent"

	// CmdStartAgent is the command name for starting an agent
	CmdStartAgent = "start_agent"

	// CmdSignalAgent is the command name for signaling an agent
	CmdSignalAgent = "signal_agent"

	// CmdRegisterTool is the command name for registering a tool
	CmdRegisterTool = "register_tool"

	// CmdDeregisterTool is the command name for deregistering a tool
	CmdDeregisterTool = "deregister_tool"

	// CmdStopWorkflow is a special command for testing only - not exposed as a tool
	CmdStopWorkflow = "_stop_workflow"
)

// Command argument keys
const (
	// ArgAgentID is the argument key for agent ID
	ArgAgentID = "agent_id"

	// ArgBlueprint is the argument key for agent blueprint
	ArgBlueprint = "blueprint"

	// ArgInput is the argument key for agent input
	ArgInput = "input"

	// ArgRunID is the argument key for run ID
	ArgRunID = "run_id"

	// ArgSignal is the argument key for signal name
	ArgSignal = "signal"

	// ArgPayload is the argument key for signal payload
	ArgPayload = "payload"

	// ArgToolDef is the argument key for tool definition
	ArgToolDef = "tool_def"

	// ArgToolName is the argument key for tool name
	ArgToolName = "tool_name"
)

// Child workflow run status values
const (
	// StatusRunning indicates the child workflow is currently running
	StatusRunning = "RUNNING"

	// StatusCompleted indicates the child workflow has completed successfully
	StatusCompleted = "COMPLETED"

	// StatusFailed indicates the child workflow has failed
	StatusFailed = "FAILED"

	// StatusCancelled indicates the child workflow was cancelled
	StatusCancelled = "CANCELLED"

	// StatusTimeout indicates the child workflow timed out
	StatusTimeout = "TIMEOUT"
)
