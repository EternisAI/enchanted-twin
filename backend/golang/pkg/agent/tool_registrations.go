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
	telegramChatServerUrl string,
) []tools.Tool {
	registeredTools := []tools.Tool{}

	// Register basic tools
	searchTool := &tools.SearchTool{}
	if err := registry.Register(searchTool); err == nil {
		registeredTools = append(registeredTools, searchTool)
	} else {
		logger.Warn("Failed to register search tool", "error", err)
	}

	imageTool := &tools.ImageTool{}
	if err := registry.Register(imageTool); err == nil {
		registeredTools = append(registeredTools, imageTool)
	} else {
		logger.Warn("Failed to register image tool", "error", err)
	}

	// Register tools that need dependencies
	if memoryStorage != nil {
		memoryTool := tools.NewMemorySearchTool(logger, memoryStorage)
		if err := registry.Register(memoryTool); err == nil {
			registeredTools = append(registeredTools, memoryTool)
		} else {
			logger.Warn("Failed to register memory tool", "error", err)
		}
	}

	// Register Telegram tool if token is available
	if telegramToken != "" && store != nil {
		telegramTool := tools.NewTelegramTool(logger, telegramToken, store, telegramChatServerUrl)
		if err := registry.Register(telegramTool); err == nil {
			registeredTools = append(registeredTools, telegramTool)
		} else {
			logger.Warn("Failed to register telegram tool", "error", err)
		}
	} else {
		logger.Info("Could not register telegram tool, token or store is not available")
	}

	// Register Twitter tool if store is available
	if store != nil {
		twitterTool := tools.NewTwitterTool(*store)
		if err := registry.Register(twitterTool); err == nil {
			registeredTools = append(registeredTools, twitterTool)
		} else {
			logger.Warn("Failed to register twitter tool", "error", err)
		}
	}

	// Register PlannedAgentTool if temporal client is available
	if temporalClient != nil && completionsModel != "" {
		plannedAgentTool := plannedv2.NewPlannedAgentTool(logger, temporalClient, completionsModel)
		if err := registry.Register(plannedAgentTool); err == nil {
			registeredTools = append(registeredTools, plannedAgentTool)
		} else {
			logger.Warn("Failed to register planned agent tool", "error", err)
		}
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
