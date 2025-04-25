package agent

import (
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	plannedv2 "github.com/EternisAI/enchanted-twin/pkg/agent/planned-v2"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"
)

// ToolProvider is an interface for services that provide tools.
type ToolProvider interface {
	Tools() []tools.Tool
}

// RegisterStandardTools registers all standard tools with the registry.
// Returns a slice of the registered tools.
func RegisterStandardTools(
	registry *tools.Registry,
	logger *log.Logger,
	memoryStorage memory.Storage,
	telegramToken string,
	store *db.Store,
	temporalClient client.Client,
	completionsModel string,
) []tools.Tool {
	registeredTools := []tools.Tool{}

	// Register basic tools
	searchTool := &tools.SearchTool{}
	registry.Register(searchTool)
	registeredTools = append(registeredTools, searchTool)

	imageTool := &tools.ImageTool{}
	registry.Register(imageTool)
	registeredTools = append(registeredTools, imageTool)

	// Register tools that need dependencies
	if memoryStorage != nil {
		memoryTool := tools.NewMemorySearchTool(logger, memoryStorage)
		registry.Register(memoryTool)
		registeredTools = append(registeredTools, memoryTool)
	}

	// Register Telegram tool if token is available
	if telegramToken != "" && store != nil {
		telegramTool := tools.NewTelegramTool(logger, telegramToken, store)
		registry.Register(telegramTool)
		registeredTools = append(registeredTools, telegramTool)
	}

	// Register Twitter tool if store is available
	if store != nil {
		twitterTool := tools.NewTwitterTool(*store)
		registry.Register(twitterTool)
		registeredTools = append(registeredTools, twitterTool)
	}

	// Register PlannedAgentTool if temporal client is available
	if temporalClient != nil && completionsModel != "" {
		plannedAgentTool := plannedv2.NewPlannedAgentTool(logger, temporalClient, completionsModel)
		registry.Register(plannedAgentTool)
		registeredTools = append(registeredTools, plannedAgentTool)
	}

	logger.Info("Registered standard tools", "count", len(registeredTools))
	return registeredTools
}

// RegisterToolProviders registers tools from a list of tool providers.
// Returns the list of successfully registered tools.
func RegisterToolProviders(
	registry *tools.Registry,
	logger *log.Logger,
	providers ...ToolProvider,
) []tools.Tool {
	registeredTools := []tools.Tool{}

	for _, provider := range providers {
		for _, tool := range provider.Tools() {
			if err := registry.Register(tool); err == nil {
				registeredTools = append(registeredTools, tool)
			} else {
				logger.Warn("Failed to register tool", "error", err)
			}
		}
	}

	if len(registeredTools) > 0 {
		logger.Info("Registered tools from providers", "count", len(registeredTools))
	}

	return registeredTools
}

// RegisterMCPTools registers MCP tools with the registry.
// This function takes a slice of MCP tools and registers them.
// Returns the list of successfully registered tools.
func RegisterMCPTools(registry *tools.Registry, mcpTools []tools.Tool) []tools.Tool {
	registeredTools := []tools.Tool{}

	for _, tool := range mcpTools {
		if err := registry.Register(tool); err == nil {
			registeredTools = append(registeredTools, tool)
		}
	}

	return registeredTools
}
