package holon

import (
	"context"
	"fmt"

	clog "github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

// HolonSyncWorkflowInput contains the input parameters for the holon sync workflow
type HolonSyncWorkflowInput struct {
	ForceSync bool
}

// HolonSyncActivities defines the activities for syncing holons
type HolonSyncActivities struct {
	logger  *clog.Logger
	manager *Manager
}

// NewHolonSyncActivities creates a new instance of holon sync activities
func NewHolonSyncActivities(logger *clog.Logger, manager *Manager) *HolonSyncActivities {
	return &HolonSyncActivities{
		logger:  logger,
		manager: manager,
	}
}

// RegisterWorkflowsAndActivities registers the workflows and activities with the Temporal worker
func (a *HolonSyncActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(HolonSyncWorkflow)
	worker.RegisterActivity(a.SyncParticipants)
	worker.RegisterActivity(a.SyncThreads)
	worker.RegisterActivity(a.SyncReplies)
}

// SyncParticipants is an activity that syncs participants from the holon API
func (a *HolonSyncActivities) SyncParticipants(ctx context.Context) error {
	a.logger.Info("Starting holon participants sync activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	_, err := a.manager.fetcherService.SyncParticipants(ctx)
	if err != nil {
		a.logger.Error("Failed to sync participants", "error", err)
		return err
	}

	a.logger.Info("Holon participants sync completed successfully")
	return nil
}

// SyncThreads is an activity that syncs threads from the holon API
func (a *HolonSyncActivities) SyncThreads(ctx context.Context) error {
	a.logger.Info("Starting holon threads sync activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	_, err := a.manager.fetcherService.SyncThreads(ctx)
	if err != nil {
		a.logger.Error("Failed to sync threads", "error", err)
		return err
	}

	a.logger.Info("Holon threads sync completed successfully")
	return nil
}

// SyncReplies is an activity that syncs replies from the holon API
func (a *HolonSyncActivities) SyncReplies(ctx context.Context) error {
	a.logger.Info("Starting holon replies sync activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	_, err := a.manager.fetcherService.SyncReplies(ctx)
	if err != nil {
		a.logger.Error("Failed to sync replies", "error", err)
		return err
	}

	a.logger.Info("Holon replies sync completed successfully")
	return nil
}
