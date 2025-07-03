package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type SlackClient struct {
	Store *db.Store
}

func (c *SlackClient) ListTools(
	ctx context.Context,
	request mcp_golang.ListToolsRequest,
) (*mcp_golang.ListToolsResult, error) {
	tools, err := GenerateSlackTools()
	if err != nil {
		return nil, err
	}

	// Slack ListTools doesn't seem to support pagination in the same way as Twitter's API in the example.
	// If Slack API does support pagination for listing tools (which is unlikely), this would need adjustment.
	// For now, return all tools without considering the cursor.
	return mcp_golang.NewListToolsResult(tools, ""), nil
}

func (c *SlackClient) CallTool(
	ctx context.Context,
	request mcp_golang.CallToolRequest,
) (*mcp_golang.CallToolResult, error) {
	fmt.Println("Call tool SLACK", request.Params.Name, request.Params.Arguments)

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

	var content []mcp_golang.Content

	switch request.Params.Name {
	case LIST_DIRECT_MESSAGE_CONVERSATIONS_TOOL_NAME:
		var argumentsTyped ListDirectMessageConversationsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err = processListDirectMessageConversations(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	case LIST_CHANNELS_TOOL_NAME:
		var argumentsTyped ListChannelsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err = processListChannels(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	case POST_MESSAGE_TOOL_NAME:
		var argumentsTyped PostMessageArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err = processPostMessage(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	case SEARCH_MESSAGES_TOOL_NAME:
		var argumentsTyped SearchMessagesArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err = processSearchMessages(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
	default:
		return mcp_golang.NewToolResultError(fmt.Sprintf("tool not found: %s", request.Params.Name)), nil
	}

	return &mcp_golang.CallToolResult{
		Content: content,
	}, nil
}
