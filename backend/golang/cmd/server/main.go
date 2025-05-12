package main

import (
	"context"
	"encoding/json"
	stderrs "errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	tdlibclient "github.com/zelenin/go-tdlib/client"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/embeddingsmemory"
	plannedv2 "github.com/EternisAI/enchanted-twin/pkg/agent/planned-v2"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	mcpRepository "github.com/EternisAI/enchanted-twin/pkg/mcpserver/repository"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	whatsapp "github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

func main() {
	whatsappQRChan := whatsapp.GetQRChannel()
	var currentWhatsAppQRCode *string
	whatsAppConnected := false

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	envs, _ := config.LoadConfig(true)
	logger.Debug("Config loaded", "envs", envs)
	logger.Info("Using database path", "path", envs.DBPath)

	var tdlib *tdlibclient.Client
	if envs.TelegramTDLibAPIID != 0 && envs.TelegramTDLibAPIHash != "" {
		var err error
		tdlib, err = bootstrapTelegramTDLib(logger, envs.TelegramTDLibAPIID, envs.TelegramTDLibAPIHash)
		if err != nil {
			logger.Warn("Failed to initialize TDLib Telegram client", "error", err)
		} else {
			logger.Info("TDLib Telegram client initialized successfully")

			defer tdlib.Close()
		}
	} else {
		logger.Info("TDLib Telegram integration not enabled. To enable, set TELEGRAM_TDLIB_API_ID and TELEGRAM_TDLIB_API_HASH environment variables. You can obtain these from https://my.telegram.org/apps")
	}

	var ollamaClient *ollamaapi.Client
	if envs.OllamaBaseURL != "" {
		baseURL, err := url.Parse(envs.OllamaBaseURL)
		if err != nil {
			logger.Error("Failed to parse Ollama base URL", slog.Any("error", err))
		} else {
			ollamaClient = ollamaapi.NewClient(baseURL, http.DefaultClient)
		}
	}

	postgresCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var postgresService *bootstrap.PostgresService
	postgresService, err := bootstrapPostgres(postgresCtx, logger)
	if err != nil {
		logger.Warn("Failed to start PostgreSQL, continuing without it", "error", err)
	}

	defer func() {
		if postgresService != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := postgresService.Stop(shutdownCtx); err != nil {
				logger.Error("Error stopping PostgreSQL", slog.Any("error", err))
			}
			if err := postgresService.Remove(shutdownCtx); err != nil {
				logger.Error("Error removing PostgreSQL container", slog.Any("error", err))
			}
		}
	}()

	var natsServer *server.Server
	natsServer, err = bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		logger.Warn("Failed to start NATS server, continuing without it", "error", err)
	} else {
		defer natsServer.Shutdown()
		logger.Info("NATS server started")
	}
	var nc *nats.Conn
	if natsServer != nil {
		nc, err = bootstrap.NewNatsClient()
		if err != nil {
			logger.Warn("Failed to create NATS client, continuing without it", "error", err)
		} else {
			logger.Info("NATS client started")
		}
	} else {
		logger.Warn("Skipping NATS client initialization as NATS server is not available")
	}

	// Move WhatsApp NATS code here after nc is initialized
	if nc != nil {
		go func() {
			for evt := range whatsappQRChan {
				if evt.Event == "code" {
					qrCode := evt.Code
					currentWhatsAppQRCode = &qrCode
					whatsAppConnected = false
					logger.Info("Received new WhatsApp QR code", "length", len(qrCode))

					// Publish QR code event to NATS for subscriptions
					qrCodeUpdate := map[string]interface{}{
						"event":        "code",
						"qr_code_data": qrCode,
						"is_connected": false,
						"timestamp":    time.Now().Format(time.RFC3339),
					}
					jsonData, err := json.Marshal(qrCodeUpdate)
					if err == nil {
						nc.Publish("whatsapp.qr_code", jsonData)
						logger.Info("Published WhatsApp QR code to NATS")
					} else {
						logger.Error("Failed to marshal WhatsApp QR code data", "error", err)
					}
				} else if evt.Event == "success" {
					whatsAppConnected = true
					currentWhatsAppQRCode = nil
					logger.Info("WhatsApp connection successful")

					// Publish success event to NATS for subscriptions
					successUpdate := map[string]interface{}{
						"event":        "success",
						"qr_code_data": nil,
						"is_connected": true,
						"timestamp":    time.Now().Format(time.RFC3339),
					}
					jsonData, err := json.Marshal(successUpdate)
					if err == nil {
						nc.Publish("whatsapp.qr_code", jsonData)
						logger.Info("Published WhatsApp connection success to NATS")
					} else {
						logger.Error("Failed to marshal WhatsApp success data", "error", err)
					}
				}
			}
		}()
	} else {
		logger.Warn("Skipping WhatsApp QR code handling as NATS client is not available")
	}

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

	aiCompletionsService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)
	aiEmbeddingsService := ai.NewOpenAIService(envs.EmbeddingsAPIKey, envs.EmbeddingsAPIURL)
	chatStorage := chatrepository.NewRepository(logger, store.DB())

	dbName := "enchanted_twin"

	if postgresService != nil {
		if err := postgresService.WaitForReady(postgresCtx, 30*time.Second); err != nil {
			logger.Warn("Failed waiting for PostgreSQL to be ready", "error", err)
		} else {
			if err := postgresService.EnsureDatabase(postgresCtx, dbName); err != nil {
				logger.Warn("Failed to ensure database exists", "error", err)
			} else {
				logger.Info(
					"PostgreSQL listening at",
					"connection",
					postgresService.GetConnectionString(dbName),
				)
			}
		}
	} else {
		logger.Warn("PostgreSQL service not available, continuing without it")
	}
	var mem memory.Storage
	if postgresService != nil {
		logger.Info(
			"PostgreSQL listening at",
			"connection",
			postgresService.GetConnectionString(dbName),
		)

		recreateMemDb := false
		var memErr error
		mem, memErr = embeddingsmemory.NewEmbeddingsMemory(
			&embeddingsmemory.Config{
				Logger:              logger,
				PgString:            postgresService.GetConnectionString(dbName),
				AI:                  aiEmbeddingsService,
				Recreate:            recreateMemDb,
				EmbeddingsModelName: envs.EmbeddingsModel,
			},
		)
		if memErr != nil {
			logger.Warn("Unable to create memory, continuing without it", "error", memErr)
		}
	} else {
		logger.Warn("Skipping memory initialization as PostgreSQL is not available")
	}

	if mem != nil {
		go bootstrapWhatsAppClient(mem, logger)
	} else {
		logger.Warn("Skipping WhatsApp client initialization as memory is not available")
	}

	var temporalClient client.Client
	temporalClient, err = bootstrapTemporalServer(logger, envs)
	if err != nil {
		logger.Warn("Failed to start Temporal server, continuing without it", "error", err)
	}

	mcpRepo := mcpRepository.NewRepository(logger, store.DB())
	mcpService := mcpserver.NewService(context.Background(), *mcpRepo, store)

	toolRegistry := tools.NewRegistry()

	standardTools := agent.RegisterStandardTools(
		toolRegistry,
		logger,
		envs.TelegramToken,
		store,
		temporalClient,
		envs.CompletionsModel,
		envs.TelegramChatServer,
	)

	twinChatService := twinchat.NewService(
		logger,
		aiCompletionsService,
		chatStorage,
		nc,
		toolRegistry,
		envs.CompletionsModel,
	)

	providerTools := agent.RegisterToolProviders(toolRegistry, logger, twinChatService)
	logger.Info("Standard tools registered", "count", len(standardTools))
	logger.Info("Provider tools registered", "count", len(providerTools))

	mcpTools, err := mcpService.GetInternalTools(context.Background())
	if err == nil {
		registeredMCPTools := agent.RegisterMCPTools(toolRegistry, mcpTools)
		logger.Info("MCP tools registered", "count", len(registeredMCPTools))
	} else {
		logger.Warn("Failed to get MCP tools", "error", err)
	}

	var temporalWorker worker.Worker
	if temporalClient != nil && mem != nil {
		temporalWorker, err = bootstrapTemporalWorker(
			&bootstrapTemporalWorkerInput{
				logger:               logger,
				temporalClient:       temporalClient,
				envs:                 envs,
				store:                store,
				nc:                   nc,
				ollamaClient:         ollamaClient,
				memory:               mem,
				aiCompletionsService: aiCompletionsService,
				registry:             toolRegistry,
			},
		)
		if err != nil {
			logger.Warn("Failed to start Temporal worker, continuing without it", "error", err)
		} else {
			defer temporalWorker.Stop()
		}
	} else {
		logger.Warn("Skipping Temporal worker initialization as dependencies are not available")
	}

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
	}
	telegramService := telegram.NewTelegramService(telegramServiceInput)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		appCtx, appCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer appCancel()

		for {
			select {
			case <-ticker.C:
				chatUUID, err := telegramService.GetChatUUID(context.Background())
				if err != nil {
					logger.Error("Error getting chat UUID", slog.Any("error", err))
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
		tdlibClient:       tdlib,
	})

	go func() {
		logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+envs.GraphqlPort)
		err := http.ListenAndServe(":"+envs.GraphqlPort, router)
		if err != nil && err != http.ErrServerClosed {
			if strings.Contains(err.Error(), "address already in use") {
				logger.Warn("GraphQL HTTP server port already in use, trying alternative port")
				altPort := strconv.Itoa(3001 + rand.Intn(100)) // Random port between 3001-3100
				logger.Info("Trying alternative port for GraphQL HTTP server", "address", "http://localhost:"+altPort)
				err = http.ListenAndServe(":"+altPort, router)
				if err != nil {
					logger.Error("HTTP server error on alternative port", slog.Any("error", err))
					logger.Warn("Continuing without GraphQL HTTP server")
					return
				}
			} else {
				logger.Error("HTTP server error", slog.Any("error", err))
				logger.Warn("Continuing without GraphQL HTTP server")
				return
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan
	logger.Info("Server shutting down...")
}

func bootstrapPostgres(
	ctx context.Context,
	logger *log.Logger,
) (*bootstrap.PostgresService, error) {
	options := bootstrap.DefaultPostgresOptions()

	postgresService, err := bootstrap.NewPostgresService(logger, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL service: %w", err)
	}

	logger.Info("Starting PostgreSQL service...")
	err = postgresService.Start(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL service: %w", err)
	}

	return postgresService, nil
}

func bootstrapTemporalServer(logger *log.Logger, envs *config.Config) (client.Client, error) {
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, envs.DBPath)
	<-ready
	logger.Info("Temporal server started")

	temporalClient, err := bootstrap.CreateTemporalClient(
		"localhost:7233",
		bootstrap.TemporalNamespace,
		"",
	)
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
	registry             tools.ToolRegistry
	aiCompletionsService *ai.Service
}

func bootstrapTemporalWorker(
	input *bootstrapTemporalWorkerInput,
) (worker.Worker, error) {
	w := worker.New(input.temporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 1,
	})

	dataProcessingWorkflow := workflows.DataProcessingWorkflows{
		Logger:       input.logger,
		Config:       input.envs,
		Store:        input.store,
		Nc:           input.nc,
		OllamaClient: input.ollamaClient,
		Memory:       input.memory,
	}
	dataProcessingWorkflow.RegisterWorkflowsAndActivities(&w)

	authActivities := auth.NewOAuthActivities(input.store)
	authActivities.RegisterWorkflowsAndActivities(&w)

	agentActivities := plannedv2.NewAgentActivities(context.Background(), input.aiCompletionsService, input.registry)
	agentActivities.RegisterPlannedAgentWorkflow(w, input.logger)
	input.logger.Info("Registered planned agent workflow", "tools", input.registry.List())

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
	whatsAppQRCode         *string
	whatsAppConnected      bool
	tdlibClient            *tdlibclient.Client
}

func bootstrapGraphqlServer(input graphqlServerInput) *chi.Mux {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            false,
	}).Handler)

	// Create resolver with pointers to whatsApp fields so they stay updated
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

func bootstrapWhatsAppClient(memoryStorage memory.Storage, logger *log.Logger) {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(whatsapp.EventHandler(memoryStorage))

	fmt.Println("Waiting for WhatsApp connection signal...")
	connectChan := whatsapp.GetConnectChannel()
	<-connectChan
	fmt.Println("Received signal to start WhatsApp connection")

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			fmt.Println("Error connecting to WhatsApp", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrEvent := whatsapp.QRCodeEvent{
					Event: evt.Event,
					Code:  evt.Code,
				}
				whatsapp.SetLatestQREvent(qrEvent)
				whatsappQRChan := whatsapp.GetQRChannel()
				select {
				case whatsappQRChan <- qrEvent:
				default:
					fmt.Println("Warning: QR channel buffer full, dropping event")
				}
				fmt.Println("QR code:", evt.Code)
				fmt.Println("Received new WhatsApp QR code")
			} else if evt.Event == "success" {
				qrEvent := whatsapp.QRCodeEvent{
					Event: "success",
					Code:  "",
				}
				whatsapp.SetLatestQREvent(qrEvent)
				whatsapp.GetQRChannel() <- qrEvent
				fmt.Println("Login successful!")
				fmt.Println("WhatsApp connection successful")
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			fmt.Println("Error connecting to WhatsApp", err)
		} else {
			qrEvent := whatsapp.QRCodeEvent{
				Event: "success",
				Code:  "",
			}
			whatsapp.SetLatestQREvent(qrEvent)
			whatsapp.GetQRChannel() <- qrEvent
			fmt.Println("Already logged in, reusing session")
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

func bootstrapTelegramTDLib(logger *log.Logger, apiID int32, apiHash string) (*tdlibclient.Client, error) {
	logger.Info("Initializing TDLib Telegram client")

	dbDir := "./tdlib-db"
	filesDir := "./tdlib-files"

	os.MkdirAll(dbDir, 0o755)
	os.MkdirAll(filesDir, 0o755)

	_, err := os.Stat(dbDir + "/db.sqlite")
	if os.IsNotExist(err) {
		logger.Info("No existing authentication data found")
		logger.Info("Will perform fresh authentication with phone number: +33616874598")
	} else {
		logger.Info("Found existing authentication data - will try to reuse it")
	}

	_, err = tdlibclient.SetLogVerbosityLevel(&tdlibclient.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 2,
	})
	if err != nil {
		logger.Error("SetLogVerbosityLevel error", "error", err)
		return nil, errors.Wrap(err, "SetLogVerbosityLevel error")
	}

	tdlibParameters := &tdlibclient.SetTdlibParametersRequest{
		UseTestDc:           false,
		DatabaseDirectory:   dbDir,
		FilesDirectory:      filesDir,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseMessageDatabase:  true,
		UseSecretChats:      false,
		ApiId:               int32(apiID),
		ApiHash:             apiHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "0.1.0",
	}

	authorizer := tdlibclient.ClientAuthorizer(tdlibParameters)

	// authChan := make(chan bool, 1)
	go func() {
		logger.Info("Starting CLI interactor for Telegram authentication")
		logger.Info("When prompted, enter phone number: 33616874598 (without the + sign)")
		logger.Info("Then enter the verification code sent to your Telegram app")
		// tdlibclient.CliInteractor(authorizer)

		for {
			select {
			case state, _ := <-authorizer.State:
				logger.Info("Authorization state", "state", state)

				if state == nil {
					logger.Info("Authorization state is nil")
					return
				}

				switch state.AuthorizationStateType() {
				case tdlibclient.TypeAuthorizationStateWaitPhoneNumber:

					logger.Info("Sending phone number")
					authorizer.PhoneNumber <- "33616874598"

				case tdlibclient.TypeAuthorizationStateWaitCode:
					logger.Info("Waiting for code")
					var code string

					fmt.Println("Enter code: ")
					fmt.Scanln(&code)

					logger.Info("Sending code")
					authorizer.Code <- code

				case tdlibclient.TypeAuthorizationStateWaitPassword:
					logger.Info("Waiting for password")
					fmt.Println("Enter password: ")
					var password string
					fmt.Scanln(&password)

					logger.Info("Sending password")
					authorizer.Password <- password

				case tdlibclient.TypeAuthorizationStateReady:
					logger.Info("Authorization state ready")

				}
			}
		}
	}()

	logger.Info("Creating new TDLib client - this may take a moment...")

	clientCh := make(chan struct {
		client *tdlibclient.Client
		err    error
	}, 1)

	go func() {
		client, err := tdlibclient.NewClient(authorizer)
		clientCh <- struct {
			client *tdlibclient.Client
			err    error
		}{client, err}
	}()

	var client *tdlibclient.Client
	select {
	case result := <-clientCh:
		if result.err != nil {
			logger.Error("NewClient error", "error", result.err)
			return nil, errors.Wrap(result.err, "NewClient error")
		}
		client = result.client
	case <-time.After(120 * time.Second):
		logger.Error("Client creation timed out")
		return nil, errors.New("client creation timed out")
	}

	logger.Info("TDLib client created successfully")

	me, getMeErr := client.GetMe()
	if getMeErr != nil {
		logger.Warn("GetMe error - authentication may be incomplete", "error", getMeErr)
		logger.Info("Check the CLI for authentication prompts")
	} else {
		logger.Info("Logged in to Telegram", "first_name", me.FirstName, "last_name", me.LastName)
	}

	logger.Info("Fetching historical messages from all chats...")
	if err := FetchHistoricalMessages(client, logger, 50); err != nil {
		logger.Warn("Failed to fetch historical messages", "error", err)
	}

	go handleTDLibUpdates(client, logger)

	return client, nil
}

func handleTDLibUpdates(client *tdlibclient.Client, logger *log.Logger) {
	listener := client.GetListener()
	defer listener.Close()

	for update := range listener.Updates {
		if update == nil {
			continue
		}

		switch updateType := update.(type) {
		case *tdlibclient.UpdateNewMessage:
			message := updateType.Message
			logger.Info("New message received", "message_id", message.Id)

			if messageText, ok := message.Content.(*tdlibclient.MessageText); ok {
				logger.Info("Text message", "text", messageText.Text.Text)
			}

		case *tdlibclient.UpdateMessageContent:
			logger.Info("Message content updated", "message_id", updateType.MessageId)

		case *tdlibclient.UpdateMessageSendSucceeded:
			logger.Info("Message sent successfully", "message_id", updateType.Message.Id)
		}
	}
}

// FetchHistoricalMessages retrieves historical messages from all chats
func FetchHistoricalMessages(client *tdlibclient.Client, logger *log.Logger, limit int) error {
	chats, err := client.GetChats(&tdlibclient.GetChatsRequest{
		Limit: 500,
	})
	if err != nil {
		logger.Error("Failed to get chats", "error", err)
		return err
	}

	logger.Info("Found chats", "count", len(chats.ChatIds))

	for _, chatID := range chats.ChatIds {

		chat, err := client.GetChat(&tdlibclient.GetChatRequest{
			ChatId: chatID,
		})
		if err != nil {
			logger.Error("Failed to get chat info", "chat_id", chatID, "error", err)
			continue
		}

		logger.Info("Getting history for chat", "chat_id", chatID, "title", chat.Title)

		var allMessages []*tdlibclient.Message
		var oldestMessageID int64 = 0
		var iterations int = 0
		var batchSize int32 = 20

		for len(allMessages) < limit && iterations < 3 {
			iterations++

			historyRequest := &tdlibclient.GetChatHistoryRequest{
				ChatId:        chatID,
				Limit:         batchSize,
				OnlyLocal:     false,
				FromMessageId: oldestMessageID,
			}

			if oldestMessageID != 0 {
				historyRequest.Offset = -1
			}

			chatHistory, err := client.GetChatHistory(historyRequest)
			if err != nil {
				logger.Error("Failed to get chat history batch", "chat_id", chatID, "iteration", iterations, "error", err)
				break
			}

			if len(chatHistory.Messages) == 0 {
				logger.Info("No more messages to fetch", "chat_id", chatID, "iteration", iterations)
				break
			}

			for _, msg := range chatHistory.Messages {
				if msg != nil {
					allMessages = append(allMessages, msg)

					if oldestMessageID == 0 || msg.Id < oldestMessageID {
						oldestMessageID = msg.Id
					}
				}
			}

			// Avoid rate limiting
			time.Sleep(300 * time.Millisecond)
		}

		logger.Info("Total messages retrieved", "chat_id", chatID, "count", len(allMessages))

		for _, message := range allMessages {
			if message == nil {
				continue
			}

			// if messageText, ok := message.Content.(*tdlibclient.MessageText); ok {
			// 	logger.Info("Message text",
			// 		"message_id", message.Id,
			// 		"text", messageText.Text.Text,
			// 	)
			// }
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}
