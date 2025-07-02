// Owner: johan@eternis.ai
package auth

import (
	"bytes"
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
	"github.com/golang-jwt/jwt/v4"

	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// generatePKCEPair generates PKCE code verifier and challenge.
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

// generateRandomState generates a random state string.
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

// TokenRequest represents the parameters for token requests (both authorization and refresh).
type TokenRequest struct {
	GrantType    string
	Code         string
	RefreshToken string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
}

// TokenResponse encapsulates the response from token endpoints.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
	Username     string
}

func StoreToken(ctx context.Context, logger *log.Logger, store *db.Store, token string, refreshToken string) error {
	provider := "firebase"
	username := ""

	parsedToken, _, err := new(jwt.Parser).ParseUnverified(token, &StandardClaims{})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := parsedToken.Claims.(*StandardClaims); ok {
		if claims.Sub == "" {
			return fmt.Errorf("no subject (sub) found in token claims")
		}
		username = claims.ID
	}

	existingTokens, err := store.GetOAuthTokensByUsername(ctx, provider, username)
	isUpdate := err == nil && existingTokens != nil

	oauthTokens := db.OAuthTokens{
		Provider:     provider,
		TokenType:    "Bearer",
		Scope:        "",
		AccessToken:  token,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
		RefreshToken: refreshToken,
		Username:     username,
	}

	if err := store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	if isUpdate {
		logger.Debug("updated existing firebase tokens", "provider", provider, "expires_at", oauthTokens.ExpiresAt)
	} else {
		logger.Debug("stored new firebase tokens", "provider", provider, "expires_at", oauthTokens.ExpiresAt)
	}

	return nil
}

// ExchangeToken handles the HTTP request to exchange a token (authorization or refresh).
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
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		}

		body, _ := io.ReadAll(resp.Body)

		// Print raw response for debugging
		fmt.Printf("Raw slack token response: %s\n", string(body))

		// Reset the reader for JSON decoding
		resp.Body = io.NopCloser(bytes.NewBuffer(body))

		if err := json.NewDecoder(resp.Body).Decode(&slackTokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse slack token response: %w", err)
		}

		// First try authed_user.access_token
		if slackTokenResp.AuthedUser.AccessToken != "" {
			tokenResp.Username = slackTokenResp.AuthedUser.ID
			tokenResp.AccessToken = slackTokenResp.AuthedUser.AccessToken
			tokenResp.TokenType = slackTokenResp.AuthedUser.TokenType
			if tokenResp.TokenType == "" {
				tokenResp.TokenType = "Bearer"
			}
		} else if slackTokenResp.AccessToken != "" {
			// Fall back to top-level access_token
			tokenResp.AccessToken = slackTokenResp.AccessToken
			tokenResp.TokenType = slackTokenResp.TokenType
			if tokenResp.TokenType == "" {
				tokenResp.TokenType = "Bearer"
			}
		}
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

// CompleteOAuthFlow handles the authorization code exchange flow.
func CompleteOAuthFlow(
	ctx context.Context,
	logger *log.Logger,
	store *db.Store,
	state string,
	authCode string,
) (string, string, error) {
	logger.Debug("starting OAuth completion", "state", state)

	// Retrieve session data using state
	provider, codeVerifier, scope, err := store.GetAndClearOAuthProviderAndVerifier(ctx, logger, state)
	if err != nil {
		return "", "", fmt.Errorf("failed to get OAuth state: %w", err)
	}

	// Load OAuth config for provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return "", "", fmt.Errorf("failed to get OAuth config: %w", err)
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
		return "", "", err
	}

	// Debug output for token response
	logger.Debug("token response received",
		"provider", provider,
		"access_token_length", len(tokenResp.AccessToken),
		"token_type", tokenResp.TokenType,
		"expires_at", tokenResp.ExpiresAt)

	var username string
	if provider == "slack" {
		username = tokenResp.Username
	} else {
		username, err = GetUserInfo(ctx, config.UserEndpoint, provider, tokenResp.AccessToken, tokenResp.TokenType)
		if err != nil {
			logger.Error("failed to get user info",
				"provider", provider,
				"error", err.Error())
			return "", "", err
		}
	}

	logger.Debug("got username from provider", "provider", provider, "username", username)

	oauthTokens := db.OAuthTokens{
		Provider:     provider,
		TokenType:    tokenResp.TokenType,
		Scope:        scope,
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    tokenResp.ExpiresAt,
		RefreshToken: tokenResp.RefreshToken,
		Username:     username,
	}

	if err := store.SetOAuthTokens(ctx, oauthTokens); err != nil {
		return "", "", fmt.Errorf("failed to store tokens: %w", err)
	}

	logger.Debug("completed OAuth flow: stored tokens to database",
		"provider", provider,
		"state", state,
		"expires_at", tokenResp.ExpiresAt,
		"scope", scope)

	return provider, username, nil
}

// GetUserInfo fetches user information from the provider's user endpoint.
func GetUserInfo(ctx context.Context, userEndpoint string, provider string, accessToken string, tokenType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", userEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("%s %s", tokenType, accessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user info: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Printf("failed to close user info response body: %s\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("user info request failed: %d %s", resp.StatusCode, body)
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", fmt.Errorf("failed to decode user info: %w", err)
	}

	var username string
	switch provider {
	case "google":
		email, ok := userInfo["email"].(string)
		if !ok {
			return "", fmt.Errorf("failed to extract email from google user info")
		}
		username = email
	case "twitter":
		data, ok := userInfo["data"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("failed to extract data from twitter user info")
		}
		username, ok = data["username"].(string)
		if !ok {
			return "", fmt.Errorf("failed to extract username from twitter user info")
		}
	case "linkedin":
		data, ok := userInfo["id"].(string)
		if !ok {
			return "", fmt.Errorf("failed to extract id from linkedin user info")
		}
		username = data
	case "slack":
		// Handle different possible structures for Slack response
		fmt.Println("userInfo", userInfo)
		if user, ok := userInfo["authed_user"].(map[string]interface{}); ok && user != nil {
			if slackID, ok := user["id"].(string); ok && slackID != "" {
				username = slackID
			} else {
				return "", fmt.Errorf("no id found in slack user info")
			}
		} else {
			// Print the response for debugging
			responseBytes, _ := json.Marshal(userInfo)
			fmt.Printf("Slack user info response: %s\n", string(responseBytes))
			return "", fmt.Errorf("unable to extract email from slack user info")
		}
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}

	if username == "" {
		return "", fmt.Errorf("no username found in user info")
	}

	return username, nil
}

// RefreshOAuthToken handles the refresh token flow.
func RefreshOAuthToken(
	ctx context.Context,
	logger *log.Logger,
	store *db.Store,
	provider string,
) (bool, error) {
	logger.Debug("refreshing OAuth tokens", "provider", provider)

	// Get existing tokens
	tokens, err := store.GetOAuthTokensArray(ctx, provider)
	if err != nil {
		return false, fmt.Errorf("failed to get existing tokens: %w", err)
	}

	if len(tokens) == 0 {
		return false, fmt.Errorf("no tokens available for provider: %s", provider)
	}

	successCount := 0
	var lastError error

	for _, token := range tokens {
		if token.RefreshToken == "" {
			logger.Warn("skipping token with no refresh token", "provider", provider)
			continue
		}

		// Load OAuth config for provider
		config, err := store.GetOAuthConfig(ctx, provider)
		if err != nil {
			logger.Error("failed to get OAuth config", "provider", provider, "error", err)
			lastError = fmt.Errorf("failed to get OAuth config: %w", err)
			continue
		}

		// Prepare token request
		tokenReq := TokenRequest{
			GrantType:    "refresh_token",
			RefreshToken: token.RefreshToken,
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
		}

		// Exchange refresh token for new access token
		tokenResp, err := ExchangeToken(ctx, logger, provider, *config, tokenReq)
		if err != nil {
			logger.Error("failed to exchange token", "provider", provider, "error", err)
			lastError = err

			token.Error = true
			if err := store.SetOAuthTokens(ctx, token); err != nil {
				// Log the error but continue processing other tokens
				logger.Error("failed to update token error status", "provider", provider, "username", token.Username, "error", err)
			}
			continue
		}

		// Update tokens in storage
		token.AccessToken = tokenResp.AccessToken
		token.ExpiresAt = tokenResp.ExpiresAt

		// Update refresh token if provided in response
		if tokenResp.RefreshToken != "" {
			token.RefreshToken = tokenResp.RefreshToken
		}

		if err := store.SetOAuthTokens(ctx, token); err != nil {
			logger.Error("failed to store refreshed tokens", "provider", provider, "error", err)
			lastError = fmt.Errorf("failed to store refreshed tokens: %w", err)
			continue
		}

		logger.Debug("successfully refreshed OAuth token",
			"provider", provider,
			"username", token.Username,
			"expires_at", tokenResp.ExpiresAt)

		successCount++
	}

	// If we successfully refreshed tokens, publish refresh event for any interested services
	if successCount > 0 {
		logger.Debug("OAuth tokens were refreshed, publishing token refresh event", "provider", provider)
		if err := PublishOAuthTokenRefresh(ctx, logger, store, provider); err != nil {
			logger.Error("Failed to publish OAuth token refresh event", "provider", provider, "error", err)
			// Don't fail the OAuth refresh if event publishing fails
		}
	}

	// Return success if at least one token was refreshed
	if successCount > 0 {
		return true, nil
	}

	// If we got here and no tokens were refreshed, return the last error
	if lastError != nil {
		return false, fmt.Errorf("failed to refresh any tokens for %s: %w", provider, lastError)
	}

	return false, fmt.Errorf("no tokens processed for provider: %s", provider)
}

func Activate(ctx context.Context, logger *log.Logger, store *db.Store, inviteCode string) (bool, error) {
	logger.Debug("activating", "inviteCode", inviteCode)

	oauthTokens, err := store.GetOAuthTokensArray(ctx, "firebase")
	if err != nil {
		return false, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}

	if len(oauthTokens) == 0 {
		return false, fmt.Errorf("no OAuth tokens found")
	}

	type RedeemInviteCodeRequest struct {
		AccessToken string `json:"access_token" binding:"required"`
	}

	redeemRequest := RedeemInviteCodeRequest{
		AccessToken: oauthTokens[0].AccessToken,
	}

	requestBody, err := json.Marshal(redeemRequest)
	if err != nil {
		return false, fmt.Errorf("failed to marshal redeem request: %w", err)
	}

	conf, err := config.LoadConfig(false)
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}

	redeemURL := fmt.Sprintf("%s/api/v1/invites/%s/redeem", conf.ProxyTeeURL, inviteCode)
	req, err := http.NewRequestWithContext(ctx, "POST", redeemURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return false, fmt.Errorf("failed to create redeem request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauthTokens[0].AccessToken))
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send redeem request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close redeem response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("failed to redeem invite code: %d: %s", resp.StatusCode, body)
	}

	logger.Debug("successfully redeemed invite code", "inviteCode", inviteCode)

	return true, nil
}

func IsWhitelisted(ctx context.Context, logger *log.Logger, store *db.Store) (bool, error) {
	oauthTokens, err := store.GetOAuthTokensArray(ctx, "firebase")
	if err != nil {
		return false, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}

	if len(oauthTokens) == 0 {
		logger.Info("no OAuth tokens found to check whitelist", "provider", "firebase")
		return false, nil
	}

	conf, err := config.LoadConfig(false)
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}
	inviteServerURL := conf.ProxyTeeURL

	for _, token := range oauthTokens {
		// Make GET request to check if this email is whitelisted
		whitelistURL := fmt.Sprintf("%s/api/v1/invites/%s/whitelist", inviteServerURL, token.Username)
		req, err := http.NewRequestWithContext(ctx, "GET", whitelistURL, nil)
		if err != nil {
			logger.Error("failed to create whitelist request", "email", token.Username, "error", err)
			continue
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauthTokens[0].AccessToken))

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("failed to send whitelist request", "email", token.Username, "error", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if closeErr := resp.Body.Close(); closeErr != nil {
				logger.Error("failed to close whitelist response body", "error", closeErr)
			}
			logger.Error("whitelist request failed", "email", token.Username, "status", resp.StatusCode)
			continue
		}

		var whitelistResp struct {
			Email       string `json:"email"`
			Whitelisted bool   `json:"whitelisted"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&whitelistResp); err != nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				logger.Error("failed to close whitelist response body", "error", closeErr)
			}
			logger.Error("failed to decode whitelist response", "email", token.Username, "error", err)
			continue
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close whitelist response body", "error", closeErr)
		}

		logger.Debug("whitelist check result", "email", whitelistResp.Email, "whitelisted", whitelistResp.Whitelisted)

		if whitelistResp.Whitelisted {
			return true, nil
		}
	}

	return false, nil
}

// PublishOAuthTokenRefresh publishes a NATS message when OAuth tokens are refreshed.
// This allows any service to subscribe to token refresh events.
func PublishOAuthTokenRefresh(ctx context.Context, logger *log.Logger, store *db.Store, provider string) error {
	logger.Debug("Publishing OAuth token refresh event", "provider", provider)

	// Get the fresh OAuth token that was just refreshed
	tokens, err := store.GetOAuthTokensArray(ctx, provider)
	if err != nil {
		logger.Error("Failed to get refreshed OAuth tokens", "provider", provider, "error", err)
		return fmt.Errorf("failed to get refreshed tokens: %w", err)
	}

	if len(tokens) == 0 {
		logger.Error("No OAuth tokens found after refresh", "provider", provider)
		return fmt.Errorf("no OAuth tokens available for provider: %s", provider)
	}

	// Use the first token (you might want to select a specific one in a multi-user system)
	token := tokens[0]

	// Get NATS client
	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		logger.Error("Failed to create NATS client for OAuth refresh notification", "error", err)
		return fmt.Errorf("failed to create NATS client: %w", err)
	}
	defer nc.Close()

	// Create refresh event payload with only essential token data
	refreshEvent := map[string]interface{}{
		"event":        "oauth_token_refreshed",
		"provider":     provider,
		"timestamp":    time.Now().Format(time.RFC3339),
		"access_token": token.AccessToken,
		"token_type":   token.TokenType,
		"expires_at":   token.ExpiresAt.Format(time.RFC3339),
		"username":     token.Username,
	}

	// Publish to OAuth refresh subject - any service can subscribe to this
	subject := fmt.Sprintf("oauth.%s.token.refreshed", provider)
	if err := helpers.NatsPublish(nc, subject, refreshEvent); err != nil {
		logger.Error("Failed to publish OAuth refresh event to NATS", "provider", provider, "error", err)
		return fmt.Errorf("failed to publish refresh event: %w", err)
	}

	logger.Info("Successfully published OAuth token refresh event to NATS",
		"provider", provider,
		"subject", subject,
		"username", token.Username,
		"expires_at", token.ExpiresAt.Format(time.RFC3339))
	return nil
}

// StandardClaims represents the standard claims in a JWT token.
type StandardClaims struct {
	// Standard JWT claims
	Sub string `json:"sub"`
	jwt.RegisteredClaims
}
