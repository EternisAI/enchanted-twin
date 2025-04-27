package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"go.temporal.io/sdk/client"
)

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	envs, _ := config.LoadConfig(true)
	logger.Debug("Config loaded", "envs", envs)
	logger.Info("Using database path", "path", envs.DBPath)

	_, err := bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start nats server"))
	}
	logger.Info("NATS server started")

	// Create NATS client
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

	telegramService := telegram.NewTelegramService(telegram.TelegramServiceInput{
		Logger:     logger,
		Token:      envs.TelegramToken,
		Client:     &http.Client{},
		Store:      store,
		NatsClient: nc,
	})

	go func() {
		if err := telegramService.Start(context.Background()); err != nil {
			logger.Error("Telegram service error", slog.Any("error", err))
		}
	}()

	graphqlPort := envs.GraphqlPort
	router := bootstrapGraphqlServer(graphqlServerInput{
		logger:          logger,
		port:            graphqlPort,
		natsClient:      nc,
		store:           store,
		telegramService: telegramService,
	})

	// Start HTTP server in a goroutine so it doesn't block signal handling

	logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+graphqlPort)
	err = http.ListenAndServe(":"+graphqlPort, router)
	if err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server error", slog.Any("error", err))
		panic(errors.Wrap(err, "Unable to start server"))
	}
}

func gqlSchema(input *graph.Resolver) graphql.ExecutableSchema {
	config := graph.Config{
		Resolvers: input,
	}
	return graph.NewExecutableSchema(config)
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
		TelegramService:        input.telegramService,
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
			input.logger.Error("gql error", "operation_name", oc.OperationName, "raw_query", oc.RawQuery, "variables", oc.Variables, "errors", resp.Errors)
		}

		return resp
	})

	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", srv)

	return router
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
}
