// Owner: ankit@eternis.ai
package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/log"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/google"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/repository"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/screenpipe"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/slack"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/twitter"
)

type ConnectedMCPServer struct {
	ID     string
	Client MCPClient
}

// service implements the MCPServerService interface.
type service struct {
	config           *config.Config
	store            *db.Store
	repo             repository.Repository
	connectedServers []*ConnectedMCPServer
	registry         tools.ToolRegistry
}

// NewService creates a new MCPServerService.
func NewService(ctx context.Context, logger *log.Logger, store *db.Store, registry tools.ToolRegistry) MCPService {
	repo := repository.NewRepository(logger, store.DB())
	config, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error loading config", "error", err)
	}
	service := &service{
		config:           config,
		repo:             repo,
		connectedServers: []*ConnectedMCPServer{},
		store:            store,
		registry:         registry,
	}

	err = service.LoadMCP(ctx)
	if err != nil {
		log.Error("Error loading MCP servers", "error", err)
	}

	return service
}

// AddMCPServer adds a new MCP server using the repository.
func (s *service) ConnectMCPServer(
	ctx context.Context,
	input model.ConnectMCPServerInput,
) (*model.MCPServer, error) {
	// Here you might add validation or other business logic before calling the repo
	enabled := true

	name := input.Name
	// Servers of MCPServerTypeFreysa are expected to have properly formatted names
	// set in the server. Names of all URL-type MCP servers are picked up automatically.
	if input.Type != model.MCPServerTypeOther && input.Type != model.MCPServerTypeFreysa {
		name = CapitalizeFirst(input.Type.String())
	}

	mcpServer, err := s.repo.GetMCPServerByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if mcpServer != nil {
		if s.isServerConnected(mcpServer.ID) {
			return nil, fmt.Errorf("mcp server with name %s already exists and is connected", name)
		}

		// Server exists in database but not connected.
		// Remove it first, then continue with fresh connection.
		log.Info("Found existing disconnected MCP server, removing it for fresh connection", "server", name, "id", mcpServer.ID)
		err = s.repo.DeleteMCPServer(ctx, mcpServer.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to remove existing disconnected server %s: %w", name, err)
		}
	}

	// MCPServerTypeFreysa is just a URL-type MCP server, it will be handled similar to other URL-type MCP servers.
	if input.Type != model.MCPServerTypeOther && input.Type != model.MCPServerTypeFreysa {
		input.Command = "npx"

		var client MCPClient

		switch input.Type {
		case model.MCPServerTypeTwitter:
			client = &twitter.TwitterClient{
				Store: s.store,
			}
		case model.MCPServerTypeGoogle:
			client = &google.GoogleClient{
				Store: s.store,
			}
		case model.MCPServerTypeSLACk:
			client = &slack.SlackClient{
				Store: s.store,
			}
		case model.MCPServerTypeScreenpipe:
			client = screenpipe.NewClient()
		case model.MCPServerTypeEnchanted:
			if s.config == nil {
				return nil, fmt.Errorf("config is nil, cannot connect to Enchanted MCP server")
			}
			// In case there is google oauth token, refresh it
			_, err := auth.RefreshOAuthToken(ctx, log.Default(), s.store, "google")
			if err != nil {
				return nil, fmt.Errorf("failed to refresh oauth tokens: %w", err)
			}

			// Add OAuth support for the new client
			oauth, err := s.store.GetOAuthTokens(ctx, "google")
			if err != nil {
				return nil, fmt.Errorf("failed to get oauth tokens: %w", err)
			}

			// Create client with OAuth authorization headers
			options := []transport.StreamableHTTPCOption{}
			if oauth != nil && oauth.AccessToken != "" {
				options = append(options, transport.WithHTTPHeaders(map[string]string{
					"Authorization": "Bearer " + oauth.AccessToken,
				}))
			}

			mcpClient, err := mcpclient.NewStreamableHttpClient(s.config.EnchantedMcpURL, options...)
			if err != nil {
				return nil, fmt.Errorf("failed to create MCP client: %w", err)
			}

			err = mcpClient.Start(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to start MCP client: %w", err)
			}

			initRequest := mcp.InitializeRequest{}
			initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
			initRequest.Params.ClientInfo = mcp.Implementation{
				Name:    "enchanted-twin-mcp-client",
				Version: "1.0.0",
			}

			_, err = mcpClient.Initialize(ctx, initRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
			}

			client = mcpClient
		default:
			return nil, fmt.Errorf("unsupported server type")
		}
		input.Name = CapitalizeFirst(input.Type.String())
		mcpServer, err = s.repo.AddMCPServer(ctx, &input, &enabled)
		if err != nil {
			return nil, err
		}

		s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
			ID:     mcpServer.ID,
			Client: client,
		})

		// Register tools with the registry
		s.registerMCPTools(ctx, client)

		return mcpServer, nil
	}

	if !checkMCPServerValid(input) {
		return nil, fmt.Errorf("invalid MCP server name")
	}

	// Convert input.Envs from []*model.KeyValueInput to []*model.KeyValue
	var transportEnvs []*model.KeyValue
	if input.Envs != nil {
		transportEnvs = make([]*model.KeyValue, len(input.Envs))
		for i, kvInput := range input.Envs {
			transportEnvs[i] = &model.KeyValue{Key: kvInput.Key, Value: kvInput.Value}
		}
	}

	command := input.Command
	if command == "docker" {
		command = getDockerCommand()
	}
	if command == "npx" {
		command = getNpxCommand()
	}

	var mcpClient *mcpclient.Client

	if command == "url" {
		// Create MCP token store that manages both tokens and client credentials.
		mcpTokenStore := NewTokenStore(s.store, input.Args[0])

		// Check if tokens exist before creating client and validate refresh capability.
		if existingToken, err := mcpTokenStore.GetToken(); err == nil {
			ValidateTokenRefreshCapability(existingToken, input.Name)
		}

		// Try to retrieve stored client credentials first.
		clientID, clientSecret, err := mcpTokenStore.GetClientCredentials()
		if err != nil {
			clientID = ""
			clientSecret = ""
		}

		oauthConfig := mcpclient.OAuthConfig{
			ClientID:              clientID,
			ClientSecret:          clientSecret,
			RedirectURI:           "http://localhost:8085/oauth/callback",
			Scopes:                []string{"mcp.read", "mcp.write"},
			TokenStore:            mcpTokenStore,
			PKCEEnabled:           true,
			AuthServerMetadataURL: "", // Auto-discovered.
		}

		mcpClient, err = mcpclient.NewOAuthStreamableHttpClient(input.Args[0], oauthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create OAuth HTTP MCP client: %w", err)
		}

		// HTTP clients need manual start
		err = mcpClient.Start(ctx)
		if err != nil {
			// If requires authorization, handle it.
			if mcpclient.IsOAuthAuthorizationRequiredError(err) {
				log.Warn("OAuth authorization required for MCP server. Server will continue running, but this MCP server will be unavailable until authorization is completed.", "server", input.Name)
				newMCPServer, err := s.repo.AddMCPServer(ctx, &input, &enabled)
				if err != nil {
					return nil, err
				}
				go func() {
					authErr := s.handleOAuthAuthorization(ctx, err, input.Args[0])
					if authErr != nil {
						log.Error("Failed to complete OAuth authorization", "server", input.Name, "error", authErr)
						return
					}
					if startErr := mcpClient.Start(ctx); startErr != nil {
						log.Error("Failed to start HTTP MCP client after OAuth", "server", input.Name, "error", startErr)
						return
					}
					initRequest := mcp.InitializeRequest{}
					initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
					initRequest.Params.ClientInfo = mcp.Implementation{
						Name:    "enchanted-twin-mcp-client",
						Version: "1.0.0",
					}
					if _, initErr := mcpClient.Initialize(ctx, initRequest); initErr != nil {
						log.Error("Failed to initialize HTTP MCP client after OAuth", "server", input.Name, "error", initErr)
						return
					}
					s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
						ID:     newMCPServer.ID,
						Client: mcpClient,
					})
					s.registerMCPTools(ctx, mcpClient)
					log.Info("OAuth MCP server successfully connected", "server", input.Name)
				}()
				return newMCPServer, nil
			} else {
				return nil, fmt.Errorf("failed to start HTTP MCP client: %w", err)
			}
		}

		// Initialize the client
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "enchanted-twin-mcp-client",
			Version: "1.0.0",
		}
		// Initialize the connection and get the server info (name, version)
		r, err := mcpClient.Initialize(ctx, initRequest)
		if err != nil {
			// If requires authorization, handle it, again
			if mcpclient.IsOAuthAuthorizationRequiredError(err) {
				err = s.handleOAuthAuthorization(ctx, err, input.Args[0])
				if err != nil {
					return nil, fmt.Errorf("failed to complete OAuth authorization: %w", err)
				}
				r, err = mcpClient.Initialize(ctx, initRequest)
				if err != nil {
					return nil, fmt.Errorf("failed to initialize HTTP MCP client after OAuth: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to initialize HTTP MCP client: %w", err)
			}
		}
		// Get the actual name of the MCP server
		// This shows up in the UI now
		input.Name = r.ServerInfo.Name
	} else {
		// Convert envs to string slice
		envStrings := make([]string, len(transportEnvs))
		for i, env := range transportEnvs {
			envStrings[i] = fmt.Sprintf("%s=%s", env.Key, env.Value)
		}
		mcpClient, err = mcpclient.NewStdioMCPClient(command, envStrings, input.Args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdio MCP client: %w", err)
		}
		// Initialize the client
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "enchanted-twin-mcp-client",
			Version: "1.0.0",
		}
		_, err = mcpClient.Initialize(ctx, initRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
		}
	}

	// Use the initialized client as an MCPClient interface
	clientInterface := mcpClient

	// Register tools with the registry
	s.registerMCPTools(ctx, clientInterface)

	mcpServer, err = s.repo.AddMCPServer(ctx, &input, &enabled)
	if err != nil {
		return nil, err
	}

	s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
		ID:     mcpServer.ID,
		Client: clientInterface,
	})

	return mcpServer, nil
}

// ConnectMCPServerIfNotExists connects a new MCP server if it doesn't exist.
func (s *service) ConnectMCPServerIfNotExists(
	ctx context.Context,
	input model.ConnectMCPServerInput,
) (*model.MCPServer, error) {
	mcpServer, err := s.repo.GetMCPServerByType(ctx, input.Type)
	if err != nil {
		return nil, err
	}

	if mcpServer != nil {
		return mcpServer, nil
	}

	return s.ConnectMCPServer(ctx, input)
}

// GetMCPServers retrieves all MCP servers using the repository.
func (s *service) GetMCPServers(ctx context.Context) ([]*model.MCPServerDefinition, error) {
	mcpservers, err := s.repo.GetMCPServers(ctx)
	if err != nil {
		return nil, err
	}

	connectedServerIds := []string{}
	connectedServerMap := make(map[string]*ConnectedMCPServer)
	for _, connectedServer := range s.connectedServers {
		connectedServerIds = append(connectedServerIds, connectedServer.ID)
		connectedServerMap[connectedServer.ID] = connectedServer
	}

	defaultServers := getDefaultMCPServers() // Get default servers
	mcpserversDefinitions := make([]*model.MCPServerDefinition, 0)
	existingTypes := make(map[string]bool) // Track types found in the repo

	// Process servers from the repository
	for _, mcpServer := range mcpservers {
		mcpServerDefinition := &model.MCPServerDefinition{
			ID:        mcpServer.ID,
			Name:      mcpServer.Name,
			Command:   mcpServer.Command,
			Args:      mcpServer.Args,
			Envs:      mcpServer.Envs,
			Connected: slices.Contains(connectedServerIds, mcpServer.ID),
			Enabled:   mcpServer.Enabled,
			Type:      mcpServer.Type,
			Tools:     []*model.Tool{},
		}

		if connectedServerMap[mcpServer.ID] != nil {
			mcpServerDefinition.Connected = true
			mcpServerDefinition.Enabled = true

			tools, err := getTools(ctx, connectedServerMap[mcpServer.ID])
			if err != nil {
				log.Error("Error getting tools for MCP server", "server", mcpServer.Name, "error", err)
			}
			mcpServerDefinition.Tools = tools
		}

		mcpserversDefinitions = append(mcpserversDefinitions, mcpServerDefinition)
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
		var client MCPClient

		if server.Type != model.MCPServerTypeOther && server.Type != model.MCPServerTypeFreysa {
			switch server.Type {
			case model.MCPServerTypeTwitter:
				client = &twitter.TwitterClient{
					Store: s.store,
				}
			// TODO: Re-enable after fixing compilation issues
			case model.MCPServerTypeGoogle:
				client = &google.GoogleClient{
					Store: s.store,
				}
			case model.MCPServerTypeSLACk:
				client = &slack.SlackClient{
					Store: s.store,
				}
			case model.MCPServerTypeScreenpipe:
				client = screenpipe.NewClient()
			case model.MCPServerTypeEnchanted:
				if s.config == nil {
					log.Error("Config is nil, cannot connect to Enchanted MCP server", "server", server.Name)
					continue
				}
				// In case there is google oauth token, refresh it
				_, err := auth.RefreshOAuthToken(ctx, log.Default(), s.store, "google")
				if err != nil {
					log.Error("Error refreshing oauth tokens", "error", err)
				}

				// Add OAuth support
				oauth, err := s.store.GetOAuthTokens(ctx, "google")
				if err != nil {
					log.Error("Error getting oauth tokens for MCP server", "server", server.Name, "error", err)
					continue
				}

				// Create client with OAuth authorization headers
				options := []transport.StreamableHTTPCOption{}
				if oauth != nil && oauth.AccessToken != "" {
					options = append(options, transport.WithHTTPHeaders(map[string]string{
						"Authorization": "Bearer " + oauth.AccessToken,
					}))
				}

				mcpClient, err := mcpclient.NewStreamableHttpClient(s.config.EnchantedMcpURL, options...)
				if err != nil {
					log.Error("Error creating MCP client", "server", server.Name, "error", err)
					continue
				}
				err = mcpClient.Start(ctx)
				if err != nil {
					log.Error("Error starting MCP server", "server", server.Name, "error", err)
					continue
				}
				// Initialize the client
				initRequest := mcp.InitializeRequest{}
				initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
				initRequest.Params.ClientInfo = mcp.Implementation{
					Name:    "enchanted-twin-mcp-client",
					Version: "1.0.0",
				}
				_, err = mcpClient.Initialize(ctx, initRequest)
				if err != nil {
					log.Error("Error initializing MCP client", "server", server.Name, "error", err)
					continue
				}
				client = mcpClient
			default:
				// nothing to do
				continue
			}

			s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
				ID:     server.ID,
				Client: client,
			})

			// Register tools with the registry
			s.registerMCPTools(ctx, client)

			continue
		}

		command := server.Command
		if command == "docker" {
			command = getDockerCommand()
		}
		if command == "npx" {
			command = getNpxCommand()
		}

		var mcpClient *mcpclient.Client
		if command == "url" {
			mcpTokenStore := NewTokenStore(s.store, server.Args[0])

			if existingToken, err := mcpTokenStore.GetToken(); err == nil {
				ValidateTokenRefreshCapability(existingToken, server.Name)
			}

			clientID, clientSecret, err := mcpTokenStore.GetClientCredentials()
			if err != nil {
				clientID = ""
				clientSecret = ""
			}

			oauthConfig := mcpclient.OAuthConfig{
				ClientID:              clientID,
				ClientSecret:          clientSecret,
				RedirectURI:           "http://localhost:8085/oauth/callback",
				Scopes:                []string{"mcp.read", "mcp.write"},
				TokenStore:            mcpTokenStore,
				PKCEEnabled:           true,
				AuthServerMetadataURL: "",
			}

			mcpClient, err = mcpclient.NewOAuthStreamableHttpClient(server.Args[0], oauthConfig)
			if err != nil {
				log.Error("Error creating OAuth HTTP MCP client", "server", server.Name, "error", err)
				continue
			}

			// Start HTTP client
			err = mcpClient.Start(ctx)
			if err != nil {
				if mcpclient.IsOAuthAuthorizationRequiredError(err) {
					log.Warn("OAuth authorization required for MCP server during startup. Server will continue running, but this MCP server will be unavailable until authorization is completed.", "server", server.Name)
					go func() {
						authErr := s.handleOAuthAuthorization(ctx, err, server.Args[0])
						if authErr != nil {
							log.Error("Failed to complete OAuth authorization during startup", "server", server.Name, "error", authErr)
							return
						}
						if startErr := mcpClient.Start(ctx); startErr != nil {
							log.Error("Error starting HTTP MCP client after OAuth during startup", "server", server.Name, "error", startErr)
							return
						}
						initRequest := mcp.InitializeRequest{}
						initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
						initRequest.Params.ClientInfo = mcp.Implementation{
							Name:    "enchanted-twin-mcp-client",
							Version: "1.0.0",
						}
						if _, initErr := mcpClient.Initialize(ctx, initRequest); initErr != nil {
							log.Error("Error initializing HTTP MCP client after OAuth during startup", "server", server.Name, "error", initErr)
							return
						}
						s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
							ID:     server.ID,
							Client: mcpClient,
						})
						s.registerMCPTools(ctx, mcpClient)
						log.Info("OAuth MCP server successfully connected during startup", "server", server.Name)
					}()
					continue
				} else {
					log.Error("Error starting HTTP MCP client", "server", server.Name, "error", err)
					continue
				}
			}

			// Initialize the client
			initRequest := mcp.InitializeRequest{}
			initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
			initRequest.Params.ClientInfo = mcp.Implementation{
				Name:    "enchanted-twin-mcp-client",
				Version: "1.0.0",
			}
			_, err = mcpClient.Initialize(ctx, initRequest)
			if err != nil {
				log.Error("Error initializing HTTP MCP client", "server", server.Name, "error", err)
				continue
			}
		} else {
			// Convert envs to string slice
			envStrings := make([]string, len(server.Envs))
			for i, env := range server.Envs {
				envStrings[i] = fmt.Sprintf("%s=%s", env.Key, env.Value)
			}
			mcpClient, err = mcpclient.NewStdioMCPClient(command, envStrings, server.Args...)
			if err != nil {
				log.Error("Error creating stdio MCP client", "server", server.Name, "error", err)
				continue
			}
			// Initialize the client
			initRequest := mcp.InitializeRequest{}
			initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
			initRequest.Params.ClientInfo = mcp.Implementation{
				Name:    "enchanted-twin-mcp-client",
				Version: "1.0.0",
			}
			_, err = mcpClient.Initialize(ctx, initRequest)
			if err != nil {
				log.Error("Error initializing MCP client", "server", server.Name, "error", err)
				continue
			}
		}

		// Use the initialized client as an MCPClient interface
		client = mcpClient

		s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
			ID:     server.ID,
			Client: client,
		})

		// Register tools with the registry
		s.registerMCPTools(ctx, client)
	}

	return nil
}

// GetTools retrieves all tools from the MCP servers.
func (s *service) GetTools(ctx context.Context) ([]mcp.Tool, error) {
	var allTools []mcp.Tool
	for _, connectedServer := range s.connectedServers {
		request := mcp.ListToolsRequest{}
		// TODO: Handle pagination if needed
		client_tools, err := connectedServer.Client.ListTools(ctx, request)
		if err != nil {
			log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
			continue
		}

		if allTools == nil {
			allTools = client_tools.Tools
		} else {
			allTools = append(allTools, client_tools.Tools...)
		}
	}
	return allTools, nil
}

func (s *service) GetInternalTools(ctx context.Context) ([]tools.Tool, error) {
	var allTools []tools.Tool

	for _, connectedServer := range s.connectedServers {
		request := mcp.ListToolsRequest{}
		client_tools, err := connectedServer.Client.ListTools(ctx, request)
		if err != nil {
			log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
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
	}
	return allTools, nil
}

func (s *service) RemoveMCPServer(ctx context.Context, id string) error {
	var client MCPClient
	// Remove the server from the connected servers
	for i, connectedServer := range s.connectedServers {
		if connectedServer.ID == id {
			if i < len(s.connectedServers)-1 {
				s.connectedServers = append(s.connectedServers[:i], s.connectedServers[i+1:]...)
			} else {
				s.connectedServers = s.connectedServers[:i]
			}
			client = connectedServer.Client
			break
		}
	}

	s.deregisterMCPTools(ctx, client)

	err := s.repo.DeleteMCPServer(ctx, id)
	if err != nil {
		return err
	}

	return nil
}

// GetRegistry returns the tool registry.
func (s *service) GetRegistry() tools.ToolRegistry {
	return s.registry
}

// isServerConnected checks if a server with the given ID is currently connected.
func (s *service) isServerConnected(serverID string) bool {
	for _, connectedServer := range s.connectedServers {
		if connectedServer.ID == serverID {
			return true
		}
	}
	return false
}

// registerMCPTools registers tools from an MCP client with the tool registry.
func (s *service) registerMCPTools(ctx context.Context, client MCPClient) {
	if s.registry == nil {
		return
	}
	request := mcp.ListToolsRequest{}
	tools, err := client.ListTools(ctx, request)
	if err != nil {
		log.Warn("Error getting tools from MCP client", "error", err)
		return
	}

	if tools == nil || len(tools.Tools) == 0 {
		return
	}

	for _, tool := range tools.Tools {
		mcpTool := &MCPTool{
			Client: client,
			Tool:   tool,
		}
		if err := s.registry.Register(mcpTool); err != nil {
			log.Warn("Error registering MCP tool", "tool", tool.GetName(), "error", err)
		}
	}
}

func (s *service) deregisterMCPTools(ctx context.Context, client MCPClient) {
	if s.registry == nil {
		return
	}
	request := mcp.ListToolsRequest{}
	tools, err := client.ListTools(ctx, request)
	if err != nil {
		log.Warn("Error getting tools from MCP client", "error", err)
		return
	}

	toolNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.GetName())
	}
	s.registry = s.registry.Excluding(toolNames...)
}

// GetTransport creates a transport based on the server configuration.
// It supports STDIN/STDOUT (stdio) and HTTPS protocols.
// Assumes model.MCPServerTransportHTTPS and model.MCPServerTransportStdio constants exist in your model package.
// Unused since #353.
// func GetTransportWithHTTP(
// 	ctx context.Context,
// 	serverURL *string,
// 	accessToken *string,
// ) (transport.Interface, error) {
// 	if serverURL == nil || *serverURL == "" {
// 		return nil, fmt.Errorf("URL is required for HTTPS transport")
// 	}
//
// 	var options []transport.StreamableHTTPCOption
// 	if accessToken != nil {
// 		options = append(options, transport.WithHTTPHeaders(map[string]string{
// 			"Authorization": "Bearer " + *accessToken,
// 		}))
// 	}
//
// 	transport, err := transport.NewStreamableHTTP(*serverURL, options...)
// 	return transport, err
// }

// Unused since #353.
// func GetTransportWithIO(
// 	ctx context.Context,
// 	command string,
// 	args []string,
// 	envs []*model.KeyValue,
// ) (transport.Interface, error) {
// 	effectiveCommand := command
// 	if command == "docker" {
// 		effectiveCommand = getDockerCommand()
// 	}
//
// 	if command == "npx" {
// 		effectiveCommand = getNpxCommand()
// 	}
//
// 	// Convert envs to string slice
// 	envStrings := make([]string, len(envs))
// 	for i, env := range envs {
// 		envStrings[i] = fmt.Sprintf("%s=%s", env.Key, env.Value)
// 	}
//
// 	return transport.NewStdio(effectiveCommand, envStrings, args...), nil
// }

func getDockerCommand() string {
	var dockerCommand string
	path, err := exec.LookPath("docker")
	if err != nil {
		commonPaths := []string{
			"/usr/local/bin/docker",
			"/usr/bin/docker",
			filepath.Join(os.Getenv("HOME"), ".local/bin/docker"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path != "" {
		dockerCommand = path
	} else {
		dockerCommand = "docker"
	}

	return dockerCommand
}

func getNpxCommand() string {
	var npxCommand string
	path, err := exec.LookPath("npx")
	if err != nil {
		commonPaths := []string{
			"/usr/local/bin/npx",
			"/usr/bin/npx",
			filepath.Join(os.Getenv("HOME"), ".local/bin/npx"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path != "" {
		npxCommand = path
	} else {
		npxCommand = "npx"
	}
	return npxCommand
}

func checkMCPServerValid(input model.ConnectMCPServerInput) bool {
	if strings.EqualFold(input.Name, model.MCPServerTypeGoogle.String()) {
		return false
	}

	if strings.EqualFold(input.Name, model.MCPServerTypeTwitter.String()) {
		return false
	}

	if strings.EqualFold(input.Name, model.MCPServerTypeSLACk.String()) {
		return false
	}

	if strings.EqualFold(input.Name, model.MCPServerTypeScreenpipe.String()) {
		return false
	}

	if input.Name == "" {
		return false
	}

	return true
}

// CapitalizeFirst capitalizes the first rune of a string
// and converts the rest of the runes to lowercase.
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return ""
	}
	runes := []rune(s)
	firstRune := unicode.ToUpper(runes[0])
	if len(runes) > 1 {
		restOfString := strings.ToLower(string(runes[1:]))
		return string(firstRune) + restOfString
	}
	return string(firstRune)
}

func getTools(ctx context.Context, connectedServer *ConnectedMCPServer) ([]*model.Tool, error) {
	allTools := []*model.Tool{}
	request := mcp.ListToolsRequest{}
	client_tools, err := connectedServer.Client.ListTools(ctx, request)
	if err != nil {
		log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
		return allTools, err
	}

	for _, tool := range client_tools.Tools {
		allTools = append(allTools, &model.Tool{
			Name:        tool.GetName(),
			Description: tool.Description,
		})
	}

	return allTools, nil
}

// handleOAuthAuthorization handles the OAuth authorization flow.
func (s *service) handleOAuthAuthorization(ctx context.Context, authErr error, serverURL string) error {
	log.Info("OAuth authorization required. Starting authorization flow...")

	oauthHandler := mcpclient.GetOAuthHandler(authErr)

	callbackChan := make(chan map[string]string, 1)
	server := s.startCallbackServer(callbackChan)
	defer server.Close() //nolint:errcheck

	codeVerifier, err := mcpclient.GenerateCodeVerifier()
	if err != nil {
		return fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := mcpclient.GenerateCodeChallenge(codeVerifier)

	state, err := mcpclient.GenerateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	err = oauthHandler.RegisterClient(ctx, "enchanted-twin-mcp-client")
	if err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	authURL, err := oauthHandler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("failed to get authorization URL: %w", err)
	}

	log.Info("Opening browser to authorization URL", "url", authURL)
	s.openBrowser(authURL)

	log.Info("Waiting for authorization callback...")
	select {
	case params := <-callbackChan:
		if params["state"] != state {
			return fmt.Errorf("state mismatch: expected %s, got %s", state, params["state"])
		}

		code := params["code"]
		if code == "" {
			return fmt.Errorf("no authorization code received")
		}

		log.Info("Exchanging authorization code for token...")
		err = oauthHandler.ProcessAuthorizationResponse(ctx, code, state, codeVerifier)
		if err != nil {
			return fmt.Errorf("failed to process authorization response: %w", err)
		}

		log.Info("Authorization successful!")

		if err := s.saveClientCredentialsFromHandler(ctx, oauthHandler, serverURL); err != nil {
			log.Warn("Failed to save client credentials after OAuth completion", "error", err)
		}

		log.Info("Validating stored token for automatic refresh capability...")

		return nil

	case <-time.After(5 * time.Minute):
		return fmt.Errorf("OAuth authorization timed out")
	}
}

// saveClientCredentialsFromHandler extracts and saves client credentials from an OAuth handler.
func (s *service) saveClientCredentialsFromHandler(ctx context.Context, oauthHandler interface{}, serverURL string) error {
	if handler, ok := oauthHandler.(*transport.OAuthHandler); ok {
		clientID := handler.GetClientID()
		clientSecret := handler.GetClientSecret()

		mcpTokenStore := NewTokenStore(s.store, serverURL)
		if err := mcpTokenStore.SetClientCredentials(clientID, clientSecret); err != nil {
			return fmt.Errorf("failed to save client credentials: %w", err)
		}

		return nil
	}

	return fmt.Errorf("OAuth handler is not of expected type: %T", oauthHandler)
}

// startCallbackServer starts a local HTTP server to handle the OAuth callback.
func (s *service) startCallbackServer(callbackChan chan<- map[string]string) *http.Server {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8085",
		Handler: mux,
	}

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received OAuth callback", "url", r.URL.String())

		params := make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		select {
		case callbackChan <- params:
			log.Info("Sent OAuth parameters to channel")
		default:
			log.Warn("Channel full, dropping OAuth callback parameters")
		}

		// User-facing response
		// Similar to the one in oauthHandler.ts
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			  <head>
			    <title>Authentication Successful</title>
			    <style>
			      body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding: 40px; }
			      h1 { color: #333; }
			      p { color: #666; }
			      .success { color: #4CAF50; font-weight: bold; }
			    </style>
			  </head>
			  <body>
			    <h1>Authentication Successful</h1>
			    <p class="success">You have successfully authenticated!</p>
			    <p>You can close this window and return to the application.</p>
			    <script>window.close();</script>
			  </body>
			</html>
        `))
		if err != nil {
			log.Error("Error writing OAuth callback response", "error", err)
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("OAuth callback server error", "error", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	log.Info("OAuth callback server started on :8085")
	return server
}

// openBrowser opens the default browser to the specified URL.
func (s *service) openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Error("Failed to open browser", "error", err)
		log.Info("Please open the following URL in your browser", "url", url)
	}
}
