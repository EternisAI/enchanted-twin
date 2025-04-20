package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
func bootstrapPostgres(ctx context.Context, logger *slog.Logger) (*bootstrap.PostgresService, error) {
	// Configure PostgreSQL options
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Set up PostgreSQL options with default values
	options := bootstrap.PostgresOptions{
		Version:       "15",
		Port:          "5432",
		DataPath:      filepath.Join(cwd, "postgres-data"),
		User:          "postgres",
		Password:      "postgres",
		Database:      "enchanted_twin",
		ContainerName: "enchanted-twin-postgres",
	}

	// Create and start PostgreSQL service
	postgresService, err := bootstrap.StartPostgresContainer(ctx, logger, options)
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL: %w", err)
	}

	logger.Info("PostgreSQL started successfully",
		slog.String("connection", postgresService.GetConnectionString("")))

	return postgresService, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Parse command line flags
	dbPath := flag.String("db-path", "./store.db", "Path to the SQLite database file")
	flag.Parse()

	logger.Info("Using database path", slog.String("path", *dbPath))

	envs, _ := config.LoadConfig(true)

	ollamaClient, err := ollamaapi.ClientFromEnvironment()
	if err != nil {
		panic(errors.Wrap(err, "Unable to create ollama client"))
	}
	logger.Info("Config loaded", slog.Any("envs", envs))

	// Start PostgreSQL
	postgresCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	postgresService, err := bootstrapPostgres(postgresCtx, logger)
	if err != nil {
		logger.Error("Failed to start PostgreSQL", slog.Any("error", err))
		// Continue with SQLite only if PostgreSQL fails
	} else {
		// Set up cleanup on shutdown
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := postgresService.StopPostgres(shutdownCtx); err != nil {
				logger.Error("Error stopping PostgreSQL", slog.Any("error", err))
			}
		}()
	}

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

	store, err := db.NewStore(*dbPath)
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

	temporalClient, err := bootstrapTemporal(logger, envs, store, nc, ollamaClient, *dbPath)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal"))
	}

	memory, err := graphmemory.NewGraphMemory(postgresService.GetConnectionString("enchanted_twin"))
	if err != nil {
		panic(errors.Wrap(err, "Unable to create graph memory"))
	}

	aiService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)
	chatStorage := chatrepository.NewRepository(logger, store.DB())
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
		logger.Info("Starting GraphQL HTTP server", slog.String("port", envs.GraphqlPort))
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

func bootstrapTemporal(logger *slog.Logger, envs *config.Config, store *db.Store, nc *nats.Conn, ollamaClient *ollamaapi.Client, dbPath string) (client.Client, error) {
	ready := make(chan struct{})
	go bootstrap.CreateTemporalServer(logger, ready, dbPath)
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
	logger          *slog.Logger
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
		Debug:            true,
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
