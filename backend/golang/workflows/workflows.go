package workflows

import (
	"log/slog"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"go.temporal.io/sdk/worker"
)

type TemporalWorkflows struct {
	Logger *slog.Logger
	Config *config.Config
	Store  *db.Store
}

func (workflows *TemporalWorkflows) RegisterWorkflows(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.IndexWorkflow)
	(*worker).RegisterActivity(workflows.FetchDataSourcesActivity)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
	(*worker).RegisterActivity(workflows.IndexDataActivity)
	(*worker).RegisterActivity(workflows.CleanUpActivity)
	(*worker).RegisterActivity(workflows.CompleteActivity)
}
