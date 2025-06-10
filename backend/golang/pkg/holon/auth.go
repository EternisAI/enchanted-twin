package holon

import (
	"context"
	"fmt"

	clog "github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// NewAuthenticatedAPIClient creates a new HolonZero API client with Google OAuth authentication.
func NewAuthenticatedAPIClient(ctx context.Context, baseURL string, store *db.Store, logger *clog.Logger, options ...ClientOption) (*APIClient, error) {
	// Get Google OAuth token from the database
	token, err := getGoogleOAuthToken(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("failed to get Google OAuth token: %w", err)
	}

	// Create client options with authentication
	clientOptions := []ClientOption{
		WithAuthToken(token),
	}

	if logger != nil {
		clientOptions = append(clientOptions, WithLogger(logger))
	}

	// Add any additional options passed in
	clientOptions = append(clientOptions, options...)

	// Create and return authenticated client
	return NewAPIClient(baseURL, clientOptions...), nil
}

// getGoogleOAuthToken retrieves a valid Google OAuth access token from the database.
func getGoogleOAuthToken(ctx context.Context, store *db.Store) (string, error) {
	// Get Google OAuth tokens from the database
	tokens, err := store.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth tokens: %w", err)
	}

	if len(tokens) == 0 {
		return "", fmt.Errorf("no Google OAuth tokens found - please authenticate with Google first")
	}

	// Use the first available token (in a production system, you might want to:
	// 1. Check token expiry and refresh if needed
	// 2. Allow selection of specific user's token
	// 3. Implement token rotation)
	token := tokens[0]

	if token.AccessToken == "" {
		return "", fmt.Errorf("Google OAuth token is empty")
	}

	if token.Error {
		return "", fmt.Errorf("Google OAuth token has error status - please re-authenticate")
	}

	return token.AccessToken, nil
}

// AuthenticateWithHolonZero authenticates with HolonZero API and returns participant info.
func AuthenticateWithHolonZero(ctx context.Context, baseURL string, store *db.Store, logger *clog.Logger) (*ParticipantAuthResponse, error) {
	// Create authenticated client
	client, err := NewAuthenticatedAPIClient(ctx, baseURL, store, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Authenticate with HolonZero API
	authResp, err := client.AuthenticateParticipant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with HolonZero: %w", err)
	}

	return authResp, nil
}
