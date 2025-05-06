package plannedv2

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type PlannedAgentTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	activities *AgentActivities
}

func (s *PlannedAgentTestSuite) SetupTest() {
	registry := tools.NewRegistry()
	s.activities = NewAgentActivities(context.Background(), &ai.Service{}, registry)
}

// TestPlannedAgentWorkflow is the entry point for running the test suite.
func TestPlannedAgentWorkflow(t *testing.T) {
	suite.Run(t, new(PlannedAgentTestSuite))
}

// TestBasicPlanExecution tests the basic execution of a plan.
func (s *PlannedAgentTestSuite) TestBasicPlanExecution() {
	t := s.T()
	t.Skip("Skipping test")
	env := s.NewTestWorkflowEnvironment()

	// Set up test timeout
	env.SetTestTimeout(10 * time.Second)

	// Set up mocks
	mockCounter := 0

	// Mock LLM completion activity with a callback function
	env.OnActivity(s.activities.LLMCompletionActivity,
		mustMatchAny, mustMatchAny, mustMatchAny, mustMatchAny).
		Return(func(ctx interface{}, messages interface{}, tools interface{}, model string) (openai.ChatCompletionMessage, error) {
			mockCounter++

			// First call - return a tool call
			if mockCounter == 1 {
				return openai.ChatCompletionMessage{
					Content: "I'll help you execute this plan. First, let me echo the plan.",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "echo",
								Arguments: `{"text": "Starting execution of the plan"}`,
							},
						},
					},
				}, nil
			}

			// Second call - final response
			return openai.ChatCompletionMessage{
				Content: "I've completed the plan successfully!",
			}, nil
		})

	// TODO: Mock execute tool activity
	// env.OnActivity(s.activities.ExecuteToolActivity, "echo", mustMatchAny).Return(
	// 	&types.ToolResult{
	// 		Tool:    "echo",
	// 		Content: "Echo: Starting execution of the plan",
	// 		Data:    "Echo: Starting execution of the plan",
	// 	}, nil)

	// Create sample input
	input := PlanInput{
		Plan:      "1. Check the weather. 2. Calculate 2+3. 3. Summarize findings.",
		ToolNames: []string{"echo", "math"},
		Model:     "test-model",
		MaxSteps:  3,
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	s.NoError(err)

	// Start the workflow
	env.ExecuteWorkflow(PlannedAgentWorkflow, inputJSON)

	// Check that the workflow completed without error
	s.NoError(env.GetWorkflowError())

	// Query for state
	var state PlanState
	queryResult, err := env.QueryWorkflow(QueryGetState)
	s.NoError(err)
	s.NoError(queryResult.Get(&state))

	// Verify state
	s.NotEmpty(state.History)
	s.Equal(input.Plan, state.Plan)
	s.False(state.CompletedAt.IsZero())
	s.Equal("I've completed the plan successfully!", state.Output)
}

// TestSleepTool tests the sleep tool functionality.
func (s *PlannedAgentTestSuite) TestSleepTool() {
	t := s.T()
	t.Skip("Skipping test")
	env := s.NewTestWorkflowEnvironment()

	// Set a generous timeout for the test
	env.SetTestTimeout(10 * time.Second)

	// Mock LLM completion activities to return sleep tool calls
	mockCalls := 0
	env.OnActivity(s.activities.LLMCompletionActivity,
		mustMatchAny, mustMatchAny, mustMatchAny, mustMatchAny).
		Return(func(ctx interface{}, messages interface{}, tools interface{}, model string) (openai.ChatCompletionMessage, error) {
			mockCalls++

			// First call: return sleep tool call
			if mockCalls == 1 {
				return openai.ChatCompletionMessage{
					Content: "I'll wait for 2 seconds before proceeding.",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID:   "sleep_call",
							Type: "function",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "sleep",
								Arguments: `{"duration": 2, "reason": "Testing sleep functionality"}`,
							},
						},
					},
				}, nil
			}

			// Second call: return final response
			return openai.ChatCompletionMessage{
				Content: "I've completed the task after waiting.",
			}, nil
		})

	// TODO: Mock for sleep tool execution
	// env.OnActivity(s.activities.ExecuteToolActivity, "sleep", mustMatchAny).Return(
	// 	&types.ToolResult{
	// 		Tool:    "sleep",
	// 		Content: "Slept for 2 seconds. Reason: Testing sleep functionality",
	// 		Data:    "Sleep completed",
	// 	}, nil)

	// Create sample input
	input := PlanInput{
		Plan:      "1. Wait for 2 seconds. 2. Complete the task.",
		ToolNames: []string{"sleep"},
		Model:     "test-model",
		MaxSteps:  10,
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	s.NoError(err)

	// Start the workflow
	env.ExecuteWorkflow(PlannedAgentWorkflow, inputJSON)

	// Check that the workflow completed without error
	s.NoError(env.GetWorkflowError())

	// Query for state
	var state PlanState
	queryResult, err := env.QueryWorkflow(QueryGetState)
	s.NoError(err)
	s.NoError(queryResult.Get(&state))

	// Verify state
	s.False(state.CompletedAt.IsZero())
	s.Equal("I've completed the task after waiting.", state.Output)

	// Verify history contains sleep entry
	hasSleepEntry := false
	for _, entry := range state.History {
		if entry.Type == "action" && len(entry.Content) > 0 {
			actionRequest := struct{ Tool string }{}
			if err := json.Unmarshal([]byte(entry.Content), &actionRequest); err == nil {
				if actionRequest.Tool == "sleep" {
					hasSleepEntry = true
					break
				}
			}
		}
	}
	s.True(hasSleepEntry, "History should contain a sleep action entry")
}

// mustMatchAny is a helper matcher for mock activity calls.
var mustMatchAny interface{}
