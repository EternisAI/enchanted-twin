package perplexity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/sirupsen/logrus"
)

const (
	PERPLEXITY_ASK_TOOL_NAME = "perplexity_ask"
)

const (
	PERPLEXITY_ASK_TOOL_DESCRIPTION = "Engages in a conversation using the Sonar API. " +
		"Accepts an array of messages (each with a role and content) " +
		"and returns a ask completion response from the Perplexity model."
)

const (
	PERPLEXITY_URL = "https://api.perplexity.ai/chat/completions"
	MODEL_NAME     = "sonar-pro"
)

type PerplexityAskArguments struct {
	Messages []Message `json:"messages" jsonschema:"required,description=Array of conversation messages"`
}

type Message struct {
	Role    string `json:"role" jsonschema:"required,description=Role of the message (e.g., system, user, assistant)"`
	Content string `json:"content" jsonschema:"required,description=The content of the message"`
}

// Structs for Perplexity API request and response.
type PerplexityRequestBody struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type PerplexityChoice struct {
	Message Message `json:"message"`
}

type PerplexityResponse struct {
	Choices   []PerplexityChoice `json:"choices"`
	Citations []string           `json:"citations,omitempty"`
}

func processPerplexityAsk(
	ctx context.Context,
	arguments PerplexityAskArguments,
) ([]*mcp_golang.Content, error) {
	if len(arguments.Messages) == 0 {
		return nil, errors.New("messages array is required")
	}

	apiKey := os.Getenv("PERPLEXITY_API_KEY")
	if apiKey == "" {
		return nil, errors.New("PERPLEXITY_API_KEY environment variable not set")
	}

	requestBody := PerplexityRequestBody{
		Model:    MODEL_NAME,
		Messages: arguments.Messages,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", PERPLEXITY_URL, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error while calling Perplexity API: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error("Error closing response body", "error", err)
		}
	}()

	if resp.StatusCode >= 400 {
		errorText, _ := io.ReadAll(resp.Body) // Read the error body
		return nil, fmt.Errorf("perplexity API error: %d %s\n%s", resp.StatusCode, resp.Status, errorText)
	}

	var perplexityResp PerplexityResponse
	if err := json.NewDecoder(resp.Body).Decode(&perplexityResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response from Perplexity API: %w", err)
	}

	if len(perplexityResp.Choices) == 0 {
		return nil, errors.New("no choices returned from Perplexity API")
	}

	messageContent := perplexityResp.Choices[0].Message.Content

	if len(perplexityResp.Citations) > 0 {
		var citationsBuilder strings.Builder
		citationsBuilder.WriteString("\n\nCitations:\n")
		for i, citation := range perplexityResp.Citations {
			citationsBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, citation))
		}
		messageContent += citationsBuilder.String()
	}

	resultContent := &mcp_golang.Content{
		Type: "text",
		TextContent: &mcp_golang.TextContent{
			Text: messageContent,
		},
	}

	return []*mcp_golang.Content{resultContent}, nil
}
