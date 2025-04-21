package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Global variables
var (
	server        *http.Server
	serverMutex   sync.Mutex
	flowWaitGroup sync.WaitGroup
)

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
func callbackHandler(ctx context.Context, logger *slog.Logger, store *db.Store, r *http.Request) (string, interface{}, error) {
	if err := r.URL.Query().Get("error"); err != "" {
		return "", nil, fmt.Errorf("error: %s", err)
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		return "", nil, fmt.Errorf("no state received")
	}

	authCode := r.URL.Query().Get("code")
	if authCode == "" {
		return "", nil, fmt.Errorf("no authorization code received")
	}
	provider, err := CompleteOAuthFlow(ctx, logger, store, state, authCode)

	if err != nil {
		return "", nil, fmt.Errorf("oauth flow completion failed: %w", err)
	}

	userInfo, err := fetchUserInfo(ctx, logger, store, provider)
	if err != nil {
		return "", nil, err
	}

	return provider, userInfo, nil
}

// fetchUserInfo fetches user information using an access token
func fetchUserInfo(ctx context.Context, _ *slog.Logger, store *db.Store, provider string) (interface{}, error) {
	// Load OAuth config for provider
	config, err := store.GetOAuthConfig(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth config: %w", err)
	}

	tokens, err := store.GetOAuthTokens(context.Background(), provider)
	if err != nil {
		return nil, fmt.Errorf("unable to get OAuth tokens: %w", err)
	}

	// Create the user info request
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		config.UserEndpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))

	// Add provider-specific headers
	if provider == "linkedin" {
		req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	}

	// Send the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send user request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to retrieve user information. Status code: %d, Response: %s",
			resp.StatusCode, string(body))
	}

	// Parse the user response
	var userInfo interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user response: %w", err)
	}

	return userInfo, nil
}

// OAuthFlow executes the OAuth PKCE flow for the specified provider
//
// This is used to test the flow in pure Go.
func OAuthFlow(provider string, logger *slog.Logger, store *db.Store) error {
	logger.Info("Starting OAuth flow...", "provider", provider)

	flowWaitGroup.Add(1)

	url, err := StartOAuthFlow(context.Background(), logger, store, provider)
	if err != nil {
		return fmt.Errorf("failed to start OAuth flow: %w", err)
	}

	// Open the authorization URL in the default browser
	logger.Info("Opening browser for authorization", "provider", provider, "url", url)

	if err := openBrowser(url); err != nil {
		logger.Error("Failed to open browser", "error", err)
		logger.Error("Please open this URL manually", "url", url)
	}

	return nil
}

// StartOAuthCallbackServer starts the HTTP server to handle OAuth callbacks
func StartOAuthCallbackServer(logger *slog.Logger, store *db.Store) error {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	// Don't start if already running
	if server != nil {
		return nil
	}

	// Create a new server
	mux := http.NewServeMux()
	server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Setup the callback handler
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer reqCancel()
		provider, userInfo, err := callbackHandler(reqCtx, logger, store, r)
		if err != nil {
			fmt.Fprintf(w, "Error in handler: %s", err)
		} else {
			logger.Info("Successfully retrieved user information", "provider", provider)
			// Log user info details
			userJSON, _ := json.MarshalIndent(userInfo, "", "  ")
			userJSONString := string(userJSON)
			logger.Debug("User info", "data", userJSONString)
			fmt.Fprintf(w, "Authentication successful! You can close this window.\nUser data:\n%s", userJSONString)
		}
		// At the end of processing, whether successful or not, signal completion
		flowWaitGroup.Done()
	})

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting OAuth callback server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// ShutdownOAuthCallbackServer gracefully shuts down the callback server
func ShutdownOAuthCallbackServer(ctx context.Context, logger *slog.Logger) error {
	// Create a channel to signal when WaitGroup is done
	done := make(chan struct{})

	go func() {
		flowWaitGroup.Wait()
		close(done)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		logger.Info("All OAuth flows completed successfully")
	case <-ctx.Done():
		logger.Warn("Context cancelled while waiting for OAuth flows")
	case <-time.After(5 * time.Minute):
		logger.Warn("Timeout waiting for OAuth flows to complete")
	}

	serverMutex.Lock()
	defer serverMutex.Unlock()

	if server == nil {
		return nil
	}

	logger.Info("Shutting down OAuth callback server")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	server = nil
	return nil
}
