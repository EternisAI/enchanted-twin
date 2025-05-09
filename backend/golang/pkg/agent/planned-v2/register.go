package planned

import (
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

func (a *AgentActivities) RegisterPlannedAgentWorkflow(w worker.Worker, logger *log.Logger) {
	logger.Info("Registering PlannedAgentWorkflow and ScheduledPlanWorkflow")

	// Register the existing workflow
	w.RegisterWorkflow(PlannedAgentWorkflow)

	// Register the new scheduler workflow
	w.RegisterWorkflow(ScheduledPlanWorkflow)

	a.RegisterActivities(w)

	logger.Info("Planned agent workflows and activities registered")
}
