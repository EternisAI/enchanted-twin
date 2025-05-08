package plannedv2

import (
	"fmt"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/root-v2"
)

// Constants for scheduler workflow.
const (
	ScheduledPlanWorkflowName = "ScheduledPlanWorkflow"
	DefaultMaxScheduledRuns   = 1000 // Safety limit
)

// ScheduledPlanInput is the input for the scheduler workflow.
type ScheduledPlanInput struct {
	// Basic plan information passed to the child execution workflow
	Name      string         `json:"name"`
	Plan      string         `json:"plan"`
	ToolNames []string       `json:"tool_names,omitempty"`
	Model     string         `json:"model"`
	MaxSteps  int            `json:"max_steps"`
	Origin    map[string]any `json:"origin,omitempty"`

	// Schedule information (required for this workflow)
	Schedule string `json:"schedule"` // iCalendar RRULE formatted schedule

	// Optional customizations
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Optional run control
	MaxRuns     int           `json:"max_runs,omitempty"`      // Maximum number of child executions to launch
	RunTimeout  time.Duration `json:"run_timeout,omitempty"`   // Timeout for each child execution
	WaitForRuns bool          `json:"wait_for_runs,omitempty"` // Whether to wait for child executions to complete before scheduling next
}

// SchedulerState represents the state of the scheduler workflow.
type SchedulerState struct {
	// Input parameters (for information)
	Input ScheduledPlanInput `json:"input"`

	// Execution state
	StartedAt     time.Time `json:"started_at"`
	NextRunTime   time.Time `json:"next_run_time,omitempty"`
	LastRunTime   time.Time `json:"last_run_time,omitempty"`
	CompletedRuns int       `json:"completed_runs"`
	ChildRunIDs   []string  `json:"child_run_ids,omitempty"`

	// Error state (if any)
	Error string `json:"error,omitempty"`
}

// QueryGetSchedulerState is the query name for getting the scheduler state.
const QueryGetSchedulerState = "get_scheduler_state"

// ScheduledPlanWorkflow is a workflow that schedules and launches PlanExecutionWorkflow
// based on an iCalendar RRULE schedule.
func ScheduledPlanWorkflow(ctx workflow.Context, input ScheduledPlanInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ScheduledPlanWorkflow",
		"WorkflowID", workflow.GetInfo(ctx).WorkflowExecution.ID,
		"TaskName", input.Name,
		"Schedule", input.Schedule)

	// Validate required inputs
	if input.Schedule == "" {
		return fmt.Errorf("schedule is required but was empty")
	}

	if input.Plan == "" {
		return fmt.Errorf("plan is required but was empty")
	}

	// Apply defaults
	if input.Model == "" {
		input.Model = "gpt-4o" // Default model
	}

	if input.MaxSteps <= 0 {
		input.MaxSteps = DefaultMaxSteps
	}

	if input.MaxRuns <= 0 {
		input.MaxRuns = DefaultMaxScheduledRuns // Use a reasonable default to prevent infinite loops
	}

	// Initialize state
	state := SchedulerState{
		Input:         input,
		StartedAt:     workflow.Now(ctx),
		CompletedRuns: 0,
		ChildRunIDs:   []string{},
	}

	// Register query handlers
	if err := workflow.SetQueryHandler(ctx, QueryGetSchedulerState, func() (SchedulerState, error) {
		return state, nil
	}); err != nil {
		logger.Error("Failed to register scheduler state query handler", "error", err)
		return fmt.Errorf("failed to register query handler: %w", err)
	}

	// Parse the initial schedule
	rruleSet, err := parseSchedule(input.Schedule)
	if err != nil {
		state.Error = fmt.Sprintf("failed to parse schedule: %v", err)
		logger.Error("Failed to parse schedule", "error", err)
		return fmt.Errorf("failed to parse schedule: %w", err)
	}

	// Main scheduling loop
	for state.CompletedRuns < input.MaxRuns {
		// Calculate the next occurrence
		now := workflow.Now(ctx)
		nextTimes := rruleSet.Between(now, now.Add(365*24*time.Hour), true)

		if len(nextTimes) == 0 {
			logger.Info("No more occurrences in schedule within the next year, ending workflow")
			break
		}

		nextRunTime := nextTimes[0]
		state.NextRunTime = nextRunTime
		logger.Info("Calculated next run time", "nextRunTime", nextRunTime)

		// Sleep until the next run time
		sleepDuration := nextRunTime.Sub(now)
		if sleepDuration > 0 {
			logger.Info("Sleeping until next run", "duration", sleepDuration.String())
			if err := workflow.Sleep(ctx, sleepDuration); err != nil {
				logger.Error("Sleep interrupted", "error", err)
				return fmt.Errorf("sleep interrupted: %w", err)
			}
		}

		// Execute the plan
		state.LastRunTime = workflow.Now(ctx)

		// Prepare input for the child workflow
		childInput := PlanInput{
			Name:         fmt.Sprintf("%s (Run %d)", input.Name, state.CompletedRuns+1),
			Plan:         input.Plan,
			ToolNames:    input.ToolNames,
			Model:        input.Model,
			MaxSteps:     input.MaxSteps,
			Origin:       input.Origin,
			SystemPrompt: input.SystemPrompt,
		}

		// Set options for the child workflow
		childWorkflowID := fmt.Sprintf("%s_%s_%d",
			workflow.GetInfo(ctx).WorkflowExecution.ID,
			strings.ReplaceAll(input.Name, " ", "_"),
			state.CompletedRuns+1)

		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID: childWorkflowID,
			TaskQueue:  root.ChildTaskQueue,
		}

		// Add timeout if specified
		if input.RunTimeout > 0 {
			childOpts.WorkflowRunTimeout = input.RunTimeout
		}

		// Create child workflow context
		childCtx := workflow.WithChildOptions(ctx, childOpts)

		logger.Info("Starting child execution workflow", "childWorkflowID", childWorkflowID)

		// Execute the child workflow
		childFuture := workflow.ExecuteChildWorkflow(childCtx, WorkflowName, childInput)

		// Get the execution info without waiting for completion
		var childExecution workflow.Execution
		if err := childFuture.GetChildWorkflowExecution().Get(ctx, &childExecution); err != nil {
			logger.Error("Failed to get child workflow execution info", "error", err)
			state.Error = fmt.Sprintf("failed to start child workflow: %v", err)
			// Continue with the next schedule - don't fail the scheduler for a single execution failure
			continue
		}

		// Add the child run ID to our state
		state.ChildRunIDs = append(state.ChildRunIDs, childExecution.RunID)

		// If configured to wait for child completion, do so
		if input.WaitForRuns {
			var result string
			if err := childFuture.Get(ctx, &result); err != nil {
				logger.Error("Child workflow execution failed", "RunID", childExecution.RunID, "error", err)
				// Don't fail the scheduler for a single execution failure
			} else {
				logger.Info("Child workflow completed", "RunID", childExecution.RunID, "result", result)
			}
		}

		// Increment completed runs counter
		state.CompletedRuns++

		// Check if we should continue
		if state.CompletedRuns >= input.MaxRuns {
			logger.Info("Reached maximum number of runs", "maxRuns", input.MaxRuns)
			break
		}
	}

	logger.Info("Scheduler workflow completed", "totalRuns", state.CompletedRuns)
	return nil
}

// parseSchedule parses an iCalendar RRULE string into an rrule.RRuleSet.
func parseSchedule(schedule string) (*rrule.Set, error) {
	// Create a new RRuleSet

	schedule = strings.TrimPrefix(schedule, "RRULE:")

	ruleSet, errSet := rrule.StrToRRuleSet(schedule)
	if errSet == nil {
		return ruleSet, nil
	}

	rule, errRule := rrule.StrToRRule(schedule)
	if errRule != nil {
		return nil, fmt.Errorf("failed to parse schedule: as ruleset %w or rule %w", errSet, errRule)
	}
	ruleSet = &rrule.Set{}
	ruleSet.RRule(rule)

	return ruleSet, nil
}
