package fx

import (
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	schedulerTools "github.com/EternisAI/enchanted-twin/pkg/agent/scheduler/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
)

// ToolsModule provides tool registry and related services.
var ToolsModule = fx.Module("tools",
	fx.Provide(
		ProvideToolRegistry,
	),
	fx.Invoke(
		RegisterCoreTools,
	),
)

// ToolRegistryResult provides tool registry.
type ToolRegistryResult struct {
	fx.Out
	ToolRegistry *tools.ToolMapRegistry
}

// ProvideToolRegistry creates tool registry.
func ProvideToolRegistry() ToolRegistryResult {
	toolRegistry := tools.NewRegistry()
	return ToolRegistryResult{ToolRegistry: toolRegistry}
}

// CoreToolsParams holds parameters for core tools registration.
type CoreToolsParams struct {
	fx.In
	LoggerFactory  *bootstrap.LoggerFactory
	Config         *config.Config
	Store          *db.Store
	ToolRegistry   *tools.ToolMapRegistry
	Memory         memory.Storage
	TemporalClient client.Client
}

// RegisterCoreTools registers core tools that don't depend on application services.
func RegisterCoreTools(params CoreToolsParams) error {
	logger := params.LoggerFactory.ForComponent("tools.core")
	logger.Info("Registering core tools")

	// Register memory search tool
	if err := params.ToolRegistry.Register(memory.NewMemorySearchTool(logger, params.Memory)); err != nil {
		logger.Error("Failed to register memory search tool", "error", err)
		return err
	}

	// Register schedule task tool
	if err := params.ToolRegistry.Register(&schedulerTools.ScheduleTask{
		Logger:         logger,
		TemporalClient: params.TemporalClient,
		ToolsRegistry:  params.ToolRegistry,
	}); err != nil {
		logger.Error("Failed to register schedule task tool", "error", err)
		return err
	}

	// Register telegram setup tool
	telegramTool, err := telegram.NewTelegramSetupTool(logger, params.Store, params.Config.TelegramChatServer, params.Config.TelegramBotName)
	if err != nil {
		logger.Error("Failed to create telegram setup tool", "error", err)
		return err
	}

	if err := params.ToolRegistry.Register(telegramTool); err != nil {
		logger.Error("Failed to register telegram tool", "error", err)
		return err
	}

	logger.Info("Core tools registered successfully", "count", len(params.ToolRegistry.List()))
	return nil
}
