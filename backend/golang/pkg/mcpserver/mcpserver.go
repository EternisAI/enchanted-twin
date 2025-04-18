package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/repository"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type ConnectedMCPServer struct {
	ID string
	Client *mcp.Client
}

// service implements the MCPServerService interface.
type service struct {
	repo repository.Repository
	connectedServers []*ConnectedMCPServer
}

// NewService creates a new MCPServerService.
func NewService(ctx context.Context, repo repository.Repository) MCPService {
	service := &service{repo: repo, connectedServers: []*ConnectedMCPServer{}}
	err := service.LoadMCP(ctx)
	if err != nil {
		fmt.Println("Error loading MCP servers", err)
	}
	return service
}

// AddMCPServer adds a new MCP server using the repository.
func (s *service) ConnectMCPServer(ctx context.Context, input model.ConnectMCPServerInput) (*model.MCPServer, error) {
	// Here you might add validation or other business logic before calling the repo
	mcpServer, err := s.repo.AddMCPServer(ctx, input.Name, input.Command, input.Args, input.Envs)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(mcpServer.Command, mcpServer.Args...)	
	cmd.Env = os.Environ()
	for _, env := range mcpServer.Envs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
	}
	transport, err := GetTransport(cmd)
	if err != nil {
		return nil, err
	}

	client := mcp.NewClient(transport)
	_, err = client.Initialize(ctx)
	if err != nil {
		fmt.Println("Error initializing mcp server", err)
		return nil, err
	}

	s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
		ID: mcpServer.ID,
		Client: client,
	})

	return mcpServer, nil
}

// GetMCPServers retrieves all MCP servers using the repository.
func (s *service) GetMCPServers(ctx context.Context) ([]*model.MCPServerDefinition, error) {

	mcpservers, err := s.repo.GetMCPServers(ctx)
	if err != nil {
		return nil, err
	}

	connectedServerIds := []string{}
	for _, connectedServer := range s.connectedServers {
		connectedServerIds = append(connectedServerIds, connectedServer.ID)
	}

	mcpserversDefinitions := make([]*model.MCPServerDefinition, len(mcpservers))
	for i, mcpServer := range mcpservers {

		mcpserversDefinitions[i] = &model.MCPServerDefinition{
			ID: mcpServer.ID,
			Name: mcpServer.Name,
			Command: mcpServer.Command,
			Args: mcpServer.Args,
			Envs: mcpServer.Envs,
			Connected: slices.Contains(connectedServerIds, mcpServer.ID),
			Enabled: mcpServer.Enabled,
		}
	}

	return mcpserversDefinitions, nil
}

// LoadMCP loads a MCP server from the repository.
func (s *service) LoadMCP(ctx context.Context) error {
	servers, err := s.repo.GetMCPServers(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		cmd := exec.Command(server.Command, server.Args...)	

		cmd.Env = os.Environ()
		for _, env := range server.Envs {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
		}

		transport, err := GetTransport(cmd)
		if err != nil {
			return err
		}

		client := mcp.NewClient(transport)
		_, err = client.Initialize(ctx)
		if err != nil {
			fmt.Println("Error initializing mcp server", err)
			continue
		}

		s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
			ID: server.ID,
			Client: client,
		})
	}

	return nil
}

// GetTools retrieves all tools from the MCP servers.
func (s *service) GetTools(ctx context.Context) ([]mcp.ToolRetType, error) {
	var allTools []mcp.ToolRetType

	for _, connectedServer := range s.connectedServers {
		cursor := ""
		for {
			client_tools, err := connectedServer.Client.ListTools(ctx, &cursor)
			if err != nil {
				fmt.Println("Error getting tools for client", connectedServer.ID, err)
				continue
			}

			if allTools == nil {
				allTools = client_tools.Tools
			} else {
				allTools = append(allTools, client_tools.Tools...)
			}
			if client_tools.NextCursor == nil || *client_tools.NextCursor == "" {
				break
			}
			cursor = *client_tools.NextCursor
		}
	}
	return allTools, nil
}

func (s *service) ExecuteTool(ctx context.Context, toolName string, args any) (*mcp.ToolResponse, error) {
	for _, connectedServer := range s.connectedServers {
		cursor := ""
		tools := []mcp.ToolRetType{}
		for {
			tool, err := connectedServer.Client.ListTools(ctx, &cursor)
			if err != nil {
				fmt.Println("Error getting tools for client", connectedServer.ID, err)
				continue
			}
			tools = append(tools, tool.Tools...)

			if tool.NextCursor == nil || *tool.NextCursor == "" {
				break
			}
			cursor = *tool.NextCursor
		}
		for _, tool := range tools {
			if tool.Name == toolName {
				response, err := connectedServer.Client.CallTool(ctx, toolName, args)
				if err != nil {
					return nil, err	
				}
				return response, nil
			}
		}
	}

	return nil, errors.New("tool not found")
}

func GetTransport(cmd *exec.Cmd) (*stdio.StdioServerTransport, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	clientTransport := stdio.NewStdioServerTransportWithIO(stdout, stdin)

	return clientTransport, nil
}

