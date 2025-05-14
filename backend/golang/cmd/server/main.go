// Owner: august@eternis.ai
package main

import (
	"context"
	stderrs "errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
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
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/embeddingsmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/scheduler"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/container"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	mcpRepository "github.com/EternisAI/enchanted-twin/pkg/mcpserver/repository"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/tts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

func main() {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	envs, _ := config.LoadConfig(true)
	logger.Debug("Config loaded", "envs", envs)
	logger.Info("Using database path", "path", envs.DBPath)

	var ollamaClient *ollamaapi.Client
	if envs.OllamaBaseURL != "" {
		baseURL, err := url.Parse(envs.OllamaBaseURL)
		if err != nil {
			logger.Error("Failed to parse Ollama base URL", slog.Any("error", err))
		} else {
			ollamaClient = ollamaapi.NewClient(baseURL, http.DefaultClient)
		}
	}

	natsServer, err := bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start nats server"))
	}
	defer natsServer.Shutdown()
	logger.Info("NATS server started")

	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		panic(errors.Wrap(err, "Unable to create nats client"))
	}
	logger.Info("NATS client started")

	store, err := db.NewStore(context.Background(), envs.DBPath)
	if err != nil {
		logger.Error("Unable to create or initialize database", "error", err)
		panic(errors.Wrap(err, "Unable to create or initialize database"))
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Error closing store", slog.Any("error", err))
		}
	}()

	logger.Info("SQLite database initialized")

	logger.Info("Initializing container runtime and Kokoro container...")
	containerID, err := bootstrapKokoro(envs)
	if err != nil {
		logger.Warn("Failed to initialize container runtime and Kokoro container", "error", err)
		logger.Info("Continuing without Kokoro container")
	} else {
		logger.Info("Kokoro container initialized successfully", "containerID", containerID)

		defer func() {
			logger.Info("Cleaning up Kokoro containers...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cleanupManager := container.NewManager(envs.ContainerRuntime)
			if err := cleanupManager.CleanupContainer(ctx, container.KokoroContainer.ContainerID); err != nil {
				logger.Error("Failed to clean up Kokoro containers", "error", err)
			} else {
				logger.Info("All Kokoro containers cleaned up successfully")
			}
		}()
	}

	// Initialize the AI service singleton
	aiCompletionsService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)
	aiEmbeddingsService := ai.NewOpenAIService(envs.EmbeddingsAPIKey, envs.EmbeddingsAPIURL)
	chatStorage := chatrepository.NewRepository(logger, store.DB())

	// Start PostgreSQL using container runtime
	postgresCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	postgresManager, err := bootstrapPostgres(postgresCtx, logger, envs)
	if err != nil {
		logger.Error("Failed to start PostgreSQL with container manager", "error", err)
		panic(errors.Wrap(err, "Failed to start PostgreSQL with container manager"))
	}

	defer func() {
		manager := container.NewManager(envs.ContainerRuntime)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		options := container.DefaultPostgresOptions()
		if err := manager.StopContainer(shutdownCtx, options.ContainerName); err != nil {
			logger.Error("Error stopping PostgreSQL", "error", err)
		}

		if err := manager.RemoveContainer(shutdownCtx, options.ContainerName); err != nil {
			logger.Error("Error removing PostgreSQL container", "error", err)
		}
	}()

	if err := postgresManager.WaitForReady(postgresCtx, 30*time.Second); err != nil {
		logger.Error("Failed waiting for PostgreSQL to be ready", "error", err)
		panic(errors.Wrap(err, "PostgreSQL failed to become ready"))
	}

	dbName := "enchanted_twin"
	if err := postgresManager.EnsureDatabase(postgresCtx, dbName); err != nil {
		logger.Error("Failed to ensure database exists", "error", err)
		panic(errors.Wrap(err, "Unable to ensure database exists"))
	}

	connString := postgresManager.GetConnectionString(dbName)
	logger.Info(
		"PostgreSQL listening at",
		"connection",
		connString,
	)

	logger.Info("Testing direct database connection")
	if err := container.TestDbConnection(context.Background(), connString, logger); err != nil {
		logger.Warn("Database connection test failed", "error", err)
	}

	recreateMemDb := false
	mem, err := embeddingsmemory.NewEmbeddingsMemory(
		&embeddingsmemory.Config{
			Logger:              logger,
			PgString:            connString,
			AI:                  aiEmbeddingsService,
			Recreate:            recreateMemDb,
			EmbeddingsModelName: envs.EmbeddingsModel,
		},
	)
	if err != nil {
		panic(errors.Wrap(err, "Unable to create memory"))
	}

	ttsSvc, err := bootstrapTTS(logger)
	if err != nil {
		logger.Error("TTS bootstrap failed", "error", err)
		panic(errors.Wrap(err, "TTS bootstrap failed"))
	}
	go func() {
		if err := ttsSvc.Start(context.Background()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("TTS service stopped unexpectedly", "error", err)
			panic(errors.Wrap(err, "TTS service stopped unexpectedly"))
		}
	}()

	temporalClient, err := bootstrapTemporalServer(logger, envs)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal server"))
	}

	toolRegistry := tools.NewRegistry()

	if err := toolRegistry.Register(memory.NewMemorySearchTool(logger, mem)); err != nil {
		logger.Error("Failed to register memory search tool", "error", err)
	}

	// Initialize MCP Service with tool registry
	mcpRepo := mcpRepository.NewRepository(logger, store.DB())
	mcpService := mcpserver.NewService(context.Background(), *mcpRepo, store, toolRegistry)

	// Register standard tools
	standardTools := agent.RegisterStandardTools(
		toolRegistry,
		logger,
		envs.TelegramToken,
		store,
		temporalClient,
		envs.CompletionsModel,
		envs.TelegramChatServer,
	)

	// Create TwinChat service with minimal dependencies
	twinChatService := twinchat.NewService(
		logger,
		aiCompletionsService,
		chatStorage,
		nc,
		toolRegistry,
		store,
		envs.CompletionsModel,
	)

	// Register tools from the TwinChat service
	providerTools := agent.RegisterToolProviders(toolRegistry, logger, twinChatService)
	logger.Info("Standard tools registered", "count", len(standardTools))
	logger.Info("Provider tools registered", "count", len(providerTools))

	// MCP tools are automatically registered by the MCP service
	mcpTools, err := mcpService.GetInternalTools(context.Background())
	if err == nil {
		logger.Info("MCP tools available", "count", len(mcpTools))
	} else {
		logger.Warn("Failed to get MCP tools", "error", err)
	}

	logger.Info(
		"Tool registry initialized with all tools",
		"count",
		len(toolRegistry.List()),
		"names",
		toolRegistry.List(),
	)

	// Initialize and start the temporal worker
	temporalWorker, err := bootstrapTemporalWorker(
		&bootstrapTemporalWorkerInput{
			logger:               logger,
			temporalClient:       temporalClient,
			envs:                 envs,
			store:                store,
			nc:                   nc,
			ollamaClient:         ollamaClient,
			memory:               mem,
			aiCompletionsService: aiCompletionsService,
			toolsRegistry:        toolRegistry,
		},
	)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal worker"))
	}
	defer temporalWorker.Stop()

	telegramServiceInput := telegram.TelegramServiceInput{
		Logger:           logger,
		Token:            envs.TelegramToken,
		Client:           &http.Client{},
		Store:            store,
		AiService:        aiCompletionsService,
		CompletionsModel: envs.CompletionsModel,
		Memory:           mem,
		AuthStorage:      store,
		NatsClient:       nc,
		ChatServerUrl:    envs.TelegramChatServer,
		ToolsRegistry:    toolRegistry,
	}
	telegramService := telegram.NewTelegramService(telegramServiceInput)

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		appCtx, appCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer appCancel()

		for {
			select {
			case <-ticker.C:
				chatUUID, err := telegramService.GetChatUUID(context.Background())
				if err != nil {
					continue
				}
				err = telegramService.Subscribe(appCtx, chatUUID)

				if err == nil {
				} else if stderrs.Is(err, telegram.ErrSubscriptionNilTextMessage) {
				} else if stderrs.Is(err, context.Canceled) || stderrs.Is(err, context.DeadlineExceeded) {
					if appCtx.Err() != nil {
						return
					}
				}

			case <-appCtx.Done():
				logger.Info("Stopping Telegram subscription poller due to application shutdown signal")
				return
			}
		}
	}()

	router := bootstrapGraphqlServer(graphqlServerInput{
		logger:           logger,
		temporalClient:   temporalClient,
		port:             envs.GraphqlPort,
		twinChatService:  *twinChatService,
		natsClient:       nc,
		store:            store,
		aiService:        aiCompletionsService,
		mcpService:       mcpService,
		telegramService:  telegramService,
		containerManager: container.NewManager(envs.ContainerRuntime),
	})

	// Start HTTP server in a goroutine so it doesn't block signal handling
	go func() {
		logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+envs.GraphqlPort)
		err := http.ListenAndServe(":"+envs.GraphqlPort, router)
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", slog.Any("error", err))
			panic(errors.Wrap(err, "Unable to start server"))
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan
	logger.Info("Server shutting down...")
}

func bootstrapTemporalServer(logger *log.Logger, envs *config.Config) (client.Client, error) {
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, envs.DBPath)
	<-ready
	logger.Info("Temporal server started")

	temporalClient, err := bootstrap.NewTemporalClient(logger)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create temporal client")
	}
	logger.Info("Temporal client created")

	return temporalClient, nil
}

type bootstrapTemporalWorkerInput struct {
	logger               *log.Logger
	temporalClient       client.Client
	envs                 *config.Config
	store                *db.Store
	nc                   *nats.Conn
	ollamaClient         *ollamaapi.Client
	memory               memory.Storage
	toolsRegistry        tools.ToolRegistry
	aiCompletionsService *ai.Service
}

func bootstrapTTS(logger *log.Logger) (*tts.Service, error) {
	const (
		kokoroPort = 8765
		ttsWsPort  = 45001
	)

	engine := tts.Kokoro{
		Endpoint: fmt.Sprintf("http://localhost:%d/api/v1/tts/generate", kokoroPort),
		Model:    "kokoro",
		Voice:    "af_bella+af_heart",
	}
	svc := tts.New(fmt.Sprintf(":%d", ttsWsPort), engine, *logger)
	return svc, nil
}

func bootstrapTemporalWorker(
	input *bootstrapTemporalWorkerInput,
) (worker.Worker, error) {
	w := worker.New(input.temporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 1,
	})

	dataProcessingWorkflow := workflows.DataProcessingWorkflows{
		Logger:        input.logger,
		Config:        input.envs,
		Store:         input.store,
		Nc:            input.nc,
		OllamaClient:  input.ollamaClient,
		Memory:        input.memory,
		OpenAIService: input.aiCompletionsService,
	}
	dataProcessingWorkflow.RegisterWorkflowsAndActivities(&w)

	// Register auth activities
	authActivities := auth.NewOAuthActivities(input.store)
	authActivities.RegisterWorkflowsAndActivities(&w)

	// Register the planned agent v2 workflow
	aiAgent := agent.NewAgent(input.logger, input.nc, input.aiCompletionsService, input.envs.CompletionsModel, nil, nil)
	schedulerActivities := scheduler.NewTaskSchedulerActivities(input.logger, input.aiCompletionsService, aiAgent, input.toolsRegistry, input.envs.CompletionsModel)
	schedulerActivities.RegisterWorkflowsAndActivities(w)

	// Start the worker
	err := w.Start()
	if err != nil {
		input.logger.Error("Error starting worker", slog.Any("error", err))
		return nil, err
	}

	return w, nil
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
	containerManager       container.ContainerManager
}

func bootstrapGraphqlServer(input graphqlServerInput) *chi.Mux {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            false,
	}).Handler)

	srv := handler.New(gqlSchema(&graph.Resolver{
		Logger:                 input.logger,
		TemporalClient:         input.temporalClient,
		TwinChatService:        input.twinChatService,
		Nc:                     input.natsClient,
		Store:                  input.store,
		AiService:              input.aiService,
		MCPService:             input.mcpService,
		DataProcessingWorkflow: input.dataProcessingWorkflow,
		ContainerManager:       input.containerManager,
	}))
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

func bootstrapKokoro(envs *config.Config) (string, error) {
	log := logrus.WithField("component", "container-runtime-bootstrap")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	manager := container.NewManager(envs.ContainerRuntime)

	installed, err := manager.IsInstalled(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check if container runtime is installed: %w", err)
	}

	if !installed {
		log.Warn("Container runtime is not installed. Please install container runtime to enable container features.")
		return "", fmt.Errorf("container runtime is not installed")
	}

	machineExists, err := manager.IsMachineInstalled(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check if container runtime machine exists: %w", err)
	}

	if !machineExists {
		log.Warn("Container runtime machine does not exist. Please initialize a container runtime machine.")
		return "", fmt.Errorf("container runtime machine does not exist")
	}

	running, err := manager.IsMachineRunning(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check if container runtime is running: %w", err)
	}

	if !running {
		log.Warn("Container runtime machine is not running. Please start the container runtime machine.")
		return "", fmt.Errorf("container runtime machine is not running")
	}

	const containerName = "kokoro-fastapi"
	containerExists, containerId, err := manager.CheckContainerExists(ctx, containerName)
	if err != nil {
		log.Warn("Failed to check if container exists: ", err)
	}

	if containerExists {
		log.Info("Container already exists with ID: ", containerId)

		containerRunning, err := manager.IsContainerRunning(ctx, containerId)
		if err != nil {
			log.Warn("Failed to check if container is running: ", err)
		}

		if containerRunning {
			log.Info("Container is already running, reusing it")
			return containerId, nil
		}

		log.Info("Container exists but is not running, attempting to start it")
		err = manager.StartContainer(ctx, containerId)
		if err == nil {
			log.Info("Successfully started existing container")
			return containerId, nil
		}

		log.Warn("Failed to start existing container, removing it: ", err)
		err = manager.RemoveContainer(ctx, containerId)
		if err != nil {
			return "", fmt.Errorf("failed to remove existing container: %w", err)
		}
		log.Info("Removed existing container")
	}

	log.Info("Pulling the Kokoro image...")
	if err := manager.PullImage(ctx, container.KokoroContainer.ImageURL); err != nil {
		return "", fmt.Errorf("failed to pull Kokoro image: %w", err)
	}

	containerConfig := container.ContainerConfig{
		ImageURL:     container.KokoroContainer.ImageURL,
		Name:         container.KokoroContainer.ContainerID,
		Ports:        map[string]string{container.KokoroContainer.DefaultPort: container.KokoroContainer.DefaultPort},
		Environment:  map[string]string{},
		PullIfNeeded: true,
	}

	log.Info("Starting the Kokoro container...")
	containerID, err := manager.RunContainer(ctx, containerConfig)
	if err != nil {
		return "", fmt.Errorf("failed to start Kokoro container: %w", err)
	}

	log.WithField("containerId", containerID).
		WithField("port", "8765").
		Info("Kokoro container started successfully")

	return containerID, nil
}

func bootstrapPostgres(ctx context.Context, logger *log.Logger, envs *config.Config) (*container.PostgresManager, error) {
	options := container.DefaultPostgresOptions()
	options.Port = "15432"

	postgresManager := container.NewPostgresManager(logger, options, envs.ContainerRuntime)

	logger.Info("Starting PostgreSQL container with container...")
	_, err := postgresManager.StartPostgresContainer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL container with container runtime: %w", err)
	}

	return postgresManager, nil
}
