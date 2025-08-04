// Owner: august@eternis.ai
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"go.uber.org/fx"

	fxbootstrap "github.com/EternisAI/enchanted-twin/pkg/bootstrap/fx"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
)

func main() {
	app := fx.New(
		fxbootstrap.AppModule,
		fx.Invoke(func(
			logger *log.Logger,
			identityService *identity.IdentityService,
		) {
			// Get user profile during startup
			userProfile, err := identityService.GetUserProfile(context.Background())
			if err != nil {
				logger.Warn("Failed to get user profile during startup - continuing without it", "error", err)
			} else {
				logger.Info("User profile", "profile", userProfile)
			}
		}),
	)

	// Create signal channel for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Start the application
	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		panic(err)
	}

	// Wait for shutdown signal
	<-signalChan
	log.Info("Server shutting down...")

	// Stop the application gracefully
	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		log.Error("Failed to stop application gracefully", "error", err)
	}
}
