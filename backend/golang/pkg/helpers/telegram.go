package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/types"
)

func GetChatURL(botName string, chatUUID string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botName, chatUUID)
}

func PostMessage(ctx context.Context, chatUUID string, message string, chatServerUrl string) (interface{}, error) {
	client := &http.Client{}
	mutationPayload := map[string]interface{}{
		"query": `
			mutation SendTelegramMessage($chatUUID: ID!, $text: String!) {
				sendTelegramMessage(chatUUID: $chatUUID, text: $text)
			}
		`,
		"variables": map[string]interface{}{
			"chatUUID": chatUUID,
			"text":     message,
		},
		"operationName": "SendTelegramMessage",
	}
	mutationBody, err := json.Marshal(mutationPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL mutation payload: %v", err)
	}

	gqlURL := chatServerUrl
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, bytes.NewBuffer(mutationBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send GraphQL mutation request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}
		return nil, fmt.Errorf("GraphQL mutation request failed: status %v, body: %v", resp.StatusCode, string(bodyBytes))
	}

	var gqlResponse struct {
		Data   interface{} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err = json.NewDecoder(resp.Body).Decode(&gqlResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL mutation response: %v", err)
	}
	return gqlResponse, nil
}

func GetTelegramEnabled(ctx context.Context, store *db.Store) (string, error) {
	telegramEnabled, err := store.GetValue(ctx, types.TelegramEnabled)
	if err != nil {
		return "", fmt.Errorf("error getting telegram enabled: %w", err)
	}
	return telegramEnabled, nil
}
