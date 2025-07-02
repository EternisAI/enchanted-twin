package bootstrap

import (
	"context"

	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"

	"github.com/charmbracelet/log"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

func StartWorker(lc fx.Lifecycle, w worker.Worker, logger *log.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting temporal worker")
			go func() {
				if err := w.Run(worker.InterruptCh()); err != nil {
					logger.Error("Worker failed to start", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping temporal worker")
			w.Stop()
			return nil
		},
	})
}



func RegisterWhatsAppServiceLifecycle(lc fx.Lifecycle, service *whatsapp.Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return service.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return service.Stop(ctx)
		},
	})
}