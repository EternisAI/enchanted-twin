package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/google"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/repository"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/slack"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/twitter"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type ConnectedMCPServer struct {
	ID     string
	Client MCPClient
}

// service implements the MCPServerService interface.
type service struct {
	store            *db.Store
	repo             repository.Repository
	connectedServers []*ConnectedMCPServer
}

// NewService creates a new MCPServerService.
func NewService(ctx context.Context, repo repository.Repository, store *db.Store) MCPService {
	service := &service{repo: repo, connectedServers: []*ConnectedMCPServer{}, store: store}
	err := service.LoadMCP(ctx)

	if err != nil {
		fmt.Println("Error loading MCP servers", err)
	}
	return service
}

// AddMCPServer adds a new MCP server using the repository.
func (s *service) ConnectMCPServer(ctx context.Context, input model.ConnectMCPServerInput) (*model.MCPServer, error) {
	// Here you might add validation or other business logic before calling the repo
	enabled := true

	if input.Type != model.MCPServerTypeOther {
		input.Command = "npx"

		mcpServer, err := s.repo.AddMCPServer(ctx, &input, &enabled)
		if err != nil {
			return nil, err
		}

		switch input.Type {
		case model.MCPServerTypeTwitter:
			s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
				ID: mcpServer.ID,
				Client: &twitter.TwitterClient{
					Store: s.store,
				},
			})
		case model.MCPServerTypeGoogle:
			s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
				ID: mcpServer.ID,
				Client: &google.GoogleClient{
					Store: s.store,
				},
			})
		case model.MCPServerTypeSLACk:
			s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
				ID: mcpServer.ID,
				Client: &slack.SlackClient{
					Store: s.store,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported server type")
		}
		return mcpServer, nil
	}

	mcpServer, err := s.repo.AddMCPServer(ctx, &input, &enabled)
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
		ID:     mcpServer.ID,
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

	defaultServers := getDefaultMCPServers() // Get default servers
	mcpserversDefinitions := make([]*model.MCPServerDefinition, 0)
	existingTypes := make(map[string]bool) // Track types found in the repo

	// Process servers from the repository
	for _, mcpServer := range mcpservers {
		mcpserversDefinitions = append(mcpserversDefinitions, &model.MCPServerDefinition{
			ID:        mcpServer.ID,
			Name:      mcpServer.Name,
			Command:   mcpServer.Command,
			Args:      mcpServer.Args,
			Envs:      mcpServer.Envs,
			Connected: slices.Contains(connectedServerIds, mcpServer.ID),
			Enabled:   mcpServer.Enabled,
			Type:      mcpServer.Type,
		})
		existingTypes[string(mcpServer.Type)] = true // Mark type as existing
	}

	// Add default servers if their type is not already present
	for _, defaultServer := range defaultServers {
		if _, exists := existingTypes[string(defaultServer.Type)]; !exists {
			mcpserversDefinitions = append(mcpserversDefinitions, &model.MCPServerDefinition{
				ID:        defaultServer.ID, // Assuming defaultServer has an ID
				Name:      defaultServer.Name,
				Command:   defaultServer.Command,
				Args:      defaultServer.Args,
				Envs:      defaultServer.Envs,
				Connected: false, // Default servers added this way are not connected
				Enabled:   false, // Default servers added this way are not enabled by default
				Type:      defaultServer.Type,
			})
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

		if server.Type != model.MCPServerTypeOther {
			switch server.Type {
			case model.MCPServerTypeTwitter:
				s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
					ID: server.ID,
					Client: &twitter.TwitterClient{
						Store: s.store,
					},
				})
			case model.MCPServerTypeGoogle:
				s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
					ID: server.ID,
					Client: &google.GoogleClient{
						Store: s.store,
					},
				})
			case model.MCPServerTypeSLACk:
				s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
					ID: server.ID,
					Client: &slack.SlackClient{
						Store: s.store,
					},
				})
			default:
				// nothing to do
			}
			continue
		}

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
			fmt.Printf("Error initializing mcp server: %s\n", server.Name)
			continue
		}

		s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
			ID:     server.ID,
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

func (s *service) GetInternalTools(ctx context.Context) ([]tools.Tool, error) {
	var allTools []tools.Tool

	for _, connectedServer := range s.connectedServers {
		cursor := ""
		for {
			client_tools, err := connectedServer.Client.ListTools(ctx, &cursor)
			if err != nil {
				fmt.Println("Error getting tools for client", connectedServer.ID, err)
				continue
			}

			if client_tools != nil && len(client_tools.Tools) > 0 {
				for _, tool := range client_tools.Tools {
					allTools = append(allTools, &MCPTool{
						Client: connectedServer.Client,
						Tool:   tool,
					})
				}
			}
			if client_tools.NextCursor == nil || *client_tools.NextCursor == "" {
				break
			}
			cursor = *client_tools.NextCursor
		}
	}
	return allTools, nil
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
