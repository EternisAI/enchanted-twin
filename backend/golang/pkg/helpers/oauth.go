package helpers

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

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Global variables
var (
	redirectURI = "http://127.0.0.1:8080/callback"
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

func StartOAuthFlow(ctx context.Context, logger *log.Logger, store *db.Store, provider string, scope string) (string, error) {
	// Get config for supported provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return "", err
	}

	// Generate PKCE codes
	codeVerifier, codeChallenge, err := generatePKCEPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE pair: %w", err)
	}

	// Generate state
	state, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	// Construct the authorization URL
	authURL, err := url.Parse(config.AuthEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid auth endpoint: %w", err)
	}

	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scope)
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
		return "", fmt.Errorf("unable to store state and verifier for provider '%s': %w", provider, err)
	}

	logger.Debug("start OAuth flow: stored stated and verifier to database", "provider", provider, "state", state, "scope", scope)

	return authURL.String(), nil
}

func CompleteOAuthFlow(ctx context.Context, logger *log.Logger, store *db.Store, state string, authCode string) (string, error) {
	logger.Debug("starting OAuth completion", "state", state)

	// Retrieve session data using state
	provider, codeVerifier, scope, err := store.GetAndClearOAuthProviderAndVerifier(ctx, state)

	if err != nil {
		return "", fmt.Errorf("failed to get OAuth state: %w", err)
	}

	// Load OAuth config for provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth config: %w", err)
	}

	// Exchange code for tokens
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", authCode)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", config.ClientID)
	data.Set("code_verifier", codeVerifier)

	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	timeBeforeTokenRequest := time.Now()
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		config.TokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to obtain token: %d: %s", resp.StatusCode, body)
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expiration
	expiresAt := timeBeforeTokenRequest.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// Store tokens
	oauthTokens := db.OAuthTokens{
		Provider:     provider,
		TokenType:    tokenResp.TokenType,
		Scope:        scope,
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    expiresAt,
		RefreshToken: tokenResp.RefreshToken,
	}

	if err := store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return "", fmt.Errorf("failed to store tokens: %w", err)
	}

	logger.Debug("completed OAuth flow: stored tokens to database", "provider", provider, "state", state, "expires_at", expiresAt, "scope", scope)

	return provider, nil
}
