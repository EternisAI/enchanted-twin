package indexing

import (
	"log/slog"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	nats "github.com/nats-io/nats.go"
	ollamaapi "github.com/ollama/ollama/api"
	"go.temporal.io/sdk/worker"
)

type IndexingWorkflow struct {
	Logger       *slog.Logger
	Config       *config.Config
	Store        *db.Store
	Nc           *nats.Conn
	OllamaClient *ollamaapi.Client
}

func (workflows *IndexingWorkflow) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.IndexWorkflow)
	(*worker).RegisterActivity(workflows.FetchDataSourcesActivity)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
	(*worker).RegisterActivity(workflows.IndexDataActivity)
	(*worker).RegisterActivity(workflows.CompleteActivity)
	(*worker).RegisterActivity(workflows.PublishIndexingStatus)
	(*worker).RegisterActivity(workflows.DownloadOllamaModel)
}
