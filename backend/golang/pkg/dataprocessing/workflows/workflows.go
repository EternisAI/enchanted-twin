package workflows

import (
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	nats "github.com/nats-io/nats.go"
	ollamaapi "github.com/ollama/ollama/api"
	"go.temporal.io/sdk/worker"
)

type DataProcessingWorkflows struct {
	Logger       *log.Logger
	Config       *config.Config
	Store        *db.Store
	Nc           *nats.Conn
	OllamaClient *ollamaapi.Client
	Memory       memory.Storage
}

func (workflows *DataProcessingWorkflows) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.InitializeWorkflow)
	(*worker).RegisterActivity(workflows.FetchDataSourcesActivity)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
	(*worker).RegisterActivity(workflows.IndexDataActivity)
	(*worker).RegisterActivity(workflows.CompleteActivity)
	(*worker).RegisterActivity(workflows.PublishIndexingStatus)
	(*worker).RegisterActivity(workflows.DownloadOllamaModel)

	(*worker).RegisterWorkflow(workflows.XSyncWorkflow)
	(*worker).RegisterActivity(workflows.XFetchActivity)
	(*worker).RegisterActivity(workflows.XIndexActivity)

	(*worker).RegisterWorkflow(workflows.GmailSyncWorkflow)
	(*worker).RegisterActivity(workflows.GmailFetchActivity)
	(*worker).RegisterActivity(workflows.GmailIndexActivity)
}
