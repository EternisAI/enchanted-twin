package fx

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/notifications"
	"github.com/EternisAI/enchanted-twin/pkg/agent/scheduler"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
)

// TemporalModule provides Temporal workflow orchestration services.
var TemporalModule = fx.Module("temporal",
	fx.Provide(
		ProvideTemporalClient,
		ProvideTemporalWorker,
		ProvideNotificationsService,
	),
	fx.Invoke(
		SetupPeriodicWorkflows,
	),
)

// TemporalClientResult provides Temporal client.
type TemporalClientResult struct {
	fx.Out
	TemporalClient client.Client
}

// ProvideTemporalClient creates and starts Temporal server and client.
func ProvideTemporalClient(
	lc fx.Lifecycle,
	logger *log.Logger,
	envs *config.Config,
) (TemporalClientResult, error) {
	logger.Info("Starting Temporal server")

	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, envs.DBPath)
	<-ready
	logger.Info("Temporal server started")

	temporalClient, err := bootstrap.NewTemporalClient(logger)
	if err != nil {
		logger.Error("Unable to create temporal client", "error", err)
		return TemporalClientResult{}, err
	}
	logger.Info("Temporal client created")

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing Temporal client")
			temporalClient.Close()
			return nil
		},
	})

	return TemporalClientResult{TemporalClient: temporalClient}, nil
}

// TemporalWorkerResult provides Temporal worker.
type TemporalWorkerResult struct {
	fx.Out
	TemporalWorker worker.Worker
}

// TemporalWorkerParams holds parameters for temporal worker.
type TemporalWorkerParams struct {
	fx.In
	Lifecycle           fx.Lifecycle
	Logger              *log.Logger
	Config              *config.Config
	TemporalClient      client.Client
	Store               *db.Store
	NATSConn            *nats.Conn
	Memory              evolvingmemory.MemoryStorage
	CompletionsService  *ai.Service
	ToolRegistry        *tools.ToolMapRegistry
	NotificationsService *notifications.Service
}

// ProvideTemporalWorker creates and starts Temporal worker with all activities.
func ProvideTemporalWorker(params TemporalWorkerParams) (TemporalWorkerResult, error) {
	params.Logger.Info("Creating Temporal worker")

	w := worker.New(params.TemporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 3,
	})

	// Register data processing workflow
	dataProcessingWorkflow := workflows.DataProcessingWorkflows{
		Logger:        params.Logger,
		Config:        params.Config,
		Store:         params.Store,
		Nc:            params.NATSConn,
		Memory:        params.Memory,
		OpenAIService: params.CompletionsService,
	}
	dataProcessingWorkflow.RegisterWorkflowsAndActivities(&w)

	// Register auth activities
	authActivities := auth.NewOAuthActivities(params.Store, params.Logger)
	authActivities.RegisterWorkflowsAndActivities(&w)

	// Register the planned agent v2 workflow
	aiAgent := agent.NewAgent(params.Logger, params.NATSConn, params.CompletionsService, params.Config.CompletionsModel, params.Config.ReasoningModel, nil, nil)
	schedulerActivities := scheduler.NewTaskSchedulerActivities(
		params.Logger,
		params.CompletionsService,
		aiAgent,
		params.ToolRegistry,
		params.Config.CompletionsModel,
		params.Store,
		params.NotificationsService,
		nil, // twinChatService - will be set later when available
	)
	schedulerActivities.RegisterWorkflowsAndActivities(w)

	// Register identity activities
	identityActivities := identity.NewIdentityActivities(params.Logger, params.Memory, params.CompletionsService, params.Config.CompletionsModel)
	identityActivities.RegisterWorkflowsAndActivities(w)

	// Register holon sync activities
	holonManager := holon.NewManager(params.Store, holon.DefaultManagerConfig(), params.Logger, params.TemporalClient, w)
	holonSyncActivities := holon.NewHolonSyncActivities(params.Logger, holonManager)
	holonSyncActivities.RegisterWorkflowsAndActivities(w)

	err := w.Start()
	if err != nil {
		params.Logger.Error("Error starting worker", "error", err)
		return TemporalWorkerResult{}, err
	}

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping Temporal worker")
			w.Stop()
			return nil
		},
	})

	params.Logger.Info("Temporal worker started successfully")
	return TemporalWorkerResult{TemporalWorker: w}, nil
}

// NotificationsServiceResult provides notifications service.
type NotificationsServiceResult struct {
	fx.Out
	NotificationsService *notifications.Service
}

// ProvideNotificationsService creates notifications service.
func ProvideNotificationsService(nc *nats.Conn) NotificationsServiceResult {
	notificationsSvc := notifications.NewService(nc)
	return NotificationsServiceResult{NotificationsService: notificationsSvc}
}

// PeriodicWorkflowsParams holds parameters for periodic workflows setup.
type PeriodicWorkflowsParams struct {
	fx.In
	Logger         *log.Logger
	TemporalClient client.Client
}

// SetupPeriodicWorkflows sets up periodic Temporal workflows.
func SetupPeriodicWorkflows(params PeriodicWorkflowsParams) error {
	params.Logger.Info("Setting up periodic workflows")

	err := helpers.DeleteScheduleIfExists(params.Logger, params.TemporalClient, identity.PersonalityWorkflowID)
	if err != nil {
		params.Logger.Warn("Failed to delete identity personality workflow - continuing without it", "error", err)
	}

	// Create holon sync schedule with override flag to ensure it uses the updated 30-second interval
	err = helpers.CreateOrUpdateSchedule(
		params.Logger,
		params.TemporalClient,
		"holon-sync-schedule",
		30*time.Second, // Use updated 30-second interval
		holon.HolonSyncWorkflow,
		[]any{holon.HolonSyncWorkflowInput{ForceSync: false}},
		true, // Override if different settings
	)
	if err != nil {
		params.Logger.Error("Failed to create holon sync schedule", "error", err)
		return err
	}

	params.Logger.Info("Periodic workflows setup completed")
	return nil
}
