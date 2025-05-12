package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type APIClient struct {
	ServiceURL string
	Client     *http.Client
	Logger     interface {
		Info(msg string, keyvals ...interface{})
		Error(msg string, keyvals ...interface{})
		Debug(msg string, keyvals ...interface{})
	}
}

func NewAPIClient(serviceURL string, logger interface {
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	Debug(msg string, keyvals ...interface{})
}) *APIClient {
	return &APIClient{
		ServiceURL: serviceURL,
		Client:     &http.Client{Timeout: 30 * time.Second},
		Logger:     logger,
	}
}

func (c *APIClient) CheckHealth(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.ServiceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *APIClient) GetMe(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/getMe", c.ServiceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GetMe request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send GetMe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetMe failed with status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GetMe response: %w", err)
	}

	return result, nil
}

func (c *APIClient) GetChats(ctx context.Context, limit int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/getChats?limit=%d", c.ServiceURL, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GetChats request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send GetChats request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetChats failed with status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GetChats response: %w", err)
	}

	return result, nil
}

func (c *APIClient) SendMessage(ctx context.Context, chatID int64, message string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/sendMessage", c.ServiceURL)
	
	requestBody := map[string]interface{}{
		"chat_id": chatID,
		"message": message,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SendMessage request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create SendMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send SendMessage request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SendMessage failed with status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode SendMessage response: %w", err)
	}
	
	return result, nil
}
