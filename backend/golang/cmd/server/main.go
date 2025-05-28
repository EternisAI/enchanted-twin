// Owner: august@eternis.ai
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi"
	"github.com/go-openapi/loads"
	"github.com/gorilla/websocket"
	flags "github.com/jessevdk/go-flags"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/adapters/handlers/rest"
	"github.com/weaviate/weaviate/adapters/handlers/rest/operations"
	"go.mau.fi/whatsmeow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
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
	"github.com/EternisAI/enchanted-twin/pkg/friend"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/tts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	whatsapp "github.com/EternisAI/enchanted-twin/pkg/whatsapp"
	waTools "github.com/EternisAI/enchanted-twin/pkg/whatsapp/tools"
)

func main() {
	whatsappQRChan := whatsapp.GetQRChannel()
	var currentWhatsAppQRCode *string
	whatsAppConnected := false
	var whatsappClient *whatsmeow.Client

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	envs, _ := config.LoadConfig(false)
	logger.Debug("Config loaded", "envs", envs)
	logger.Info("Using database path", "path", envs.DBPath)

	var ollamaClient *ollamaapi.Client
	if envs.OllamaBaseURL != "" {
		baseURL, err := url.Parse(envs.OllamaBaseURL)
		if err != nil {
			logger.Error("Failed to parse Ollama base URL", "error", err)
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

	go func() {
		for evt := range whatsappQRChan {
			switch evt.Event {
			case "code":
				qrCode := evt.Code
				currentWhatsAppQRCode = &qrCode
				whatsAppConnected = false
				logger.Info("Received new WhatsApp QR code, length:", "length", len(qrCode))

				qrCodeUpdate := map[string]interface{}{
					"event":        "code",
					"qr_code_data": qrCode,
					"is_connected": false,
					"timestamp":    time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(qrCodeUpdate)
				if err == nil {
					err = nc.Publish("whatsapp.qr_code", jsonData)
					if err != nil {
						logger.Error("Failed to publish WhatsApp QR code to NATS", "error", err)
					} else {
						logger.Info("Published WhatsApp QR code to NATS")
					}
				} else {
					logger.Error("Failed to marshal WhatsApp QR code data", "error", err)
				}
			case "success":
				whatsAppConnected = true
				currentWhatsAppQRCode = nil
				logger.Info("WhatsApp connection successful")

				whatsapp.StartSync()
				whatsapp.UpdateSyncStatus(whatsapp.SyncStatus{
					IsSyncing:      true,
					IsCompleted:    false,
					ProcessedItems: 0,
					TotalItems:     0,
					StatusMessage:  "Waiting for history sync to begin",
				})
				whatsapp.PublishSyncStatus(nc, logger) //nolint:errcheck

				successUpdate := map[string]interface{}{
					"event":        "success",
					"qr_code_data": nil,
					"is_connected": true,
					"timestamp":    time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(successUpdate)
				if err == nil {
					err = nc.Publish("whatsapp.qr_code", jsonData)
					if err != nil {
						logger.Error("Failed to publish WhatsApp connection success to NATS", "error", err)
					} else {
						logger.Info("Published WhatsApp connection success to NATS")
					}
				} else {
					logger.Error("Failed to marshal WhatsApp success data", "error", err)
				}
			}
		}
	}()

	store, err := db.NewStore(context.Background(), envs.DBPath)
	if err != nil {
		logger.Error("Unable to create or initialize database", "error", err)
		panic(errors.Wrap(err, "Unable to create or initialize database"))
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Error closing store", "error", err)
		}
	}()

	// Initialize the AI service singleton
	aiCompletionsService := ai.NewOpenAIService(logger, envs.CompletionsAPIKey, envs.CompletionsAPIURL)
	aiEmbeddingsService := ai.NewOpenAIService(logger, envs.EmbeddingsAPIKey, envs.EmbeddingsAPIURL)
	chatStorage := chatrepository.NewRepository(logger, store.DB())

	weaviatePath := filepath.Join(envs.AppDataPath, "weaviate")
	logger.Info("Starting Weaviate bootstrap process", "path", weaviatePath, "port", envs.WeaviatePort)
	weaviateBootstrapStart := time.Now()

	if _, err := bootstrapWeaviateServer(context.Background(), logger, envs.WeaviatePort, weaviatePath); err != nil {
		logger.Error("Failed to bootstrap Weaviate server", slog.Any("error", err))
		panic(errors.Wrap(err, "Failed to bootstrap Weaviate server"))
	}
	logger.Info("Weaviate server bootstrap completed", "elapsed", time.Since(weaviateBootstrapStart))

	logger.Info("Creating Weaviate client")
	clientCreateStart := time.Now()
	weaviateClient, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", envs.WeaviatePort),
		Scheme: "http",
	})
	if err != nil {
		logger.Error("Failed to create Weaviate client", "error", err)
		panic(errors.Wrap(err, "Failed to create Weaviate client"))
	}
	logger.Info("Weaviate client created", "elapsed", time.Since(clientCreateStart))

	logger.Info("Initializing Weaviate schema")
	schemaInitStart := time.Now()
	if err := InitSchema(weaviateClient, logger); err != nil {
		logger.Error("Failed to initialize Weaviate schema", "error", err)
		panic(errors.Wrap(err, "Failed to initialize Weaviate schema"))
	}
	logger.Info("Weaviate schema initialized", "elapsed", time.Since(schemaInitStart))

	logger.Info("Creating evolving memory instance")
	memoryCreateStart := time.Now()
	mem, err := evolvingmemory.New(logger, weaviateClient, aiCompletionsService, aiEmbeddingsService)
	if err != nil {
		logger.Error("Failed to create evolving memory", "error", err)
		panic(errors.Wrap(err, "Failed to create evolving memory"))
	}
	logger.Info("Evolving memory created", "elapsed", time.Since(memoryCreateStart))
	logger.Info("Total Weaviate setup completed", "total_elapsed", time.Since(weaviateBootstrapStart))

	whatsapp.TriggerConnect()

	whatsappClientChan := make(chan *whatsmeow.Client)
	go func() {
		client := whatsapp.BootstrapWhatsAppClient(mem, logger, nc, envs.DBPath)
		whatsappClientChan <- client
	}()

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

	// Tools
	toolRegistry := tools.NewRegistry()

	if err := toolRegistry.Register(memory.NewMemorySearchTool(logger, mem)); err != nil {
		logger.Error("Failed to register memory search tool", "error", err)
	}

	if err := toolRegistry.Register(&schedulerTools.ScheduleTask{
		Logger:         logger,
		TemporalClient: temporalClient,
	}); err != nil {
		logger.Error("Failed to register schedule task tool", "error", err)
		panic(errors.Wrap(err, "Failed to register schedule task tool"))
	}

	go func() {
		logger.Info("Waiting for WhatsApp client to register tool...")
		whatsappClient = <-whatsappClientChan
		if whatsappClient != nil {
			if err := toolRegistry.Register(waTools.NewWhatsAppTool(logger, whatsappClient)); err != nil {
				logger.Error("Failed to register WhatsApp tool", "error", err)
			} else {
				logger.Info("WhatsApp tools registered")
			}
		}
	}()

	telegramTool, err := telegram.NewTelegramSetupTool(logger, envs.TelegramToken, store, envs.TelegramChatServer)
	if err != nil {
		panic(errors.Wrap(err, "Failed to create telegram setup tool"))
	}
	err = toolRegistry.Register(telegramTool)
	if err != nil {
		panic(errors.Wrap(err, "Failed to register telegram tool"))
	}

	twinChatService := twinchat.NewService(
		logger,
		aiCompletionsService,
		chatStorage,
		nc,
		mem,
		toolRegistry,
		store,
		envs.CompletionsModel,
		envs.ReasoningModel,
	)

	sendToChatTool := twinchat.NewSendToChatTool(chatStorage, nc)
	err = toolRegistry.Register(sendToChatTool)
	if err != nil {
		logger.Error("Failed to register send to chat tool", "error", err)
		panic(errors.Wrap(err, "Failed to register send to chat tool"))
	}

	// Initialize MCP Service with tool registry
	mcpService := mcpserver.NewService(context.Background(), logger, store, toolRegistry)

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

	for _, toolName := range toolRegistry.List() {
		logger.Info("Tool registered", "name", toolName)
	}

	notificationsSvc := notifications.NewService(nc)

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
			notifications:        notificationsSvc,
			twinchatService:      twinChatService,
		},
	)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal worker"))
	}
	defer temporalWorker.Stop()

	if err := bootstrapPeriodicWorkflows(logger, temporalClient); err != nil {
		logger.Error("Failed to bootstrap periodic workflows", "error", err)
		panic(errors.Wrap(err, "Failed to bootstrap periodic workflows"))
	}

	identitySvc := identity.NewIdentityService(temporalClient)
	personality, err := identitySvc.GetPersonality(context.Background())
	if err != nil {
		logger.Error("Failed to get personality", "error", err)
		panic(errors.Wrap(err, "Failed to get personality"))
	}
	logger.Info("Personality", "personality", personality)

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

	go telegram.SubscribePoller(telegramService, logger)
	go telegram.MonitorAndRegisterTelegramTool(context.Background(), telegramService, logger, toolRegistry, store, envs)

	router := bootstrapGraphqlServer(graphqlServerInput{
		logger:            logger,
		temporalClient:    temporalClient,
		port:              envs.GraphqlPort,
		twinChatService:   *twinChatService,
		natsClient:        nc,
		store:             store,
		aiService:         aiCompletionsService,
		mcpService:        mcpService,
		telegramService:   telegramService,
		whatsAppQRCode:    currentWhatsAppQRCode,
		whatsAppConnected: whatsAppConnected,
	})

	go func() {
		logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+envs.GraphqlPort)
		err := http.ListenAndServe(":"+envs.GraphqlPort, router)
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
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
	notifications        *notifications.Service
	twinchatService      *twinchat.Service
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
	aiAgent := agent.NewAgent(input.logger, input.nc, input.aiCompletionsService, input.envs.CompletionsModel, input.envs.ReasoningModel, nil, nil)
	schedulerActivities := scheduler.NewTaskSchedulerActivities(input.logger, input.aiCompletionsService, aiAgent, input.toolsRegistry, input.envs.CompletionsModel, input.store, input.notifications)
	schedulerActivities.RegisterWorkflowsAndActivities(w)

	// Register identity activities
	identityActivities := identity.NewIdentityActivities(input.logger, input.memory, input.aiCompletionsService, input.envs.CompletionsModel)
	identityActivities.RegisterWorkflowsAndActivities(w)

	friendService := friend.NewFriendService(friend.FriendServiceConfig{
		Logger:          input.logger,
		MemoryService:   input.memory,
		IdentityService: identity.NewIdentityService(input.temporalClient),
		TwinchatService: input.twinchatService,
		AiService:       input.aiCompletionsService,
		ToolRegistry:    input.toolsRegistry,
	})
	friendService.RegisterWorkflowsAndActivities(&w, input.temporalClient)
	err := w.Start()
	if err != nil {
		input.logger.Error("Error starting worker", "error", err)
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

func bootstrapWeaviateServer(ctx context.Context, logger *log.Logger, port string, dataPath string) (*rest.Server, error) {
	startTime := time.Now()
	logger.Info("Starting Weaviate server bootstrap", "port", port, "dataPath", dataPath)

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		logger.Info("Creating Weaviate data directory", "path", dataPath)
		if err := os.MkdirAll(dataPath, 0o755); err != nil {
			return nil, errors.Wrap(err, "Failed to create Weaviate data directory")
		}
		logger.Info("Weaviate data directory created", "elapsed", time.Since(startTime))
	} else {
		logger.Info("Weaviate data directory exists", "path", dataPath, "elapsed", time.Since(startTime))
	}

	logger.Debug("Setting PERSISTENCE_DATA_PATH environment variable", "path", dataPath)
	err := os.Setenv("PERSISTENCE_DATA_PATH", dataPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set PERSISTENCE_DATA_PATH")
	}
	logger.Debug("PERSISTENCE_DATA_PATH set", "elapsed", time.Since(startTime))

	logger.Debug("Loading Weaviate swagger specification")
	swaggerSpec, err := loads.Embedded(rest.SwaggerJSON, rest.FlatSwaggerJSON)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load swagger spec")
	}
	logger.Debug("Swagger specification loaded", "elapsed", time.Since(startTime))

	logger.Debug("Creating Weaviate API instance")
	api := operations.NewWeaviateAPI(swaggerSpec)
	api.Logger = func(s string, i ...any) {
		logger.Debug(s, i...)
	}
	server := rest.NewServer(api)
	logger.Debug("Weaviate API and server created", "elapsed", time.Since(startTime))

	logger.Debug("Configuring Weaviate server", "port", port)
	server.EnabledListeners = []string{"http"}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert port to int")
	}
	server.Port = p
	logger.Debug("Server port configured", "port", p, "elapsed", time.Since(startTime))

	logger.Debug("Setting up command line parser")
	parser := flags.NewParser(server, flags.Default)
	parser.ShortDescription = "Weaviate"
	server.ConfigureFlags()
	logger.Debug("Command line flags configured", "elapsed", time.Since(startTime))

	logger.Debug("Adding command line option groups")
	for i, optsGroup := range api.CommandLineOptionsGroups {
		logger.Debug("Adding option group", "index", i, "description", optsGroup.ShortDescription)
		_, err := parser.AddGroup(optsGroup.ShortDescription, optsGroup.LongDescription, optsGroup.Options)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to add flag group")
		}
	}
	logger.Debug("All option groups added", "elapsed", time.Since(startTime))

	logger.Debug("Parsing command line arguments")
	if _, err := parser.Parse(); err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			return nil, nil
		}
		return nil, err
	}
	logger.Debug("Command line arguments parsed", "elapsed", time.Since(startTime))

	logger.Debug("Configuring Weaviate API")
	server.ConfigureAPI()
	logger.Info("Weaviate API configured", "elapsed", time.Since(startTime))

	logger.Info("Starting Weaviate server goroutine")
	go func() {
		logger.Debug("Weaviate server.Serve() starting")
		if err := server.Serve(); err != nil && err != http.ErrServerClosed {
			logger.Error("Weaviate serve error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Debug("Context canceled, shutting down Weaviate server")
		_ = server.Shutdown()
	}()

	// Give the server a moment to start listening before beginning readiness checks
	time.Sleep(100 * time.Millisecond)

	readyURL := fmt.Sprintf("http://localhost:%d/v1/.well-known/ready", p)
	deadline := time.Now().Add(15 * time.Second)
	logger.Info("Waiting for Weaviate to become ready", "url", readyURL, "timeout", "15s")

	checkCount := 0
	for {
		checkCount++
		if time.Now().After(deadline) {
			logger.Error("Weaviate readiness timeout",
				"url", readyURL,
				"elapsed", time.Since(startTime),
				"checks_performed", checkCount)
			return nil, fmt.Errorf("weaviate did not become ready in time on %s", readyURL)
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, readyURL, nil)
		resp, err := http.DefaultClient.Do(req)

		if err != nil {
			// Log connection errors more frequently for better debugging
			if checkCount <= 5 || checkCount%5 == 0 {
				logger.Debug("Weaviate readiness check failed",
					"error", err,
					"attempt", checkCount,
					"elapsed", time.Since(startTime))
			}
		} else {
			// Always close the response body to prevent resource leaks
			defer func() {
				if resp != nil && resp.Body != nil {
					resp.Body.Close() //nolint:errcheck
				}
			}()

			if resp.StatusCode == http.StatusOK {
				logger.Info("Weaviate server is ready",
					"elapsed", time.Since(startTime),
					"checks_performed", checkCount)
				return server, nil
			} else {
				// Log non-OK status responses more frequently for better debugging
				if checkCount <= 5 || checkCount%5 == 0 {
					logger.Debug("Weaviate not ready yet",
						"status_code", resp.StatusCode,
						"attempt", checkCount,
						"elapsed", time.Since(startTime))
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// ClassExists checks if a class already exists in Weaviate.
func ClassExists(client *weaviate.Client, className string) (bool, error) {
	schema, err := client.Schema().Getter().Do(context.Background())
	if err != nil {
		return false, err
	}
	for _, class := range schema.Classes {
		if class.Class == className {
			return true, nil
		}
	}
	return false, nil
}

func InitSchema(client *weaviate.Client, logger *log.Logger) error {
	logger.Debug("Starting schema initialization")
	start := time.Now()

	if err := evolvingmemory.EnsureSchemaExistsInternal(client, logger); err != nil {
		return err
	}

	logger.Debug("Schema initialization completed", "elapsed", time.Since(start))
	return nil
}

func bootstrapPeriodicWorkflows(logger *log.Logger, temporalClient client.Client) error {
	err := helpers.CreateScheduleIfNotExists(logger, temporalClient, identity.PersonalityWorkflowID, time.Hour, identity.DerivePersonalityWorkflow, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create identity personality workflow")
	}
	return nil
}
