package main

import (
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/notifications"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	chatrepository "github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

func main() {
	fx.New(
		fx.Provide(
			bootstrap.NewLogger,
			config.LoadConfigForFx,
			bootstrap.NewNATSConnection,
			db.NewStoreFromConfig,
			db.NewDatabaseFromStore,
			ai.NewServicesFromConfig,
			ai.NewCompletionsService,
			bootstrap.NewTemporalClientFromConfig,
			bootstrap.NewWeaviateClientFromConfig,
			chatrepository.NewRepository,
			evolvingmemory.NewEvolvingMemory,
			bootstrap.CreateTemporalWorker,
			bootstrap.NewTTSService,
			bootstrap.NewToolRegistry,
			bootstrap.NewIdentityService,
			twinchat.NewServiceForFx,
			mcpserver.NewServiceForFx,
			telegram.NewServiceForFx,
			holon.NewServiceForFx,
			notifications.NewServiceForFx,
			whatsapp.NewServiceForFx,
			bootstrap.NewGraphQLRouter,
		),
		fx.Invoke(
			bootstrap.RegisterIdentityWorkflows,
			bootstrap.RegisterHolonWorkflows,
			bootstrap.StartWorker,
			bootstrap.StartGraphQLServer,
			telegram.StartProcesses,
			holon.StartProcesses,
			bootstrap.RegisterWhatsAppServiceLifecycle,
		),
	).Run()
}
