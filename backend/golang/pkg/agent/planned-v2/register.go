package plannedv2

import (
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

// WorkflowName is the name of the planned agent workflow
const WorkflowName = "PlannedAgentWorkflow"

// RegisterPlannedAgentWorkflow registers the planner workflow and activities with a Temporal worker
func RegisterPlannedAgentWorkflow(w worker.Worker, logger *log.Logger) {
	logger.Info("Registering PlannedAgentWorkflow")

	// Register the workflow
	w.RegisterWorkflow(PlannedAgentWorkflow)

	// Register the activities
	RegisterActivities(w)

	logger.Info("Planned agent workflow and activities registered")
}
