package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// InitOAuth initializes both OAuth providers and tokens tables
func (s *Store) InitOAuth(ctx context.Context) error {
	if err := s.InitOAuthProviders(ctx); err != nil {
		return err
	}

	if err := s.InitOAuthTokens(ctx); err != nil {
		return err
	}

	if err := s.InitOAuthSessions(ctx); err != nil {
		return err
	}

	return nil
}

// InitOAuthProviders initializes the OAuth providers in the database
func (s *Store) InitOAuthProviders(ctx context.Context) error {
	// Create the table if it doesn't exist
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS oauth_providers (
			provider TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			client_secret TEXT NOT NULL,
			auth_endpoint TEXT NOT NULL,
			token_endpoint TEXT NOT NULL,
			user_endpoint TEXT NOT NULL,
			default_scope TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create oauth_providers table: %w", err)
	}

	// Prepare for bulk insert
	var placeholders []string
	var values []interface{}

	for provider, config := range oauthConfig {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?)")
		values = append(values,
			provider,
			config.ClientID,
			config.RedirectURI,
			config.ClientSecret,
			config.AuthEndpoint,
			config.TokenEndpoint,
			config.UserEndpoint,
			config.DefaultScope,
		)
	}

	// Insert or update all providers in a single statement
	query := fmt.Sprintf(`
		INSERT INTO oauth_providers (
			provider, 
			client_id,
			redirect_uri, 
			client_secret,
			auth_endpoint, 
			token_endpoint, 
			user_endpoint, 
			default_scope
		) VALUES %s
		ON CONFLICT(provider) DO UPDATE SET
			client_id = excluded.client_id,
			redirect_uri = excluded.redirect_uri,
			client_secret = excluded.client_secret,
			auth_endpoint = excluded.auth_endpoint,
			token_endpoint = excluded.token_endpoint,
			user_endpoint = excluded.user_endpoint,
			default_scope = excluded.default_scope
	`, strings.Join(placeholders, ","))

	_, err = s.db.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to initialize OAuth providers: %w", err)
	}

	return nil
}

// InitOAuthTokens initializes the OAuth tokens table
func (s *Store) InitOAuthTokens(ctx context.Context) error {
	// Create the tokens table
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS oauth_tokens (
			provider TEXT PRIMARY KEY,
			token_type TEXT NOT NULL,
			scope TEXT NOT NULL ,
			access_token TEXT NOT NULL,
			expires_at DATETIME,
			refresh_token TEXT NOT NULL,
			FOREIGN KEY (provider) REFERENCES oauth_providers(provider)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create oauth_tokens table: %w", err)
	}

	return nil
}

// InitOAuthTokens initializes the OAuth tokens table
func (s *Store) InitOAuthSessions(ctx context.Context) error {
	// Create the tokens table
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS oauth_sessions (
			state TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			code_verifier TEXT NOT NULL,
			state_created_at DATETIME,
			scope TEXT NOT NULL,
			FOREIGN KEY (provider) REFERENCES oauth_providers(provider)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create oauth_sessions table: %w", err)
	}

	return nil
}

// OAuthConfig stores configuration for OAuth providers
type OAuthConfig struct {
	ClientID      string `db:"client_id"`
	RedirectURI   string `db:"redirect_uri"`
	ClientSecret  string `db:"client_secret"`
	AuthEndpoint  string `db:"auth_endpoint"`
	TokenEndpoint string `db:"token_endpoint"`
	UserEndpoint  string `db:"user_endpoint"`
	DefaultScope  string `db:"default_scope"`
}

var (
	// TODO: The ClientSecret should not be stored in code.
	oauthConfig = map[string]OAuthConfig{
		"twitter": {
			ClientID:      "bEFtUmtyNm1wUFNtRUlqQTdmQmE6MTpjaQ",
			RedirectURI:   "http://127.0.0.1:8080/callback",
			AuthEndpoint:  "https://twitter.com/i/oauth2/authorize",
			TokenEndpoint: "https://api.twitter.com/2/oauth2/token",
			UserEndpoint:  "https://api.twitter.com/2/users/me",
			DefaultScope:  "tweet.read users.read offline.access",
		},
		"google": {
			ClientID:      "993981911648-vtgfk8g1am6kp36pubo5l46902ua1g4t.apps.googleusercontent.com",
			RedirectURI:   "http://127.0.0.1:8080/callback",
			ClientSecret:  "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",
			AuthEndpoint:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenEndpoint: "https://oauth2.googleapis.com/token",
			UserEndpoint:  "https://www.googleapis.com/oauth2/v3/userinfo",
			DefaultScope:  "openid profile email",
		},
		"linkedin": {
			ClientID:      "779sgzrvca0z5a",
			RedirectURI:   "http://127.0.0.1:8080/callback",
			ClientSecret:  "WPL_AP1.vfwo58d3MCsGiFht.izlFiA==",
			AuthEndpoint:  "https://www.linkedin.com/oauth/v2/authorization",
			TokenEndpoint: "https://www.linkedin.com/oauth/v2/accessToken",
			UserEndpoint:  "https://api.linkedin.com/v2/me",
			DefaultScope:  "r_basicprofile",
		},
		"slack": {
			ClientID:      "6687557443010.8799848778913",
			RedirectURI:   "https://127.0.0.1:8443/callback",
			ClientSecret:  "aefeb979cb95332bd556f27b7e52b5cb",
			AuthEndpoint:  "https://slack.com/oauth/v2/authorize",
			TokenEndpoint: "https://slack.com/api/oauth.v2.access",
			UserEndpoint:  "https://slack.com/api/users.identity",
			DefaultScope:  "identity.basic identity.email identity.avatar identity.team",
		},
	}
)

// OAuthTokens represents oauth tokens for various providers
type OAuthTokens struct {
	Provider     string    `db:"provider"`
	TokenType    string    `db:"token_type"`
	Scope        string    `db:"scope"`
	AccessToken  string    `db:"access_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	RefreshToken string    `db:"refresh_token"`
}

// For logging with Charmbracelet log
func (o OAuthTokens) String() string {
	// Safe display of token prefixes only
	accessTokenValue := "<empty>"
	if o.AccessToken != "" {
		if len(o.AccessToken) > 12 {
			accessTokenValue = o.AccessToken[:8] + "..."
		} else {
			accessTokenValue = "<short-token>"
		}
	}

	refreshTokenValue := "<empty>"
	if o.RefreshToken != "" {
		if len(o.RefreshToken) > 12 {
			refreshTokenValue = (o.RefreshToken)[:8] + "..."
		} else {
			refreshTokenValue = "<short-token>"
		}
	}

	return fmt.Sprintf("OAuthTokens{provider=%s, token_type=%s, access_token=%s, expires_at=%s, refresh_token=%s}",
		o.Provider,
		o.TokenType,
		accessTokenValue,
		o.ExpiresAt.Format(time.RFC3339),
		refreshTokenValue)
}

// GetOAuthTokens retrieves tokens for a specific provider
func (s *Store) GetOAuthTokens(ctx context.Context, provider string) (*OAuthTokens, error) {
	var tokens OAuthTokens
	err := s.db.GetContext(ctx, &tokens, `
		SELECT 
			provider, 
			token_type,
			scope,
			access_token,
			expires_at,
			refresh_token
		FROM oauth_tokens
		WHERE provider = ?
	`, provider)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Provider not found
		}
		return nil, fmt.Errorf("failed to get OAuth tokens for provider '%s': %w", provider, err)
	}
	return &tokens, nil
}

func (s *Store) SetOAuthStateAndVerifier(ctx context.Context, provider string, state string, codeVerifier string, scope string) error {
	query := `
        INSERT OR REPLACE INTO oauth_sessions 
        (state, provider, code_verifier, state_created_at, scope)
        VALUES (?, ?, ?, ?, ?)
    `
	_, err := s.db.ExecContext(ctx, query, state, provider, codeVerifier, time.Now(), scope)
	return err
}

func (s *Store) GetAndClearOAuthProviderAndVerifier(ctx context.Context, logger *log.Logger, state string) (string, string, string, error) {
	// Start a transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Complicated defer to satisfy linter which requires all errors be handled.
	defer func() {
		err := tx.Rollback()
		// ErrTxDone happens if tx.Commit() is called
		if err != nil && err != sql.ErrTxDone {
			// Only log the error since we can't really handle it in a defer
			// The original error from the function is more important
			logger.Printf("Error rolling back transaction: %v", err)
		}
	}()

	var dest struct {
		Provider     string    `db:"provider"`
		CodeVerifier string    `db:"code_verifier"`
		CreatedAt    time.Time `db:"state_created_at"`
		Scope        string    `db:"scope"`
	}

	// First retrieve the session data
	err = tx.GetContext(ctx, &dest, `
        SELECT 
            provider, 
            code_verifier,
            state_created_at,
            scope
        FROM oauth_sessions 
        WHERE state = ?
    `, state)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", fmt.Errorf("no OAuth session found for state '%s'", state)
		}
		return "", "", "", fmt.Errorf("failed to get OAuth session for state '%s': %w", state, err)
	}

	now := time.Now()
	sessionExpiryDuration := 10 * time.Minute

	// Check if state is expired (10 minutes)
	if now.Sub(dest.CreatedAt) > sessionExpiryDuration {
		return "", "", "", fmt.Errorf("OAuth state expired")
	}

	// Delete the record instead of just clearing fields
	_, err = tx.ExecContext(ctx, `
        DELETE FROM oauth_sessions
        WHERE state = ?
    `, state)

	if err != nil {
		return "", "", "", fmt.Errorf("failed to delete session: %w", err)
	}

	// Cleanup expired sessions while we're at it
	expiryTime := now.Add(-sessionExpiryDuration)
	deleteResult, err := tx.ExecContext(ctx, `
        DELETE FROM oauth_sessions
        WHERE state_created_at < ?
    `, expiryTime)

	if err != nil {
		logger.Warnf("Failed to cleanup expired sessions: %v", err)
		// Continue with the function, this error shouldn't cause the main operation to fail
	} else {
		// Log how many expired sessions were cleaned up
		rowsAffected, err := deleteResult.RowsAffected()
		if err != nil && rowsAffected > 0 {
			logger.Debugf("Cleaned up %d expired OAuth sessions", rowsAffected)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return "", "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dest.Provider, dest.CodeVerifier, dest.Scope, nil
}

// SetOAuthTokens saves or updates tokens for a provider
func (s *Store) SetOAuthTokens(ctx context.Context, tokens OAuthTokens) error {
	query := `
        INSERT OR REPLACE INTO oauth_tokens (
            provider,
            token_type, 
			scope,
            access_token, 
            expires_at, 
            refresh_token
        ) VALUES (?, ?, ?, ?, ?, ?)
    `

	_, err := s.db.ExecContext(ctx, query,
		tokens.Provider,
		tokens.TokenType,
		tokens.Scope,
		tokens.AccessToken,
		tokens.ExpiresAt,
		tokens.RefreshToken,
	)
	if err != nil {
		return fmt.Errorf("failed to save OAuth tokens: %w", err)
	}

	return nil
}

// GetOAuthConfig retrieves the OAuth configuration for a provider
func (s *Store) GetOAuthConfig(ctx context.Context, provider string) (*OAuthConfig, error) {
	var config OAuthConfig
	err := s.db.GetContext(ctx, &config, `
		SELECT 
			client_id,
			redirect_uri,
			client_secret,
			auth_endpoint,
			token_endpoint,
			user_endpoint,
			default_scope
		FROM oauth_providers 
		WHERE provider = ?
	`, provider)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("provider '%s' not found", provider)
		}
		return nil, fmt.Errorf("failed to get OAuth configuration for provider '%s': %w", provider, err)
	}

	return &config, nil
}

// GetOAuthProviders returns a list of all available OAuth providers
func (s *Store) GetOAuthProviders(ctx context.Context) ([]string, error) {
	var providers []string
	err := s.db.SelectContext(ctx, &providers, `
		SELECT provider FROM oauth_providers ORDER BY provider
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth providers: %w", err)
	}

	return providers, nil
}

// DeleteOAuthTokens removes tokens for a specific provider
func (s *Store) ClearOAuthTokens(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE oauth_tokens 
		SET 
			token_type = '',
			scope = '',
			access_token = '',
			expires_at = NULL,
			refresh_token = ''
		WHERE provider = ?
	`, provider)

	if err != nil {
		return fmt.Errorf("failed to clear OAuth tokens for provider '%s': %w", provider, err)
	}

	return nil
}

type OAuthStatus struct {
	Provider  string    `db:"provider"`
	ExpiresAt time.Time `db:"expires_at"`
	Scope     string    `db:"scope"`
}

// Returns a list of all providers that have a non-expired access token.
func (s *Store) GetOAuthStatus(ctx context.Context) ([]OAuthStatus, error) {
	var dest []OAuthStatus
	err := s.db.SelectContext(ctx, &dest, `
        SELECT provider, expires_at, scope FROM oauth_tokens
        WHERE access_token != '' AND expires_at > ?
    `, time.Now())

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve providers: %w", err)
	}

	return dest, nil
}
