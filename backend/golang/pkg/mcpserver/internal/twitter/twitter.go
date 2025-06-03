package twitter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
)

type TwitterClient struct {
	Store *db.Store
}

func (c *TwitterClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*mcp_golang.ToolsResponse, error) {
	inputSchema, err := utils.ConverToInputSchema(ListFeedTweetsArguments{})
	if err != nil {
		return nil, err
	}

	description := LIST_FEED_TOOL_DESCRIPTION
	tools := []mcp_golang.ToolRetType{
		{
			Name:        LIST_FEED_TOOL_NAME,
			Description: &description,
			InputSchema: inputSchema,
		},
	}

	inputSchema, err = utils.ConverToInputSchema(PostTweetArguments{})
	if err != nil {
		return nil, err
	}

	description = POST_TWEET_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        POST_TWEET_TOOL_NAME,
		Description: &description,
		InputSchema: inputSchema,
	})

	inputSchema, err = utils.ConverToInputSchema(SearchTweetsArguments{})
	if err != nil {
		return nil, err
	}

	description = SEARCH_TWEETS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        SEARCH_TWEETS_TOOL_NAME,
		Description: &description,
		InputSchema: inputSchema,
	})

	inputSchema, err = utils.ConverToInputSchema(ListBookmarksArguments{})
	if err != nil {
		return nil, err
	}

	description = LIST_BOOKMARKS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        LIST_BOOKMARKS_TOOL_NAME,
		Description: &description,
		InputSchema: inputSchema,
	})

	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *TwitterClient) CallTool(
	ctx context.Context,
	name string,
	arguments any,
) (*mcp_golang.ToolResponse, error) {
	fmt.Println("Call tool TWITTER", name, arguments)

	bytes, err := utils.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}

	logger := log.Default()

	logger.Debug("Refreshing token for twitter")
	_, err = auth.RefreshOAuthToken(ctx, logger, c.Store, "twitter")
	if err != nil {
		return nil, err
	}
	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "twitter")
	if err != nil {
		return nil, err
	}

	var content []*mcp_golang.Content

	switch name {
	case LIST_FEED_TOOL_NAME:
		var argumentsTyped ListFeedTweetsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processListFeedTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case POST_TWEET_TOOL_NAME:
		var argumentsTyped PostTweetArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processPostTweet(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case SEARCH_TWEETS_TOOL_NAME:
		var argumentsTyped SearchTweetsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processSearchTweets(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case LIST_BOOKMARKS_TOOL_NAME:
		var argumentsTyped ListBookmarksArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processListBookmarks(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	default:
		return nil, fmt.Errorf("tool not found")
	}

	return &mcp_golang.ToolResponse{
		Content: content,
	}, nil
}
