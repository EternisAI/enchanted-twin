package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
)

type TwitterClient struct {
	Store *db.Store
}

func (c *TwitterClient) ListTools(
	ctx context.Context,
	request mcp_golang.ListToolsRequest,
) (*mcp_golang.ListToolsResult, error) {
	// Create tools using the new SDK
	tools := []mcp_golang.Tool{
		mcp_golang.NewTool(LIST_FEED_TOOL_NAME, mcp_golang.WithDescription(LIST_FEED_TOOL_DESCRIPTION)),
		mcp_golang.NewTool(POST_TWEET_TOOL_NAME, mcp_golang.WithDescription(POST_TWEET_TOOL_DESCRIPTION)),
		mcp_golang.NewTool(SEARCH_TWEETS_TOOL_NAME, mcp_golang.WithDescription(SEARCH_TWEETS_TOOL_DESCRIPTION)),
		mcp_golang.NewTool(LIST_BOOKMARKS_TOOL_NAME, mcp_golang.WithDescription(LIST_BOOKMARKS_TOOL_DESCRIPTION)),
	}

	return mcp_golang.NewListToolsResult(tools, ""), nil
}

func (c *TwitterClient) CallTool(
	ctx context.Context,
	request mcp_golang.CallToolRequest,
) (*mcp_golang.CallToolResult, error) {
	fmt.Println("Call tool TWITTER", request.Params.Name, request.Params.Arguments)

	bytes, err := utils.ConvertToBytes(request.Params.Arguments)
	if err != nil {
		return nil, err
	}

	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "twitter")
	if err != nil {
		return nil, err
	}

	logger := log.Default()
	if oauthTokens.ExpiresAt.Before(time.Now()) {
		logger.Debug("Refreshing token for twitter")
		_, err = auth.RefreshOAuthToken(ctx, logger, c.Store, "twitter")
		if err != nil {
			return nil, err
		}
		oauthTokens, err = c.Store.GetOAuthTokens(ctx, "twitter")
		if err != nil {
			return nil, err
		}
	}

	switch request.Params.Name {
	case LIST_FEED_TOOL_NAME:
		var argumentsTyped ListFeedTweetsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to parse arguments", err), nil
		}
		content, err := processListFeedTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to list feed tweets", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case POST_TWEET_TOOL_NAME:
		var argumentsTyped PostTweetArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to parse arguments", err), nil
		}
		content, err := processPostTweet(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to post tweet", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case SEARCH_TWEETS_TOOL_NAME:
		var argumentsTyped SearchTweetsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to parse arguments", err), nil
		}
		content, err := processSearchTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to search tweets", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case LIST_BOOKMARKS_TOOL_NAME:
		var argumentsTyped ListBookmarksArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to parse arguments", err), nil
		}
		content, err := processListBookmarks(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to list bookmarks", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	default:
		return mcp_golang.NewToolResultError("Tool not found"), nil
	}
}
