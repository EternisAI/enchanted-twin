package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/charmbracelet/log"
	"github.com/go-chi/chi"
	"github.com/rs/cors"

	twinnetwork "github.com/EternisAI/enchanted-twin/twin_network"
	"github.com/EternisAI/enchanted-twin/twin_network/graph"
)

const defaultPort = "8082"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
		TimeFormat:      time.Kitchen,
	})

	store := twinnetwork.NewMessageStore()

	resolver := &graph.Resolver{
		Store:  store,
		Logger: logger,
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Options{})
	srv.Use(extension.Introspection{})

	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
	}).Handler)

	router.Handle("/", playground.Handler("Twin Network GraphQL playground", "/query"))
	router.Handle("/query", srv)

	logger.Info("Twin Network GraphQL server running", "address", fmt.Sprintf("http://localhost:%s/", port))
	if err := http.ListenAndServe(":"+port, router); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
