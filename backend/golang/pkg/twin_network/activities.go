package twin_network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

func (a *TwinNetworkWorkflow) MonitorNetworkActivity(ctx context.Context, input NetworkMonitorInput) (*NetworkMonitorOutput, error) {
	a.logger.Info("Starting network activity monitoring",
		"networkID", input.NetworkID,
		"lastMessageID", input.LastMessageID)

	messages, err := a.getNewMessages(ctx, input.NetworkID, input.LastMessageID, nil)
	if err != nil {
		a.logger.Error("Failed to fetch new messages", "error", err)
		return nil, fmt.Errorf("failed to fetch new messages: %w", err)
	}

	a.logger.Info("Retrieved messages",
		"count", len(messages),
		"networkID", input.NetworkID)

	var lastMessageID int64 = input.LastMessageID
	for _, msg := range messages {

		if msg.AuthorPubKey == a.agentKey.PubKeyHex() {
			a.logger.Debug("Skipping message authored by our agent",
				"messageID", msg.ID,
				"networkID", input.NetworkID)
			continue
		}

		if msg.ID > lastMessageID {
			lastMessageID = msg.ID
		}

		identityContext, err := a.identityService.GetPersonality(ctx)
		if err != nil {
			a.logger.Error("Failed to get identity context",
				"error", err,
				"messageID", msg.ID)
			continue
		}

		response, err := a.readNetworkTool.Execute(ctx, map[string]any{
			"network_message": []NetworkMessage{msg},
		}, identityContext)
		if err != nil {
			a.logger.Error("Failed to process message with agent tool",
				"error", err,
				"messageID", msg.ID)
			continue
		}

		responseString, ok := response.(*types.StructuredToolResult).Output["response"].(string)
		if !ok {
			a.logger.Error("Unexpected response type from tool",
				"messageID", msg.ID,
				"resultType", fmt.Sprintf("%T", response))
			continue
		}

		signature, err := a.agentKey.SignMessage(responseString)
		if err != nil {
			a.logger.Error("Failed to sign message",
				"error", err,
				"messageID", msg.ID)
			continue
		}
		err = a.postMessage(ctx, input.NetworkID, responseString, a.agentKey.PubKeyHex(), signature)
		if err != nil {
			a.logger.Error("Failed to post message",
				"error", err,
				"messageID", msg.ID)
			continue
		}

		a.logger.Info("Successfully processed and responded to message",
			"messageID", msg.ID,
			"networkID", input.NetworkID)
	}

	a.logger.Info("Completed network activity monitoring",
		"networkID", input.NetworkID,
		"lastMessageID", lastMessageID)

	return &NetworkMonitorOutput{
		LastMessageID: lastMessageID,
	}, nil
}

func (a *TwinNetworkWorkflow) getNewMessages(ctx context.Context, networkID string, fromID int64, limit *int) ([]NetworkMessage, error) {
	a.logger.Debug("Fetching new messages",
		"networkID", networkID,
		"fromID", fromID,
		"limit", limit)

	query := `
		query GetNewMessages($networkID: String!, $fromID: Int!, $limit: Int) {
			getNewMessages(networkID: $networkID, fromID: $fromID, limit: $limit) {
				id
				authorPubKey
				networkID
				content
				createdAt
				isMine
			}
		}
	`

	variables := map[string]interface{}{
		"networkID": networkID,
		"fromID":    fromID,
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
			GetNewMessages []struct {
				ID           string    `json:"id"`
				AuthorPubKey string    `json:"authorPubKey"`
				NetworkID    string    `json:"networkID"`
				Content      string    `json:"content"`
				CreatedAt    time.Time `json:"createdAt"`
				IsMine       bool      `json:"isMine"`
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

	messages := make([]NetworkMessage, len(response.Data.GetNewMessages))
	for i, msg := range response.Data.GetNewMessages {
		id, _ := strconv.ParseInt(msg.ID, 10, 64)
		messages[i] = NetworkMessage{
			ID:           id,
			AuthorPubKey: msg.AuthorPubKey,
			NetworkID:    msg.NetworkID,
			Content:      msg.Content,
			CreatedAt:    msg.CreatedAt,
		}
	}

	a.logger.Info("Successfully retrieved messages",
		"count", len(messages),
		"networkID", networkID)
	return messages, nil
}

func (a *TwinNetworkWorkflow) postMessage(ctx context.Context, networkID string, content string, authorPubKey string, signature string) error {
	a.logger.Debug("Posting message",
		"networkID", networkID,
		"content", content,
		"authorPubKey", authorPubKey)

	query := `
		mutation PostMessage($networkID: String!, $content: String!, $authorPubKey: String!, $signature: String!) {
			postMessage(networkID: $networkID, content: $content, authorPubKey: $authorPubKey, signature: $signature) {
				id
				authorPubKey
				networkID
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
