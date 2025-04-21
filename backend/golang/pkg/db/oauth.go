package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// OAuthConfig stores configuration for OAuth providers
type OAuthConfig struct {
	ClientID      string `db:"client_id"`
	ClientSecret  string `db:"client_secret"`
	AuthEndpoint  string `db:"auth_endpoint"`
	TokenEndpoint string `db:"token_endpoint"`
	UserEndpoint  string `db:"user_endpoint"`
	Scope         string `db:"scope"`
}

var (
	// TODO: The ClientSecret should not be stored in code.
	oauthConfig = map[string]OAuthConfig{
		"twitter": {
			ClientID:      "bEFtUmtyNm1wUFNtRUlqQTdmQmE6MTpjaQ",
			AuthEndpoint:  "https://twitter.com/i/oauth2/authorize",
			TokenEndpoint: "https://api.twitter.com/2/oauth2/token",
			UserEndpoint:  "https://api.twitter.com/2/users/me",
			Scope:         "tweet.read users.read offline.access",
		},
		"google": {
			ClientID:      "993981911648-vtgfk8g1am6kp36pubo5l46902ua1g4t.apps.googleusercontent.com",
			ClientSecret:  "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",
			AuthEndpoint:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenEndpoint: "https://oauth2.googleapis.com/token",
			UserEndpoint:  "https://www.googleapis.com/oauth2/v3/userinfo",
			Scope:         "openid profile email",
		},
		"linkedin": {
			ClientID:      "779sgzrvca0z5a",
			ClientSecret:  "WPL_AP1.vfwo58d3MCsGiFht.izlFiA==",
			AuthEndpoint:  "https://www.linkedin.com/oauth/v2/authorization",
			TokenEndpoint: "https://www.linkedin.com/oauth/v2/accessToken",
			UserEndpoint:  "https://api.linkedin.com/v2/me",
			Scope:         "r_basicprofile",
		},
	}
)

// OAuthTokens represents oauth tokens for various providers
type OAuthTokens struct {
	Provider     string    `db:"provider"`
	TokenType    string    `db:"token_type"`
	AccessToken  string    `db:"access_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	RefreshToken string    `db:"refresh_token"`
}

// LogValue implements slog.LogValuer interface to safely log tokens
func (o OAuthTokens) LogValue() slog.Value {
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

	return slog.GroupValue(
		slog.String("provider", o.Provider),
		slog.String("token_type", o.TokenType),
		slog.String("access_token", accessTokenValue),
		slog.Time("expires_at", o.ExpiresAt),
		slog.String("refresh_token", refreshTokenValue),
	)
}

// GetOAuthTokens retrieves tokens for a specific provider
func (s *Store) GetOAuthTokens(ctx context.Context, provider string) (*OAuthTokens, error) {
	var tokens OAuthTokens
	err := s.db.GetContext(ctx, &tokens, `
		SELECT 
			provider, 
			token_type, 
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

func (s *Store) SetOAuthStateAndVerifier(ctx context.Context, provider string, state string, codeVerifier string) error {
	// First, try to update an existing record
	query := `
        INSERT OR REPLACE INTO oauth_tokens 
        (provider, state, code_verifier, state_created_at)
        VALUES (?, ?, ?, ?)
    `
	_, err := s.db.ExecContext(ctx, query, provider, state, codeVerifier, time.Now())
	return err
}

func (s *Store) GetAndClearOAuthProviderAndVerifier(ctx context.Context, state string) (string, string, error) {
	// Start a transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() is called

	var dest struct {
		Provider     string    `db:"provider"`
		CodeVerifier string    `db:"code_verifier"`
		CreatedAt    time.Time `db:"state_created_at"`
	}

	// First retrieve the data
	err = tx.GetContext(ctx, &dest, `
        SELECT 
            provider, 
            code_verifier,
            state_created_at
        FROM oauth_tokens 
        WHERE state = ?
    `, state)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("no OAuth session found for state '%s'", state)
		}
		return "", "", fmt.Errorf("failed to get OAuth session for state '%s': %w", state, err)
	}

	// Check if state is expired (10 minutes)
	if time.Since(dest.CreatedAt) > 10*time.Minute {
		return "", "", fmt.Errorf("OAuth state expired")
	}

	// Clear the state and code_verifier
	_, err = tx.ExecContext(ctx, `
        UPDATE oauth_tokens
        SET state = NULL, code_verifier = '', state_created_at = NULL
        WHERE state = ?
    `, state)

	if err != nil {
		return "", "", fmt.Errorf("failed to clear state: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dest.Provider, dest.CodeVerifier, nil
}

// func (s *Store) GetOAuthProviderAndVerifier(ctx context.Context, state string) (string, string, error) {
// 	var dest struct {
// 		Provider     string    `db:"provider"`
// 		CodeVerifier string    `db:"code_verifier"`
// 		CreatedAt    time.Time `db:"state_created_at"`
// 	}
// 	err := s.db.GetContext(ctx, &dest, `
// 		SELECT
// 			provider,
// 			code_verifier,
//             state_created_at
// 		FROM oauth_tokens
// 		WHERE state = ?
// 	`, state)

// 	// Check if state is expired (10 minutes)
// 	if time.Since(dest.CreatedAt) > 10*time.Minute {
// 		return "", "", fmt.Errorf("OAuth state expired")
// 	}

// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return "", "", fmt.Errorf("no OAuth session found for state '%s'", state)
// 		}
// 		return "", "", fmt.Errorf("failed to get OAuth session for state '%s': %w", state, err)
// 	}
// 	return dest.Provider, dest.CodeVerifier, nil
// }

// SetOAuthTokens saves or updates tokens for a provider (clearing state and code_verifier)
func (s *Store) SetOAuthTokens(ctx context.Context, tokens OAuthTokens) error {
	query := `
        INSERT OR REPLACE INTO oauth_tokens (
            provider,
			state, 
			code_verifier,
            token_type, 
            access_token, 
            expires_at, 
            refresh_token
        ) VALUES (?, NULL, '', ?, ?, ?, ?)
    `

	_, err := s.db.ExecContext(ctx, query,
		tokens.Provider,
		tokens.TokenType,
		tokens.AccessToken,
		tokens.ExpiresAt,
		tokens.RefreshToken,
	)
	if err != nil {
		return fmt.Errorf("failed to save OAuth tokens: %w", err)
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
			client_secret TEXT NOT NULL,
			auth_endpoint TEXT NOT NULL,
			token_endpoint TEXT NOT NULL,
			user_endpoint TEXT NOT NULL,
			scope TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create oauth_providers table: %w", err)
	}

	// Prepare for bulk insert
	var placeholders []string
	var values []interface{}

	for provider, config := range oauthConfig {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?)")
		values = append(values,
			provider,
			config.ClientID,
			config.ClientSecret,
			config.AuthEndpoint,
			config.TokenEndpoint,
			config.UserEndpoint,
			config.Scope,
		)
	}

	// Insert or update all providers in a single statement
	query := fmt.Sprintf(`
		INSERT INTO oauth_providers (
			provider, 
			client_id, 
			client_secret,
			auth_endpoint, 
			token_endpoint, 
			user_endpoint, 
			scope
		) VALUES %s
		ON CONFLICT(provider) DO UPDATE SET
			client_id = excluded.client_id,
			client_secret = excluded.client_secret,
			auth_endpoint = excluded.auth_endpoint,
			token_endpoint = excluded.token_endpoint,
			user_endpoint = excluded.user_endpoint,
			scope = excluded.scope
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
			state TEXT,
			code_verifier TEXT NOT NULL DEFAULT '',
			state_created_at DATETIME,
			token_type TEXT NOT NULL DEFAULT '',
			access_token TEXT NOT NULL DEFAULT '',
			expires_at DATETIME,
			refresh_token TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (provider) REFERENCES oauth_providers(provider)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create oauth_tokens table: %w", err)
	}

	return nil
}

// GetOAuthConfig retrieves the OAuth configuration for a provider
func (s *Store) GetOAuthConfig(ctx context.Context, provider string) (*OAuthConfig, error) {
	var config OAuthConfig
	err := s.db.GetContext(ctx, &config, `
		SELECT 
			client_id,
			client_secret,
			auth_endpoint,
			token_endpoint,
			user_endpoint,
			scope
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
			state = '',
			code_verifier = '',
			state_created_at = NULL,
			access_token = '',
			refresh_token = '',
			expires_at = NULL,
			token_type = ''
		WHERE provider = ?
	`, provider)

	if err != nil {
		return fmt.Errorf("failed to clear OAuth tokens for provider '%s': %w", provider, err)
	}

	return nil
}

func (s *Store) IsProviderConnected(ctx context.Context, provider string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `
        SELECT COUNT(*) FROM oauth_tokens
        WHERE provider = ? AND access_token != '' AND (expires_at IS NULL OR expires_at > ?)
    `, provider, time.Now())

	if err != nil {
		return false, fmt.Errorf("failed to check provider connection: %w", err)
	}

	return count > 0, nil
}

// InitOAuth initializes both OAuth providers and tokens tables
func (s *Store) InitOAuth(ctx context.Context) error {
	if err := s.InitOAuthProviders(ctx); err != nil {
		return err
	}

	if err := s.InitOAuthTokens(ctx); err != nil {
		return err
	}

	return nil
}
