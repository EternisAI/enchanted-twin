package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/charmbracelet/log"
	mcp_golang "github.com/metoro-io/mcp-golang"
)

type SlackClient struct {
	Store *db.Store
}

func (c *SlackClient) ListTools(ctx context.Context, cursor *string) (*mcp_golang.ToolsResponse, error) {
	tools, err := GenerateSlackTools()
	if err != nil {
		return nil, err
	}

	// Slack ListTools doesn't seem to support pagination in the same way as Twitter's API in the example.
	// If Slack API does support pagination for listing tools (which is unlikely), this would need adjustment.
	// For now, return all tools without considering the cursor.
	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *SlackClient) CallTool(ctx context.Context, name string, arguments any) (*mcp_golang.ToolResponse, error) {

	fmt.Println("Call tool SLACK", name, arguments)

	bytes, err := helpers.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}

	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "slack")
	if err != nil {
		return nil, err
	}

	logger := log.Default()
	if oauthTokens.ExpiresAt.Before(time.Now()) {
		logger.Debug("Refreshing token for slack")
		_, err = auth.RefreshOAuthToken(ctx, logger, c.Store, "slack")
		if err != nil {
			return nil, err
		}
		oauthTokens, err = c.Store.GetOAuthTokens(ctx, "slack")
		if err != nil {
			return nil, err
		}
	}

	var content []*mcp_golang.Content

	switch name {
	case LIST_CHANNELS_TOOL_NAME:
		var argumentsTyped ListChannelsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		content, err = processListChannels(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	case POST_MESSAGE_TOOL_NAME:
		var argumentsTyped PostMessageArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		content, err = processPostMessage(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	case SEARCH_MESSAGES_TOOL_NAME:
		var argumentsTyped SearchMessagesArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		content, err = processSearchMessages(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("tool not found: %s", name)
	}


	return &mcp_golang.ToolResponse{
		Content: content,
	}, nil
}
