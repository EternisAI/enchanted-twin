package mcpserver

import (
	"context"

	"github.com/EternisAI/enchanted-twin/graph/model"
	mcp "github.com/metoro-io/mcp-golang"
)

// MCPService defines the interface for managing MCP server data.
type MCPService interface {
	// AddMCPServer adds a new MCP server based on the provided input.
	AddMCPServer(ctx context.Context, input model.AddMCPServerInput) (*model.MCPServer, error)
	// GetMCPServers retrieves all MCP servers.
	GetMCPServers(ctx context.Context) ([]*model.MCPServer, error)
	// Load MCP Server from database
	LoadMCP(ctx context.Context) error
	// Get Tools from MCP Servers
	GetTools(ctx context.Context) ([]mcp.ToolRetType, error)
	// Execute Tool
	ExecuteTool(ctx context.Context, toolName string, args any) (*mcp.ToolResponse, error)
}