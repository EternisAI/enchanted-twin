package helpers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// OAuthConfig stores configuration for OAuth providers
type OAuthConfig struct {
	ClientID      string
	ClientSecret  string
	AuthEndpoint  string
	TokenEndpoint string
	UserEndpoint  string
	Scopes        []string
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// Global variables
var (
	authCode    string
	redirectURI = "http://127.0.0.1:8080/callback"
	oauthConfig = map[string]OAuthConfig{
		"twitter": {
			ClientID:      "bEFtUmtyNm1wUFNtRUlqQTdmQmE6MTpjaQ",
			AuthEndpoint:  "https://twitter.com/i/oauth2/authorize",
			TokenEndpoint: "https://api.twitter.com/2/oauth2/token",
			UserEndpoint:  "https://api.twitter.com/2/users/me",
			Scopes:        []string{"tweet.read", "users.read", "offline.access"},
		},
		"google": {
			ClientID:      "993981911648-vtgfk8g1am6kp36pubo5l46902ua1g4t.apps.googleusercontent.com",
			ClientSecret:  "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",
			AuthEndpoint:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenEndpoint: "https://oauth2.googleapis.com/token",
			UserEndpoint:  "https://www.googleapis.com/oauth2/v3/userinfo",
			Scopes:        []string{"openid", "profile", "email"},
		},
		"linkedin": {
			ClientID:      "779sgzrvca0z5a",
			ClientSecret:  "WPL_AP1.vfwo58d3MCsGiFht.izlFiA==",
			AuthEndpoint:  "https://www.linkedin.com/oauth/v2/authorization",
			TokenEndpoint: "https://www.linkedin.com/oauth/v2/accessToken",
			UserEndpoint:  "https://api.linkedin.com/v2/me",
			Scopes:        []string{"r_liteprofile", "r_emailaddress"},
		},
	}
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

// openBrowser opens the provided URL in the default browser
func openBrowser(u string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", u)
	}

	return cmd.Start()
}

// callbackHandler handles the OAuth callback
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.URL.Query().Get("error"); err != "" {
		fmt.Fprintf(w, "Error: %s", err)
		return
	}

	authCode = r.URL.Query().Get("code")
	if authCode == "" {
		fmt.Fprintf(w, "Error: No authorization code received")
		return
	}

	fmt.Fprint(w, "Authentication successful! You can close this window.")
}

// OauthFlow executes the OAuth PKCE flow for the specified provider
func OauthFlow(provider string, logger *slog.Logger, store *db.Store) error {
	// Check if provider is supported
	config, ok := oauthConfig[provider]
	if !ok {
		return fmt.Errorf("provider '%s' is not supported. Supported providers: %s",
			provider, strings.Join(mapKeys(oauthConfig), ", "))
	}

	// Check for required configuration
	if strings.HasPrefix(config.ClientID, "YOUR_") {
		return fmt.Errorf("you need to set a valid client_id for %s", provider)
	}

	// Generate PKCE codes
	codeVerifier, codeChallenge, err := generatePKCEPair()
	if err != nil {
		return fmt.Errorf("failed to generate PKCE pair: %w", err)
	}

	// Generate state
	state, err := generateRandomState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	// Start HTTP server in a goroutine
	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/callback", callbackHandler)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Construct the authorization URL
	authURL, err := url.Parse(config.AuthEndpoint)
	if err != nil {
		return fmt.Errorf("invalid auth endpoint: %w", err)
	}

	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(config.Scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	// Add provider-specific parameters
	if provider == "google" {
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
	}

	authURL.RawQuery = q.Encode()

	// Open the authorization URL in the default browser
	logger.Info("Starting OAuth flow...", "provider", provider)
	logger.Info("Opening browser for authentication", "url", authURL.String())

	if err := openBrowser(authURL.String()); err != nil {
		logger.Info("Failed to open browser", "error", err)
		logger.Info("Please open this URL manually", "url", authURL.String())
	}

	// Wait for the authorization code
	logger.Info("Waiting for authorization...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Check for auth code until timeout
	authCode = ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for authCode == "" {
		select {
		case <-ticker.C:
			// Continue checking
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for authorization")
		}
	}

	logger.Info("Authorization code received", "code", authCode)

	// Exchange the authorization code for tokens
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", authCode)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", config.ClientID)
	data.Set("code_verifier", codeVerifier)

	// Add client_secret for providers that require it
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	// Create a context with timeout for the request
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reqCancel()

	// Create the token request
	logger.Info("Exchanging authorization code for tokens", "endpoint", config.TokenEndpoint)

	timeBeforeTokenRequest := time.Now()

	req, err := http.NewRequestWithContext(
		reqCtx,
		"POST",
		config.TokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send the token request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to obtain access token. Status code: %d, Response: %s",
			resp.StatusCode, string(body))
	}

	// Parse the token response
	var tokens TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	logger.Info("Access token obtained", "access_tokens", tokens.AccessToken[:8])

	if tokens.RefreshToken != "" {
		logger.Info("Refresh token obtained", "refresh_token", tokens.RefreshToken[:8])
	}

	expiresAt := timeBeforeTokenRequest.Add(time.Duration(tokens.ExpiresIn) * time.Second)

	// Save tokens to database
	oauth_tokens := db.OAuthTokens{
		Provider:     provider,
		TokenType:    tokens.TokenType,
		Scope:        strings.Join(config.Scopes, " "),
		AccessToken:  tokens.AccessToken,
		ExpiresAt:    expiresAt,
		RefreshToken: &tokens.RefreshToken,
	}

	err = store.SetOAuthTokens(context.Background(), oauth_tokens)
	if err != nil {
		return fmt.Errorf("failed to store OAuth tokens in database: %w", err)
	}

	logger.Info("Stored OAuth tokens", "tokens", oauth_tokens)

	// Use the access token to access the user's profile
	userReq, err := http.NewRequestWithContext(
		reqCtx,
		"GET",
		config.UserEndpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create user request: %w", err)
	}

	userReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))

	// Add provider-specific headers
	if provider == "linkedin" {
		userReq.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	}

	logger.Info("Fetching user information", "endpoint", config.UserEndpoint)
	userResp, err := client.Do(userReq)
	if err != nil {
		return fmt.Errorf("failed to send user request: %w", err)
	}
	defer userResp.Body.Close()

	// Check response status
	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(userResp.Body)
		return fmt.Errorf("failed to retrieve user information. Status code: %d, Response: %s",
			userResp.StatusCode, string(body))
	}

	// Parse the user response
	var userInfo interface{}
	if err := json.NewDecoder(userResp.Body).Decode(&userInfo); err != nil {
		return fmt.Errorf("failed to parse user response: %w", err)
	}

	// Print user information
	fmt.Println("\nUser Information:")
	userJSON, err := json.MarshalIndent(userInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format user information: %w", err)
	}
	fmt.Println(string(userJSON))

	// Shut down the HTTP server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Info("HTTP server shutdown error", "error", err)
	}

	logger.Info("OAuth flow completed successfully", "provider", provider)
	return nil
}

// mapKeys returns a slice of map keys
func mapKeys(m map[string]OAuthConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
