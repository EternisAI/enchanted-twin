package workflows

import (
	"log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"go.temporal.io/sdk/worker"
)

type SyncWorkflows struct {
	Logger *log.Logger
	Store  *db.Store
}

func (w *SyncWorkflows) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(w.XSyncWorkflow)
	(*worker).RegisterActivity(w.XSyncActivity)
}
