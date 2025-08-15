package fx

import (
	"context"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/rs/cors"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/graph"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/directorywatcher"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

// ServerModule provides HTTP GraphQL server.
var ServerModule = fx.Module("server",
	fx.Provide(
		ProvideGraphQLServer,
	),
	fx.Invoke(
		StartGraphQLServer,
	),
)

// GraphQLServerResult provides GraphQL server.
type GraphQLServerResult struct {
	fx.Out
	Router *chi.Mux
}

// GraphQLServerParams holds parameters for GraphQL server.
type GraphQLServerParams struct {
	fx.In
	LoggerFactory      *bootstrap.LoggerFactory
	Config             *config.Config
	TemporalClient     client.Client
	TwinChatService    *twinchat.Service
	NATSConn           *nats.Conn
	Store              *db.Store
	CompletionsService *ai.Service
	MCPService         mcpserver.MCPService
	TelegramService    *telegram.TelegramService
	HolonService       *holon.Service
	DirectoryWatcher   *directorywatcher.DirectoryWatcher
	WhatsAppService    *whatsapp.Service
}

// ProvideGraphQLServer creates GraphQL server with all dependencies.
func ProvideGraphQLServer(params GraphQLServerParams) GraphQLServerResult {
	logger := params.LoggerFactory.ForServer("graphql.server")
	logger.Info("Creating GraphQL server")

	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		Debug:            false,
	}).Handler)

	resolver := &graph.Resolver{
		Logger:                 logger,
		TemporalClient:         params.TemporalClient,
		TwinChatService:        params.TwinChatService,
		Nc:                     params.NATSConn,
		Store:                  params.Store,
		AiService:              params.CompletionsService,
		MCPService:             params.MCPService,
		DataProcessingWorkflow: nil, // Will be set later if needed
		TelegramService:        params.TelegramService,
		HolonService:           params.HolonService,
		DirectoryWatcher:       params.DirectoryWatcher,
		WhatsAppService:        params.WhatsAppService,
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
			logger.Error(
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

	logger.Info("GraphQL server created successfully")
	return GraphQLServerResult{Router: router}
}

// gqlSchema creates GraphQL executable schema.
func gqlSchema(resolver *graph.Resolver) graphql.ExecutableSchema {
	config := graph.Config{
		Resolvers: resolver,
	}
	return graph.NewExecutableSchema(config)
}

// StartGraphQLServerParams holds parameters for starting GraphQL server.
type StartGraphQLServerParams struct {
	fx.In
	Lifecycle     fx.Lifecycle
	LoggerFactory *bootstrap.LoggerFactory
	Config        *config.Config
	Router        *chi.Mux
}

// StartGraphQLServer starts the HTTP server.
func StartGraphQLServer(params StartGraphQLServerParams) {
	logger := params.LoggerFactory.ForServer("graphql.http")
	var server *http.Server

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			server = &http.Server{
				Addr:    ":" + params.Config.GraphqlPort,
				Handler: params.Router,
			}

			go func() {
				logger.Info("Starting GraphQL HTTP server", "address", "http://localhost:"+params.Config.GraphqlPort)
				err := server.ListenAndServe()
				if err != nil && err != http.ErrServerClosed {
					logger.Error("HTTP server error", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down GraphQL HTTP server")
			if server != nil {
				return server.Shutdown(ctx)
			}
			return nil
		},
	})
}
