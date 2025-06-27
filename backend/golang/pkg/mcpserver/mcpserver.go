// Owner: ankit@eternis.ai
package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
	if input.Type != model.MCPServerTypeOther {
		name = CapitalizeFirst(input.Type.String())
	}

	mcpServer, err := s.repo.GetMCPServerByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if mcpServer != nil {
		return nil, fmt.Errorf("mcp server with name %s already exists", name)
	}

	if input.Type != model.MCPServerTypeOther {
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

			// TODO: Add OAuth support for the new client
			// oauth, err := s.store.GetOAuthTokens(ctx, "google")
			// if err != nil {
			// 	return nil, fmt.Errorf("failed to get oauth tokens: %w", err)
			// }

			// TODO: Need to determine the correct client constructor based on transport type
			// For now, using NewStreamableHttpClient
			mcpClient, err := mcpclient.NewStreamableHttpClient(s.config.EnchantedMcpURL)
			if err != nil {
				return nil, fmt.Errorf("failed to create MCP client: %w", err)
			}
			err = mcpClient.Start(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to start MCP client: %w", err)
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
		mcpClient, err = mcpclient.NewStreamableHttpClient(input.Args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP MCP client: %w", err)
		}
		// HTTP clients need manual start
		err = mcpClient.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to start HTTP MCP client: %w", err)
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
			return nil, fmt.Errorf("failed to initialize HTTP MCP client: %w", err)
		}
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

	// Only start HTTP clients, stdio clients auto-start
	if command == "url" {
		err = mcpClient.Start(ctx)
		if err != nil {
			log.Error("Error starting mcp client", "error", err)
			return nil, err
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

		if server.Type != model.MCPServerTypeOther {
			switch server.Type {
			case model.MCPServerTypeTwitter:
				client = &twitter.TwitterClient{
					Store: s.store,
				}
			// TODO: Re-enable after fixing compilation issues
			// case model.MCPServerTypeGoogle:
			// 	client = &google.GoogleClient{
			// 		Store: s.store,
			// 	}
			case model.MCPServerTypeSLACk:
				client = &slack.SlackClient{
					Store: s.store,
				}
			// case model.MCPServerTypeScreenpipe:
			// 	client = screenpipe.NewClient()
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

				// TODO: Add OAuth support
				// oauth, err := s.store.GetOAuthTokens(ctx, "google")
				// if err != nil {
				// 	log.Error("Error getting oauth tokens for MCP server", "server", server.Name, "error", err)
				// 	continue
				// }

				// TODO: Add OAuth support
				// transport, err := GetTransportWithHTTP(ctx, &s.config.EnchantedMcpURL, &oauth.AccessToken)
				// if err != nil {
				// 	log.Error("Error getting transport for MCP server", "server", server.Name, "error", err)
				// 	continue
				// }
				// TODO: Replace with new client
				mcpClient, err := mcpclient.NewStreamableHttpClient(s.config.EnchantedMcpURL)
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
			mcpClient, err = mcpclient.NewStreamableHttpClient(server.Args[0])
			if err != nil {
				log.Error("Error creating HTTP MCP client", "server", server.Name, "error", err)
				continue
			}
			// Start HTTP client
			err = mcpClient.Start(ctx)
			if err != nil {
				log.Error("Error starting HTTP MCP client", "server", server.Name, "error", err)
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
func GetTransportWithHTTP(
	ctx context.Context,
	serverURL *string,
	accessToken *string,
) (transport.Interface, error) {
	if serverURL == nil || *serverURL == "" {
		return nil, fmt.Errorf("URL is required for HTTPS transport")
	}

	var options []transport.StreamableHTTPCOption
	if accessToken != nil {
		options = append(options, transport.WithHTTPHeaders(map[string]string{
			"Authorization": "Bearer " + *accessToken,
		}))
	}

	transport, err := transport.NewStreamableHTTP(*serverURL, options...)
	return transport, err
}

func GetTransportWithIO(
	ctx context.Context,
	command string,
	args []string,
	envs []*model.KeyValue,
) (transport.Interface, error) {
	effectiveCommand := command
	if command == "docker" {
		effectiveCommand = getDockerCommand()
	}

	if command == "npx" {
		effectiveCommand = getNpxCommand()
	}

	// Convert envs to string slice
	envStrings := make([]string, len(envs))
	for i, env := range envs {
		envStrings[i] = fmt.Sprintf("%s=%s", env.Key, env.Value)
	}

	return transport.NewStdio(effectiveCommand, envStrings, args...), nil
}

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
