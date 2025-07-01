package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/agent/notifications"
	"github.com/EternisAI/enchanted-twin/pkg/agent/scheduler"
	schedulerTools "github.com/EternisAI/enchanted-twin/pkg/agent/scheduler/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/engagement"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/tts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
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

type AIServices struct {
	Completions *ai.Service
	Embeddings  *ai.Service
}

func NewLogger() *log.Logger {
	return log.NewWithOptions(&customLogWriter{}, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})
}

func LoadConfig() (*config.Config, error) {
	envs, err := config.LoadConfig(false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load config")
	}
	return envs, nil
}

func NewContext() context.Context {
	return context.Background()
}

type NATSServer struct{}

func NewNATSServer(logger *log.Logger) (*NATSServer, error) {
	_, err := bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to start nats server")
	}
	logger.Info("NATS server started")
	return &NATSServer{}, nil
}

func NewNATSClient(logger *log.Logger, natsServer *NATSServer) (*nats.Conn, error) {
	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create nats client")
	}
	logger.Info("NATS client started")
	return nc, nil
}

func NewStore(ctx context.Context, cfg *config.Config, logger *log.Logger) (*db.Store, error) {
	store, err := db.NewStore(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("Unable to create or initialize database", "error", err)
		return nil, errors.Wrap(err, "Unable to create or initialize database")
	}
	return store, nil
}

func NewDatabase(store *db.Store, logger *log.Logger) (*db.DB, error) {
	dbsqlc, err := db.New(store.DB().DB, logger)
	if err != nil {
		logger.Error("Error creating database", "error", err)
		return nil, errors.Wrap(err, "Error creating database")
	}
	return dbsqlc, nil
}

func NewAIServices(cfg *config.Config, logger *log.Logger) AIServices {
	tokenFunc := func() string { return "12345" }

	var aiCompletionsService *ai.Service
	if cfg.ProxyTeeURL != "" {
		aiCompletionsService = ai.NewOpenAIServiceProxy(logger, cfg.ProxyTeeURL, tokenFunc, cfg.CompletionsAPIURL)
	} else {
		aiCompletionsService = ai.NewOpenAIService(logger, cfg.CompletionsAPIKey, cfg.CompletionsAPIURL)
	}

	var aiEmbeddingsService *ai.Service
	if cfg.ProxyTeeURL != "" {
		aiEmbeddingsService = ai.NewOpenAIServiceProxy(logger, cfg.ProxyTeeURL, tokenFunc, cfg.EmbeddingsAPIURL)
	} else {
		aiEmbeddingsService = ai.NewOpenAIService(logger, cfg.EmbeddingsAPIKey, cfg.EmbeddingsAPIURL)
	}

	return AIServices{
		Completions: aiCompletionsService,
		Embeddings:  aiEmbeddingsService,
	}
}

func NewCompletionsAIService(aiServices AIServices) *ai.Service {
	return aiServices.Completions
}

func NewChatStorage(logger *log.Logger, store *db.Store) *chatrepository.Repository {
	return chatrepository.NewRepository(logger, store.DB())
}

func NewWeaviateClient(ctx context.Context, cfg *config.Config, aiServices AIServices, logger *log.Logger) (*weaviate.Client, error) {
	weaviatePath := filepath.Join(cfg.AppDataPath, "db", "weaviate")
	logger.Info("Starting Weaviate bootstrap process", "path", weaviatePath, "port", cfg.WeaviatePort)

	if _, err := bootstrap.BootstrapWeaviateServer(ctx, logger, cfg.WeaviatePort, weaviatePath); err != nil {
		return nil, errors.Wrap(err, "Failed to bootstrap Weaviate server")
	}

	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", cfg.WeaviatePort),
		Scheme: "http",
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Weaviate client")
	}

	if err := bootstrap.InitSchema(weaviateClient, logger, aiServices.Embeddings, cfg.EmbeddingsModel); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize Weaviate schema")
	}

	return weaviateClient, nil
}

func NewEvolvingMemory(weaviateClient *weaviate.Client, aiServices AIServices, cfg *config.Config, logger *log.Logger) (memory.Storage, error) {
	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiServices.Embeddings, cfg.EmbeddingsModel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create embedding wrapper")
	}

	storageInterface, err := storage.New(storage.NewStorageInput{
		Client:            weaviateClient,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create storage interface")
	}

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: aiServices.Completions,
		CompletionsModel:   cfg.CompletionsModel,
		EmbeddingsWrapper:  embeddingsWrapper,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create evolving memory")
	}

	return mem, nil
}

func NewTemporalClient(logger *log.Logger, cfg *config.Config) (client.Client, error) {
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, cfg.DBPath)
	<-ready
	logger.Info("Temporal server started")

	temporalClient, err := bootstrap.NewTemporalClient(logger)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create temporal client")
	}
	logger.Info("Temporal client created")
	return temporalClient, nil
}

func NewTemporalWorker(
	temporalClient client.Client,
	logger *log.Logger,
	cfg *config.Config,
	store *db.Store,
	nc *nats.Conn,
	mem memory.Storage,
	toolsRegistry *tools.ToolMapRegistry,
	aiServices AIServices,
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

	err := w.Start()
	if err != nil {
		return nil, errors.Wrap(err, "Error starting worker")
	}

	return w, nil
}

func bootstrapTTS(logger *log.Logger) (*tts.Service, error) {
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

func NewTTSService(logger *log.Logger) (*tts.Service, error) {
	return bootstrapTTS(logger)
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

func NewTwinChatService(
	logger *log.Logger,
	aiServices AIServices,
	chatStorage *chatrepository.Repository,
	nc *nats.Conn,
	mem memory.Storage,
	toolRegistry *tools.ToolMapRegistry,
	store *db.Store,
	cfg *config.Config,
	identitySvc *identity.IdentityService,
) *twinchat.Service {
	return twinchat.NewService(
		logger,
		aiServices.Completions,
		chatStorage,
		nc,
		mem,
		toolRegistry,
		store,
		cfg.CompletionsModel,
		cfg.ReasoningModel,
		identitySvc,
	)
}

func NewMCPService(ctx context.Context, logger *log.Logger, store *db.Store, toolRegistry *tools.ToolMapRegistry) mcpserver.MCPService {
	return mcpserver.NewService(ctx, logger, store, toolRegistry)
}

func NewTelegramService(
	logger *log.Logger,
	cfg *config.Config,
	store *db.Store,
	aiServices AIServices,
	mem memory.Storage,
	nc *nats.Conn,
	toolsRegistry *tools.ToolMapRegistry,
) *telegram.TelegramService {
	telegramServiceInput := telegram.TelegramServiceInput{
		Logger:           logger,
		Token:            cfg.TelegramToken,
		Client:           &http.Client{},
		Store:            store,
		AiService:        aiServices.Completions,
		CompletionsModel: cfg.CompletionsModel,
		Memory:           mem,
		AuthStorage:      store,
		NatsClient:       nc,
		ChatServerUrl:    cfg.TelegramChatServer,
		ToolsRegistry:    toolsRegistry,
	}
	return telegram.NewTelegramService(telegramServiceInput)
}

func NewHolonService(store *db.Store, logger *log.Logger, aiServices AIServices, cfg *config.Config, mem memory.Storage, toolRegistry *tools.ToolMapRegistry) *holon.Service {
	holonConfig := holon.DefaultManagerConfig()
	holonService := holon.NewServiceWithConfig(store, logger, holonConfig.HolonAPIURL)

	processingInterval := 30 * time.Second
	holonService.InitializeBackgroundProcessor(processingInterval)

	threadPreviewTool := holon.NewThreadPreviewTool(holonService)
	if err := toolRegistry.Register(threadPreviewTool); err != nil {
		logger.Error("Failed to register thread preview tool", "error", err)
	}

	sendToHolonTool := holon.NewSendToHolonTool(holonService)
	if err := toolRegistry.Register(sendToHolonTool); err != nil {
		logger.Error("Failed to register send to holon tool", "error", err)
	}

	sendMessageToHolonTool := holon.NewAddMessageToThreadTool(holonService)
	if err := toolRegistry.Register(sendMessageToHolonTool); err != nil {
		logger.Error("Failed to register send message to holon tool", "error", err)
	}

	return holonService
}

func NewNotificationsService(nc *nats.Conn) *notifications.Service {
	return notifications.NewService(nc)
}

func NewWhatsAppService(
	logger *log.Logger,
	nc *nats.Conn,
	database *db.DB,
	mem memory.Storage,
	cfg *config.Config,
	aiServices AIServices,
	toolRegistry *tools.ToolMapRegistry,
) *whatsapp.Service {
	return whatsapp.NewService(whatsapp.ServiceConfig{
		Logger:        logger,
		NatsClient:    nc,
		Database:      database,
		MemoryStorage: mem,
		Config:        cfg,
		AIService:     aiServices.Completions,
		ToolRegistry:  toolRegistry,
	})
}

func RegisterWhatsAppServiceLifecycle(lc fx.Lifecycle, service *whatsapp.Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return service.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return service.Stop(ctx)
		},
	})
}

type graphqlServerInput struct {
	logger                 *log.Logger
	temporalClient         client.Client
	port                   string
	twinChatService        twinchat.Service
	natsClient             *nats.Conn
	store                  *db.Store
	aiService              *ai.Service
	mcpService             mcpserver.MCPService
	dataProcessingWorkflow *workflows.DataProcessingWorkflows
	telegramService        *telegram.TelegramService
	holonService           *holon.Service
	whatsAppQRCode         *string
	whatsAppConnected      bool
}

func bootstrapGraphqlServer(input graphqlServerInput) *chi.Mux {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            false,
	}).Handler)

	resolver := &graph.Resolver{
		Logger:                 input.logger,
		TemporalClient:         input.temporalClient,
		TwinChatService:        input.twinChatService,
		Nc:                     input.natsClient,
		Store:                  input.store,
		AiService:              input.aiService,
		MCPService:             input.mcpService,
		DataProcessingWorkflow: input.dataProcessingWorkflow,
		TelegramService:        input.telegramService,
		HolonService:           input.holonService,
		WhatsAppQRCode:         input.whatsAppQRCode,
		WhatsAppConnected:      input.whatsAppConnected,
	}

	srv := handler.New(gqlSchema(resolver))

	srv.AddTransport(transport.SSE{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	})

	srv.Use(extension.Introspection{})
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		resp := next(ctx)

		if resp != nil && resp.Errors != nil && len(resp.Errors) > 0 {
			oc := graphql.GetOperationContext(ctx)
			input.logger.Error(
				"gql error",
				"operation_name",
				oc.OperationName,
				"raw_query",
				oc.RawQuery,
				"variables",
				oc.Variables,
				"errors",
				resp.Errors,
			)
		}

		return resp
	})

	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", srv)

	return router
}

func gqlSchema(input *graph.Resolver) graphql.ExecutableSchema {
	config := graph.Config{
		Resolvers: input,
	}
	return graph.NewExecutableSchema(config)
}

func NewGraphQLRouter(
	logger *log.Logger,
	temporalClient client.Client,
	cfg *config.Config,
	twinChatService *twinchat.Service,
	nc *nats.Conn,
	store *db.Store,
	aiServices AIServices,
	mcpService mcpserver.MCPService,
	telegramService *telegram.TelegramService,
	holonService *holon.Service,
	whatsappService *whatsapp.Service,
) *chi.Mux {
	return bootstrapGraphqlServer(graphqlServerInput{
		logger:            logger,
		temporalClient:    temporalClient,
		port:              cfg.GraphqlPort,
		twinChatService:   *twinChatService,
		natsClient:        nc,
		store:             store,
		aiService:         aiServices.Completions,
		mcpService:        mcpService,
		telegramService:   telegramService,
		holonService:      holonService,
		whatsAppQRCode:    whatsappService.GetCurrentQRCode(),
		whatsAppConnected: whatsappService.IsConnected(),
	})
}

func bootstrapPeriodicWorkflows(logger *log.Logger, temporalClient client.Client) error {
	err := helpers.CreateScheduleIfNotExists(logger, temporalClient, identity.PersonalityWorkflowID, time.Hour*12, identity.DerivePersonalityWorkflow, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create identity personality workflow")
	}

	err = helpers.CreateOrUpdateSchedule(
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

	return nil
}

func RegisterPeriodicWorkflows(logger *log.Logger, temporalClient client.Client) error {
	return bootstrapPeriodicWorkflows(logger, temporalClient)
}

func StartGraphQLServer(lc fx.Lifecycle, router *chi.Mux, cfg *config.Config, logger *log.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+cfg.GraphqlPort)
				err := http.ListenAndServe(":"+cfg.GraphqlPort, router)
				if err != nil && err != http.ErrServerClosed {
					logger.Error("HTTP server error", "error", err)
				}
			}()
			return nil
		},
	})
}

func StartTelegramProcesses(lc fx.Lifecycle, telegramService *telegram.TelegramService, logger *log.Logger, cfg *config.Config, toolRegistry *tools.ToolMapRegistry, database *db.DB) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go telegram.SubscribePoller(telegramService, logger)
			go telegram.MonitorAndRegisterTelegramTool(context.Background(), telegramService, logger, toolRegistry, database.ConfigQueries, cfg)
			return nil
		},
	})
}

func StartHolonProcesses(lc fx.Lifecycle, holonService *holon.Service, store *db.Store, logger *log.Logger, temporalClient client.Client, temporalWorker worker.Worker) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			backgroundCtx, cancelBackgroundProcessing := context.WithCancel(ctx)

			if err := holonService.StartBackgroundProcessing(backgroundCtx); err != nil {
				logger.Error("Failed to start background thread processing", "error", err)
				cancelBackgroundProcessing()
				return err
			}

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					cancelBackgroundProcessing()
					return nil
				},
			})

			holonConfig := holon.DefaultManagerConfig()
			holonManager := holon.NewManager(store, holonConfig, logger, temporalClient, temporalWorker)
			if err := holonManager.Start(); err != nil {
				logger.Error("Failed to start HolonZero fetcher service", "error", err)
			} else {
				logger.Info("HolonZero API fetcher service started successfully")
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			holonService.StopBackgroundProcessing()
			return nil
		},
	})
}
