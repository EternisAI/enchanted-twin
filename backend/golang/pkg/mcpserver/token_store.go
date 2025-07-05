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

	log.Debug("retrieved MCP OAuth token from database",
		"provider", s.provider,
		"username", s.username,
		"expires_at", mcpToken.ExpiresAt,
		"token_type", mcpToken.TokenType)

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

// KeyringTokenStore implements the TokenStore interface using OS keyring.
type KeyringTokenStore struct {
	identifier string
	timeout    time.Duration
	mu         sync.RWMutex
}

// NewKeyringTokenStore creates a new KeyringTokenStore.
func NewKeyringTokenStore(identifier string) *KeyringTokenStore {
	return &KeyringTokenStore{
		identifier: identifier,
		timeout:    3 * time.Second,
	}
}

// serviceName prefixes the identifier with "mcp:" for consistency.
func (k *KeyringTokenStore) serviceName() string {
	return "mcp:" + k.identifier
}

// GetToken retrieves a token from the OS keyring.
func (k *KeyringTokenStore) GetToken() (*mcpclient.Token, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Check availability first to fallback
	if !k.IsAvailable() {
		return nil, fmt.Errorf("keyring not available on this system")
	}

	tokenData, err := k.getWithTimeout(k.serviceName(), "token")
	if err != nil {
		return nil, fmt.Errorf("no token available in keyring for MCP server %s: %w", k.identifier, err)
	}

	var token mcpclient.Token
	if err := json.Unmarshal([]byte(tokenData), &token); err != nil {
		return nil, fmt.Errorf("failed to parse token from keyring: %w", err)
	}

	if !token.ExpiresAt.IsZero() {
		expiresIn := time.Until(token.ExpiresAt).Seconds()
		if expiresIn > 0 {
			token.ExpiresIn = int64(expiresIn)
		}
	}

	log.Debug("retrieved MCP OAuth token from keyring",
		"identifier", k.identifier,
		"expires_at", token.ExpiresAt,
		"token_type", token.TokenType)

	return &token, nil
}

// SaveToken saves a token to the OS keyring.
func (k *KeyringTokenStore) SaveToken(token *mcpclient.Token) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.IsAvailable() {
		return fmt.Errorf("keyring not available on this system")
	}

	if token.ExpiresAt.IsZero() && token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	tokenData, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token for keyring: %w", err)
	}

	if err := k.setWithTimeout(k.serviceName(), "token", string(tokenData)); err != nil {
		return fmt.Errorf("failed to save token to keyring: %w", err)
	}

	log.Debug("saved MCP OAuth token to keyring",
		"identifier", k.identifier,
		"expires_at", token.ExpiresAt,
		"token_type", token.TokenType)

	return nil
}

// IsAvailable checks if the keyring is available on the current system.
// Would likely return true for all systems. If false, fall back to the database.
// Checks by trying to perform a simple set and delete operation.
func (k *KeyringTokenStore) IsAvailable() bool {
	testService := "mcp:test-availability"
	testUser := "test"
	testSecret := "test-secret"

	if err := k.setWithTimeout(testService, testUser, testSecret); err != nil {
		return false
	}

	k.deleteWithTimeout(testService, testUser)
	return true
}

// setWithTimeout wraps the keyring.Set operation with a timeout.
func (k *KeyringTokenStore) setWithTimeout(service, user, secret string) error {
	type result struct {
		err error
	}

	ch := make(chan result, 1)
	go func() {
		err := keyring.Set(service, user, secret)
		ch <- result{err: err}
	}()

	select {
	case res := <-ch:
		return res.err
	case <-time.After(k.timeout):
		return fmt.Errorf("keyring set operation timed out after %v", k.timeout)
	}
}

// getWithTimeout wraps the keyring.Get operation with a timeout.
func (k *KeyringTokenStore) getWithTimeout(service, user string) (string, error) {
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
	case <-time.After(k.timeout):
		return "", fmt.Errorf("keyring get operation timed out after %v", k.timeout)
	}
}

// deleteWithTimeout performs a keyring Delete operation with timeout.
func (k *KeyringTokenStore) deleteWithTimeout(service, user string) error {
	type result struct {
		err error
	}

	ch := make(chan result, 1)
	go func() {
		err := keyring.Delete(service, user)
		ch <- result{err: err}
	}()

	select {
	case res := <-ch:
		return res.err
	case <-time.After(k.timeout):
		return fmt.Errorf("keyring delete operation timed out after %v", k.timeout)
	}
}

// isSecretNotFoundError checks if the error indicates
// a secret was not found in the keyring.
func isSecretNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "secret not found")
}

// FallbackTokenStore implements the TokenStore interface with a primary and fallback store.
type FallbackTokenStore struct {
	primary  mcpclient.TokenStore
	fallback mcpclient.TokenStore
}

// NewFallbackTokenStore creates a new FallbackTokenStore with the given primary and fallback stores.
// This is useful to arbitrarily combine any two token stores, like a keyring and a memory store.
func NewFallbackTokenStore(primary, fallback mcpclient.TokenStore) mcpclient.TokenStore {
	return &FallbackTokenStore{
		primary:  primary,
		fallback: fallback,
	}
}

// NewKeyringDatabaseTokenStore creates a TokenStore that tries keyring first, then falls back to database.
func NewKeyringDatabaseTokenStore(store *db.Store, identifier string) mcpclient.TokenStore {
	keyringStore := NewKeyringTokenStore(identifier)
	databaseStore := NewDatabaseTokenStore(store, identifier)
	return NewFallbackTokenStore(keyringStore, databaseStore)
}

// GetToken retrieves a token from the primary store first, then the fallback store if primary fails.
func (f *FallbackTokenStore) GetToken() (*mcpclient.Token, error) {
	// Try primary store first
	token, err := f.primary.GetToken()
	if err == nil {
		return token, nil
	}

	// Log the primary failure if it's not just a "not found" error.
	// i.e., we can know that the primary store is not available.
	if !isSecretNotFoundError(err) {
		log.Debug("primary token store failed, trying fallback", "error", err)
	}

	return f.fallback.GetToken()
}

// SaveToken saves a token to the primary store first, then the fallback store if primary fails.
func (f *FallbackTokenStore) SaveToken(token *mcpclient.Token) error {
	// Try primary store first
	if err := f.primary.SaveToken(token); err == nil {
		return nil
	} else {
		// If primary fails, log and try fallback
		log.Debug("primary token store save failed, trying fallback", "error", err)
	}

	// Try fallback store
	return f.fallback.SaveToken(token)
}
