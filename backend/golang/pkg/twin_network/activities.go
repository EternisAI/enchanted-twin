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

	openai "github.com/openai/openai-go"
)

type MonitorNetworkActivityInput struct {
	NetworkID     string
	LastMessageID string
	Messages      []NetworkMessage
}

type QueryNetworkActivityInput struct {
	NetworkID string
	FromID    string
	Limit     int
}

func (a *TwinNetworkWorkflow) QueryNetworkActivity(ctx context.Context, input QueryNetworkActivityInput) ([]NetworkMessage, error) {
	a.logger.Debug("Querying network activity",
		"networkID", input.NetworkID,
		"fromID", input.FromID,
		"limit", input.Limit)

	allNewMessages, err := a.getNewMessages(ctx, input.NetworkID, input.FromID, input.Limit)
	if err != nil {
		a.logger.Error("Failed to fetch new messages", "error", err)
		return nil, fmt.Errorf("failed to fetch new messages: %w", err)
	}

	return allNewMessages, nil
}

func (a *TwinNetworkWorkflow) EvaluateMessage(ctx context.Context, message NetworkMessage) (string, error) {
	personality, err := a.identityService.GetPersonality(ctx)
	if err != nil {
		a.logger.Error("Failed to get identity context for batch processing", "error", err, "networkID", message.NetworkID)
		return "", fmt.Errorf("failed to get identity context for batch: %w", err)
	}
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fmt.Sprintf(`Analyze the conversation and provide your analysis in two parts:\n
		1. Reasoning: Your analysis of the conversation flow, message patterns, and the roles of each participant\n
		2. Response: A suggested next response that would be appropriate in this context\n

		If you think this message is useful to your human, send it to the chat by calling "send_to_chat" tool and specifying "chat_id" to be empty string.
		If the bulletin board message is interesting to your human and requires response and if you know the correct response call "send_bulletin_board_message" tool. 
		If you're missing some information nescessary to respond, only send message to your human chat.

		Here is some context about your personality and identity:

		Thread ID: %s
		%s`, personality, message.ThreadID)),
		openai.UserMessage(message.String()),
	}

	tools := a.toolRegistry.GetAll()

	response, err := a.agent.Execute(ctx, nil, messages, tools)
	if err != nil {
		a.logger.Error("Failed to execute agent", "error", err)
		return "", fmt.Errorf("failed to execute agent: %w", err)
	}

	return response.Content, nil
}

func (a *TwinNetworkWorkflow) getNewMessages(ctx context.Context, networkID string, fromID string, limit int) ([]NetworkMessage, error) {
	a.logger.Debug("Fetching new messages",
		"networkID", networkID,
		"fromID", fromID,
		"limit", limit)

	fromIDInt := 0
	if fromID != "" {
		parsedFromID, err := strconv.Atoi(fromID)
		if err != nil {
			a.logger.Error("Failed to parse fromID to integer", "fromID", fromID, "error", err)
		} else {
			fromIDInt = parsedFromID
		}
	}

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
		"fromID":    fromIDInt,
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
