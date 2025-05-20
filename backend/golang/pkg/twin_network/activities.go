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
	openai "github.com/openai/openai-go"
)

func (a *TwinNetworkWorkflow) MonitorNetworkActivity(ctx context.Context, input NetworkMonitorInput) (*NetworkMonitorOutput, error) {
	a.logger.Info("Starting network activity monitoring",
		"networkID", input.NetworkID,
		"lastMessageID", input.LastMessageID)

	allNewMessages, err := a.twinNetworkAPI.GetNewMessages(ctx, input.NetworkID, input.LastMessageID, 30)
	if err != nil {
		a.logger.Error("Failed to fetch new messages", "error", err)
		return nil, fmt.Errorf("failed to fetch new messages: %w", err)
	}

	a.logger.Info("Retrieved new messages",
		"count", len(allNewMessages),
		"networkID", input.NetworkID)

	currentRunMaxID := input.LastMessageID
	if len(allNewMessages) > 0 {
		// Assuming messages are sorted by ID ascending from getNewMessages,
		// the last message in the slice has the highest ID.
		currentRunMaxID = allNewMessages[len(allNewMessages)-1].ID
	}

	if len(allNewMessages) == 0 {
		a.logger.Info("No new messages to process.", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, nil
	}

	var nonSelfAuthoredMessages []NetworkMessage
	for _, msg := range allNewMessages {
		if msg.AuthorPubKey == a.agentKey.PubKeyHex() {
			a.logger.Debug("Skipping message authored by our agent", "messageID", msg.ID, "networkID", input.NetworkID)
			continue
		}
		// TODO: Add message to message store
		nonSelfAuthoredMessages = append(nonSelfAuthoredMessages, NetworkMessage{
			// ID:           msg.ID,
			// AuthorPubKey: msg.AuthorPubKey,
			// NetworkID:    msg.NetworkID,
			// Content:      msg.Content,
			// CreatedAt:    msg.CreatedAt,
		})
	}

	if len(nonSelfAuthoredMessages) == 0 {
		a.logger.Info("No non-self-authored new messages to process after filtering.", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, nil
	}

	const messageProcessingLimit = 20
	var messagesForTool []NetworkMessage
	if len(nonSelfAuthoredMessages) > messageProcessingLimit {
		messagesForTool = nonSelfAuthoredMessages[len(nonSelfAuthoredMessages)-messageProcessingLimit:]
	} else {
		messagesForTool = nonSelfAuthoredMessages
	}

	if len(messagesForTool) == 0 {
		// This case should ideally be covered by the previous check (len(nonSelfAuthoredMessages) == 0)
		// but kept for robustness.
		a.logger.Info("Message batch for tool is empty after selection. No action taken.", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, nil
	}

	a.logger.Info("Processing batch of messages for tool", "count", len(messagesForTool), "networkID", input.NetworkID)

	personality, err := a.identityService.GetPersonality(ctx)
	if err != nil {
		a.logger.Error("Failed to get identity context for batch processing", "error", err, "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, fmt.Errorf("failed to get identity context for batch: %w", err)
	}

	response, err := a.readNetworkTool.Execute(ctx, map[string]any{
		"network_message": messagesForTool,
	}, personality)
	if err != nil {
		a.logger.Error("Failed to process message batch with agent tool", "error", err, "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, fmt.Errorf("agent tool failed for batch: %w", err)
	}

	responseString, ok := response.(*types.StructuredToolResult).Output["response"].(string)
	if !ok {
		a.logger.Error("Unexpected response type from tool for batch", "resultType", fmt.Sprintf("%T", response), "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, fmt.Errorf("unexpected response type from tool for batch: %T", response)
	}

	if responseString == "" {
		a.logger.Info("Agent tool returned an empty response for the batch. No message will be posted.", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, nil
	}

	// Prevent posting generic "Unable to generate response" messages
	if responseString == "Unable to generate response." {
		a.logger.Info("Agent tool generated a generic 'Unable to generate response' message for the batch. No message will be posted.", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, nil
	}

	signature, err := a.agentKey.SignMessage(responseString)
	if err != nil {
		a.logger.Error("Failed to sign batch response message", "error", err, "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, fmt.Errorf("failed to sign batch response: %w", err)
	}

	err = a.postMessage(ctx, input.NetworkID, responseString, a.agentKey.PubKeyHex(), signature)
	if err != nil {
		a.logger.Error("Failed to post batch response message", "error", err, "networkID", input.NetworkID)
		return &NetworkMonitorOutput{LastMessageID: currentRunMaxID}, fmt.Errorf("failed to post batch response: %w", err)
	}

	_, err = a.twinChatService.SendMessage(ctx, "default", fmt.Sprintf("I've replied to a group of messages in network %s: %s", input.NetworkID, responseString), false)
	if err != nil {
		// Log error but don't fail the entire workflow activity for a chat notification failure
		a.logger.Error("Failed to send chat notification for batch response", "error", err, "networkID", input.NetworkID)
	}

	a.logger.Info("Successfully processed and responded to message batch", "networkID", input.NetworkID)

	a.logger.Info("Completed network activity monitoring cycle",
		"networkID", input.NetworkID,
		"nextLastMessageID", currentRunMaxID)

	return &NetworkMonitorOutput{
		LastMessageID: currentRunMaxID,
	}, nil
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

func (a *TwinNetworkWorkflow) getNewMessages(ctx context.Context, networkID string, fromID int64, limit int) ([]NetworkMessage, error) {
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
