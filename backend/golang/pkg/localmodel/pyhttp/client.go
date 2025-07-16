package pyhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

const (
	serverURL = "http://0.0.0.0:8000"
)

type Client struct {
	logger     *log.Logger
	httpClient *http.Client
}

type GenerateRequest struct {
	Prompt string `json:"prompt"`
}

type GenerateResponse struct {
	Generated string            `json:"generated"`
	Extracted string            `json:"extracted"`
	Parsed    map[string]string `json:"parsed"`
}

func NewClient(logger *log.Logger) (*Client, error) {
	client := &Client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Check if server is already running
	if err := client.waitForServerReady(5 * time.Second); err != nil {
		return nil, fmt.Errorf("server is not running: %w", err)
	}

	return client, nil
}

func (c *Client) waitForServerReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/docs")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready within %v", timeout)
}

func (c *Client) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	start := time.Now()

	requestBody := GenerateRequest{Prompt: prompt}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response GenerateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	elapsed := time.Since(start)
	c.logger.Info("generate completed", "duration", elapsed, "prompt", prompt)

	return response.Parsed, nil
}

func (c *Client) Close() error {
	return nil
}
