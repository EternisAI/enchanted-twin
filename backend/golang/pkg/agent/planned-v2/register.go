package plannedv2

import (
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

// WorkflowName is the name of the planned agent workflow.
const WorkflowName = "PlannedAgentWorkflow"

func (a *AgentActivities) RegisterPlannedAgentWorkflow(w worker.Worker, logger *log.Logger) {
	logger.Info("Registering PlannedAgentWorkflow")

	w.RegisterWorkflow(PlannedAgentWorkflow)

	a.RegisterActivities(w)

	logger.Info("Planned agent workflow and activities registered")
}
