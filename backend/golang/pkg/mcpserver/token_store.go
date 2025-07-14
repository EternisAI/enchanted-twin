package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/zalando/go-keyring"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// TokenStore implements the mcpclient.TokenStore interface with automatic keyring-to-database fallback
// and OAuth client credential management for MCP servers.
type TokenStore struct {
	store      *db.Store
	identifier string
	timeout    time.Duration
	mu         sync.RWMutex
}

// NewTokenStore creates a new TokenStore that handles both token storage and client credentials.
func NewTokenStore(store *db.Store, identifier string) *TokenStore {
	return &TokenStore{
		store:      store,
		identifier: identifier,
		timeout:    3 * time.Second,
	}
}

// GetToken retrieves a token, trying keyring first, then database.
func (t *TokenStore) GetToken() (*mcpclient.Token, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try keyring first
	if token, err := t.getFromKeyring(); err == nil {
		return token, nil
	} else if !isSecretNotFoundError(err) {
		log.Debug("Keyring unavailable, trying database", "error", err)
	}

	// Fall back to database
	return t.getFromDatabase()
}

// SaveToken saves a token, trying keyring first, then database.
func (t *TokenStore) SaveToken(token *mcpclient.Token) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Try keyring first
	if err := t.saveToKeyring(token); err == nil {
		return nil
	} else {
		log.Debug("Keyring save failed, trying database", "error", err)
	}

	// Fall back to database
	return t.saveToDatabase(token)
}

// SetClientCredentials stores the OAuth client credentials for this MCP server.
func (t *TokenStore) SetClientCredentials(clientID, clientSecret string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ctx := context.Background()
	return t.store.SetMCPOAuthConfig(ctx, t.identifier, clientID, clientSecret)
}

// GetClientCredentials retrieves the stored OAuth client credentials for this MCP server.
func (t *TokenStore) GetClientCredentials() (clientID, clientSecret string, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ctx := context.Background()

	config, err := t.store.GetMCPOAuthConfig(ctx, t.identifier)
	if err != nil {
		return "", "", err
	}

	return config.ClientID, config.ClientSecret, nil
}

// getFromKeyring retrieves a token from the OS keyring.
func (t *TokenStore) getFromKeyring() (*mcpclient.Token, error) {
	if !t.isKeyringAvailable() {
		return nil, fmt.Errorf("keyring not available")
	}

	tokenData, err := t.keyringGetWithTimeout(t.serviceName(), "token")
	if err != nil {
		return nil, fmt.Errorf("no token in keyring: %w", err)
	}

	var token mcpclient.Token
	if err := json.Unmarshal([]byte(tokenData), &token); err != nil {
		return nil, fmt.Errorf("failed to parse token from keyring: %w", err)
	}

	t.calculateExpiresIn(&token)
	return &token, nil
}

// saveToKeyring saves a token to the OS keyring.
func (t *TokenStore) saveToKeyring(token *mcpclient.Token) error {
	if !t.isKeyringAvailable() {
		return fmt.Errorf("keyring not available")
	}

	t.ensureExpiresAt(token)

	tokenData, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	return t.keyringSetWithTimeout(t.serviceName(), "token", string(tokenData))
}

// getFromDatabase retrieves a token from the database.
func (t *TokenStore) getFromDatabase() (*mcpclient.Token, error) {
	ctx := context.Background()
	oauthToken, err := t.store.GetOAuthTokensByUsername(ctx, "mcp", t.identifier)
	if err != nil {
		return nil, fmt.Errorf("no token in database: %w", err)
	}

	token := &mcpclient.Token{
		AccessToken:  oauthToken.AccessToken,
		TokenType:    oauthToken.TokenType,
		RefreshToken: oauthToken.RefreshToken,
		ExpiresAt:    oauthToken.ExpiresAt,
		Scope:        oauthToken.Scope,
	}

	t.calculateExpiresIn(token)
	return token, nil
}

// saveToDatabase saves a token to the database.
func (t *TokenStore) saveToDatabase(token *mcpclient.Token) error {
	ctx := context.Background()
	t.ensureExpiresAt(token)

	oauthTokens := db.OAuthTokens{
		Provider:     "mcp",
		TokenType:    token.TokenType,
		Scope:        token.Scope,
		AccessToken:  token.AccessToken,
		ExpiresAt:    token.ExpiresAt,
		RefreshToken: token.RefreshToken,
		Username:     t.identifier,
		Error:        false,
	}

	if err := t.store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return fmt.Errorf("failed to save token to database: %w", err)
	}

	return nil
}

// serviceName returns the keyring service name.
func (t *TokenStore) serviceName() string {
	return "mcp:" + t.identifier
}

// calculateExpiresIn sets the ExpiresIn field based on ExpiresAt.
func (t *TokenStore) calculateExpiresIn(token *mcpclient.Token) {
	if !token.ExpiresAt.IsZero() {
		expiresIn := time.Until(token.ExpiresAt).Seconds()
		token.ExpiresIn = int64(expiresIn) // Set even if negative to indicate expiration
	}
}

// ensureExpiresAt sets ExpiresAt if it's zero but ExpiresIn is set.
func (t *TokenStore) ensureExpiresAt(token *mcpclient.Token) {
	if token.ExpiresAt.IsZero() && token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
}

// isKeyringAvailable checks if the keyring is available by trying a test operation.
func (t *TokenStore) isKeyringAvailable() bool {
	testService := "mcp:test-availability"
	testUser := "test"
	testSecret := "test-secret"

	if err := t.keyringSetWithTimeout(testService, testUser, testSecret); err != nil {
		return false
	}

	// Clean up test entry
	if err := t.keyringDeleteWithTimeout(testService, testUser); err != nil {
		log.Debug("keyring cleanup failed during availability check", "error", err)
	}

	return true
}

// keyringSetWithTimeout wraps keyring.Set with a timeout.
func (t *TokenStore) keyringSetWithTimeout(service, user, secret string) error {
	type result struct{ err error }
	ch := make(chan result, 1)

	go func() {
		ch <- result{err: keyring.Set(service, user, secret)}
	}()

	select {
	case res := <-ch:
		return res.err
	case <-time.After(t.timeout):
		return fmt.Errorf("keyring set operation timed out after %v", t.timeout)
	}
}

// keyringGetWithTimeout wraps keyring.Get with a timeout.
func (t *TokenStore) keyringGetWithTimeout(service, user string) (string, error) {
	type result struct {
		secret string
		err    error
	}
	ch := make(chan result, 1)

	go func() {
		secret, err := keyring.Get(service, user)
		ch <- result{secret: secret, err: err}
	}()

	select {
	case res := <-ch:
		return res.secret, res.err
	case <-time.After(t.timeout):
		return "", fmt.Errorf("keyring get operation timed out after %v", t.timeout)
	}
}

// keyringDeleteWithTimeout wraps keyring.Delete with a timeout.
func (t *TokenStore) keyringDeleteWithTimeout(service, user string) error {
	type result struct{ err error }
	ch := make(chan result, 1)

	go func() {
		ch <- result{err: keyring.Delete(service, user)}
	}()

	select {
	case res := <-ch:
		return res.err
	case <-time.After(t.timeout):
		return fmt.Errorf("keyring delete operation timed out after %v", t.timeout)
	}
}

// isSecretNotFoundError checks if the error indicates a secret was not found.
func isSecretNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "secret not found")
}

// ValidateTokenRefreshCapability checks if a token has the necessary components for automatic refresh.
func ValidateTokenRefreshCapability(token *mcpclient.Token, identifier string) {
	if token == nil {
		log.Warn("Token refresh validation: token is nil", "identifier", identifier)
		return
	}

	issues := []string{}

	if token.AccessToken == "" {
		issues = append(issues, "missing access token")
	}

	if token.RefreshToken == "" {
		issues = append(issues, "missing refresh token - automatic refresh will not work")
	}

	if token.ExpiresAt.IsZero() && token.ExpiresIn <= 0 {
		issues = append(issues, "missing expiration information - cannot determine when to refresh")
	}

	if token.TokenType == "" {
		issues = append(issues, "missing token type")
	}

	if len(issues) > 0 {
		log.Warn("Token refresh validation issues found",
			"identifier", identifier,
			"issues", issues,
			"expires_at", token.ExpiresAt,
			"expires_in", token.ExpiresIn,
			"is_expired", token.IsExpired())
	}
}
