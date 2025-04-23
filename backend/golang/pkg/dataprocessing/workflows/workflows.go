package workflows

import (
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

type SyncWorkflows struct {
	Logger *log.Logger
	Memory memory.Storage
	Store  *db.Store
}

func (w *SyncWorkflows) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(w.XSyncWorkflow)
	(*worker).RegisterActivity(w.XFetchActivity)
	(*worker).RegisterActivity(w.XIndexActivity)
}
