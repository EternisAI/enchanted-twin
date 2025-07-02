package bootstrap

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/notifications"
	"github.com/EternisAI/enchanted-twin/pkg/agent/scheduler"
	schedulerTools "github.com/EternisAI/enchanted-twin/pkg/agent/scheduler/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/engagement"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/tts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

// customLogWriter routes logs to stderr if they contain "err" or "error", otherwise to stdout.
type customLogWriter struct{}

func (w *customLogWriter) Write(p []byte) (n int, err error) {
	logContent := strings.ToLower(string(p))
	if strings.Contains(logContent, "err") || strings.Contains(logContent, "error") || strings.Contains(logContent, "failed") {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func NewLogger() *log.Logger {
	return log.NewWithOptions(&customLogWriter{}, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})
}


func NewTTSService(logger *log.Logger) (*tts.Service, error) {
	const (
		kokoroPort = 45000
		ttsWsPort  = 45001
	)

	engine := tts.Kokoro{
		Endpoint: fmt.Sprintf("http://localhost:%d/v1/audio/speech", kokoroPort),
		Model:    "kokoro",
		Voice:    "af_bella+af_heart",
	}
	svc := tts.New(fmt.Sprintf(":%d", ttsWsPort), engine, *logger)
	return svc, nil
}

func NewToolRegistry(
	mem memory.Storage,
	temporalClient client.Client,
	chatStorage *chatrepository.Repository,
	nc *nats.Conn,
	store *db.Store,
	cfg *config.Config,
	logger *log.Logger,
) *tools.ToolMapRegistry {
	toolRegistry := tools.NewRegistry()

	if err := toolRegistry.Register(memory.NewMemorySearchTool(logger, mem)); err != nil {
		logger.Error("Failed to register memory search tool", "error", err)
	}

	if err := toolRegistry.Register(&schedulerTools.ScheduleTask{
		Logger:         logger,
		TemporalClient: temporalClient,
	}); err != nil {
		logger.Error("Failed to register schedule task tool", "error", err)
	}

	sendToChatTool := twinchat.NewSendToChatTool(chatStorage, nc)
	if err := toolRegistry.Register(sendToChatTool); err != nil {
		logger.Error("Failed to register send to chat tool", "error", err)
	}

	telegramTool, err := telegram.NewTelegramSetupTool(logger, cfg.TelegramToken, store, cfg.TelegramChatServer)
	if err != nil {
		logger.Error("Failed to create telegram setup tool", "error", err)
	} else {
		if err := toolRegistry.Register(telegramTool); err != nil {
			logger.Error("Failed to register telegram tool", "error", err)
		}
	}

	return toolRegistry
}

func NewIdentityService(temporalClient client.Client) *identity.IdentityService {
	return identity.NewIdentityService(temporalClient)
}




func CreateTemporalWorker(
	temporalClient client.Client,
	logger *log.Logger,
	cfg *config.Config,
	store *db.Store,
	nc *nats.Conn,
	mem memory.Storage,
	toolsRegistry *tools.ToolMapRegistry,
	aiServices ai.Services,
	notifications *notifications.Service,
	twinchatService *twinchat.Service,
) (worker.Worker, error) {
	w := worker.New(temporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 3,
	})

	dataProcessingWorkflow := workflows.DataProcessingWorkflows{
		Logger:        logger,
		Config:        cfg,
		Store:         store,
		Nc:            nc,
		Memory:        mem,
		OpenAIService: aiServices.Completions,
	}
	dataProcessingWorkflow.RegisterWorkflowsAndActivities(&w)

	authActivities := auth.NewOAuthActivities(store)
	authActivities.RegisterWorkflowsAndActivities(&w)

	aiAgent := agent.NewAgent(logger, nc, aiServices.Completions, cfg.CompletionsModel, cfg.ReasoningModel, nil, nil)
	schedulerActivities := scheduler.NewTaskSchedulerActivities(logger, aiServices.Completions, aiAgent, toolsRegistry, cfg.CompletionsModel, store, notifications)
	schedulerActivities.RegisterWorkflowsAndActivities(w)

	identityActivities := identity.NewIdentityActivities(logger, mem, aiServices.Completions, cfg.CompletionsModel)
	identityActivities.RegisterWorkflowsAndActivities(w)

	identityService := identity.NewIdentityService(temporalClient)
	friendService := engagement.NewFriendService(engagement.FriendServiceConfig{
		Logger:          logger,
		MemoryService:   mem,
		IdentityService: identityService,
		TwinchatService: twinchatService,
		AiService:       aiServices.Completions,
		ToolRegistry:    toolsRegistry,
		Store:           store,
	})
	friendService.RegisterWorkflowsAndActivities(&w, temporalClient)

	holonManager := holon.NewManager(store, holon.DefaultManagerConfig(), logger, temporalClient, w)
	holonSyncActivities := holon.NewHolonSyncActivities(logger, holonManager)
	holonSyncActivities.RegisterWorkflowsAndActivities(w)

	return w, nil
}



func RegisterIdentityWorkflows(logger *log.Logger, temporalClient client.Client) error {
	logger.Debug("Registering identity scheduled workflows")
	
	// Register scheduled workflow for personality derivation
	err := helpers.CreateScheduleIfNotExists(logger, temporalClient, identity.PersonalityWorkflowID, time.Hour*12, identity.DerivePersonalityWorkflow, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create identity personality workflow")
	}
	
	logger.Debug("Identity scheduled workflows registered successfully")
	return nil
}

func RegisterHolonWorkflows(logger *log.Logger, temporalClient client.Client) error {
	logger.Debug("Registering holon scheduled workflows")
	
	// Register scheduled workflow for holon data synchronization
	err := helpers.CreateOrUpdateSchedule(
		logger,
		temporalClient,
		"holon-sync-schedule",
		30*time.Second,
		holon.HolonSyncWorkflow,
		[]any{holon.HolonSyncWorkflowInput{ForceSync: false}},
		true,
	)
	if err != nil {
		return errors.Wrap(err, "Failed to create holon sync schedule")
	}
	
	logger.Debug("Holon scheduled workflows registered successfully")
	return nil
}