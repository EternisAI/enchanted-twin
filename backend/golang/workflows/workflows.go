package workflows

import (
	"log/slog"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"go.temporal.io/sdk/worker"
)

type TemporalWorkflows struct {
	Logger *slog.Logger
	Config *config.Config
}

func (workflows *TemporalWorkflows) RegisterWorkflows(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.IndexWorkflow)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
}
