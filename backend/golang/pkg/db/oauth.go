package db

import (
	"context"
	"database/sql"
	"time"
)

// OAuthTokens represents oauth tokens for various providers
type OAuthTokens struct {
	Provider     string    `db:"provider"`
	TokenType    string    `db:"token_type"`
	Scope        string    `db:"scope"`
	AccessToken  string    `db:"access_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	RefreshToken *string   `db:"refresh_token"` // Optional, some providers don't return refresh tokens
}

// GetOAuthTokens retrieves tokens for a specific provider
func (s *Store) GetOAuthTokens(ctx context.Context, provider string) (*OAuthTokens, error) {
	var tokens OAuthTokens
	err := s.db.GetContext(ctx, &tokens, `SELECT provider, token_type, scope, access_token, expires_at, refresh_token FROM oauth_tokens WHERE provider = ?`, provider)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Provider not found
		}
		return nil, err
	}
	return &tokens, nil
}

// SetOAuthTokens saves or updates tokens for a provider
func (s *Store) SetOAuthTokens(ctx context.Context, tokens OAuthTokens) error {
	query := `
		INSERT INTO oauth_tokens (provider, token_type, scope, access_token, expires_at, refresh_token)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider) DO UPDATE SET
			token_type = excluded.token_type,
			scope = excluded.scope,
			access_token = excluded.access_token,
			expires_at = excluded.expires_at,
			refresh_token = excluded.refresh_token
	`
	_, err := s.db.ExecContext(ctx, query,
		tokens.Provider,
		tokens.TokenType,
		tokens.Scope,
		tokens.AccessToken,
		tokens.ExpiresAt,
		tokens.RefreshToken,
	)
	return err
}

// DeleteOAuthTokens removes tokens for a specific provider
func (s *Store) DeleteOAuthTokens(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE provider = ?`, provider)
	return err
}
