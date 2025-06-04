package twinnetwork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// HolonZeroClient represents a client for the HolonZero REST API
type HolonZeroClient interface {
	CreateThread(ctx context.Context, input NewThreadInput) (*Thread, error)
	ListThreads(ctx context.Context) ([]Thread, error)
	GetThread(ctx context.Context, id int) (*Thread, error)
	UpdateThread(ctx context.Context, id int, input UpdateThreadInput) (*Thread, error)
	DeleteThread(ctx context.Context, id int) error
}

// UserIdentificationData represents comprehensive user information for HolonZero
type UserIdentificationData struct {
	Name              string             `json:"name,omitempty"`
	Bio               string             `json:"bio,omitempty"`
	PrimaryEmail      string             `json:"primaryEmail,omitempty"`
	ConnectedAccounts []ConnectedAccount `json:"connectedAccounts"`
	UserProfile       *UserProfileData   `json:"userProfile,omitempty"`
}

type ConnectedAccount struct {
	Provider    string `json:"provider"`
	Username    string `json:"username"`
	UserID      string `json:"userId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	ConnectedAt string `json:"connectedAt,omitempty"`
}

type UserProfileData struct {
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	Location    string `json:"location,omitempty"`
}

// REST API types matching the HolonZero schema
type Thread struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Content     *string   `json:"content"`
	CreatorID   int       `json:"creatorId"`
	CreatorName string    `json:"creatorName"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type NewThreadInput struct {
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
}

type UpdateThreadInput struct {
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
}

// REST API response types
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// holonZeroClient implements the HolonZeroClient interface using REST API
type holonZeroClient struct {
	baseURL    string
	httpClient *http.Client
	store      *db.Store // Add database store for OAuth token access
}

// NewHolonZeroClient creates a new HolonZero REST API client
func NewHolonZeroClient(baseURL string, store *db.Store) HolonZeroClient {
	// Ensure the baseURL has the correct API path
	if baseURL != "" {
		baseURL = strings.TrimSuffix(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/api/v1") {
			baseURL = baseURL + "/api/v1"
		}
	}

	return &holonZeroClient{
		baseURL: baseURL,
		store:   store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// collectUserIdentificationData gathers comprehensive user information from all available sources
func (c *holonZeroClient) collectUserIdentificationData(ctx context.Context) (*UserIdentificationData, error) {
	if c.store == nil {
		return nil, fmt.Errorf("no database store available")
	}

	userData := &UserIdentificationData{
		ConnectedAccounts: []ConnectedAccount{},
	}

	// Get user profile information
	userProfile, err := c.store.GetUserProfile(ctx)
	if err == nil && userProfile != nil {
		if userProfile.Name != nil {
			userData.Name = *userProfile.Name
		}
		if userProfile.Bio != nil {
			userData.Bio = *userProfile.Bio
		}
	}

	// Collect OAuth tokens from all providers
	providers := []string{"google", "twitter", "linkedin", "slack", "github", "facebook"}

	for _, provider := range providers {
		tokens, err := c.store.GetOAuthTokensArray(ctx, provider)
		if err != nil {
			continue // Skip providers that aren't connected
		}

		for _, token := range tokens {
			account := ConnectedAccount{
				Provider: provider,
				Username: token.Username,
				// ConnectedAt: Use current time since CreatedAt is not available in OAuthTokens
				ConnectedAt: time.Now().Format(time.RFC3339),
			}

			// Extract additional information based on provider
			switch provider {
			case "google":
				account.Email = token.Username // For Google, username is the email
				userData.PrimaryEmail = token.Username
			case "twitter":
				// For Twitter, username is the handle
				account.UserID = token.Username
			case "linkedin":
				// For LinkedIn, might have additional profile data
				account.UserID = token.Username
			case "slack":
				account.UserID = token.Username
			}

			userData.ConnectedAccounts = append(userData.ConnectedAccounts, account)
		}
	}

	// Get source usernames for additional profile data
	sourceUsernames, err := c.store.GetAllSourceUsernames(ctx)
	if err == nil {
		for _, source := range sourceUsernames {
			// Update existing accounts with additional profile data or add new ones
			found := false
			for i, account := range userData.ConnectedAccounts {
				if account.Provider == source.Source && account.Username == source.Username {
					if source.FirstName != nil {
						account.DisplayName = *source.FirstName
						if source.LastName != nil {
							account.DisplayName += " " + *source.LastName
						}
					}
					if source.PhoneNumber != nil && userData.UserProfile == nil {
						userData.UserProfile = &UserProfileData{}
					}
					if userData.UserProfile != nil && source.PhoneNumber != nil {
						userData.UserProfile.PhoneNumber = *source.PhoneNumber
					}
					userData.ConnectedAccounts[i] = account
					found = true
					break
				}
			}

			// If not found in OAuth tokens, add as additional account info
			if !found && source.Source != "" && source.Username != "" {
				account := ConnectedAccount{
					Provider: source.Source,
					Username: source.Username,
				}
				if source.UserID != nil {
					account.UserID = *source.UserID
				}
				if source.FirstName != nil {
					account.DisplayName = *source.FirstName
					if source.LastName != nil {
						account.DisplayName += " " + *source.LastName
					}
				}
				userData.ConnectedAccounts = append(userData.ConnectedAccounts, account)
			}
		}
	}

	return userData, nil
}

// getGoogleAccessToken retrieves a valid Google OAuth access token
func (c *holonZeroClient) getGoogleAccessToken(ctx context.Context) (string, error) {
	if c.store == nil {
		return "", fmt.Errorf("no database store available for OAuth tokens")
	}

	// Try to get Google OAuth tokens
	tokens, err := c.store.GetOAuthTokens(ctx, "google")
	if err != nil {
		return "", fmt.Errorf("failed to get Google OAuth tokens: %w", err)
	}

	if tokens == nil {
		return "", fmt.Errorf("no Google OAuth tokens found - user needs to authenticate with Google first")
	}

	// Check if token is expired or will expire soon (within 5 minutes)
	if time.Now().Add(5 * time.Minute).After(tokens.ExpiresAt) {
		return "", fmt.Errorf("Google OAuth token is expired or expires soon (expires at %v)", tokens.ExpiresAt)
	}

	if tokens.Error {
		return "", fmt.Errorf("Google OAuth token has error flag set")
	}

	return tokens.AccessToken, nil
}

// executeRequest executes an HTTP request against the HolonZero REST API with Google OAuth authentication and comprehensive user identification
func (c *holonZeroClient) executeRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Add Google OAuth authentication if available
	if c.store != nil {
		accessToken, err := c.getGoogleAccessToken(ctx)
		if err != nil {
			// Log the error but continue without authentication
			// This allows HolonZero to work even without Google OAuth
			fmt.Printf("Warning: Could not get Google OAuth token for HolonZero authentication: %v\n", err)
		} else {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		}

		// Collect and send comprehensive user identification data
		userIdentification, err := c.collectUserIdentificationData(ctx)
		if err != nil {
			fmt.Printf("Warning: Could not collect user identification data: %v\n", err)
		} else {
			// Send user identification data as custom headers
			userDataJSON, err := json.Marshal(userIdentification)
			if err == nil {
				httpReq.Header.Set("X-User-Identification", string(userDataJSON))

				// Also send key identification fields as separate headers for easier parsing
				if userIdentification.Name != "" {
					httpReq.Header.Set("X-User-Name", userIdentification.Name)
				}
				if userIdentification.PrimaryEmail != "" {
					httpReq.Header.Set("X-User-Email", userIdentification.PrimaryEmail)
				}
				if userIdentification.Bio != "" {
					httpReq.Header.Set("X-User-Bio", userIdentification.Bio)
				}

				// Send connected accounts count and providers
				if len(userIdentification.ConnectedAccounts) > 0 {
					providers := make([]string, len(userIdentification.ConnectedAccounts))
					for i, account := range userIdentification.ConnectedAccounts {
						providers[i] = account.Provider
					}
					providersJSON, _ := json.Marshal(providers)
					httpReq.Header.Set("X-User-Connected-Providers", string(providersJSON))
				}
			}
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request to %s: %w", url, err)
	}

	return resp, nil
}

// handleAPIResponse processes the HTTP response and handles common error cases
func (c *holonZeroClient) handleAPIResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HolonZero authentication failed - check Google OAuth token validity")
	}

	// Check if response looks like HTML (error page)
	bodyStr := string(bodyBytes)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
		return nil, fmt.Errorf("HolonZero server returned HTML instead of JSON (endpoint may be wrong or server not running): %s", bodyStr[:min(200, len(bodyStr))])
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		var apiError APIError
		if err := json.Unmarshal(bodyBytes, &apiError); err == nil && apiError.Error != "" {
			return nil, fmt.Errorf("HolonZero API error (%d): %s", resp.StatusCode, apiError.Error)
		}
		return nil, fmt.Errorf("HolonZero API returned status %d: %s", resp.StatusCode, bodyStr[:min(200, len(bodyStr))])
	}

	return bodyBytes, nil
}

// CreateThread creates a new thread in HolonZero using REST API
func (c *holonZeroClient) CreateThread(ctx context.Context, input NewThreadInput) (*Thread, error) {
	resp, err := c.executeRequest(ctx, "POST", "/threads", input)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := c.handleAPIResponse(resp)
	if err != nil {
		return nil, err
	}

	var thread Thread
	if err := json.Unmarshal(bodyBytes, &thread); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thread response: %w", err)
	}

	return &thread, nil
}

// ListThreads retrieves the list of threads from HolonZero
func (c *holonZeroClient) ListThreads(ctx context.Context) ([]Thread, error) {
	resp, err := c.executeRequest(ctx, "GET", "/threads", nil)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := c.handleAPIResponse(resp)
	if err != nil {
		return nil, err
	}

	var threads []Thread
	if err := json.Unmarshal(bodyBytes, &threads); err != nil {
		return nil, fmt.Errorf("failed to unmarshal threads response: %w", err)
	}

	return threads, nil
}

// GetThread retrieves a specific thread by ID from HolonZero
func (c *holonZeroClient) GetThread(ctx context.Context, id int) (*Thread, error) {
	resp, err := c.executeRequest(ctx, "GET", fmt.Sprintf("/threads/%d", id), nil)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := c.handleAPIResponse(resp)
	if err != nil {
		return nil, err
	}

	var thread Thread
	if err := json.Unmarshal(bodyBytes, &thread); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thread response: %w", err)
	}

	return &thread, nil
}

// UpdateThread updates an existing thread in HolonZero
func (c *holonZeroClient) UpdateThread(ctx context.Context, id int, input UpdateThreadInput) (*Thread, error) {
	resp, err := c.executeRequest(ctx, "PUT", fmt.Sprintf("/threads/%d", id), input)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := c.handleAPIResponse(resp)
	if err != nil {
		return nil, err
	}

	var thread Thread
	if err := json.Unmarshal(bodyBytes, &thread); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thread response: %w", err)
	}

	return &thread, nil
}

// DeleteThread deletes a thread in HolonZero
func (c *holonZeroClient) DeleteThread(ctx context.Context, id int) error {
	_, err := c.executeRequest(ctx, "DELETE", fmt.Sprintf("/threads/%d", id), nil)
	return err
}

// createTwinThread represents a tool for creating new threads in the twins network using HolonZero
type createTwinThread struct {
	holonZeroClient HolonZeroClient
	nc              *nats.Conn
	participantName string // The name of this AI participant
}

// NewCreateTwinThreadTool creates a new instance of the create twin thread tool
func NewCreateTwinThreadTool(holonZeroEndpoint string, nc *nats.Conn, participantName string, store *db.Store) *createTwinThread {
	return &createTwinThread{
		holonZeroClient: NewHolonZeroClient(holonZeroEndpoint, store),
		nc:              nc,
		participantName: participantName,
	}
}

// Execute creates a new thread in the twins network using HolonZero REST API
func (t *createTwinThread) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	title, ok := inputs["title"].(string)
	if !ok {
		return nil, errors.New("title is not a string")
	}

	description, ok := inputs["description"].(string)
	if !ok {
		return nil, errors.New("description is not a string")
	}

	// Optional tags for categorization - we'll include them in the content
	var tags []string
	if tagsRaw, ok := inputs["tags"]; ok {
		if arr, ok := tagsRaw.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					tags = append(tags, s)
				}
			}
		}
	}

	// Check if HolonZero endpoint is configured
	if t.holonZeroClient.(*holonZeroClient).baseURL == "" {
		return &types.StructuredToolResult{
			ToolName: "create_twin_thread",
			Output: map[string]any{
				"content": fmt.Sprintf("HolonZero endpoint not configured. Thread '%s' creation skipped.", title),
			},
			ToolParams: map[string]any{
				"title":       title,
				"description": description,
				"tags":        tags,
				"error":       "HolonZero endpoint not configured (HOLON_ZERO_ENDPOINT environment variable not set)",
			},
		}, nil
	}

	// Prepare thread content with description and tags
	content := description
	if len(tags) > 0 {
		content += fmt.Sprintf("\n\nTags: %v", tags)
	}

	// Create thread in HolonZero using REST API
	threadInput := NewThreadInput{
		Title:   title,
		Content: content,
	}

	thread, err := t.holonZeroClient.CreateThread(ctx, threadInput)
	if err != nil {
		// If thread creation fails, return graceful error
		return &types.StructuredToolResult{
			ToolName: "create_twin_thread",
			Output: map[string]any{
				"content": fmt.Sprintf("Failed to create thread '%s' in HolonZero network: %v", title, err),
			},
			ToolParams: map[string]any{
				"title":       title,
				"description": description,
				"tags":        tags,
				"error":       err.Error(),
			},
		}, nil
	}

	// Publish thread creation event to twins network via NATS
	threadCreatedEvent := map[string]any{
		"type":             "thread_created",
		"thread_id":        thread.ID,
		"title":            thread.Title,
		"content":          thread.Content,
		"creator_name":     thread.CreatorName,
		"tags":             tags,
		"created_at":       thread.CreatedAt.Format(time.RFC3339),
		"holonzero_thread": true,
	}

	err = helpers.NatsPublish(t.nc, "twins.network.thread.created", threadCreatedEvent)
	if err != nil {
		// Don't fail the whole operation if NATS publish fails
		// Just log the error (you might want to add proper logging here)
		fmt.Printf("Warning: failed to publish to twins network: %v\n", err)
	}

	return &types.StructuredToolResult{
		ToolName: "create_twin_thread",
		Output: map[string]any{
			"content": fmt.Sprintf("Successfully created thread '%s' in HolonZero twins network", title),
		},
		ToolParams: map[string]any{
			"thread_id":     thread.ID,
			"title":         thread.Title,
			"content":       thread.Content,
			"creator_name":  thread.CreatorName,
			"tags":          tags,
			"created_at":    thread.CreatedAt.Format(time.RFC3339),
			"holonzero_url": fmt.Sprintf("https://holonzero.example.com/threads/%d", thread.ID), // Adjust URL as needed
		},
	}, nil
}

// Definition returns the OpenAI function definition for this tool
func (t *createTwinThread) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "create_twin_thread",
			Description: param.NewOpt("Creates a new thread in the HolonZero twins network for collaborative discussions and knowledge sharing"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]string{
						"type":        "string",
						"description": "The title of the thread",
					},
					"description": map[string]string{
						"type":        "string",
						"description": "A detailed description of what the thread is about",
					},
					"tags": map[string]any{
						"type":        "array",
						"description": "Optional tags to categorize the thread",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"title", "description"},
			},
		},
	}
}
