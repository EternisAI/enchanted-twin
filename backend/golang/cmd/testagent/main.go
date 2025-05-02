package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	plannedv2 "github.com/EternisAI/enchanted-twin/pkg/agent/planned-v2"
	"github.com/EternisAI/enchanted-twin/pkg/agent/root"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
)

var (
	commandFlag   = flag.String("cmd", "", "Command to send to the agent (create_agent, start_agent, signal_agent, list_agents, list_runs, get_agent, stop, test_planned)")
	agentIDFlag   = flag.String("agent-id", "", "Agent ID for the command")
	blueprintFlag = flag.String("blueprint", "", "Path to blueprint JSON file (for create_agent)")
	runIDFlag     = flag.String("run-id", "", "Run ID (for signal_agent)")
	signalFlag    = flag.String("signal", "", "Signal name (for signal_agent)")
	payloadFlag   = flag.String("payload", "{}", "JSON payload (for signal_agent or start_agent)")
	testPlanFlag  = flag.String("plan", "1. Say hello.\n2. Tell me a hilarious joke about cats\n3. Wait 5 seconds so I can catch my breath.\n4. Tell me a hilarious joke about dogs", "Plan for the test-planned command")
)

func main() {
	flag.Parse()

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Stamp,
	})

	// Load configuration
	envs, _ := config.LoadConfig(true)
	logger.Debug("Config loaded", "envs", envs)

	// Start temporal server
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, envs.DBPath)
	<-ready
	logger.Info("Temporal server started")

	// Create temporal client
	temporalClient, err := bootstrap.CreateTemporalClient("localhost:7233", bootstrap.TemporalNamespace, "")
	if err != nil {
		logger.Error("Unable to create temporal client", "error", err)
		os.Exit(1)
	}
	logger.Info("Temporal client created")

	// Start worker
	w := worker.New(temporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 1,
	})

	aiService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)

	// Register workflows
	registry := tools.GetGlobal(logger)
	agentActivities := plannedv2.NewAgentActivities(context.Background(), aiService, registry)
	w.RegisterWorkflow(plannedv2.PlannedAgentWorkflow)
	agentActivities.RegisterPlannedAgentWorkflow(w, logger)
	logger.Info("Registered PlannedV2 Workflows and Activities")

	w.RegisterWorkflow(root.RootWorkflow)
	logger.Info("Registered RootWorkflow")

	// Start worker
	err = w.Start()
	if err != nil {
		logger.Error("Error starting worker", "error", err)
		os.Exit(1)
	}
	defer w.Stop()

	// Ensure root workflow is running
	// rootRun, err := root.EnsureRootWorkflow(temporalClient)
	// if err != nil {
	// 	logger.Error("Failed to ensure root workflow is running", "error", err)
	// 	os.Exit(1)
	// }
	// logger.Info("Root workflow is running", "id", rootRun.GetID(), "run_id", rootRun.GetRunID())

	// Handle command
	switch *commandFlag {
	case "create_agent":
		err = handleCreateAgent(temporalClient, logger)
	case "start_agent":
		err = handleStartAgent(temporalClient, logger)
	case "signal_agent":
		err = handleSignalAgent(temporalClient, logger)
	case "list_agents":
		err = handleListAgents(temporalClient, logger)
	case "list_runs":
		err = handleListRuns(temporalClient, logger)
	case "get_agent":
		if *agentIDFlag == "" {
			logger.Error("agent-id is required for get_agent")
			os.Exit(1)
		}
		err = handleGetAgent(temporalClient, *agentIDFlag, logger)
	case "test_planned":
		err = handleTestPlanned(temporalClient, logger, envs)
	case "stop":
		err = handleStopWorkflow(temporalClient, logger)
	case "":
		// If no command specified, start interactive mode
		err = interactiveMode(temporalClient, logger)
	default:
		logger.Error("Unknown command", "cmd", *commandFlag)
		os.Exit(1)
	}

	if err != nil {
		logger.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func handleCreateAgent(c client.Client, logger *log.Logger) error {
	if *agentIDFlag == "" {
		return fmt.Errorf("agent-id is required")
	}
	if *blueprintFlag == "" {
		return fmt.Errorf("blueprint is required")
	}

	// Read blueprint file
	blueprint, err := os.ReadFile(*blueprintFlag)
	if err != nil {
		return fmt.Errorf("failed to read blueprint file: %w", err)
	}

	// Signal root workflow to create agent
	command := root.Command{
		Cmd: root.CmdCreateAgent,
		Args: map[string]any{
			root.ArgAgentID:   *agentIDFlag,
			root.ArgBlueprint: string(blueprint),
		},
		CmdID: fmt.Sprintf("create_agent_%d", time.Now().UnixNano()),
	}

	ctx := context.Background()
	err = c.SignalWorkflow(ctx, root.RootWorkflowID, "", root.SignalCommand, command)
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	logger.Info("Agent created successfully", "id", *agentIDFlag)
	return nil
}

func handleStartAgent(c client.Client, logger *log.Logger) error {
	if *agentIDFlag == "" {
		return fmt.Errorf("agent-id is required")
	}

	// Parse input payload
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(*payloadFlag), &input); err != nil {
		return fmt.Errorf("invalid input payload JSON: %w", err)
	}

	// Signal root workflow to start agent
	command := root.Command{
		Cmd: root.CmdStartAgent,
		Args: map[string]any{
			root.ArgAgentID: *agentIDFlag,
			root.ArgInput:   input,
		},
		CmdID: fmt.Sprintf("start_agent_%d", time.Now().UnixNano()),
	}

	ctx := context.Background()
	err := c.SignalWorkflow(ctx, root.RootWorkflowID, "", root.SignalCommand, command)
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	logger.Info("Agent start request sent", "id", *agentIDFlag)

	// Wait a moment for the agent to start
	time.Sleep(500 * time.Millisecond)

	// Query for active runs to find the run ID
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryListRuns)
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var runs map[string]*root.ChildRunInfo
	if err := resp.Get(&runs); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Find the run for this agent
	found := false
	for id, run := range runs {
		if run.AgentID == *agentIDFlag {
			logger.Info("Agent started", "agent_id", *agentIDFlag, "run_id", id, "started", run.Started)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("agent started but run not found in active runs")
	}

	return nil
}

func handleSignalAgent(c client.Client, logger *log.Logger) error {
	if *runIDFlag == "" {
		return fmt.Errorf("run-id is required")
	}
	if *signalFlag == "" {
		return fmt.Errorf("signal is required")
	}

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(*payloadFlag), &payload); err != nil {
		return fmt.Errorf("invalid payload JSON: %w", err)
	}

	// Signal root workflow to signal agent
	command := root.Command{
		Cmd: root.CmdSignalAgent,
		Args: map[string]any{
			root.ArgRunID:   *runIDFlag,
			root.ArgSignal:  *signalFlag,
			root.ArgPayload: payload,
		},
		CmdID: fmt.Sprintf("signal_agent_%d", time.Now().UnixNano()),
	}

	ctx := context.Background()
	err := c.SignalWorkflow(ctx, root.RootWorkflowID, "", root.SignalCommand, command)
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	logger.Info("Signal sent to agent", "run_id", *runIDFlag, "signal", *signalFlag)
	return nil
}

func handleListAgents(c client.Client, logger *log.Logger) error {
	ctx := context.Background()
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryListAgents, nil)
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var agents map[string]*root.AgentInfo
	if err := resp.Get(&agents); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Registered agents:", "count", len(agents))
	for id, agent := range agents {
		logger.Info("Agent", "id", id, "version", agent.Version, "created_at", agent.CreatedAt)
	}

	return nil
}

func handleGetAgent(c client.Client, agentID string, logger *log.Logger) error {
	ctx := context.Background()
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryGetAgent, map[string]string{
		"agent_id": agentID,
	})
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var agent *root.AgentInfo
	if err := resp.Get(&agent); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Agent details:",
		"id", agent.AgentID,
		"version", agent.Version,
		"created_at", agent.CreatedAt,
		"blueprint_size", len(agent.Blueprint))

	return nil
}

func handleListRuns(c client.Client, logger *log.Logger) error {
	ctx := context.Background()
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryListRuns, nil)
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var runs map[string]*root.ChildRunInfo
	if err := resp.Get(&runs); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Active runs:", "count", len(runs))
	for id, run := range runs {
		logger.Info("Run", "id", id, "agent_id", run.AgentID, "started", run.Started, "status", run.Status)
	}

	return nil
}

func handleListTools(c client.Client, logger *log.Logger) error {
	ctx := context.Background()
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryListTools, nil)
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var tools map[string]types.ToolDef
	if err := resp.Get(&tools); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Registered tools:", "count", len(tools))
	for name, tool := range tools {
		logger.Info("Tool",
			"name", name,
			"type", tool.Entrypoint.Type,
			"description", tool.Description)
	}

	return nil
}

func handleGetState(c client.Client, logger *log.Logger) error {
	ctx := context.Background()
	resp, err := c.QueryWorkflow(ctx, root.RootWorkflowID, "", root.QueryGetState, nil)
	if err != nil {
		return fmt.Errorf("failed to query workflow: %w", err)
	}

	var state root.RootState
	if err := resp.Get(&state); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Debug("Full state:",
		"agents", len(state.Registry),
		"runs", len(state.ActiveRuns),
		"tools", len(state.Tools),
		"seen_cmds", len(state.SeenCmdIDs),
		"shutdown_requested", state.ShutdownRequested)

	return nil
}

func handleStopWorkflow(c client.Client, logger *log.Logger) error {
	// Signal root workflow to stop
	command := root.Command{
		Cmd:   root.CmdStopWorkflow,
		Args:  map[string]any{},
		CmdID: fmt.Sprintf("stop_workflow_%d", time.Now().UnixNano()),
	}

	ctx := context.Background()
	err := c.SignalWorkflow(ctx, root.RootWorkflowID, "", root.SignalCommand, command)
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	logger.Info("Stop signal sent to root workflow")

	// Wait for a moment to allow the workflow to process the shutdown signal
	logger.Info("Waiting for workflow to gracefully shut down...")
	time.Sleep(2 * time.Second)

	// Verify the workflow status
	desc, err := c.DescribeWorkflowExecution(ctx, root.RootWorkflowID, "")
	if err != nil {
		logger.Warn("Unable to verify workflow status", "error", err)
	} else {
		status := desc.WorkflowExecutionInfo.Status
		logger.Info("Workflow status", "status", status.String())
	}

	return nil
}

// handleTestPlanned tests the planned agent workflow
func handleTestPlanned(c client.Client, logger *log.Logger, envs *config.Config) error {
	// Get the selected tool
	logger.Info("Testing planned agent", "plan", *testPlanFlag)

	// Create unique workflow ID
	workflowID := fmt.Sprintf("planned-test-%s", uuid.New().String())

	// Create input based on selected tool
	// Use the model from config
	model := "gpt-4o" // Default to Claude
	if envs != nil && envs.CompletionsModel != "" {
		model = envs.CompletionsModel
	}

	logger.Info("Using model", "model", model)

	input := plannedv2.PlanInput{
		Plan: *testPlanFlag,
		// ToolNames: []string{"echo", "sleep"},  // TODO: add flag for these
		Model:    model,
		MaxSteps: 100,
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	// Start the workflow
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	workflowOptions := client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                "default",
		WorkflowExecutionTimeout: 120 * time.Second,
		WorkflowTaskTimeout:      10 * time.Second,
	}

	logger.Info("Starting workflow", "id", workflowID)
	we, err := c.ExecuteWorkflow(ctx, workflowOptions, plannedv2.PlannedAgentWorkflow, inputJSON)
	if err != nil {
		return fmt.Errorf("failed to start workflow: %w", err)
	}

	logger.Info("Workflow started", "id", we.GetID(), "run_id", we.GetRunID())

	// Wait for completion
	var result interface{}
	logger.Info("Waiting for workflow to complete...")
	if err := we.Get(ctx, &result); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	// Query state
	resp, err := c.QueryWorkflow(ctx, workflowID, we.GetRunID(), plannedv2.QueryGetState, nil)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	var state plannedv2.PlanState
	if err := resp.Get(&state); err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	// Display results
	logger.Info("Workflow completed successfully", "output", state.Output, "steps", state.CurrentStep)

	// Display history
	logger.Info("Workflow history:")
	for i, entry := range state.History {
		logger.Info(fmt.Sprintf("%d. [%s] %s", i+1, entry.Type, entry.Content))
	}

	return nil
}

func interactiveMode(c client.Client, logger *log.Logger) error {
	logger.Info("Starting interactive mode. Press Ctrl+C to exit.")

	// Set up signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Display initial state
	if err := handleListAgents(c, logger); err != nil {
		logger.Warn("Failed to list agents", "error", err)
	}

	if err := handleListRuns(c, logger); err != nil {
		logger.Warn("Failed to list runs", "error", err)
	}

	if err := handleListTools(c, logger); err != nil {
		logger.Warn("Failed to list tools", "error", err)
	}

	logger.Info("Interactive mode is running. Polling for state updates every 5 seconds. Press Ctrl+C to exit.")

	// Set up ticker for periodic updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Create a context that will be canceled on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to handle the signal
	go func() {
		<-sigs
		logger.Info("Shutting down...")
		cancel()
	}()

	// Poll periodically until context is canceled
	for {
		select {
		case <-ctx.Done():
			// Exit the loop when context is canceled
			return nil
		case <-ticker.C:
			// Fetch the latest state
			logger.Info("--- Polling root workflow state ---")

			// Query for registered agents
			if err := handleListAgents(c, logger); err != nil {
				logger.Warn("Failed to list agents", "error", err)
			}

			// Query for active runs
			if err := handleListRuns(c, logger); err != nil {
				logger.Warn("Failed to list runs", "error", err)
			}

			// Query for tools
			if err := handleListTools(c, logger); err != nil {
				logger.Warn("Failed to list tools", "error", err)
			}

			// Query for full state (debug info)
			if err := handleGetState(c, logger); err != nil {
				logger.Warn("Failed to get full state", "error", err)
			}

			logger.Info("--- End of state update ---")
		}
	}
}
