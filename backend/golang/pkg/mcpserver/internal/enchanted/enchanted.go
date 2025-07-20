package enchanted

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// EnchantedMCPClient wraps the MCP client to ensure that tool calls don't
// try to use expired access tokens. This is a workaround to prevent the client
// from not being able to use the MCP server after the initial token is expired.
// TODO: @pottekkat Refactor MCP server to use MCP-spec compliant transport.
type EnchantedMCPClient struct {
	store           *db.Store
	logger          *log.Logger
	enchantedMcpURL string

	mu        sync.RWMutex
	client    *mcpclient.Client
	lastToken string
}

// NewEnchantedMCPClient creates a new Enchanted MCP client.
func NewEnchantedMCPClient(store *db.Store, logger *log.Logger, enchantedMcpURL string) *EnchantedMCPClient {
	return &EnchantedMCPClient{
		store:           store,
		logger:          logger,
		enchantedMcpURL: enchantedMcpURL,
	}
}

// getCurrentFirebaseToken gets the current Firebase
// token from the store and refreshes it if needed.
func (d *EnchantedMCPClient) getCurrentFirebaseToken(ctx context.Context) (string, error) {
	oauth, err := d.store.GetOAuthTokens(ctx, "firebase")
	if err != nil {
		return "", err
	}

	if oauth != nil && (oauth.ExpiresAt.Before(time.Now()) || oauth.Error) {
		d.logger.Debug("Refreshing expired Firebase token for Enchanted MCP server")
		_, err = auth.RefreshOAuthToken(ctx, d.logger, d.store, "firebase")
		if err != nil {
			return "", err
		}
		oauth, err = d.store.GetOAuthTokens(ctx, "firebase")
		if err != nil {
			return "", err
		}
	}

	if oauth == nil || oauth.AccessToken == "" {
		return "", nil
	}

	return oauth.AccessToken, nil
}

// ensureClientWithFreshToken ensures that the client has a fresh token.
// Creates a new client if the token has changed.
func (d *EnchantedMCPClient) ensureClientWithFreshToken(ctx context.Context) error {
	currentToken, err := d.getCurrentFirebaseToken(ctx)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Token is valid, no need to recreate.
	if d.client != nil && d.lastToken == currentToken {
		return nil
	}

	d.logger.Debug("Firebase token changed, recreating Enchanted MCP client",
		"token_changed", d.lastToken != currentToken,
		"client_exists", d.client != nil)

	// Close the current client.
	if d.client != nil {
		d.client = nil
	}

	options := []transport.StreamableHTTPCOption{}
	if currentToken != "" {
		options = append(options, transport.WithHTTPHeaders(map[string]string{
			"Authorization": "Bearer " + currentToken,
		}))
	}

	mcpClient, err := mcpclient.NewStreamableHttpClient(d.enchantedMcpURL, options...)
	if err != nil {
		return err
	}

	err = mcpClient.Start(ctx)
	if err != nil {
		return err
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "enchanted-twin-mcp-client",
		Version: "1.0.0",
	}

	_, err = mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		return err
	}

	d.client = mcpClient
	d.lastToken = currentToken

	d.logger.Debug("Successfully created new Enchanted MCP client with fresh Firebase token")
	return nil
}

// CallTool wraps CallTool from the SDK with refreshed tokens.
func (d *EnchantedMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.ensureClientWithFreshToken(ctx); err != nil {
		return nil, err
	}

	d.mu.RLock()
	client := d.client
	d.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no valid MCP client available")
	}

	return client.CallTool(ctx, request)
}

// ListTools wraps ListTools from the SDK with refreshed tokens.
func (d *EnchantedMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	if err := d.ensureClientWithFreshToken(ctx); err != nil {
		return nil, err
	}

	d.mu.RLock()
	client := d.client
	d.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no valid MCP client available")
	}

	return client.ListTools(ctx, request)
}
