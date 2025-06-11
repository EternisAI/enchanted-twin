package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/config"
	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/models"
)

const (
	ComposioBaseURL = "https://backend.composio.dev/api/v3"
)

type ComposioService struct {
	logger     *log.Logger
	httpClient *http.Client
	apiKey     string
}

// NewComposioService creates a new instance of ComposioService
func NewComposioService() *ComposioService {
	return &ComposioService{
		logger: log.New(log.Writer(), "[Composio] ", log.LstdFlags),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: config.AppConfig.ComposioAPIKey,
	}
}

func (s *ComposioService) getComposioConfig(toolkitSlug string) (string, error) {
	switch toolkitSlug {
	case "twitter":
		return config.AppConfig.ComposioTwitterConfig, nil
	default:
		return "", fmt.Errorf("unsupported toolkit slug: %s", toolkitSlug)
	}
}

// CreateConnectedAccount creates a new connected account and returns the redirect URL
func (s *ComposioService) CreateConnectedAccount(userID, toolkitSlug, callbackURL string) (*models.CreateConnectedAccountResponse, error) {
	s.logger.Printf("Creating connected account for user %s with toolkit %s", userID, toolkitSlug)

	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	// Based on the API documentation, we need to use the v2 endpoint for initiate connection
	// as v1 is deprecated. However, since we're implementing v3 service, we'll use the
	// connected accounts endpoint pattern but adapt it for the current API structure
	url := fmt.Sprintf("%s/connected_accounts", ComposioBaseURL)
	composioConfig, err := s.getComposioConfig(toolkitSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get composio config: %w", err)
	}

	// Prepare request payload
	payload := map[string]interface{}{
		"auth_config": map[string]interface{}{
			"id": composioConfig,
		},
		"connection": map[string]interface{}{
			"user_id":      userID,
			"callback_url": callbackURL,
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Create HTTP request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var response models.ConnectedAccountResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &models.CreateConnectedAccountResponse{
		ID:  response.ID,
		URL: response.RedirectURL,
	}, nil
}

func (s *ComposioService) GetConnectedAccount(accountID string) (*models.ConnectedAccountDetailResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	// Based on the API documentation, we need to use the v2 endpoint for initiate connection
	// as v1 is deprecated. However, since we're implementing v3 service, we'll use the
	// connected accounts endpoint pattern but adapt it for the current API structure
	url := fmt.Sprintf("%s/connected_accounts/%s", ComposioBaseURL, accountID)

	// Create HTTP request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var response models.ConnectedAccountDetailResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

func (s *ComposioService) RefreshToken(accountID string) (*models.ComposioRefreshTokenResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	// Based on the API documentation, we need to use the v2 endpoint for initiate connection
	// as v1 is deprecated. However, since we're implementing v3 service, we'll use the
	// connected accounts endpoint pattern but adapt it for the current API structure
	url := fmt.Sprintf("%s/connected_accounts/%s/refresh", ComposioBaseURL, accountID)

	// Create HTTP request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var response models.ComposioRefreshTokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetToolBySlug retrieves toolkit information by slug
func (s *ComposioService) GetToolBySlug(slug string) (*models.Toolkit, error) {
	s.logger.Printf("Getting toolkit by slug: %s", slug)

	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	url := fmt.Sprintf("%s/toolkits/%s", ComposioBaseURL, slug)

	// Create HTTP request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("x-api-key", s.apiKey)

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var toolkit models.Toolkit
	if err := json.Unmarshal(body, &toolkit); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Printf("Successfully retrieved toolkit: %s", toolkit.Name)
	return &toolkit, nil
}

// ExecuteTool executes a tool with the provided parameters
func (s *ComposioService) ExecuteTool(toolSlug string, req models.ExecuteToolRequest) (*models.ExecuteToolResponse, error) {
	s.logger.Printf("Executing tool: %s for user: %s", toolSlug, req.UserID)

	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	url := fmt.Sprintf("%s/tools/execute/%s", ComposioBaseURL, toolSlug)

	// If UserID is provided but EntityID is not, use UserID as EntityID for backward compatibility
	if req.UserID != "" && req.EntityID == "" {
		req.EntityID = req.UserID
	}

	jsonPayload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Create HTTP request
	ctx := context.Background()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", s.apiKey)

	// Execute request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var response models.ExecuteToolResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Printf("Successfully executed tool: %s, successful: %t", toolSlug, response.Successful)
	return &response, nil
}

// GetConnectedAccountByUserID retrieves connected accounts for a specific user
func (s *ComposioService) GetConnectedAccountByUserID(userID string) ([]map[string]interface{}, error) {
	s.logger.Printf("Getting connected accounts for user: %s", userID)

	if s.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}

	url := fmt.Sprintf("%s/connected_accounts?entity_id=%s", ComposioBaseURL, userID)

	// Create HTTP request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("x-api-key", s.apiKey)

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var composioErr models.ComposioError
		if jsonErr := json.Unmarshal(body, &composioErr); jsonErr == nil {
			return nil, composioErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var accounts []map[string]interface{}
	if err := json.Unmarshal(body, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Printf("Successfully retrieved %d connected accounts for user: %s", len(accounts), userID)
	return accounts, nil
}
