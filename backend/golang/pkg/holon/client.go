package holon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	clog "github.com/charmbracelet/log"
)

// APIClient represents the HolonZero API client.
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *clog.Logger
	authToken  string
}

// NewAPIClient creates a new HolonZero API client.
func NewAPIClient(baseURL string, options ...ClientOption) *APIClient {
	client := &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// ClientOption represents configuration options for the API client.
type ClientOption func(*APIClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *APIClient) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *APIClient) {
		c.httpClient.Timeout = timeout
	}
}

// WithLogger sets a logger for the API client.
func WithLogger(logger *clog.Logger) ClientOption {
	return func(c *APIClient) {
		c.logger = logger
	}
}

// WithAuthToken sets the Bearer authentication token.
func WithAuthToken(token string) ClientOption {
	return func(c *APIClient) {
		c.authToken = token
	}
}

// doRequest performs an HTTP request and handles common response processing.
func (c *APIClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	// Log the request path if logger is available
	if c.logger != nil {
		c.logger.Debug("HolonZero API request", "method", method, "path", path, "url", c.baseURL+path)
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add Bearer token if available
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("HolonZero API request failed", "method", method, "path", path, "error", err)
		}
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Log successful response
	if c.logger != nil {
		c.logger.Debug("HolonZero API response", "method", method, "path", path, "status", resp.StatusCode)
	}

	return resp, nil
}

// handleResponse processes HTTP response and unmarshals JSON.
func (c *APIClient) handleResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Health check endpoints

// GetHealth checks the health status of the application.
func (c *APIClient) GetHealth(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}

	var result HealthResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetStatus gets detailed status information.
func (c *APIClient) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Authentication endpoints

// AuthenticateParticipant authenticates a participant using OAuth token.
func (c *APIClient) AuthenticateParticipant(ctx context.Context) (*ParticipantAuthResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/participant", nil)
	if err != nil {
		return nil, err
	}

	var result ParticipantAuthResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Participant endpoints

// ListParticipants gets all participants.
func (c *APIClient) ListParticipants(ctx context.Context) ([]Participant, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/participants", nil)
	if err != nil {
		return nil, err
	}

	var result []Participant
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetParticipant gets a participant by ID.
func (c *APIClient) GetParticipant(ctx context.Context, id int) (*Participant, error) {
	path := fmt.Sprintf("/api/v1/participants/%d", id)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result Participant
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateParticipant creates a new participant.
func (c *APIClient) CreateParticipant(ctx context.Context, req CreateParticipantRequest) (*Participant, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/participants", req)
	if err != nil {
		return nil, err
	}

	var result Participant
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateParticipant updates a participant.
func (c *APIClient) UpdateParticipant(ctx context.Context, id int, req UpdateParticipantRequest) (*Participant, error) {
	path := fmt.Sprintf("/api/v1/participants/%d", id)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return nil, err
	}

	var result Participant
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteParticipant deletes a participant.
func (c *APIClient) DeleteParticipant(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v1/participants/%d", id)
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// Thread endpoints

// ListThreads gets all threads with optional query parameters.
func (c *APIClient) ListThreads(ctx context.Context, query *ThreadsQuery) ([]Thread, error) {
	path := "/api/v1/threads"
	if query != nil {
		params := url.Values{}
		if query.Page > 0 {
			params.Add("page", strconv.Itoa(query.Page))
		}
		if query.Limit > 0 {
			params.Add("limit", strconv.Itoa(query.Limit))
		}
		if !query.UpdatedAfter.IsZero() {
			params.Add("updatedAfter", query.UpdatedAfter.Format(time.RFC3339))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Thread
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ListThreadsPaginated gets paginated threads with filtering.
func (c *APIClient) ListThreadsPaginated(ctx context.Context, query *ThreadsQuery) (*PaginatedThreadsResponse, error) {
	path := "/api/v1/threads/paginated"
	if query != nil {
		params := url.Values{}
		if query.Page > 0 {
			params.Add("page", strconv.Itoa(query.Page))
		}
		if query.Limit > 0 {
			params.Add("limit", strconv.Itoa(query.Limit))
		}
		if !query.UpdatedAfter.IsZero() {
			params.Add("updatedAfter", query.UpdatedAfter.Format(time.RFC3339))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result PaginatedThreadsResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetThread gets a thread by ID.
func (c *APIClient) GetThread(ctx context.Context, id int) (*Thread, error) {
	path := fmt.Sprintf("/api/v1/threads/%d", id)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result Thread
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateThread creates a new thread.
func (c *APIClient) CreateThread(ctx context.Context, req CreateThreadRequest) (*Thread, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/threads", req)
	if err != nil {
		return nil, err
	}

	var result Thread
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateThread updates a thread.
func (c *APIClient) UpdateThread(ctx context.Context, id int, req UpdateThreadRequest) (*Thread, error) {
	path := fmt.Sprintf("/api/v1/threads/%d", id)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return nil, err
	}

	var result Thread
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteThread deletes a thread.
func (c *APIClient) DeleteThread(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v1/threads/%d", id)
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// GetThreadsByCreator gets all threads created by a specific participant.
func (c *APIClient) GetThreadsByCreator(ctx context.Context, participantID int) ([]Thread, error) {
	path := fmt.Sprintf("/api/v1/participants/%d/threads", participantID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Thread
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetSyncMetadata gets metadata for sync optimization.
func (c *APIClient) GetSyncMetadata(ctx context.Context) (*SyncMetadataResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/threads/sync-metadata", nil)
	if err != nil {
		return nil, err
	}

	var result SyncMetadataResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Reply endpoints

// GetThreadReplies gets all replies for a specific thread.
func (c *APIClient) GetThreadReplies(ctx context.Context, threadID int) ([]Reply, error) {
	path := fmt.Sprintf("/api/v1/threads/%d/replies", threadID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Reply
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetThreadRepliesPaginated gets paginated replies for a specific thread.
func (c *APIClient) GetThreadRepliesPaginated(ctx context.Context, threadID int, query *RepliesQuery) (*PaginatedRepliesResponse, error) {
	path := fmt.Sprintf("/api/v1/threads/%d/replies/paginated", threadID)
	if query != nil {
		params := url.Values{}
		if query.Page > 0 {
			params.Add("page", strconv.Itoa(query.Page))
		}
		if query.Limit > 0 {
			params.Add("limit", strconv.Itoa(query.Limit))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result PaginatedRepliesResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetReply gets a reply by ID.
func (c *APIClient) GetReply(ctx context.Context, id int) (*Reply, error) {
	path := fmt.Sprintf("/api/v1/replies/%d", id)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result Reply
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateReply creates a new reply.
func (c *APIClient) CreateReply(ctx context.Context, req CreateReplyRequest) (*Reply, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/replies", req)
	if err != nil {
		return nil, err
	}

	var result Reply
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateReply updates a reply.
func (c *APIClient) UpdateReply(ctx context.Context, id int, req UpdateReplyRequest) (*Reply, error) {
	path := fmt.Sprintf("/api/v1/replies/%d", id)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return nil, err
	}

	var result Reply
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteReply deletes a reply.
func (c *APIClient) DeleteReply(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/v1/replies/%d", id)
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// GetRepliesByParticipant gets all replies created by a specific participant.
func (c *APIClient) GetRepliesByParticipant(ctx context.Context, participantID int) ([]Reply, error) {
	path := fmt.Sprintf("/api/v1/participants/%d/replies", participantID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Reply
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}
