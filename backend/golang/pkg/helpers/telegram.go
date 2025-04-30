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
		if err := resp.Body.Close(); err != nil {
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GraphQL mutation request failed: %v", resp.StatusCode, string(bodyBytes))
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

func SetTelegramEnabled(ctx context.Context, store *db.Store, enabled bool) error {
	err := store.SetValue(ctx, types.TelegramEnabled, fmt.Sprintf("%t", enabled))
	if err != nil {
		return fmt.Errorf("error setting telegram enabled: %w", err)
	}
	return nil
}
