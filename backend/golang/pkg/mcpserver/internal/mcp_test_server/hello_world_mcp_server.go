package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Content struct {
	Title       string  `json:"title"       jsonschema:"required,description=The title to submit"`
	Description *string `json:"description" jsonschema:"description=The description to submit"`
}
type MyFunctionsArguments struct {
	Submitter string  `json:"submitter" jsonschema:"required,description=The name of the thing calling this tool (openai, google, claude, etc)"`
	Content   Content `json:"content"   jsonschema:"required,description=The content of the message"`
}

func main() {
	// Create a new MCP server
	mcpServer := server.NewMCPServer(
		"Hello World MCP Server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add a simple hello tool
	tool := mcp.NewTool("hello",
		mcp.WithDescription("Say hello to a person"),
		mcp.WithString("submitter",
			mcp.Required(),
			mcp.Description("The name of the thing calling this tool (openai, google, claude, etc)"),
		),
		mcp.WithObject("content",
			mcp.Required(),
			mcp.Description("The content of the message"),
			mcp.Properties(map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "The title to submit",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "The description to submit",
				},
			}),
		),
	)

	// Add tool handler using the typed handler
	mcpServer.AddTool(tool, mcp.NewTypedToolHandler(helloHandler))

	// Start the stdio server
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Handler function for the hello tool.
func helloHandler(ctx context.Context, request mcp.CallToolRequest, args MyFunctionsArguments) (*mcp.CallToolResult, error) {
	if args.Submitter == "" {
		return mcp.NewToolResultError("submitter is required"), nil
	}

	message := fmt.Sprintf("Hello, %s!", args.Submitter)
	if args.Content.Title != "" {
		message += fmt.Sprintf(" Your title is: %s", args.Content.Title)
	}
	if args.Content.Description != nil && *args.Content.Description != "" {
		message += fmt.Sprintf(" Description: %s", *args.Content.Description)
	}

	return mcp.NewToolResultText(message), nil
}
