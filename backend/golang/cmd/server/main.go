package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"

	"github.com/EternisAI/enchanted-twin/graph"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
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

	router := bootstrapGraphqlServer(logger, temporalClient, envs.GraphqlPort)

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

func bootstrapGraphqlServer(logger *slog.Logger, temporalClient client.Client, port string) *chi.Mux {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            false,
	}).Handler)

	srv := handler.New(gqlSchema(GqlSchemaInput{
		Logger:         logger,
		TemporalClient: temporalClient,
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
			logger.Error("gql error", "operation_name", oc.OperationName, "raw_query", oc.RawQuery, "errors", resp.Errors)
		}

		return resp
	})

	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", srv)

	logger.Info(fmt.Sprintf("connect to http://localhost:%s/ for GraphQL playground", port))

	return router
}

type GqlSchemaInput struct {
	Logger         *slog.Logger
	TemporalClient client.Client
}

func gqlSchema(input GqlSchemaInput) graphql.ExecutableSchema {
	config := graph.Config{
		Resolvers: &graph.Resolver{
			Logger:         input.Logger,
			TemporalClient: input.TemporalClient,
		},
	}
	return graph.NewExecutableSchema(config)
}
