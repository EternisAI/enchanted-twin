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
	mcp "github.com/metoro-io/mcp-golang"
	mcptransport "github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	"github.com/metoro-io/mcp-golang/transport/stdio"

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

// NewServiceForFx creates a new MCPService for fx dependency injection
func NewServiceForFx(logger *log.Logger, store *db.Store, toolRegistry *tools.ToolMapRegistry) MCPService {
	return NewService(context.Background(), logger, store, toolRegistry)
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

			oauth, err := s.store.GetOAuthTokens(ctx, "google")
			if err != nil {
				return nil, fmt.Errorf("failed to get oauth tokens: %w", err)
			}

			transport, err := GetTransportWithHTTP(ctx, &s.config.EnchantedMcpURL, &oauth.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("failed to get transport: %w", err)
			}
			mcpClient := mcp.NewClient(transport)
			_, err = mcpClient.Initialize(ctx)
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

	transport, err := GetTransportWithIO(ctx, input.Command, input.Args, transportEnvs)
	if err != nil {
		return nil, fmt.Errorf("failed to get transport: %w", err)
	}

	// Create the client using direct mcp.NewClient call to get Initialize method
	mcpClient := mcp.NewClient(transport)
	_, err = mcpClient.Initialize(ctx)
	if err != nil {
		log.Error("Error initializing mcp server", "error", err)
		return nil, err
	}

	// Use the initialized client as an MCPClient interface
	client := mcpClient

	// Register tools with the registry
	s.registerMCPTools(ctx, client)

	mcpServer, err = s.repo.AddMCPServer(ctx, &input, &enabled)
	if err != nil {
		return nil, err
	}

	s.connectedServers = append(s.connectedServers, &ConnectedMCPServer{
		ID:     mcpServer.ID,
		Client: client,
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

				oauth, err := s.store.GetOAuthTokens(ctx, "google")
				if err != nil {
					log.Error("Error getting oauth tokens for MCP server", "server", server.Name, "error", err)
					continue
				}

				transport, err := GetTransportWithHTTP(ctx, &s.config.EnchantedMcpURL, &oauth.AccessToken)
				if err != nil {
					log.Error("Error getting transport for MCP server", "server", server.Name, "error", err)
					continue
				}
				mcpClient := mcp.NewClient(transport)
				_, err = mcpClient.Initialize(ctx)
				if err != nil {
					log.Error("Error initializing MCP server", "server", server.Name, "error", err)
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

		transport, err := GetTransportWithIO(ctx, command, server.Args, server.Envs)
		if err != nil {
			log.Error("Error getting transport for MCP server", "server", server.Name, "error", err)
			continue
		}

		// Create the client using direct mcp.NewClient call to get Initialize method
		mcpClient := mcp.NewClient(transport)
		_, err = mcpClient.Initialize(ctx)
		if err != nil {
			log.Error("Error initializing mcp server", "server", server.Name)
			continue
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
func (s *service) GetTools(ctx context.Context) ([]mcp.ToolRetType, error) {
	var allTools []mcp.ToolRetType

	for _, connectedServer := range s.connectedServers {
		cursor := ""
		for {
			client_tools, err := connectedServer.Client.ListTools(ctx, &cursor)
			if err != nil {
				log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
				break
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
				log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
				break
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
	cursor := ""
	tools, err := client.ListTools(ctx, &cursor)
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
			log.Warn("Error registering MCP tool", "tool", tool.Name, "error", err)
		}
	}
}

func (s *service) deregisterMCPTools(ctx context.Context, client MCPClient) {
	if s.registry == nil {
		return
	}
	cursor := ""
	tools, err := client.ListTools(ctx, &cursor)
	if err != nil {
		log.Warn("Error getting tools from MCP client", "error", err)
		return
	}

	toolNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
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
) (mcptransport.Transport, error) {
	if serverURL == nil || *serverURL == "" {
		return nil, fmt.Errorf("URL is required for HTTPS transport")
	}
	// mcphttp.NewHTTPClientTransport takes (baseURL string, client *stdhttp.Client)
	// Using nil for the client will use http.DefaultClient.
	transport := mcphttp.NewHTTPClientTransport(*serverURL)
	if accessToken != nil {
		transport.WithHeader("Authorization", "Bearer "+*accessToken)
	}
	return transport, nil
}

func GetTransportWithIO(
	ctx context.Context,
	command string,
	args []string,
	envs []*model.KeyValue,
) (mcptransport.Transport, error) {
	effectiveCommand := command
	if command == "docker" {
		effectiveCommand = getDockerCommand()
	}

	if command == "npx" {
		effectiveCommand = getNpxCommand()
	}

	cmd := exec.Command(effectiveCommand, args...)
	cmd.Env = os.Environ()
	for _, env := range envs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command for stdio transport: %w", err)
	}
	return stdio.NewStdioServerTransportWithIO(stdout, stdin), nil
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
	cursor := ""
	for {
		client_tools, err := connectedServer.Client.ListTools(ctx, &cursor)
		if err != nil {
			log.Warn("Error getting tools for client", "clientID", connectedServer.ID, "error", err)
			break
		}

		for _, tool := range client_tools.Tools {
			allTools = append(allTools, &model.Tool{
				Name:        tool.Name,
				Description: *tool.Description,
			})
		}

		if client_tools.NextCursor == nil || *client_tools.NextCursor == "" {
			break
		}
		cursor = *client_tools.NextCursor
	}
	return allTools, nil
}
