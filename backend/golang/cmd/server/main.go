package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/fx"
)

var AppModule = fx.Module("server",
	fx.Provide(
		NewLogger,
		LoadConfig,
		NewContext,
		NewNATSServer,
		NewNATSClient,
		NewStore,
		NewDatabase,
		NewAIServices,
		NewCompletionsAIService,
		NewChatStorage,
		NewWeaviateClient,
		NewEvolvingMemory,
		NewTemporalClient,
		NewTemporalWorker,
		NewTTSService,
		NewToolRegistry,
		NewIdentityService,
		NewTwinChatService,
		NewMCPService,
		NewTelegramService,
		NewHolonService,
		NewNotificationsService,
		NewGraphQLRouter,
	),

	fx.Provide(NewWhatsAppService),
	fx.Invoke(RegisterWhatsAppServiceLifecycle),

	fx.Invoke(
		RegisterPeriodicWorkflows,
		StartGraphQLServer,
		StartTelegramProcesses,
		StartHolonProcesses,
	),
)

func main() {
	app := fx.New(
		AppModule,
		fx.StartTimeout(30*time.Second),
		fx.StopTimeout(15*time.Second),
	)

	startCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		panic(err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan

	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		panic(err)
	}
}
