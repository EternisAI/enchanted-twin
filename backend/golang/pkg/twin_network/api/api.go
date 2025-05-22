package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/pkg/twin_network/graph/model"
)

type TwinNetworkAPI struct {
	logger           *log.Logger
	networkServerURL string
}

func NewTwinNetworkAPI(logger *log.Logger, networkServerURL string) *TwinNetworkAPI {
	return &TwinNetworkAPI{
		logger:           logger,
		networkServerURL: networkServerURL,
	}
}

func (a *TwinNetworkAPI) GetNewMessages(ctx context.Context, networkID string, fromTime time.Time, limit int) ([]*model.NetworkThread, error) {
	a.logger.Debug("Fetching new messages",
		"networkID", networkID,
		"fromTime", fromTime,
		"limit", limit)

	query := `
		query GetNewMessages($networkID: String!, $from: DateTime!, $limit: Int) {
			getNewMessages(networkID: $networkID, from: $from, limit: $limit) {
				id
				updatedAt
				messages {
					id
					authorPubKey
					networkID
					threadID
					content
					createdAt
					isMine
					signature
				}
			}
		}
	`

	variables := map[string]interface{}{
		"networkID": networkID,
		"from":      fromTime.Format(time.RFC3339),
		"limit":     limit,
	}

	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("Failed to marshal GraphQL request", "error", err)
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	a.logger.Debug("Sending GraphQL request",
		"url", a.networkServerURL,
		"payload", string(payloadBytes))

	req, err := http.NewRequestWithContext(ctx, "POST", a.networkServerURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		a.logger.Error("Failed to create request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Error("Failed to send request", "error", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("Network server returned non-200 status",
			"status", resp.StatusCode,
			"body", string(body),
			"url", a.networkServerURL,
			"headers", resp.Header)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("Failed to read response body", "error", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	a.logger.Debug("Received response from network server",
		"body", string(body),
		"url", a.networkServerURL,
		"headers", resp.Header)

	var response struct {
		Data struct {
			GetNewMessages []*struct {
				ID        string `json:"id"`
				UpdatedAt string `json:"updatedAt"`
				Messages  []struct {
					ID           string    `json:"id"`
					AuthorPubKey string    `json:"authorPubKey"`
					NetworkID    string    `json:"networkID"`
					ThreadID     string    `json:"threadID"`
					Content      string    `json:"content"`
					CreatedAt    time.Time `json:"createdAt"`
					IsMine       bool      `json:"isMine"`
					Signature    string    `json:"signature"`
				} `json:"messages"`
			} `json:"getNewMessages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&response); err != nil {
		a.logger.Error("Failed to decode response",
			"error", err,
			"rawBody", string(body))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Errors) > 0 {
		a.logger.Error("GraphQL errors in response",
			"errors", response.Errors)
		return nil, fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	threads := make([]*model.NetworkThread, len(response.Data.GetNewMessages))
	for i, thread := range response.Data.GetNewMessages {
		messages := make([]*model.NetworkMessage, len(thread.Messages))
		for j, msg := range thread.Messages {
			id, _ := strconv.ParseInt(msg.ID, 10, 64)
			messages[j] = &model.NetworkMessage{
				ID:           strconv.FormatInt(id, 10),
				AuthorPubKey: msg.AuthorPubKey,
				NetworkID:    msg.NetworkID,
				ThreadID:     msg.ThreadID,
				Content:      msg.Content,
				CreatedAt:    msg.CreatedAt.Format(time.RFC3339),
				IsMine:       msg.IsMine,
				Signature:    msg.Signature,
			}
		}

		threads[i] = &model.NetworkThread{
			ID:        thread.ID,
			UpdatedAt: thread.UpdatedAt,
			Messages:  messages,
		}
	}

	a.logger.Info("Successfully retrieved threads",
		"count", len(threads),
		"networkID", networkID)
	return threads, nil
}

func (a *TwinNetworkAPI) PostMessage(ctx context.Context, networkID string, threadID string, content string, authorPubKey string, signature string) error {
	// If threadID is empty, generate a new UUID
	if threadID == "" {
		threadID = uuid.New().String()
		a.logger.Debug("Generated new threadID for empty input", "threadID", threadID)
	}

	a.logger.Debug("Info message",
		"networkID", networkID,
		"content", content,
		"authorPubKey", authorPubKey,
		"threadID", threadID)

	query := `
		mutation PostMessage($networkID: String!, $content: String!, $authorPubKey: String!, $signature: String!, $threadID: String!) {
			postMessage(networkID: $networkID, content: $content, authorPubKey: $authorPubKey, signature: $signature, threadID: $threadID) {
				id
				authorPubKey
				networkID
				threadID
				content
				createdAt
				isMine
				signature
			}
		}
	`

	variables := map[string]interface{}{
		"networkID":    networkID,
		"content":      content,
		"authorPubKey": authorPubKey,
		"signature":    signature,
		"threadID":     threadID,
	}

	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("Failed to marshal GraphQL request", "error", err)
		return fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	a.logger.Debug("Sending GraphQL request",
		"url", a.networkServerURL,
		"payload", string(payloadBytes))

	req, err := http.NewRequestWithContext(ctx, "POST", a.networkServerURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		a.logger.Error("Failed to create request", "error", err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Error("Failed to send request", "error", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("Network server returned non-200 status",
			"status", resp.StatusCode,
			"body", string(body),
			"url", a.networkServerURL,
			"headers", resp.Header)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("Failed to read response body", "error", err)
		return fmt.Errorf("failed to read response body: %w", err)
	}

	a.logger.Debug("Received response from network server",
		"body", string(body),
		"url", a.networkServerURL,
		"headers", resp.Header)

	var response struct {
		Data struct {
			PostMessage struct {
				ID           string    `json:"id"`
				AuthorPubKey string    `json:"authorPubKey"`
				NetworkID    string    `json:"networkID"`
				Content      string    `json:"content"`
				CreatedAt    time.Time `json:"createdAt"`
				IsMine       bool      `json:"isMine"`
				Signature    string    `json:"signature"`
			} `json:"postMessage"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&response); err != nil {
		a.logger.Error("Failed to decode response",
			"error", err,
			"rawBody", string(body))
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Errors) > 0 {
		a.logger.Error("GraphQL errors in response",
			"errors", response.Errors)
		return fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	a.logger.Info("Successfully posted message",
		"messageID", response.Data.PostMessage.ID,
		"networkID", networkID)
	return nil
}
