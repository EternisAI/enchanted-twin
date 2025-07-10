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
		logger.Info("claims", "claims", claims)
		username = claims.Email
		logger.Info("got username from token", "username", username)
	}

	existingTokens, err := store.GetOAuthTokens(ctx, provider)
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
		logger.Debug("updated existing firebase tokens", "provider", provider, "expires_at", oauthTokens.ExpiresAt, "username", username)
	} else {
		logger.Debug("stored new firebase tokens", "provider", provider, "expires_at", oauthTokens.ExpiresAt, "username", username)
	}

	return nil
}

// ExchangeToken handles the HTTP request to exchange an authorization code for tokens.
func ExchangeToken(ctx context.Context, logger *log.Logger, store *db.Store, provider string, oauthConfig db.OAuthConfig, tokenReq TokenRequest) (*TokenResponse, error) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	firebaseToken, err := store.GetOAuthTokens(ctx, "firebase")
	if err != nil {
		return nil, fmt.Errorf("failed to get firebase token: %w", err)
	}

	// Prepare request data for the new API
	requestData := TokenExchangeRequest{
		GrantType:    tokenReq.GrantType,
		Platform:     provider,
		Code:         tokenReq.Code,
		CodeVerifier: tokenReq.CodeVerifier,
		RefreshToken: tokenReq.RefreshToken,
		RedirectURI:  tokenReq.RedirectURI,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create and execute request to /auth/exchange
	exchangeURL := fmt.Sprintf("%s/auth/exchange", conf.ProxyTeeURL)
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		exchangeURL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", firebaseToken.AccessToken))

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
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshTokens handles the refresh token flow using the new API endpoint.
func RefreshTokenCall(ctx context.Context, logger *log.Logger, store *db.Store, provider string, refreshToken string) (*TokenResponse, error) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	firebaseToken, err := store.GetOAuthTokens(ctx, "firebase")
	if err != nil {
		return nil, fmt.Errorf("failed to get firebase token: %w", err)
	}

	// Prepare request data for the new API
	requestData := RefreshTokenRequest{
		RefreshToken: refreshToken,
		Platform:     provider,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create and execute request to /auth/refresh
	refreshURL := fmt.Sprintf("%s/auth/refresh", conf.ProxyTeeURL)
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		refreshURL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", firebaseToken.AccessToken))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close refresh response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to refresh token: %d: %s", resp.StatusCode, body)
	}

	// Parse token response (same format as exchange response)
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

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
	tokenResp, err := ExchangeToken(ctx, logger, store, provider, *config, tokenReq)
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
			successCount++
			continue
		}

		// Use the new RefreshTokens function
		tokenResp, err := RefreshTokenCall(ctx, logger, store, provider, token.RefreshToken)
		if err != nil {
			logger.Error("failed to refresh token", "provider", provider, "error", err)
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
		logger.Debug("successfully refreshed OAuth token",
			"provider", provider,
			"username", token.Username,
			"expires_at", tokenResp.ExpiresAt)

		if err := store.SetOAuthTokens(ctx, token); err != nil {
			logger.Error("failed to store refreshed tokens", "provider", provider, "error", err)
			lastError = fmt.Errorf("failed to store refreshed tokens: %w", err)
			continue
		}

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
	oauthToken, err := store.GetOAuthTokens(ctx, "firebase")
	if err != nil {
		logger.Error("failed to get OAuth tokens in isWhitelisted check", "error", err)
		return false, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}

	if oauthToken == nil {
		logger.Error("no OAuth tokens found in isWhitelisted check", "provider", "firebase")
		return false, nil
	}

	conf, err := config.LoadConfig(false)
	if err != nil {
		return false, fmt.Errorf("failed to load config: %w", err)
	}
	inviteServerURL := conf.ProxyTeeURL

	// Make GET request to check if this email is whitelisted
	whitelistURL := fmt.Sprintf("%s/api/v1/invites/%s/whitelist", inviteServerURL, oauthToken.Username)
	req, err := http.NewRequestWithContext(ctx, "GET", whitelistURL, nil)
	if err != nil {
		logger.Error("failed to create whitelist request", "email", oauthToken.Username, "error", err)
		return false, fmt.Errorf("failed to create whitelist request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauthToken.AccessToken))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to send whitelist request", "email", oauthToken.Username, "error", err)
	}

	if resp.StatusCode != http.StatusOK {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close whitelist response body", "error", closeErr)
		}
		logger.Error("whitelist request failed", "email", oauthToken.Username, "status", resp.StatusCode)
		return false, fmt.Errorf("whitelist request failed: %d", resp.StatusCode)
	}

	var whitelistResp struct {
		UserID      string `json:"userID"`
		Whitelisted bool   `json:"whitelisted"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&whitelistResp); err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close whitelist response body", "error", closeErr)
		}
		logger.Error("failed to decode whitelist response", "email", oauthToken.Username, "error", err)
		return false, fmt.Errorf("failed to decode whitelist response: %w", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		logger.Error("failed to close whitelist response body", "error", closeErr)
	}

	logger.Debug("whitelist check result", "user", whitelistResp.UserID, "whitelisted", whitelistResp.Whitelisted)

	if whitelistResp.Whitelisted {
		return true, nil
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
	Sub    string `json:"sub"`
	UserId string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}
