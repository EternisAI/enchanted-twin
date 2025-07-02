package twitter

import (
	"context"
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
	var tools []mcp_golang.Tool

	// List feed tweets tool
	listFeedSchema, err := utils.ConverToInputSchema(ListFeedTweetsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_feed_tweets: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:           LIST_FEED_TOOL_NAME,
		Description:    LIST_FEED_TOOL_DESCRIPTION,
		RawInputSchema: listFeedSchema,
	})

	// Post tweet tool
	postTweetSchema, err := utils.ConverToInputSchema(PostTweetArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for post_tweet: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:           POST_TWEET_TOOL_NAME,
		Description:    POST_TWEET_TOOL_DESCRIPTION,
		RawInputSchema: postTweetSchema,
	})

	// Search tweets tool
	searchTweetsSchema, err := utils.ConverToInputSchema(SearchTweetsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for search_tweets: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:           SEARCH_TWEETS_TOOL_NAME,
		Description:    SEARCH_TWEETS_TOOL_DESCRIPTION,
		RawInputSchema: searchTweetsSchema,
	})

	// List bookmarks tool
	listBookmarksSchema, err := utils.ConverToInputSchema(ListBookmarksArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_bookmarks: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:           LIST_BOOKMARKS_TOOL_NAME,
		Description:    LIST_BOOKMARKS_TOOL_DESCRIPTION,
		RawInputSchema: listBookmarksSchema,
	})

	return mcp_golang.NewListToolsResult(tools, ""), nil
}

func (c *TwitterClient) CallTool(
	ctx context.Context,
	request mcp_golang.CallToolRequest,
) (*mcp_golang.CallToolResult, error) {
	fmt.Println("Call tool TWITTER", request.Params.Name, request.Params.Arguments)

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
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err := processListFeedTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to list feed tweets", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case POST_TWEET_TOOL_NAME:
		var argumentsTyped PostTweetArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err := processPostTweet(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to post tweet", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case SEARCH_TWEETS_TOOL_NAME:
		var argumentsTyped SearchTweetsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		content, err := processSearchTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return mcp_golang.NewToolResultErrorFromErr("Failed to search tweets", err), nil
		}
		return &mcp_golang.CallToolResult{Content: content}, nil
	case LIST_BOOKMARKS_TOOL_NAME:
		var argumentsTyped ListBookmarksArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
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
