package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	mcpclient "github.com/mark3labs/mcp-go/client"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// DatabaseTokenStore implements the TokenStore interface
// using the existing oauth_tokens table. Uses `mcp` as
// the provider identifier and server URL as the username.
// This is to be used by external MCP servers to persist
// tokens in the database.
type DatabaseTokenStore struct {
	store    *db.Store
	provider string
	username string
	mu       sync.RWMutex
}

// NewDatabaseTokenStore creates a new DatabaseTokenStore.
func NewDatabaseTokenStore(store *db.Store, identifier string) *DatabaseTokenStore {
	provider := "mcp"

	return &DatabaseTokenStore{
		store:    store,
		provider: provider,
		username: identifier,
	}
}

// GetToken retrieves a token by username from the database.
func (s *DatabaseTokenStore) GetToken() (*mcpclient.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx := context.Background()

	oauthToken, err := s.store.GetOAuthTokensByUsername(ctx, s.provider, s.username)
	if err != nil {
		return nil, fmt.Errorf("no token available for MCP server provider %s, username %s: %w", s.provider, s.username, err)
	}

	// Convert OAuth token to MCP client token
	mcpToken := &mcpclient.Token{
		AccessToken:  oauthToken.AccessToken,
		TokenType:    oauthToken.TokenType,
		RefreshToken: oauthToken.RefreshToken,
		ExpiresAt:    oauthToken.ExpiresAt,
		Scope:        oauthToken.Scope,
	}

	// Calculate ExpiresIn
	if !oauthToken.ExpiresAt.IsZero() {
		expiresIn := time.Until(oauthToken.ExpiresAt).Seconds()
		if expiresIn > 0 {
			mcpToken.ExpiresIn = int64(expiresIn)
		}
	}

	return mcpToken, nil
}

// SaveToken saves a token to the database.
func (s *DatabaseTokenStore) SaveToken(token *mcpclient.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	// Calculate ExpiresAt
	expiresAt := token.ExpiresAt
	if expiresAt.IsZero() && token.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	oauthTokens := db.OAuthTokens{
		Provider:     s.provider,
		TokenType:    token.TokenType,
		Scope:        token.Scope,
		AccessToken:  token.AccessToken,
		ExpiresAt:    expiresAt,
		RefreshToken: token.RefreshToken,
		Username:     s.username,
		Error:        false,
	}

	if err := s.store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return fmt.Errorf("failed to save MCP OAuth token: %w", err)
	}

	log.Debug("saved MCP OAuth token to database",
		"provider", s.provider,
		"username", s.username,
		"expires_at", expiresAt,
		"token_type", token.TokenType)

	return nil
}
