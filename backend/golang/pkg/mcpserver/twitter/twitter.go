package twitter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	mcp_golang "github.com/metoro-io/mcp-golang"
)


type TwitterClient struct {
	Store   *db.Store
}


func (c *TwitterClient) ListTools(ctx context.Context, cursor *string) (*mcp_golang.ToolsResponse, error) {


	inputSchema, err := helpers.ConverToInputSchema(ListFeedTweetsArguments{})
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

	inputSchema, err = helpers.ConverToInputSchema(PostTweetArguments{})
	if err != nil {
		return nil, err
	}

	description = POST_TWEET_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        POST_TWEET_TOOL_NAME,
		Description: &description,
		InputSchema: inputSchema,
	})

	inputSchema, err = helpers.ConverToInputSchema(SearchTweetsArguments{})
	if err != nil {
		return nil, err
	}

	description = SEARCH_TWEETS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        SEARCH_TWEETS_TOOL_NAME,
		Description: &description,
		InputSchema: inputSchema,
	})

	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *TwitterClient) CallTool(ctx context.Context, name string, arguments any) (*mcp_golang.ToolResponse, error) {
	
	fmt.Println("Call tool TWITTER", name, arguments)


	bytes, err := helpers.ConvertToBytes(arguments)
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
			break
		case POST_TWEET_TOOL_NAME:
			var argumentsTyped PostTweetArguments
			if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
				return nil, err
			}			
			result, err := processPostTweet(oauthTokens.AccessToken, argumentsTyped)
			if err != nil {
				return nil, err
			}
			content = result
			break
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
			break
		default:
			return nil, fmt.Errorf("tool not found")
	}


	return &mcp_golang.ToolResponse{
		Content: content,
	}, nil
}
