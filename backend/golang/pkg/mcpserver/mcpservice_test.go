package mcpserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/EternisAI/enchanted-twin/graph/model"
	db "github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/repository"
	"github.com/charmbracelet/log"
)


func TestMCPService_GetTools(t *testing.T) {
	ctx := context.Background()

	logger := log.Default()
	db, err := db.NewStore(ctx, "./test.db")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}

	repo := repository.NewRepository(logger, db.DB())
	s := NewService(ctx, *repo)


	_, err = s.AddMCPServer(ctx, model.AddMCPServerInput{
		Name:    "hello_world_mcp_server",
		Command: []string{"go", "run", "./mcp_test_server/hello_world_mcp_server.go"},
	})

	if err != nil {
		t.Fatalf("Failed to add MCPServer: %v", err)
	}

	_, err = s.AddMCPServer(ctx, model.AddMCPServerInput{
		Name:    "filesystem_mcp_server",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/Users/username/Desktop", "/Users/username/Downloads"},
	})

	if err != nil {
		t.Fatalf("Failed to add MCPServer: %v", err)
	}

	_, err = s.AddMCPServer(ctx, model.AddMCPServerInput{
		Name:    "dbhub_mcp_server",
		Command: []string{"docker", "run", "-i", "--rm", "mcp/postgres", "postgresql://username:password@host.docker.internal:5432/postgres"},
	})

	if err != nil {
		t.Fatalf("Failed to add MCPServer: %v", err)
	}

	fmt.Println("MCPServer added")


	tools, err := s.GetTools(ctx)
	if err != nil {
		t.Fatalf("Failed to get tools: %v", err)
	}

	for _, tool := range tools {
		fmt.Println(tool.Name)
		fmt.Println(*tool.Description)
	}
}


func TestMCPService_ExecuteTool(t *testing.T) {
	ctx := context.Background()

	logger := log.Default()
	db, err := db.NewStore(ctx, "./test.db")
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}

	repo := repository.NewRepository(logger, db.DB())
	s := NewService(ctx, *repo)	
	

	_, err = s.AddMCPServer(ctx, model.AddMCPServerInput{
		Name:    "hello_world_mcp_server",
		Command: []string{"go", "run", "./mcp_test_server/hello_world_mcp_server.go"},
	})

	if err != nil {
		t.Fatalf("Failed to add MCPServer: %v", err)
	}

	tools, err := s.GetTools(ctx)	
	if err != nil {
		t.Fatalf("Failed to get tools: %v", err)
	}

	tool_response, err := s.ExecuteTool(ctx, tools[0].Name, map[string]any{"submitter": "John Doe"})
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	fmt.Println(tool_response.Content[0].TextContent.Text)
}
