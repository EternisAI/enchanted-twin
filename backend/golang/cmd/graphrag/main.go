package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Initialize GraphRAG service
	logger.Info("Initializing GraphRAG service...")
	graphragConfig := bootstrap.DefaultGraphRAGConfig()
	graphragService, err := bootstrap.InitGraphRAG(ctx, graphragConfig, logger)
	if err != nil {
		logger.Error("Failed to initialize GraphRAG service", slog.Any("error", err))
		os.Exit(1)
	}

	// Ensure GraphRAG service is properly shut down
	defer func() {
		logger.Info("Shutting down GraphRAG service...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := bootstrap.ShutdownGraphRAG(shutdownCtx, graphragService); err != nil {
			logger.Error("Error shutting down GraphRAG service", slog.Any("error", err))
		}
	}()

	// Example: Getting logs from the GraphRAG container
	time.Sleep(2 * time.Second) // Give it a moment to start
	logs, err := graphragService.GetContainerLogs(ctx)
	if err != nil {
		logger.Error("Failed to get GraphRAG logs", slog.Any("error", err))
	} else {
		logger.Info("GraphRAG container logs", slog.String("logs", logs))
	}

	logger.Info("Service is running. Press Ctrl+C to stop.")

	// Wait for termination signal
	<-sigCh
	logger.Info("Received termination signal, shutting down...")
}
