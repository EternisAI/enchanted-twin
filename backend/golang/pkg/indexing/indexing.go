package indexing

import (
	"log/slog"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	nats "github.com/nats-io/nats.go"
	"go.temporal.io/sdk/worker"
)

type IndexingWorkflow struct {
	Logger *slog.Logger
	Config *config.Config
	Store  *db.Store
	Nc     *nats.Conn
}

func (workflows *IndexingWorkflow) RegisterWorkflows(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.IndexWorkflow)
	(*worker).RegisterActivity(workflows.FetchDataSourcesActivity)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
	(*worker).RegisterActivity(workflows.IndexDataActivity)
	(*worker).RegisterActivity(workflows.CompleteActivity)
	(*worker).RegisterActivity(workflows.PublishIndexingStatus)
}
