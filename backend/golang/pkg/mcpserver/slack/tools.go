package slack

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/slack-go/slack"
)

const LIST_CHANNELS_TOOL_NAME = "list_slack_channels"
const POST_MESSAGE_TOOL_NAME = "post_slack_message"
const SEARCH_MESSAGES_TOOL_NAME = "search_slack_messages"

const LIST_CHANNELS_TOOL_DESCRIPTION = "List the channels the user is a member of"
const POST_MESSAGE_TOOL_DESCRIPTION = "Post a message to a channel"
const SEARCH_MESSAGES_TOOL_DESCRIPTION = "Search for messages in channels"

type ListChannelsArguments struct {
	Cursor string `json:"cursor" jsonschema:"description=The cursor for pagination, empty if first page"`
	Limit  int    `json:"limit" jsonschema:"required,description=The number of channels to list, minimum 10, maximum 50"`
}

type PostMessageArguments struct {
	ChannelID string `json:"channel_id" jsonschema:"required,description=The ID of the channel to post the message to"`
	Text      string `json:"text" jsonschema:"required,description=The content of the message"`
}

type SearchMessagesArguments struct {
	Query  string `json:"query" jsonschema:"required,description=The query to search for"`
	Cursor string `json:"cursor" jsonschema:"description=The cursor for pagination, empty if first page"`
	ID     string `json:"id" jsonschema:"description=The ID of the channel/conversation to search in"`
}

func processListChannels(ctx context.Context, accessToken string, arguments ListChannelsArguments) ([]*mcp_golang.Content, error) {
	api := slack.New(accessToken)
	params := &slack.GetConversationsParameters{
		Cursor: arguments.Cursor,
		Limit:  arguments.Limit,
		Types:  []string{"public_channel", "private_channel", "mpim", "im"}, // Adjust types as needed
	}

	channels, nextCursor, err := api.GetConversationsContext(ctx, params)
	if err != nil {
		fmt.Println("Error getting channels:", err)
		return nil, err
	}

	contents := []*mcp_golang.Content{}
	for _, channel := range channels {
		channelInfo := fmt.Sprintf("Channel: %s (ID: %s)", channel.Name, channel.ID)
		if channel.IsIM {
			// For DMs, you might want to fetch the user's name
			// This requires additional API calls and permissions
			channelInfo = fmt.Sprintf("Direct Message (ID: %s)", channel.ID)
		} else if channel.IsMpIM {
			channelInfo = fmt.Sprintf("Group Direct Message (ID: %s)", channel.ID)
		}
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: channelInfo,
			},
		})
	}

	// Append the next cursor information as a text content if it exists
	if nextCursor != "" {
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("Next page cursor: %s", nextCursor),
			},
		})
	}

	return contents, nil
}

func processPostMessage(ctx context.Context, accessToken string, arguments PostMessageArguments) ([]*mcp_golang.Content, error) {
	api := slack.New(accessToken)

	// Post the message
	channelID, timestamp, err := api.PostMessageContext(
		ctx,
		arguments.ChannelID,
		slack.MsgOptionText(arguments.Text, false),
		slack.MsgOptionAsUser(true), // Post as the authenticated user
	)
	if err != nil {
		fmt.Printf("Error posting message to channel %s: %v\n", arguments.ChannelID, err)
		return nil, err
	}

	messageLink := fmt.Sprintf("https://slack.com/archives/%s/p%s", channelID, timestamp)

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("Message posted successfully: %s", messageLink),
			},
		},
	}, nil
}

func processSearchMessages(ctx context.Context, accessToken string, arguments SearchMessagesArguments) ([]*mcp_golang.Content, error) {
	api := slack.New(accessToken)

	// Slack's search parameters are a bit different.
	// We use SearchMessages instead of a dedicated timeline endpoint like Twitter.
	// Pagination is handled via page numbers, not cursors/tokens.
	page := 1
	if arguments.Cursor != "" {
		_, err := fmt.Sscan(arguments.Cursor, &page)
		if err != nil {
			// Handle error: invalid cursor format
			page = 1 // Default to page 1 if cursor is invalid
		}
	}

	channelID := arguments.ID

	searchParams := slack.NewSearchParameters()
	searchParams.Page = page

	// searchParams.Count = arguments.Limit // Slack API uses 'count', not 'limit'

	var results *slack.SearchMessages

	if channelID == "" {
		messageResults, err := api.SearchMessagesContext(ctx, arguments.Query, searchParams)
		if err != nil {
			fmt.Println("Error searching messages:", err)
			return nil, err
		}
		results = messageResults
	} else {
		messageResults, err := api.SearchMessages(channelID, searchParams)
		if err != nil {
			fmt.Println("Error searching messages:", err)
			return nil, err
		}
		results = messageResults
	}

	contents := []*mcp_golang.Content{}
	for _, match := range results.Matches {
		// Format the message content for display
		// Note: match.User might be a user ID. You might need another API call to get the username.
		messageInfo := fmt.Sprintf("Channel: %s\nUser: %s\nTimestamp: %s\nLink: %s\nText: %s\n---",
			match.Channel.Name, match.User, match.Timestamp, match.Permalink, match.Text)
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: messageInfo,
			},
		})
	}

	// Determine the next page cursor (which is just the next page number as a string)
	var nextCursor *string
	// if results.Paging.Page < results.Paging.Pages {
	// 	nextPageStr := fmt.Sprintf("%d", results.Paging.Page+1)
	// 	nextCursor = &nextPageStr
	// }

	// Append the next cursor information as a text content if it exists
	if nextCursor != nil && *nextCursor != "" {
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("Next page cursor: %s", *nextCursor),
			},
		})
	}

	return contents, nil
}

func GenerateSlackTools() ([]mcp_golang.ToolRetType, error) {
	var tools []mcp_golang.ToolRetType

	// List Channels Tool
	listChannelsSchema, err := helpers.ConverToInputSchema(ListChannelsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_channels: %w", err)
	}
	listChannelsDesc := LIST_CHANNELS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        LIST_CHANNELS_TOOL_NAME,
		Description: &listChannelsDesc,
		InputSchema: listChannelsSchema,
	})

	// Post Message Tool
	postMessageSchema, err := helpers.ConverToInputSchema(PostMessageArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for post_message: %w", err)
	}
	postMessageDesc := POST_MESSAGE_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        POST_MESSAGE_TOOL_NAME,
		Description: &postMessageDesc,
		InputSchema: postMessageSchema,
	})

	// Search Messages Tool
	searchMessagesSchema, err := helpers.ConverToInputSchema(SearchMessagesArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for search_messages: %w", err)
	}
	searchMessagesDesc := SEARCH_MESSAGES_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        SEARCH_MESSAGES_TOOL_NAME,
		Description: &searchMessagesDesc,
		InputSchema: searchMessagesSchema,
	})

	return tools, nil
}
