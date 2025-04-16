package main

import (
	"context"
	"enchanted-twin/graph"
	"enchanted-twin/pkg/ai"
	"enchanted-twin/pkg/bootstrap"
	"enchanted-twin/pkg/config"
	"enchanted-twin/pkg/twinchat"
	chatrepository "enchanted-twin/pkg/twinchat/repository"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("Starting server...")

	envs, err := config.LoadConfig(true)
	if err != nil {
		panic(errors.Wrap(err, "Unable to load config"))
	}
	logger.Info("Config loaded", slog.Any("envs", envs))

	logger.Info("Starting temporal server and client")
	temporalClient, err := bootstrapTemporal(logger)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start temporal"))
	}

	logger.Info("Starting nats server")
	_, err = bootstrap.StartEmbeddedNATSServer()
	if err != nil {
		panic(errors.Wrap(err, "Unable to start nats server"))
	}

	logger.Info("Starting nats client")
	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		panic(errors.Wrap(err, "Unable to create nats client"))
	}

	aiService := ai.NewOpenAIService(envs.OpenAIAPIKey, envs.OpenAIBaseURL)
	chatStorage := chatrepository.NewRepository(logger)
	twinChatService := twinchat.NewService(aiService, chatStorage, nc)

	router := bootstrapGraphqlServer(graphqlServerInput{
		logger:          logger,
		temporalClient:  temporalClient,
		port:            envs.GraphqlPort,
		twinChatService: twinChatService,
		natsClient:      nc,
	})

	logger.Info("Starting server")
	err = http.ListenAndServe(":"+envs.GraphqlPort, router)
	if err != nil {
		panic(errors.Wrap(err, "Unable to start server"))
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan
	logger.Info("Server shutting down...")
}

func bootstrapTemporal(logger *slog.Logger) (client.Client, error) {
	logger.Info("Starting temporal server")
	go bootstrap.CreateTemporalServer()

	time.Sleep(5 * time.Second)

	logger.Info("Starting temporal client")
	client, err := bootstrap.CreateTemporalClient("localhost:7233", bootstrap.TemporalNamespace, "")
	if err != nil {
		panic(errors.Wrap(err, "Unable to create temporal client"))
	}
	return client, nil
}

type graphqlServerInput struct {
	logger          *slog.Logger
	temporalClient  client.Client
	port            string
	twinChatService *twinchat.Service
	natsClient      *nats.Conn
}

func bootstrapGraphqlServer(input graphqlServerInput) *chi.Mux {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            true,
	}).Handler)

	srv := handler.New(gqlSchema(GqlSchemaInput{
		Logger:          input.logger,
		TemporalClient:  input.temporalClient,
		TwinChatService: input.twinChatService,
		Nc:              input.natsClient,
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

	input.logger.Info(fmt.Sprintf("connect to http://localhost:%s/ for GraphQL playground", input.port))

	return router
}

type GqlSchemaInput struct {
	Logger          *slog.Logger
	TemporalClient  client.Client
	TwinChatService *twinchat.Service
	Nc              *nats.Conn
}

func gqlSchema(input GqlSchemaInput) graphql.ExecutableSchema {
	config := graph.Config{
		Resolvers: &graph.Resolver{
			Logger:          input.Logger,
			TemporalClient:  input.TemporalClient,
			TwinChatService: *input.TwinChatService,
			Nc:              input.Nc,
		},
	}
	return graph.NewExecutableSchema(config)
}
