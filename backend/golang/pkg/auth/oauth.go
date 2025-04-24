package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
)

// generatePKCEPair generates PKCE code verifier and challenge
func generatePKCEPair() (string, string, error) {
	// Generate a random byte slice for the verifier
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", "", err
	}

	// Base64 URL encode the verifier
	codeVerifier := base64.RawURLEncoding.EncodeToString(b)

	// Create SHA256 hash of the verifier for the challenge
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return codeVerifier, challenge, nil
}

// generateRandomState generates a random state string
func generateRandomState() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func StartOAuthFlow(ctx context.Context, logger *log.Logger, store *db.Store, provider string, scope string) (string, string, error) {
	// Get config for supported provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return "", "", err
	}

	// Generate PKCE codes
	codeVerifier, codeChallenge, err := generatePKCEPair()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate PKCE pair: %w", err)
	}

	// Generate state
	state, err := generateRandomState()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	// Construct the authorization URL
	authURL, err := url.Parse(config.AuthEndpoint)
	if err != nil {
		return "", "", fmt.Errorf("invalid auth endpoint: %w", err)
	}

	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", config.RedirectURI)
	if provider != "slack" {
		q.Set("scope", scope)
	} else {
		q.Set("user_scope", scope)
	}
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	// Add provider-specific parameters
	if provider == "google" {
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
	}

	authURL.RawQuery = q.Encode()

	err = store.SetOAuthStateAndVerifier(ctx, provider, state, codeVerifier, scope)

	if err != nil {
		return "", "", fmt.Errorf("unable to store state and verifier for provider '%s': %w", provider, err)
	}

	logger.Debug("start OAuth flow: stored state and verifier to database", "provider", provider, "state", state, "scope", scope)

	return authURL.String(), config.RedirectURI, nil
}

func RefreshExpiredTokens(ctx context.Context, logger *log.Logger, store *db.Store) ([]db.OAuthStatus, error) {
	logger.Debug("refreshing expired tokens")
	providers, err := store.GetProvidersForRefresh(ctx)
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		_, err := RefreshOAuthToken(ctx, logger, store, provider.Provider)
		if err != nil {
			logger.Error("failed to refresh OAuth token", "provider", provider.Provider, "error", err)
		}
	}
	return store.GetOAuthStatus(ctx)
}

// TokenRequest represents the parameters for token requests (both authorization and refresh)
type TokenRequest struct {
	GrantType    string
	Code         string
	RefreshToken string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
}

// TokenResponse encapsulates the response from token endpoints
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
}

// ExchangeToken handles the HTTP request to exchange a token (authorization or refresh)
func ExchangeToken(ctx context.Context, logger *log.Logger, provider string, config db.OAuthConfig, tokenReq TokenRequest) (*TokenResponse, error) {
	// Prepare request data
	data := url.Values{}
	data.Set("grant_type", tokenReq.GrantType)
	data.Set("client_id", tokenReq.ClientID)

	// Set appropriate params based on grant type
	switch tokenReq.GrantType {
	case "authorization_code":
		data.Set("code", tokenReq.Code)
		data.Set("redirect_uri", tokenReq.RedirectURI)
		if tokenReq.CodeVerifier != "" {
			data.Set("code_verifier", tokenReq.CodeVerifier)
		}
	case "refresh_token":
		data.Set("refresh_token", tokenReq.RefreshToken)
	}

	// Add client secret if available
	if tokenReq.ClientSecret != "" {
		data.Set("client_secret", tokenReq.ClientSecret)
	}

	// Track time before request for accurate expiry calculation
	timeBeforeTokenRequest := time.Now()

	// Create and execute request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		config.TokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close token response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to obtain token: %d: %s", resp.StatusCode, body)
	}

	// Parse token response based on provider
	var tokenResp TokenResponse
	var expiresIn int

	if provider == "slack" {
		// Special handling for Slack's response format
		var slackTokenResp struct {
			OK         bool `json:"ok"`
			AuthedUser struct {
				ID          string `json:"id"`
				AccessToken string `json:"access_token"`
				TokenType   string `json:"token_type"`
			} `json:"authed_user"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&slackTokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse slack token response: %w", err)
		}
		tokenResp.AccessToken = slackTokenResp.AuthedUser.AccessToken
		tokenResp.TokenType = slackTokenResp.AuthedUser.TokenType
		// No expiry: set to approx 10 years
		expiresIn = 10 * 365 * 24 * 3600
	} else {
		// Standard OAuth token response
		var stdResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token,omitempty"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int    `json:"expires_in,omitempty"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&stdResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}
		tokenResp.AccessToken = stdResp.AccessToken
		tokenResp.RefreshToken = stdResp.RefreshToken
		tokenResp.TokenType = stdResp.TokenType
		expiresIn = stdResp.ExpiresIn
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	if expiresIn < 60 {
		return nil, fmt.Errorf("access token expiry too soon: %ds", expiresIn)
	}

	// Calculate expiration
	tokenResp.ExpiresAt = timeBeforeTokenRequest.Add(time.Duration(expiresIn) * time.Second)

	return &tokenResp, nil
}

// CompleteOAuthFlow handles the authorization code exchange flow
func CompleteOAuthFlow(ctx context.Context, logger *log.Logger, store *db.Store, state string, authCode string) (string, error) {
	logger.Debug("starting OAuth completion", "state", state)

	// Retrieve session data using state
	provider, codeVerifier, scope, err := store.GetAndClearOAuthProviderAndVerifier(ctx, logger, state)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth state: %w", err)
	}

	// Load OAuth config for provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth config: %w", err)
	}

	// Prepare token request
	tokenReq := TokenRequest{
		GrantType:    "authorization_code",
		Code:         authCode,
		RedirectURI:  config.RedirectURI,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		CodeVerifier: codeVerifier,
	}

	// Exchange authorization code for tokens
	tokenResp, err := ExchangeToken(ctx, logger, provider, *config, tokenReq)
	if err != nil {
		return "", err
	}

	// Store tokens
	oauthTokens := db.OAuthTokens{
		Provider:     provider,
		TokenType:    tokenResp.TokenType,
		Scope:        scope,
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    tokenResp.ExpiresAt,
		RefreshToken: tokenResp.RefreshToken,
	}

	if err := store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return "", fmt.Errorf("failed to store tokens: %w", err)
	}

	logger.Debug("completed OAuth flow: stored tokens to database",
		"provider", provider,
		"state", state,
		"expires_at", tokenResp.ExpiresAt,
		"scope", scope)

	return provider, nil
}

// RefreshOAuthToken handles the refresh token flow
func RefreshOAuthToken(ctx context.Context, logger *log.Logger, store *db.Store, provider string) (TokenRequest, error) {
	logger.Debug("refreshing OAuth token", "provider", provider)

	// Get existing tokens
	tokens, err := store.GetOAuthTokens(ctx, provider)
	if err != nil {
		return TokenRequest{}, fmt.Errorf("failed to get existing tokens: %w", err)
	}

	if tokens.RefreshToken == "" {
		return TokenRequest{}, fmt.Errorf("no refresh token available for provider: %s", provider)
	}

	// Load OAuth config for provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return TokenRequest{}, fmt.Errorf("failed to get OAuth config: %w", err)
	}

	// Prepare token request
	tokenReq := TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: tokens.RefreshToken,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
	}

	// Exchange refresh token for new access token
	tokenResp, err := ExchangeToken(ctx, logger, provider, *config, tokenReq)
	if err != nil {
		return TokenRequest{}, err
	}

	// Update tokens in storage
	tokens.AccessToken = tokenResp.AccessToken
	tokens.ExpiresAt = tokenResp.ExpiresAt

	// Update refresh token if provided in response
	if tokenResp.RefreshToken != "" {
		tokens.RefreshToken = tokenResp.RefreshToken
	}

	if err := store.SetOAuthTokens(ctx, *tokens); err != nil {
		return TokenRequest{}, fmt.Errorf("failed to store refreshed tokens: %w", err)
	}

	logger.Debug("successfully refreshed OAuth token",
		"provider", provider,
		"expires_at", tokenResp.ExpiresAt)

	return tokenReq, nil
}
