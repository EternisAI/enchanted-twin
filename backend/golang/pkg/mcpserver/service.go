package mcpserver

import (
	"context"

	mcp "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
)

// MCPService defines the interface for managing MCP server data.
type MCPService interface {
	// ConnectMCPServer connects a new MCP server based on the provided input.
	ConnectMCPServer(
		ctx context.Context,
		input model.ConnectMCPServerInput,
	) (*model.MCPServer, error)

	// ConnectMCPServerIfNotExists connects a new MCP server if it doesn't exist.
	ConnectMCPServerIfNotExists(
		ctx context.Context,
		input model.ConnectMCPServerInput,
	) (*model.MCPServer, error)

	// GetMCPServers retrieves all MCP servers.
	GetMCPServers(ctx context.Context) ([]*model.MCPServerDefinition, error)
	// Load MCP Server from database
	LoadMCP(ctx context.Context) error
	// Get Tools from MCP Servers
	GetTools(ctx context.Context) ([]mcp.ToolRetType, error)
	// Get Tools for internal use
	GetInternalTools(ctx context.Context) ([]tools.Tool, error)
	// Remove MCP Server
	RemoveMCPServer(ctx context.Context, id string) error
	// Get the tool registry
	GetRegistry() tools.ToolRegistry
}
