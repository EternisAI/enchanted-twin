package fx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/directorywatcher"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/tts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

// ServicesModule provides all application services.
var ServicesModule = fx.Module("services",
	fx.Provide(
		ProvideChatStorage,
		ProvideIdentityService,
		ProvideTwinChatService,
		ProvideTTSService,
		ProvideWhatsAppService,
		ProvideTelegramService,
		ProvideHolonService,
		ProvideMCPService,
		ProvideDirectoryWatcher,
	),
	fx.Invoke(
		StartBackgroundServices,
		RegisterApplicationTools,
	),
)

// ChatStorageResult provides chat storage repository.
type ChatStorageResult struct {
	fx.Out
	ChatStorage *chatrepository.Repository
}

// ProvideChatStorage creates chat storage repository.
func ProvideChatStorage(logger *log.Logger, store *db.Store) ChatStorageResult {
	chatStorage := chatrepository.NewRepository(logger, store.DB())
	return ChatStorageResult{ChatStorage: chatStorage}
}

// IdentityServiceResult provides identity service.
type IdentityServiceResult struct {
	fx.Out
	IdentityService *identity.IdentityService
}

// ProvideIdentityService creates identity service.
func ProvideIdentityService(temporalClient client.Client) IdentityServiceResult {
	identitySvc := identity.NewIdentityService(temporalClient)
	return IdentityServiceResult{IdentityService: identitySvc}
}

// TwinChatServiceResult provides twin chat service.
type TwinChatServiceResult struct {
	fx.Out
	TwinChatService *twinchat.Service
}

// TwinChatServiceParams holds parameters for twin chat service.
type TwinChatServiceParams struct {
	fx.In
	Logger             *log.Logger
	Config             *config.Config
	CompletionsService *ai.Service
	ChatStorage        *chatrepository.Repository
	NATSConn           *nats.Conn
	Memory             memory.Storage
	ToolRegistry       *tools.ToolMapRegistry
	Store              *db.Store
	IdentityService    *identity.IdentityService
}

// ProvideTwinChatService creates twin chat service.
func ProvideTwinChatService(params TwinChatServiceParams) TwinChatServiceResult {
	twinChatService := twinchat.NewService(
		params.Logger,
		params.CompletionsService,
		params.ChatStorage,
		params.NATSConn,
		params.Memory,
		params.ToolRegistry,
		params.Store,
		params.Config.CompletionsModel,
		params.Config.ReasoningModel,
		params.IdentityService,
		params.Config.AnonymizerType,
	)

	return TwinChatServiceResult{TwinChatService: twinChatService}
}

// TTSServiceResult provides TTS service.
type TTSServiceResult struct {
	fx.Out
	TTSService *tts.Service
}

// ProvideTTSService creates and configures TTS service.
func ProvideTTSService(lc fx.Lifecycle, logger *log.Logger, envs *config.Config) (TTSServiceResult, error) {
	const (
		kokoroPort = 45000
		ttsWsPort  = 45001
	)

	engine := tts.Kokoro{
		Endpoint: envs.TTSEndpoint,
		Model:    "kokoro",
		Voice:    "af_bella+af_heart",
	}
	ttsSvc := tts.New(fmt.Sprintf(":%d", ttsWsPort), engine, *logger)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting TTS service")
			go func() {
				if err := ttsSvc.Start(ctx); err != nil && err != http.ErrServerClosed {
					logger.Error("TTS service stopped unexpectedly", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping TTS service")
			// TTS service doesn't have explicit stop method, relies on context cancellation
			return nil
		},
	})

	return TTSServiceResult{TTSService: ttsSvc}, nil
}

// WhatsAppServiceResult provides WhatsApp service.
type WhatsAppServiceResult struct {
	fx.Out
	WhatsAppService *whatsapp.Service
}

// WhatsAppServiceParams holds parameters for WhatsApp service.
type WhatsAppServiceParams struct {
	fx.In
	Lifecycle          fx.Lifecycle
	Logger             *log.Logger
	Config             *config.Config
	NATSConn           *nats.Conn
	Database           *db.DB
	Memory             memory.Storage
	CompletionsService *ai.Service
	ToolRegistry       *tools.ToolMapRegistry
}

// ProvideWhatsAppService creates WhatsApp service.
func ProvideWhatsAppService(params WhatsAppServiceParams) (WhatsAppServiceResult, error) {
	whatsappService := whatsapp.NewService(whatsapp.ServiceConfig{
		Logger:        params.Logger,
		NatsClient:    params.NATSConn,
		Database:      params.Database,
		MemoryStorage: params.Memory,
		Config:        params.Config,
		AIService:     params.CompletionsService,
		ToolRegistry:  params.ToolRegistry,
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.Logger.Info("Starting WhatsApp service")
			if err := whatsappService.Start(ctx); err != nil {
				params.Logger.Error("Failed to start WhatsApp service", "error", err)
				return err
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping WhatsApp service")
			if err := whatsappService.Stop(ctx); err != nil {
				params.Logger.Error("Failed to stop WhatsApp service", "error", err)
				return err
			}
			return nil
		},
	})

	return WhatsAppServiceResult{WhatsAppService: whatsappService}, nil
}

// TelegramServiceResult provides Telegram service.
type TelegramServiceResult struct {
	fx.Out
	TelegramService *telegram.TelegramService
}

// TelegramServiceParams holds parameters for Telegram service.
type TelegramServiceParams struct {
	fx.In
	Logger             *log.Logger
	Config             *config.Config
	Store              *db.Store
	CompletionsService *ai.Service
	Memory             memory.Storage
	NATSConn           *nats.Conn
	ToolRegistry       *tools.ToolMapRegistry
}

// ProvideTelegramService creates Telegram service.
func ProvideTelegramService(params TelegramServiceParams) TelegramServiceResult {
	telegramServiceInput := telegram.TelegramServiceInput{
		Logger:           params.Logger,
		Client:           &http.Client{},
		Store:            params.Store,
		AiService:        params.CompletionsService,
		CompletionsModel: params.Config.CompletionsModel,
		Memory:           params.Memory,
		AuthStorage:      params.Store,
		NatsClient:       params.NATSConn,
		ChatServerUrl:    params.Config.TelegramChatServer,
		ToolsRegistry:    params.ToolRegistry,
	}
	telegramService := telegram.NewTelegramService(telegramServiceInput)

	return TelegramServiceResult{TelegramService: telegramService}
}

// HolonServiceResult provides Holon service.
type HolonServiceResult struct {
	fx.Out
	HolonService *holon.Service
}

// HolonServiceParams holds parameters for Holon service.
type HolonServiceParams struct {
	fx.In
	Lifecycle          fx.Lifecycle
	Logger             *log.Logger
	Config             *config.Config
	Store              *db.Store
	CompletionsService *ai.Service
	Memory             evolvingmemory.MemoryStorage
	TemporalClient     client.Client
	TemporalWorker     worker.Worker
}

// ProvideHolonService creates Holon service with background processing.
func ProvideHolonService(params HolonServiceParams) HolonServiceResult {
	holonConfig := holon.DefaultManagerConfig()
	holonService := holon.NewServiceWithConfig(params.Store, params.Logger, holonConfig.HolonAPIURL)

	// Initialize thread processor with AI and memory services for LLM-based filtering
	params.Logger.Info("Initializing thread processor with LLM-based evaluation")
	holonService.InitializeThreadProcessor(params.CompletionsService, params.Config.CompletionsModel, params.Memory)

	// Initialize and start background processor for automatic thread processing
	processingInterval := 30 * time.Second // Process received threads every 30 seconds
	holonService.InitializeBackgroundProcessor(processingInterval)

	// Initialize HolonZero API fetcher service with the main logger
	holonManager := holon.NewManager(params.Store, holonConfig, params.Logger, params.TemporalClient, params.TemporalWorker)

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.Logger.Info("Starting Holon services")

			// Start background processing
			if err := holonService.StartBackgroundProcessing(ctx); err != nil {
				params.Logger.Error("Failed to start background thread processing", "error", err)
				return err
			}
			params.Logger.Info("Background thread processing started successfully")

			// Start holon manager
			if err := holonManager.Start(); err != nil {
				params.Logger.Error("Failed to start HolonZero fetcher service", "error", err)
			} else {
				params.Logger.Info("HolonZero API fetcher service started successfully")
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping Holon services")
			holonService.StopBackgroundProcessing()
			if err := holonManager.Stop(); err != nil {
				params.Logger.Error("Failed to stop holon manager", "error", err)
				return err
			}
			return nil
		},
	})

	return HolonServiceResult{HolonService: holonService}
}

// MCPServiceResult provides MCP service.
type MCPServiceResult struct {
	fx.Out
	MCPService mcpserver.MCPService
}

// MCPServiceParams holds parameters for MCP service.
type MCPServiceParams struct {
	fx.In
	Logger       *log.Logger
	Store        *db.Store
	ToolRegistry *tools.ToolMapRegistry
}

// ProvideMCPService creates MCP service.
func ProvideMCPService(params MCPServiceParams) MCPServiceResult {
	mcpService := mcpserver.NewService(context.Background(), params.Logger, params.Store, params.ToolRegistry)
	return MCPServiceResult{MCPService: mcpService}
}

// DirectoryWatcherResult provides directory watcher.
type DirectoryWatcherResult struct {
	fx.Out
	DirectoryWatcher *directorywatcher.DirectoryWatcher
}

// DirectoryWatcherParams holds parameters for directory watcher.
type DirectoryWatcherParams struct {
	fx.In
	Lifecycle      fx.Lifecycle
	Logger         *log.Logger
	Store          *db.Store
	Memory         evolvingmemory.MemoryStorage
	TemporalClient client.Client
}

// ProvideDirectoryWatcher creates directory watcher.
func ProvideDirectoryWatcher(params DirectoryWatcherParams) (DirectoryWatcherResult, error) {
	directoryWatcher, err := directorywatcher.NewDirectoryWatcher(
		params.Store,
		params.Memory,
		params.Logger,
		params.TemporalClient,
	)
	if err != nil {
		params.Logger.Error("Failed to create directory watcher", "error", err)
		return DirectoryWatcherResult{}, err
	}

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.Logger.Info("Starting directory watcher")
			if err := directoryWatcher.Start(ctx); err != nil {
				params.Logger.Error("Failed to start directory watcher", "error", err)
				return err
			}
			params.Logger.Info("Directory watcher started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping directory watcher")
			if err := directoryWatcher.Stop(); err != nil {
				params.Logger.Error("Error stopping directory watcher", "error", err)
				return err
			}
			params.Logger.Info("Directory watcher stopped")
			return nil
		},
	})

	return DirectoryWatcherResult{DirectoryWatcher: directoryWatcher}, nil
}

// BackgroundServicesParams holds parameters for background services.
type BackgroundServicesParams struct {
	fx.In
	Lifecycle       fx.Lifecycle
	Logger          *log.Logger
	Config          *config.Config
	TelegramService *telegram.TelegramService
	ToolRegistry    *tools.ToolMapRegistry
	Database        *db.DB
}

// StartBackgroundServices starts services that need background goroutines.
func StartBackgroundServices(params BackgroundServicesParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.Logger.Info("Starting background services")

			// Start telegram background services
			go telegram.SubscribePoller(ctx, params.TelegramService, params.Logger)
			go telegram.MonitorAndRegisterTelegramTool(
				ctx,
				params.TelegramService,
				params.Logger,
				params.ToolRegistry,
				params.Database.ConfigQueries,
				params.Config,
			)

			params.Logger.Info("Background services started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Stopping background services")
			// Context cancellation will automatically stop the background goroutines
			return nil
		},
	})
}

// ApplicationToolsParams holds parameters for application tools registration.
type ApplicationToolsParams struct {
	fx.In
	Logger       *log.Logger
	ToolRegistry *tools.ToolMapRegistry
	ChatStorage  *chatrepository.Repository
	NATSConn     *nats.Conn
	HolonService *holon.Service
	MCPService   mcpserver.MCPService
}

// RegisterApplicationTools registers tools that depend on application services.
func RegisterApplicationTools(params ApplicationToolsParams) error {
	params.Logger.Info("Registering application tools")

	// Register send to chat tool
	sendToChatTool := twinchat.NewSendToChatTool(params.ChatStorage, params.NATSConn)
	if err := params.ToolRegistry.Register(sendToChatTool); err != nil {
		params.Logger.Error("Failed to register send to chat tool", "error", err)
		return err
	}

	// Register holon tools
	threadPreviewTool := holon.NewThreadPreviewTool(params.HolonService)
	if err := params.ToolRegistry.Register(threadPreviewTool); err != nil {
		params.Logger.Error("Failed to register thread preview tool", "error", err)
		return err
	}

	sendToHolonTool := holon.NewSendToHolonTool(params.HolonService)
	if err := params.ToolRegistry.Register(sendToHolonTool); err != nil {
		params.Logger.Error("Failed to register send to holon tool", "error", err)
		return err
	}

	sendMessageToHolonTool := holon.NewAddMessageToThreadTool(params.HolonService)
	if err := params.ToolRegistry.Register(sendMessageToHolonTool); err != nil {
		params.Logger.Error("Failed to register send message to holon tool", "error", err)
		return err
	}

	// MCP tools are automatically registered by the MCP service
	mcpTools, err := params.MCPService.GetInternalTools(context.Background())
	if err == nil {
		params.Logger.Info("MCP tools available", "count", len(mcpTools))
	} else {
		params.Logger.Warn("Failed to get MCP tools", "error", err)
	}

	params.Logger.Info("Application tools registered successfully",
		"total_count", len(params.ToolRegistry.List()),
		"tools", params.ToolRegistry.List())

	return nil
}
