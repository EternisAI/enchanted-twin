package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"

	"github.com/pkg/errors"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	// Set up command line flags
	providerFlag := flag.String("provider", "", "OAuth provides to authenticate - comma separated, e.g. 'twitter,google,linkedin'")
	refreshFlag := flag.Bool("refresh", false, "Specify to refresh all expired tokens")
	dbPath := flag.String("db-path", "./store.db", "Path to the SQLite database file")
	logger.Info("Using database path", "path", *dbPath)
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
			logger.Error("failed to start server", "error", err)
			os.Exit(1)
		}
		for _, p := range strings.Split(*providerFlag, ",") {
			// Run OAuth flow with selected provider
			if err := helpers.OAuthFlow(ctx, logger, store, p); err != nil {
				logger.Errorf("oauth flow: %v\n", err)
				os.Exit(1)
			}
		}
		if err := helpers.ShutdownOAuthCallbackServer(ctx, logger); err != nil {
			logger.Error("failed to shutdown server", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *refreshFlag {
		status, err := helpers.RefreshExpiredTokens(ctx, logger, store)
		if err != nil {
			logger.Errorf("refresh tokens: %v\n", err)
			os.Exit(1)
		}
		for _, s := range status {
			logger.Infof("provider %s has scope %s and expires at %s", s.Provider, s.Scope, s.ExpiresAt.Format(time.RFC3339))
		}
	}

	if *providerFlag == "" && !*refreshFlag {
		flag.Usage()
		os.Exit(1)
	}
}
