// Owner: august@eternis.ai
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/charmbracelet/log"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	fxbootstrap "github.com/EternisAI/enchanted-twin/pkg/bootstrap/fx"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
)

func showComponents() {
	// Create a simple logger and factory for component listing
	// We don't need full config loading just to list components
	baseLogger := bootstrap.NewBootstrapLogger()
	loggerFactory := bootstrap.NewLoggerFactory(baseLogger)

	// Simulate registration of common components by creating loggers for them
	// This ensures they get registered in the component registry
	commonComponents := []string{
		"holon.manager",
		"holon.service",
		"ai.service",
		"ai.anonymizer",
		"whatsapp.service",
		"telegram.service",
		"twinchat.service",
		"memory.storage",
		"database.store",
		"nats.client",
		"temporal.client",
		"temporal.worker",
	}

	for _, component := range commonComponents {
		loggerFactory.ForComponent(component)
	}

	// Get all registered components
	registry := loggerFactory.GetComponentRegistry()
	components := registry.ListComponents()

	fmt.Println("Registered Logging Components:")
	fmt.Println("==============================")

	if len(components) == 0 {
		fmt.Println("No components registered.")
		return
	}

	// Sort components by ID
	componentNames := make([]string, len(components))
	for i, comp := range components {
		componentNames[i] = comp.ID
	}
	sort.Strings(componentNames)

	// Display components with their types and current log levels
	for _, name := range componentNames {
		level := registry.GetComponentLogLevel(name)
		enabled := registry.IsComponentEnabled(name)
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		fmt.Printf("  %-25s level=%s (%s)\n", name, level.String(), status)
	}

	fmt.Printf("\nTotal: %d components\n", len(components))
	fmt.Println("\nTo set component log levels:")
	fmt.Println("  export LOG_LEVEL_<COMPONENT_NAME>=<LEVEL>")
	fmt.Println("  Example: export LOG_LEVEL_holon_manager=debug")
}

func main() {
	// Parse command line flags
	listComponents := flag.Bool("list-components", false, "List all registered logging components and exit")
	flag.Parse()

	// Handle --list-components flag
	if *listComponents {
		showComponents()
		return
	}

	app := fx.New(
		fxbootstrap.AppModule,
		// Use our custom charmbracelet logger for fx events
		fx.WithLogger(func(logger *log.Logger) fxevent.Logger {
			return fxbootstrap.NewCharmLoggerWithComponent(logger, "fx.framework")
		}),
		fx.Invoke(func(
			loggerFactory *bootstrap.LoggerFactory,
			identityService *identity.IdentityService,
		) {
			logger := loggerFactory.ForComponent("main.startup")
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
