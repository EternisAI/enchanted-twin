package root

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type RootWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// Test_RootWorkflow is the entry point for running the test suite
func Test_RootWorkflow(t *testing.T) {
	suite.Run(t, new(RootWorkflowTestSuite))
}

// Test_RootWorkflow_Basic tests the basic functionality of the root workflow
func (s *RootWorkflowTestSuite) Test_RootWorkflow_Basic() {
	// Skip for now until we have a way to terminate the workflow properly
	s.T().Skip("Workflow tests will be fixed in a future PR - skipping for now")
	// Setup the test environment with a short history cutoff to force fast completion
	env := s.NewTestWorkflowEnvironment()

	// Set a reasonable timeout for the test
	env.SetTestTimeout(1 * time.Second)

	// Mock the environment for the PlanAgentWorkflow
	env.RegisterWorkflow(func(ctx workflow.Context, blueprint []byte, input []byte) error {
		// Mock implementation of PlanAgentWorkflow
		return nil
	})

	// Mock expectations
	// We expect PlanAgentWorkflow to be called when we start an agent
	env.OnWorkflow("PlanAgentWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Mock SignalExternalWorkflow function
	env.OnSignalExternalWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(workflow.Future(nil))

	// Start the root workflow with nil state (will initialize)
	env.ExecuteWorkflow(RootWorkflow, (*RootState)(nil))

	// Note: The workflow is designed to run continuously with ContinueAsNew
	// In a test environment, we're just checking basic functionality
	s.NoError(env.GetWorkflowError())

	// ---- Test Agent Operations ----

	// 1. Test registering an agent
	blueprintJSON := `{"agent":{"id":"test_agent","version":"1.0.0","description":"Test Agent","budget":{"max_steps":100,"max_tokens_total":10000},"llm":{"model":"claude-3-sonnet-20240229"}},"graph":{"nodes":[{"id":"test_node","prompt":"Hello world","spawn":"inline"}],"revision":1},"on_finish":{"emit_signal":{"name":"done","target":"root"}}}`

	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdCreateAgent,
		Args:  map[string]any{ArgAgentID: "test_agent", ArgBlueprint: blueprintJSON},
		CmdID: "cmd1",
	})

	// Query to verify agent was registered
	var agents map[string]*AgentInfo
	result, err := env.QueryWorkflow(QueryListAgents)
	s.NoError(err)
	err = result.Get(&agents)
	s.NoError(err)
	s.Len(agents, 1)
	s.Contains(agents, "test_agent")
	s.Equal("1.0.0", agents["test_agent"].Version)

	// 2. Test starting the agent
	input := map[string]interface{}{"key": "value"}
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdStartAgent,
		Args:  map[string]any{ArgAgentID: "test_agent", ArgInput: input},
		CmdID: "cmd2",
	})

	// Query to check active runs
	var runs map[string]*ChildRunInfo
	result2, err2 := env.QueryWorkflow(QueryListRuns)
	s.NoError(err2)
	err2 = result2.Get(&runs)
	s.NoError(err2)
	s.Len(runs, 1)

	// Get the first run ID (only one exists)
	var runID string
	for id := range runs {
		runID = id
		break
	}
	s.NotEmpty(runID)

	// Verify the agent was started
	env.AssertCalled(s.T(), "PlanAgentWorkflow", mock.Anything, mock.Anything, mock.Anything)

	// 3. Test signaling the agent
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdSignalAgent,
		Args:  map[string]any{ArgRunID: runID, ArgSignal: "test_signal", ArgPayload: map[string]string{"message": "hello"}},
		CmdID: "cmd3",
	})

	// ---- Test Tool Operations ----

	// 4. Test registering a tool
	toolDefJSON := `{
		"name": "test_tool",
		"description": "A test tool",
		"entrypoint": {
			"type": "activity"
		},
		"parameters": {
			"type": "object",
			"properties": {
				"input": {
					"type": "string"
				}
			}
		}
	}`

	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdRegisterTool,
		Args:  map[string]any{ArgToolDef: toolDefJSON},
		CmdID: "cmd4",
	})

	// Query to verify tool was registered
	var tools map[string]types.ToolDef
	result3, err3 := env.QueryWorkflow(QueryListTools)
	s.NoError(err3)
	err3 = result3.Get(&tools)
	s.NoError(err3)
	s.Len(tools, 1)
	s.Contains(tools, "test_tool")

	// 5. Test deregistering a tool
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdDeregisterTool,
		Args:  map[string]any{ArgToolName: "test_tool"},
		CmdID: "cmd5",
	})

	// Query to verify tool was deregistered
	result4, err4 := env.QueryWorkflow(QueryListTools)
	s.NoError(err4)
	err4 = result4.Get(&tools)
	s.NoError(err4)
	s.Len(tools, 0)

	// 6. Test deleting an agent
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdDeleteAgent,
		Args:  map[string]any{ArgAgentID: "test_agent"},
		CmdID: "cmd6",
	})

	// This should fail since the agent has active runs
	result5, err5 := env.QueryWorkflow(QueryListAgents)
	s.NoError(err5)
	err5 = result5.Get(&agents)
	s.NoError(err5)
	s.Len(agents, 1) // Still there because it has active runs
}

// Test_RootWorkflow_ErrorCases tests error handling in the root workflow
func (s *RootWorkflowTestSuite) Test_RootWorkflow_ErrorCases() {
	// Skip for now until we have a way to terminate the workflow properly
	s.T().Skip("Workflow tests will be fixed in a future PR - skipping for now")
	// Setup the test environment
	env := s.NewTestWorkflowEnvironment()

	// Set a reasonable timeout for the test
	env.SetTestTimeout(1 * time.Second)

	// Register mock PlanAgentWorkflow
	env.RegisterWorkflow(func(ctx workflow.Context, blueprint []byte, input []byte) error {
		return nil
	})

	// Mock SignalExternalWorkflow function
	env.OnSignalExternalWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(workflow.Future(nil))

	// Start the root workflow
	env.ExecuteWorkflow(RootWorkflow, (*RootState)(nil))

	// Verify workflow started without errors
	s.NoError(env.GetWorkflowError())

	// Test invalid blueprint
	invalidJSON := `{this is not valid json}`
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdCreateAgent,
		Args:  map[string]any{ArgAgentID: "bad_agent", ArgBlueprint: invalidJSON},
		CmdID: "cmd1",
	})

	// Verify agent wasn't registered
	var agents map[string]*AgentInfo
	result, err := env.QueryWorkflow(QueryListAgents)
	s.NoError(err)
	err = result.Get(&agents)
	s.NoError(err)
	s.Len(agents, 0)

	// Test missing required arguments
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdStartAgent,
		Args:  map[string]any{}, // Missing agent_id
		CmdID: "cmd2",
	})

	// Verify no runs were created
	var runs map[string]*ChildRunInfo
	result2, err2 := env.QueryWorkflow(QueryListRuns)
	s.NoError(err2)
	err2 = result2.Get(&runs)
	s.NoError(err2)
	s.Len(runs, 0)

	// Test duplicate command IDs (idempotency)
	blueprintJSON := `{"agent":{"id":"test_agent","version":"1.0.0","description":"Test Agent","budget":{"max_steps":100,"max_tokens_total":10000},"llm":{"model":"claude-3-sonnet-20240229"}},"graph":{"nodes":[{"id":"test_node","prompt":"Hello world","spawn":"inline"}],"revision":1},"on_finish":{"emit_signal":{"name":"done","target":"root"}}}`

	// First command execution
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdCreateAgent,
		Args:  map[string]any{ArgAgentID: "dup_agent", ArgBlueprint: blueprintJSON},
		CmdID: "dup_cmd",
	})

	// Duplicate command with same ID
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdCreateAgent,
		Args:  map[string]any{ArgAgentID: "another_agent", ArgBlueprint: blueprintJSON},
		CmdID: "dup_cmd", // Same ID
	})

	// Verify only the first agent was registered
	result3, err3 := env.QueryWorkflow(QueryListAgents)
	s.NoError(err3)
	err3 = result3.Get(&agents)
	s.NoError(err3)
	s.Len(agents, 1)
	s.Contains(agents, "dup_agent")
	s.NotContains(agents, "another_agent")
}

// Test_RootWorkflow_ContinueAsNew tests the ContinueAsNew functionality
func (s *RootWorkflowTestSuite) Test_RootWorkflow_ContinueAsNew() {
	// This test requires modifying the historyCutoff constant to force ContinueAsNew
	// We'll skip it for now as it would require a more complex test setup
	s.T().Skip("ContinueAsNew test requires more sophisticated test setup - deferred")
}

// Test_RootWorkflow_MessageQueue tests failing message queue operations
// which will be implemented in the next PR
func (s *RootWorkflowTestSuite) Test_RootWorkflow_MessageQueue() {
	s.T().Skip("MessageQueue not yet implemented - will be completed in PR 0-B")

	// This test will fail until MessageQueue is implemented
	// Placeholder for next PR - will implement:
	// - Test message queue append/pop operations
	// - Test deterministic replay safety
}

// Test_RootWorkflow_StopWorkflow tests the ability to stop the workflow with the special command
func (s *RootWorkflowTestSuite) Test_RootWorkflow_StopWorkflow() {
	// Setup the test environment
	env := s.NewTestWorkflowEnvironment()

	// Set a 2-second timeout for the test
	env.SetTestTimeout(2 * time.Second)

	// Register mock PlanAgentWorkflow
	env.RegisterWorkflow(func(ctx workflow.Context, blueprint []byte, input []byte) error {
		return nil
	})

	// Start the root workflow with nil state (will initialize)
	env.ExecuteWorkflow(RootWorkflow, (*RootState)(nil))

	// Send the stop workflow command immediately
	env.SignalWorkflow(SignalCommand, Command{
		Cmd:   CmdStopWorkflow,
		Args:  map[string]any{},
		CmdID: "stop_cmd",
	})

	// Workflow should have panicked, which Temporal test env converts to an error
	// We expect an error containing our panic message
	err := env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "Test-only workflow termination")
}
