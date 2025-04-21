package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/graphmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	indexing "github.com/EternisAI/enchanted-twin/pkg/indexing"

	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"

	"github.com/EternisAI/enchanted-twin/graph"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	ollamaapi "github.com/ollama/ollama/api"
)

// bootstrapPostgres initializes and starts a PostgreSQL service
func bootstrapPostgres(ctx context.Context, logger *log.Logger) (*bootstrap.PostgresService, error) {
	// Get default options
	options := bootstrap.DefaultPostgresOptions()

	// Create and start PostgreSQL service
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

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	recreateMemDb := flag.Bool("recreate-mem-db", false, "Recreate the postgres memory database")
	flag.Parse()

	envs, _ := config.LoadConfig(true)
	logger.Debug("Config loaded", "envs", envs)

	logger.Info("Using database path", "path", envs.DBPath)

	ollamaClient, err := ollamaapi.ClientFromEnvironment()
	if err != nil {
		panic(errors.Wrap(err, "Unable to create ollama client"))
	}

	// Start PostgreSQL
	postgresCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	postgresService, err := bootstrapPostgres(postgresCtx, logger)
	if err != nil {
		logger.Error("Failed to start PostgreSQL", slog.Any("error", err))
		panic(errors.Wrap(err, "Failed to start PostgreSQL"))
	}

	// Set up cleanup on shutdown
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Stop the container
		if err := postgresService.Stop(shutdownCtx); err != nil {
			logger.Error("Error stopping PostgreSQL", slog.Any("error", err))
		}
		// Remove the container to ensure clean state for next startup
		if err := postgresService.Remove(shutdownCtx); err != nil {
			logger.Error("Error removing PostgreSQL container", slog.Any("error", err))
		}
	}()

	_, err = bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start nats server"))
	}
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

	if err := postgresService.WaitForReady(postgresCtx, 60*time.Second); err != nil {
		logger.Error("Failed waiting for PostgreSQL to be ready", slog.Any("error", err))
		panic(errors.Wrap(err, "PostgreSQL failed to become ready"))
	}
	aiService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)
	chatStorage := chatrepository.NewRepository(logger, store.DB())

	// Ensure enchanted_twin database exists
	dbName := "enchanted_twin"
	if err := postgresService.EnsureDatabase(postgresCtx, dbName); err != nil {
		logger.Error("Failed to ensure database exists", slog.Any("error", err))
		panic(errors.Wrap(err, "Unable to ensure database exists"))
	}

	logger.Info("PostgreSQL listening at", "connection", postgresService.GetConnectionString(dbName))
	memory, err := graphmemory.NewGraphMemory(logger, postgresService.GetConnectionString(dbName), aiService, *recreateMemDb, envs.CompletionsModel)
	if err != nil {
		panic(errors.Wrap(err, "Unable to create graph memory"))
	}

	temporalClient, err := bootstrapTemporal(logger, envs, store, nc, ollamaClient, memory)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal"))
	}

	twinChatService := twinchat.NewService(logger, aiService, chatStorage, nc, memory)

	router := bootstrapGraphqlServer(graphqlServerInput{
		logger:          logger,
		temporalClient:  temporalClient,
		port:            envs.GraphqlPort,
		twinChatService: twinChatService,
		natsClient:      nc,
		store:           store,
	})

	// Start HTTP server in a goroutine so it doesn't block signal handling
	go func() {
		logger.Info("Starting GraphQL HTTP server", "port", envs.GraphqlPort)
		err := http.ListenAndServe(":"+envs.GraphqlPort, router)
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", slog.Any("error", err))
			panic(errors.Wrap(err, "Unable to start server"))
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-signalChan
	logger.Info("Server shutting down...")
}

func bootstrapTemporal(logger *log.Logger, envs *config.Config, store *db.Store, nc *nats.Conn, ollamaClient *ollamaapi.Client, memory memory.Storage) (client.Client, error) {
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, envs.DBPath)
	<-ready
	logger.Info("Temporal server started")

	temporalClient, err := bootstrap.CreateTemporalClient("localhost:7233", bootstrap.TemporalNamespace, "")
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create temporal client")
	}
	logger.Info("Temporal client created")

	w := worker.New(temporalClient, "default", worker.Options{
		MaxConcurrentActivityExecutionSize: 1,
	})

	indexingWorkflow := indexing.IndexingWorkflow{
		Logger:       logger,
		Config:       envs,
		Store:        store,
		Nc:           nc,
		OllamaClient: ollamaClient,
		Memory:       memory,
	}
	indexingWorkflow.RegisterWorkflowsAndActivities(&w)

	err = w.Start()
	if err != nil {
		logger.Error("Error starting worker", slog.Any("error", err))
		return nil, err
	}

	return temporalClient, nil
}

type graphqlServerInput struct {
	logger          *log.Logger
	temporalClient  client.Client
	port            string
	twinChatService *twinchat.Service
	natsClient      *nats.Conn
	store           *db.Store
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
		Logger:          input.logger,
		TemporalClient:  input.temporalClient,
		TwinChatService: *input.twinChatService,
		Nc:              input.natsClient,
		Store:           input.store,
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
			input.logger.Error("gql error", "operation_name", oc.OperationName, "raw_query", oc.RawQuery, "errors", resp.Errors)
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
