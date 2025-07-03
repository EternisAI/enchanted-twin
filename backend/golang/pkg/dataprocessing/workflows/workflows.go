package workflows

import (
	"github.com/charmbracelet/log"
	nats "github.com/nats-io/nats.go"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type DataProcessingWorkflows struct {
	Logger        *log.Logger
	Config        *config.Config
	Store         *db.Store
	Nc            *nats.Conn
	Memory        memory.Storage
	OpenAIService *ai.Service
}

func (workflows *DataProcessingWorkflows) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(workflows.InitializeWorkflow)
	(*worker).RegisterActivity(workflows.FetchDataSourcesActivity)
	(*worker).RegisterActivity(workflows.ProcessDataActivity)
	(*worker).RegisterActivity(workflows.GetBatchesActivity)
	(*worker).RegisterActivity(workflows.IndexBatchActivity)
	(*worker).RegisterActivity(workflows.UpdateDataSourceStateActivity)
	(*worker).RegisterActivity(workflows.PublishIndexingStatus)

	(*worker).RegisterWorkflow(workflows.XSyncWorkflow)
	(*worker).RegisterActivity(workflows.XFetchActivity)
	(*worker).RegisterActivity(workflows.XIndexActivity)

	(*worker).RegisterWorkflow(workflows.GmailSyncWorkflow)
	(*worker).RegisterActivity(workflows.GmailSyncActivity)
	(*worker).RegisterActivity(workflows.GmailStoreActivity)

	(*worker).RegisterWorkflow(workflows.GmailHistoryWorkflow)
	(*worker).RegisterActivity(workflows.GmailFetchHistoryActivity)
	(*worker).RegisterActivity(workflows.GmailHistoryIndexActivity)

	(*worker).RegisterWorkflow(workflows.DriveSyncWorkflow)
	(*worker).RegisterActivity(workflows.DriveFetchActivity)
	(*worker).RegisterActivity(workflows.DriveIndexActivity)
}
