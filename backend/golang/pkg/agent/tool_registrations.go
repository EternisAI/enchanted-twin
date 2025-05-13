package agent

import (
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"

	schedulerTools "github.com/EternisAI/enchanted-twin/pkg/agent/scheduler/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// ToolProvider is an interface for services that provide tools.
type ToolProvider interface {
	Tools() []tools.Tool
}

// CreateStandardTools creates the standard set of tools based on available dependencies.
func CreateStandardTools(
	logger *log.Logger,
	telegramToken string,
	store *db.Store,
	temporalClient client.Client,
	completionsModel string,
	telegramChatServerUrl string,
) []tools.Tool {
	standardTools := []tools.Tool{}

	// Create Telegram tool if token is available
	if telegramToken != "" && store != nil {
		telegramTool := tools.NewTelegramTool(logger, telegramToken, store, telegramChatServerUrl)
		standardTools = append(standardTools, telegramTool)
	}

	if temporalClient != nil && completionsModel != "" {
		scheduledTaskTool := &schedulerTools.ScheduleTask{
			Logger:         logger,
			TemporalClient: temporalClient,
		}
		standardTools = append(standardTools, scheduledTaskTool)
	}

	return standardTools
}

// RegisterStandardTools registers all standard tools with the registry.
// Returns a slice of the registered tools.
func RegisterStandardTools(
	registry tools.ToolRegistry,
	logger *log.Logger,
	telegramToken string,
	store *db.Store,
	temporalClient client.Client,
	completionsModel string,
	telegramChatServerUrl string,
) []tools.Tool {
	// Create standard tools
	standardTools := CreateStandardTools(
		logger,
		telegramToken,
		store,
		temporalClient,
		completionsModel,
		telegramChatServerUrl,
	)

	// Register all tools at once
	registeredTools := []tools.Tool{}
	for _, tool := range standardTools {
		if err := registry.Register(tool); err == nil {
			registeredTools = append(registeredTools, tool)
		} else {
			logger.Warn("Failed to register tool", "name", tool.Definition().Function.Name, "error", err)
		}
	}
	logger.Info("Registered standard tools", "count", len(registeredTools))
	return registeredTools
}

// RegisterToolProviders registers tools from a list of tool providers.
// Returns the list of successfully registered tools.
func RegisterToolProviders(
	registry tools.ToolRegistry,
	logger *log.Logger,
	providers ...ToolProvider,
) []tools.Tool {
	registeredTools := []tools.Tool{}

	for _, provider := range providers {
		providerTools := provider.Tools()
		for _, tool := range providerTools {
			if err := registry.Register(tool); err == nil {
				registeredTools = append(registeredTools, tool)
			} else {
				logger.Warn("Failed to register tool from provider",
					"name", tool.Definition().Function.Name,
					"error", err)
			}
		}
	}
	if len(registeredTools) > 0 {
		logger.Info("Registered tools from providers", "count", len(registeredTools))
	}

	return registeredTools
}
