package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"

	"github.com/pkg/errors"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Set up command line flags
	providerFlag := flag.String("provider", "", "OAuth provides to authenticate - comma separated, e.g. 'twitter,google,linkedin'")
	dbPath := flag.String("db-path", "./store.db", "Path to the SQLite database file")
	logger.Info("Using database path", slog.String("path", *dbPath))
	flag.Parse()

	// Open database
	store, err := db.NewStore(ctx, *dbPath)
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

	if *providerFlag != "" {
		if err := helpers.StartOAuthCallbackServer(logger, store); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		for _, p := range strings.Split(*providerFlag, ",") {
			// Run OAuth flow with selected provider
			if err := helpers.OAuthFlow(ctx, logger, store, p); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		}
		if err := helpers.ShutdownOAuthCallbackServer(ctx, logger); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	} else {
		flag.Usage()
		os.Exit(1)
	}
}
