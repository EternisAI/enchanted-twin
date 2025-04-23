package root

// Root workflow (signal‑driven) with automatic ContinueAsNew to keep history
// small. The workflow accepts an *optional* previous state; on first run the
// argument is nil. On each loop iteration we check `workflow.GetInfo(ctx).
// GetCurrentHistorySize()` and migrate via `workflow.NewContinueAsNewError` when the
// size exceeds `historyCutoff`.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// Time-based constants
const (
	heartbeat     = 5 * time.Minute
	historyCutoff = DefaultHistoryCutoff // ContinueAsNew when history size exceeds this
)

// -----------------------------------------------------------------------------
// Types (unchanged)
// -----------------------------------------------------------------------------

type Command struct {
	Cmd   string         `json:"cmd"`
	Args  map[string]any `json:"args"`
	CmdID string         `json:"cmd_id"`
}

type AgentInfo struct {
	AgentID   string
	Version   string
	Blueprint []byte
	CreatedAt time.Time
}

type ChildRunInfo struct {
	RunID   string
	AgentID string
	Started time.Time
	Status  string
}

type RootState struct {
	Registry   map[string]*AgentInfo
	ActiveRuns map[string]*ChildRunInfo
	Tools      map[string]types.ToolDef
	SeenCmdIDs map[string]struct{}
}

// -----------------------------------------------------------------------------
// Root workflow entry point
// -----------------------------------------------------------------------------

func RootWorkflow(ctx workflow.Context, prev *RootState) error {
	logger := workflow.GetLogger(ctx)

	// Restore or init state
	var st *RootState
	if prev != nil {
		st = prev
	} else {
		st = &RootState{
			Registry:   map[string]*AgentInfo{},
			ActiveRuns: map[string]*ChildRunInfo{},
			Tools:      map[string]types.ToolDef{},
			SeenCmdIDs: map[string]struct{}{},
		}
	}

	// Setup query handlers
	if err := setupQueryHandlers(ctx, st); err != nil {
		logger.Error("Failed to set up query handlers", "error", err)
		return err
	}

	cmdCh := workflow.GetSignalChannel(ctx, SignalCommand)

	for {
		sel := workflow.NewSelector(ctx)

		sel.AddReceive(cmdCh, func(c workflow.ReceiveChannel, ok bool) {
			var cmd Command
			c.Receive(ctx, &cmd)
			handleCommand(ctx, st, cmd)
		})
		sel.AddFuture(workflow.NewTimer(ctx, heartbeat), func(workflow.Future) {})
		sel.Select(ctx)

		// ---- ContinueAsNew check ----
		info := workflow.GetInfo(ctx)
		historySize := info.GetCurrentHistorySize()
		if historySize > historyCutoff {
			logger.Info("Continuing as new", "hist_size_bytes", historySize)

			for drained := false; !drained; {
				sel := workflow.NewSelector(ctx)
				sel.AddReceive(cmdCh, func(c workflow.ReceiveChannel, ok bool) {
					if ok {
						var cmd Command
						c.Receive(ctx, &cmd)
						handleCommand(ctx, st, cmd)
					}
				})
				sel.AddDefault(func() { drained = true })
				sel.Select(ctx)
			}
			return workflow.NewContinueAsNewError(ctx, RootWorkflow, st)
		}
	}
}

// -----------------------------------------------------------------------------
// Command handler
// -----------------------------------------------------------------------------

// setupQueryHandlers configures the query handlers for the root workflow
func setupQueryHandlers(ctx workflow.Context, st *RootState) error {
	logger := workflow.GetLogger(ctx)

	// Standard state query handler (read-only)
	if err := workflow.SetQueryHandler(ctx, QueryGetState, func() (RootState, error) {
		return *st, nil
	}); err != nil {
		logger.Error("Failed to set query handler", "query", QueryGetState, "error", err)
		return err
	}

	// List registered agents
	if err := workflow.SetQueryHandler(ctx, QueryListAgents, func() (map[string]*AgentInfo, error) {
		return st.Registry, nil
	}); err != nil {
		logger.Error("Failed to set query handler", "query", QueryListAgents, "error", err)
		return err
	}

	// Get specific agent info
	if err := workflow.SetQueryHandler(ctx, QueryGetAgent, func(agentID string) (*AgentInfo, error) {
		agent, ok := st.Registry[agentID]
		if !ok {
			return nil, fmt.Errorf("agent %s not found", agentID)
		}
		return agent, nil
	}); err != nil {
		logger.Error("Failed to set query handler", "query", QueryGetAgent, "error", err)
		return err
	}

	// List active runs
	if err := workflow.SetQueryHandler(ctx, QueryListRuns, func() (map[string]*ChildRunInfo, error) {
		return st.ActiveRuns, nil
	}); err != nil {
		logger.Error("Failed to set query handler", "query", QueryListRuns, "error", err)
		return err
	}

	// List registered tools
	if err := workflow.SetQueryHandler(ctx, QueryListTools, func() (map[string]types.ToolDef, error) {
		return st.Tools, nil
	}); err != nil {
		logger.Error("Failed to set query handler", "query", QueryListTools, "error", err)
		return err
	}

	return nil
}

// handleCommand processes incoming commands from the signal channel
func handleCommand(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{"cmd": c.Cmd}).Counter("root.commands").Inc(1)

	// Check for duplicate command ID
	if _, dup := st.SeenCmdIDs[c.CmdID]; dup {
		logger.Debug("Duplicate command ignored", "cmd_id", c.CmdID)
		return
	}
	st.SeenCmdIDs[c.CmdID] = struct{}{}

	switch c.Cmd {
	case CmdCreateAgent:
		handleCreateAgent(ctx, st, c)
	case CmdDeleteAgent:
		handleDeleteAgent(ctx, st, c)
	case CmdStartAgent:
		handleStartAgent(ctx, st, c)
	case CmdSignalAgent:
		handleSignalAgent(ctx, st, c)
	case CmdRegisterTool:
		handleRegisterTool(ctx, st, c)
	case CmdDeregisterTool:
		handleDeregisterTool(ctx, st, c)
	default:
		logger.Warn("Unknown command received", "cmd", c.Cmd)
	}
}

// handleCreateAgent registers a new agent with the system
func handleCreateAgent(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	agentID, ok := c.Args[ArgAgentID].(string)
	if !ok {
		logger.Error("Missing or invalid agent_id")
		return
	}

	blueprintRaw, ok := c.Args[ArgBlueprint].(string)
	if !ok {
		logger.Error("Missing or invalid blueprint")
		return
	}
	blueprint := []byte(blueprintRaw)

	// Check for duplicate
	if _, exists := st.Registry[agentID]; exists {
		logger.Warn("Agent already exists", "id", agentID)
		return
	}

	// Parse and validate blueprint (basic validation - schema validation will come later)
	var dsl map[string]interface{}
	if err := json.Unmarshal(blueprint, &dsl); err != nil {
		logger.Error("Invalid agent blueprint", "id", agentID, "error", err)
		return
	}

	// Get version from the blueprint if possible
	version := "0.1.0" // Default
	if agentMap, ok := dsl["agent"].(map[string]interface{}); ok {
		if v, ok := agentMap["version"].(string); ok {
			version = v
		}
	}

	// Register the agent
	st.Registry[agentID] = &AgentInfo{
		AgentID:   agentID,
		Version:   version,
		Blueprint: blueprint,
		CreatedAt: workflow.Now(ctx),
	}

	logger.Info("Agent registered successfully", "id", agentID, "version", version)
}

// handleDeleteAgent removes an agent from the registry
func handleDeleteAgent(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	agentID, ok := c.Args[ArgAgentID].(string)
	if !ok {
		logger.Error("Missing or invalid agent_id")
		return
	}

	// Check if agent exists
	if _, exists := st.Registry[agentID]; !exists {
		logger.Warn("Cannot delete: agent not found", "id", agentID)
		return
	}

	// Check if agent has active runs
	for _, run := range st.ActiveRuns {
		if run.AgentID == agentID {
			logger.Error("Cannot delete agent with active runs", "id", agentID)
			return
		}
	}

	// Delete the agent
	delete(st.Registry, agentID)
	logger.Info("Agent deleted successfully", "id", agentID)
}

// handleStartAgent launches a new agent workflow instance
func handleStartAgent(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	agentID, ok := c.Args[ArgAgentID].(string)
	if !ok {
		logger.Error("Missing or invalid agent_id")
		return
	}

	input := c.Args[ArgInput]

	// Check if agent exists
	ai, ok := st.Registry[agentID]
	if !ok {
		logger.Error("Cannot start: agent not found", "id", agentID)
		return
	}

	// Prepare input
	inputJSON, err := json.Marshal(input)
	if err != nil {
		logger.Error("Failed to marshal input", "error", err)
		return
	}

	// Launch child workflow
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		TaskQueue: WorkflowTaskQueue,
	})

	var runID string
	fut := workflow.ExecuteChildWorkflow(childCtx, "PlanAgentWorkflow", ai.Blueprint, inputJSON)
	if err := fut.GetChildWorkflowExecution().Get(ctx, &runID); err != nil {
		logger.Error("Failed to start agent workflow", "id", agentID, "error", err)
		return
	}

	// Record active run
	st.ActiveRuns[runID] = &ChildRunInfo{
		RunID:   runID,
		AgentID: agentID,
		Started: workflow.Now(ctx),
		Status:  StatusRunning,
	}

	logger.Info("Agent workflow launched", "agent_id", agentID, "run_id", runID)
}

// handleSignalAgent sends a signal to a running agent workflow
func handleSignalAgent(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	runID, ok := c.Args[ArgRunID].(string)
	if !ok {
		logger.Error("Missing or invalid run_id")
		return
	}

	signal, ok := c.Args[ArgSignal].(string)
	if !ok {
		logger.Error("Missing or invalid signal name")
		return
	}

	payload := c.Args[ArgPayload]

	// Check if run exists
	if _, exists := st.ActiveRuns[runID]; !exists {
		logger.Warn("Cannot signal: run not found", "run_id", runID)
		return
	}

	// Forward signal to child workflow - use the runID directly
	if err := workflow.SignalExternalWorkflow(ctx, runID, "", signal, payload).Get(ctx, nil); err != nil {
		logger.Error("Failed to signal child workflow", "run_id", runID, "error", err)
		return
	}

	logger.Info("Signal forwarded to agent workflow", "run_id", runID, "signal", signal)
}

// handleRegisterTool adds a new tool to the registry
func handleRegisterTool(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	toolDefRaw, ok := c.Args[ArgToolDef].(string)
	if !ok {
		logger.Error("Missing or invalid tool_def")
		return
	}

	var toolDef types.ToolDef
	if err := json.Unmarshal([]byte(toolDefRaw), &toolDef); err != nil {
		logger.Error("Invalid tool definition", "error", err)
		return
	}

	// Check for duplicate
	if _, exists := st.Tools[toolDef.Name]; exists {
		logger.Warn("Tool already registered", "name", toolDef.Name)
		return
	}

	// Register the tool
	st.Tools[toolDef.Name] = toolDef
	logger.Info("Tool registered successfully", "name", toolDef.Name)
}

// handleDeregisterTool removes a tool from the registry
func handleDeregisterTool(ctx workflow.Context, st *RootState, c Command) {
	logger := workflow.GetLogger(ctx)

	toolName, ok := c.Args[ArgToolName].(string)
	if !ok {
		logger.Error("Missing or invalid tool_name")
		return
	}

	// Check if tool exists
	if _, exists := st.Tools[toolName]; !exists {
		logger.Warn("Cannot deregister: tool not found", "name", toolName)
		return
	}

	// Deregister the tool
	delete(st.Tools, toolName)
	logger.Info("Tool deregistered successfully", "name", toolName)
}

// -----------------------------------------------------------------------------
// Helper to start or fetch the Root workflow (called by subscriber startup)
// -----------------------------------------------------------------------------

func EnsureRootWorkflow(c client.Client) (client.WorkflowRun, error) {
	// Try to get existing workflow
	ctx := context.Background()

	if _, err := c.DescribeWorkflowExecution(ctx, RootWorkflowID, ""); err == nil {
		// Workflow exists, return it
		return c.GetWorkflow(ctx, RootWorkflowID, ""), nil
	}

	// Start new workflow if not exists
	opts := client.StartWorkflowOptions{
		ID:                 RootWorkflowID,
		TaskQueue:          WorkflowTaskQueue,
		WorkflowRunTimeout: 365 * 24 * time.Hour,
		// Use default reuse policy
	}
	return c.ExecuteWorkflow(context.Background(), opts, RootWorkflow, (*RootState)(nil))
}
