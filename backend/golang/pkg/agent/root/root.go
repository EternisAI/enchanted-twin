package root

// Root workflow with autowiring.
// Every command/query is implemented as a method of *RootHandlers and
// documented with `@tool:` tags. At startup we reflect over the methods to:
//   1. push a ToolDef into st.Tools (so other agents can `uses: ["<name>"]`)
//   2. register a Temporal Query or route a Signal.
// This keeps code and tool metadata in one place.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

/*───────────────────────────────────────────────────────────────────────────*
| 1. Constants & Arg names                                                 |
*───────────────────────────────────────────────────────────────────────────*/

const (
	heartbeat       = 5 * time.Minute
	historyEventCut = 1024 * 1024 // 1MB
)

/*───────────────────────────────────────────────────────────────────────────*
| 2. Domain structs                                                        |
*───────────────────────────────────────────────────────────────────────────*/

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
	Registry          map[string]*AgentInfo
	ActiveRuns        map[string]*ChildRunInfo
	Tools             map[string]types.ToolDef
	SeenCmdIDs        map[string]struct{}
	ShutdownRequested bool // Flag to indicate a graceful shutdown request
}

/*───────────────────────────────────────────────────────────────────────────*
| 3. Handler receiver – methods tagged as @tool                           |
*───────────────────────────────────────────────────────────────────────────*/

type RootHandlers struct{ st *RootState }

// ---------- Queries ----------

// QueryGetState returns the entire root state (for debugging).
func (h *RootHandlers) QueryGetState() (RootState, error) { return *h.st, nil }

// QueryListAgents returns a map of all registered agents.
func (h *RootHandlers) QueryListAgents() (map[string]*AgentInfo, error) {
	return h.st.Registry, nil
}

// QueryGetAgent returns metadata for a specific agent.
func (h *RootHandlers) QueryGetAgent(agentID string) (*AgentInfo, error) {
	a, ok := h.st.Registry[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return a, nil
}

// QueryListRuns returns a map of all active workflow runs.
func (h *RootHandlers) QueryListRuns() (map[string]*ChildRunInfo, error) {
	return h.st.ActiveRuns, nil
}

// QueryListTools returns a map of all registered tools.
func (h *RootHandlers) QueryListTools() (map[string]types.ToolDef, error) {
	return h.st.Tools, nil
}

// ---------- Commands (Signal handlers) ----------

// CmdCreateAgent registers a new agent blueprint.
func (h *RootHandlers) CmdCreateAgent(ctx workflow.Context, args struct {
	AgentID   string `json:"agent_id"`
	Blueprint string `json:"blueprint"`
},
) error {
	if _, dup := h.st.Registry[args.AgentID]; dup {
		return nil
	}
	h.st.Registry[args.AgentID] = &AgentInfo{
		AgentID:   args.AgentID,
		Version:   "0.1.0",
		Blueprint: []byte(args.Blueprint),
		CreatedAt: workflow.Now(ctx),
	}
	return nil
}

// CmdDeleteAgent removes an agent (fails if the agent has active runs).
func (h *RootHandlers) CmdDeleteAgent(ctx workflow.Context, args struct {
	AgentID string `json:"agent_id"`
},
) error {
	for _, r := range h.st.ActiveRuns {
		if r.AgentID == args.AgentID {
			return fmt.Errorf("agent has active runs")
		}
	}
	delete(h.st.Registry, args.AgentID)
	return nil
}

// CmdStartAgent creates a new workflow run of a registered agent.
func (h *RootHandlers) CmdStartAgent(ctx workflow.Context, args struct {
	AgentID string `json:"agent_id"`
	Input   any    `json:"input"`
},
) (string, error) {
	ai, ok := h.st.Registry[args.AgentID]
	if !ok {
		return "", fmt.Errorf("agent not found")
	}
	inputJSON, _ := json.Marshal(args.Input)
	childCtx := workflow.WithChildOptions(
		ctx,
		workflow.ChildWorkflowOptions{TaskQueue: WorkflowTaskQueue},
	)
	var runID string
	fut := workflow.ExecuteChildWorkflow(childCtx, "PlanAgentWorkflow", ai.Blueprint, inputJSON)
	if err := fut.GetChildWorkflowExecution().Get(ctx, &runID); err != nil {
		return "", err
	}
	h.st.ActiveRuns[runID] = &ChildRunInfo{
		RunID:   runID,
		AgentID: args.AgentID,
		Started: workflow.Now(ctx),
		Status:  StatusRunning,
	}
	return runID, nil
}

// CmdSignalAgent forwards a signal to a running agent workflow.
func (h *RootHandlers) CmdSignalAgent(ctx workflow.Context, args struct {
	RunID   string `json:"run_id"`
	Signal  string `json:"signal"`
	Payload any    `json:"payload"`
},
) error {
	return workflow.SignalExternalWorkflow(ctx, args.RunID, "", args.Signal, args.Payload).
		Get(ctx, nil)
}

// CmdRegisterTool adds a global tool definition.
func (h *RootHandlers) CmdRegisterTool(_ workflow.Context, args struct {
	ToolDef string `json:"tool_def"`
},
) error {
	var td types.ToolDef
	if err := json.Unmarshal([]byte(args.ToolDef), &td); err != nil {
		return err
	}
	h.st.Tools[td.Name] = td
	return nil
}

// CmdDeregisterTool removes a global tool definition.
func (h *RootHandlers) CmdDeregisterTool(_ workflow.Context, args struct {
	ToolName string `json:"tool_name"`
},
) error {
	delete(h.st.Tools, args.ToolName)
	return nil
}

/*───────────────────────────────────────────────────────────────────────────*
| 4. Root workflow implementation                                          |
*───────────────────────────────────────────────────────────────────────────*/

func RootWorkflow(ctx workflow.Context, prev *RootState) error {
	st := prev
	if st == nil {
		st = &RootState{
			Registry:          map[string]*AgentInfo{},
			ActiveRuns:        map[string]*ChildRunInfo{},
			Tools:             map[string]types.ToolDef{},
			SeenCmdIDs:        map[string]struct{}{},
			ShutdownRequested: false,
		}
	}
	h := &RootHandlers{st: st}
	if err := autoWire(ctx, h); err != nil {
		return err
	}

	cmdCh := workflow.GetSignalChannel(ctx, SignalCommand)

	for {
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(cmdCh, func(c workflow.ReceiveChannel, ok bool) {
			var cmd Command
			c.Receive(ctx, &cmd)

			// Special handling for stop workflow command - not exposed as a tool
			if cmd.Cmd == CmdStopWorkflow {
				logger := workflow.GetLogger(ctx)
				logger.Info("Stopping workflow by request")

				// Set graceful shutdown flag in state
				st.ShutdownRequested = true
			}

			if _, dup := st.SeenCmdIDs[cmd.CmdID]; !dup {
				st.SeenCmdIDs[cmd.CmdID] = struct{}{}
				dispatch(cmd, ctx, h)
			}
		})
		sel.AddFuture(workflow.NewTimer(ctx, heartbeat), func(workflow.Future) {})
		sel.Select(ctx)

		// Check if shutdown was requested via command
		if st.ShutdownRequested {
			logger := workflow.GetLogger(ctx)
			logger.Info("Graceful shutdown requested, saving state and exiting")
			return workflow.NewContinueAsNewError(ctx, RootWorkflow, st)
		}

		info := workflow.GetInfo(ctx)
		if info.GetCurrentHistorySize() > historyEventCut {
			return workflow.NewContinueAsNewError(ctx, RootWorkflow, st)
		}
	}
}

/*───────────────────────────────────────────────────────────────────────────*
| 5. Reflection & dispatch utilities                                       |
*───────────────────────────────────────────────────────────────────────────*/

// ToolReg represents a tool registration with its metadata and handler.
type ToolReg struct {
	Tool  types.ToolDef
	Query func(*RootHandlers, any) (any, error)
	Cmd   func(*RootHandlers, workflow.Context, any) error
}

// RegisteredTools returns the list of tool registrations.
func (h *RootHandlers) RegisteredTools() []ToolReg {
	emptyObj := types.JSONSchema{"type": "object"}
	emptyParams := types.JSONSchema{"type": "object", "properties": map[string]any{}}

	return []ToolReg{
		// Queries
		{
			Tool: types.ToolDef{
				Name:        "get_state",
				Description: "Return entire root state (debug-only)",
				Parameters:  emptyParams,
				Returns:     emptyObj,
				Entrypoint:  types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeQuery},
			},
			Query: func(h *RootHandlers, _ any) (any, error) {
				return h.QueryGetState()
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "list_agents",
				Description: "List registered agents",
				Parameters:  emptyParams,
				Returns:     emptyObj,
				Entrypoint:  types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeQuery},
			},
			Query: func(h *RootHandlers, _ any) (any, error) {
				return h.QueryListAgents()
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "get_agent",
				Description: "Get metadata for one agent",
				Parameters: types.JSONSchema{
					"type":       "object",
					"properties": map[string]any{"agent_id": map[string]any{"type": "string"}},
				},
				Returns:    emptyObj,
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeQuery},
			},
			Query: func(h *RootHandlers, args any) (any, error) {
				m, ok := args.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid args")
				}

				agentID, ok := m["agent_id"].(string)
				if !ok {
					return nil, fmt.Errorf("agent_id parameter is required")
				}

				return h.QueryGetAgent(agentID)
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "list_runs",
				Description: "List active child workflow runs",
				Parameters:  emptyParams,
				Returns:     emptyObj,
				Entrypoint:  types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeQuery},
			},
			Query: func(h *RootHandlers, _ any) (any, error) {
				return h.QueryListRuns()
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "list_tools",
				Description: "List globally registered tools",
				Parameters:  emptyParams,
				Returns:     emptyObj,
				Entrypoint:  types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeQuery},
			},
			Query: func(h *RootHandlers, _ any) (any, error) {
				return h.QueryListTools()
			},
		},

		// Commands
		{
			Tool: types.ToolDef{
				Name:        "create_agent",
				Description: "Register a new agent blueprint",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"agent_id":  map[string]any{"type": "string"},
					"blueprint": map[string]any{"type": "string"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				agentID, _ := m["agent_id"].(string)
				blueprint, _ := m["blueprint"].(string)

				return h.CmdCreateAgent(ctx, struct {
					AgentID   string `json:"agent_id"`
					Blueprint string `json:"blueprint"`
				}{
					AgentID:   agentID,
					Blueprint: blueprint,
				})
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "delete_agent",
				Description: "Delete an agent (must have no active runs)",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"agent_id": map[string]any{"type": "string"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				agentID, _ := m["agent_id"].(string)

				return h.CmdDeleteAgent(ctx, struct {
					AgentID string `json:"agent_id"`
				}{
					AgentID: agentID,
				})
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "start_agent",
				Description: "Start a workflow run of a registered agent",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"agent_id": map[string]any{"type": "string"},
					"input":    map[string]any{"type": "object"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
				Returns: types.JSONSchema{
					"type":       "object",
					"properties": map[string]any{"run_id": map[string]any{"type": "string"}},
				},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				agentID, _ := m["agent_id"].(string)
				input := m["input"]

				runID, err := h.CmdStartAgent(ctx, struct {
					AgentID string `json:"agent_id"`
					Input   any    `json:"input"`
				}{
					AgentID: agentID,
					Input:   input,
				})

				logger := workflow.GetLogger(ctx)
				if err != nil {
					logger.Error("Failed to start agent", "agent_id", agentID, "error", err)
				} else {
					logger.Info("Started agent", "agent_id", agentID, "run_id", runID)
				}

				return err
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "signal_agent",
				Description: "Forward a signal to a running agent workflow",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"run_id":  map[string]any{"type": "string"},
					"signal":  map[string]any{"type": "string"},
					"payload": map[string]any{"type": "object"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				runID, _ := m["run_id"].(string)
				signal, _ := m["signal"].(string)
				payload := m["payload"]

				return h.CmdSignalAgent(ctx, struct {
					RunID   string `json:"run_id"`
					Signal  string `json:"signal"`
					Payload any    `json:"payload"`
				}{
					RunID:   runID,
					Signal:  signal,
					Payload: payload,
				})
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "register_tool",
				Description: "Add a global ToolDef",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"tool_def": map[string]any{"type": "string"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				toolDef, _ := m["tool_def"].(string)

				return h.CmdRegisterTool(ctx, struct {
					ToolDef string `json:"tool_def"`
				}{
					ToolDef: toolDef,
				})
			},
		},
		{
			Tool: types.ToolDef{
				Name:        "deregister_tool",
				Description: "Remove a global tool",
				Parameters: types.JSONSchema{"type": "object", "properties": map[string]any{
					"tool_name": map[string]any{"type": "string"},
				}},
				Entrypoint: types.ToolDefEntrypoint{Type: types.ToolDefEntrypointTypeSignal},
			},
			Cmd: func(h *RootHandlers, ctx workflow.Context, args any) error {
				m, ok := args.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid args")
				}

				toolName, _ := m["tool_name"].(string)

				return h.CmdDeregisterTool(ctx, struct {
					ToolName string `json:"tool_name"`
				}{
					ToolName: toolName,
				})
			},
		},
	}
}

func autoWire(ctx workflow.Context, h *RootHandlers) error {
	// Get tool registrations
	toolRegs := h.RegisteredTools()

	logger := workflow.GetLogger(ctx)
	logger.Info("Registering root workflow tools", "count", len(toolRegs))

	// Register each tool
	for _, reg := range toolRegs {
		// Add to tool registry
		h.st.Tools[reg.Tool.Name] = reg.Tool
		logger.Info("Registered tool", "name", reg.Tool.Name, "type", reg.Tool.Entrypoint.Type)

		// Register handler based on type
		if reg.Query != nil {
			// Register query handler
			queryName := reg.Tool.Name
			if err := workflow.SetQueryHandler(ctx, queryName, func(args any) (any, error) {
				return reg.Query(h, args)
			}); err != nil {
				logger.Error("Failed to set query handler", "name", queryName, "error", err)
				return err
			}
		}

		if reg.Cmd != nil {
			// Register command handler
			signalDispatch[reg.Tool.Name] = reg.Cmd
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Signal dispatch table (populated at runtime)
// -----------------------------------------------------------------------------

var signalDispatch = map[string]func(*RootHandlers, workflow.Context, any) error{}

func dispatch(c Command, ctx workflow.Context, h *RootHandlers) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Dispatching command", "cmd", c.Cmd, "cmd_id", c.CmdID)

	if f, ok := signalDispatch[c.Cmd]; ok {
		err := f(h, ctx, c.Args)
		if err != nil {
			logger.Error("Command execution failed", "cmd", c.Cmd, "cmd_id", c.CmdID, "error", err)
		} else {
			logger.Info("Command executed successfully", "cmd", c.Cmd, "cmd_id", c.CmdID)
		}
	} else {
		logger.Error("Unknown command", "cmd", c.Cmd, "cmd_id", c.CmdID)
	}
}

// -----------------------------------------------------------------------------
// EnsureRootWorkflow
// -----------------------------------------------------------------------------

func EnsureRootWorkflow(c client.Client) (client.WorkflowRun, error) {
	ctx := context.Background()
	if desc, err := c.DescribeWorkflowExecution(ctx, RootWorkflowID, ""); err != nil {
		fmt.Printf("error reading workflow execution: %v", err)
	} else {
		fmt.Printf("root workflow already running in state: %v", desc.GetWorkflowExecutionInfo().GetStatus())
		// return c.GetWorkflow(ctx, RootWorkflowID, ""), nil
	}
	opts := client.StartWorkflowOptions{
		ID:                 RootWorkflowID,
		TaskQueue:          WorkflowTaskQueue,
		WorkflowRunTimeout: 365 * 24 * time.Hour,
	}
	return c.ExecuteWorkflow(ctx, opts, RootWorkflow, (*RootState)(nil))
}
