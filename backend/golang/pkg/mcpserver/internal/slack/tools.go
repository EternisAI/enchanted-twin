package slack

import (
	"context"
	"fmt"
	"strings"

	mcp_golang "github.com/mark3labs/mcp-go/mcp"
	"github.com/slack-go/slack"

	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
)

const (
	LIST_CHANNELS_TOOL_NAME                     = "list_slack_channels"
	LIST_DIRECT_MESSAGE_CONVERSATIONS_TOOL_NAME = "list_slack_direct_message_conversations"
	POST_MESSAGE_TOOL_NAME                      = "post_slack_message"
	SEARCH_MESSAGES_TOOL_NAME                   = "search_slack_messages"
)

const (
	LIST_CHANNELS_TOOL_DESCRIPTION                     = "List the channels the user is a member of"
	LIST_DIRECT_MESSAGE_CONVERSATIONS_TOOL_DESCRIPTION = "List the direct message conversations the user is a member of"
	POST_MESSAGE_TOOL_DESCRIPTION                      = "Post a message to a channel"
	SEARCH_MESSAGES_TOOL_DESCRIPTION                   = "Search for messages in channels"
)

type ListDirectMessageConversationsArguments struct {
	Cursor string `json:"cursor" jsonschema:"description=The cursor for pagination, empty if first page"`
	Limit  int    `json:"limit"  jsonschema:"required,description=The number of channels to list, minimum 10, maximum 50"`
}

type ListChannelsArguments struct {
	Cursor string `json:"cursor" jsonschema:"description=The cursor for pagination, empty if first page"`
	Limit  int    `json:"limit"  jsonschema:"required,description=The number of channels to list, minimum 10, maximum 50"`
}

type PostMessageArguments struct {
	ChannelID string `json:"channel_id" jsonschema:"required,description=The ID of the channel to post the message to"`
	Text      string `json:"text"       jsonschema:"required,description=The content of the message"`
}

type SearchMessagesArguments struct {
	Query  string `json:"query"  jsonschema:"required,description=The query to search for"`
	Cursor string `json:"cursor" jsonschema:"description=The cursor for pagination, empty if first page"`
	ID     string `json:"id"     jsonschema:"description=The ID of the channel/conversation to search in"`
}

func processListDirectMessageConversations(
	ctx context.Context,
	accessToken string,
	arguments ListDirectMessageConversationsArguments,
) ([]mcp_golang.Content, error) {
	api := slack.New(accessToken)

	allChannels := []slack.Channel{}
	cursor := arguments.Cursor
	var finalNextCursor string

	fmt.Println("Fetching all available channels")

	for {
		params := &slack.GetConversationsParameters{
			Cursor: cursor,
			Limit:  50, // Use max per call to minimize API calls
			Types: []string{
				"mpim",
				"im",
			}, // Adjust types as needed
		}

		channels, nextCursor, err := api.GetConversationsContext(ctx, params)
		if err != nil {
			fmt.Println("Error getting channels:", err)
			return nil, err
		}

		allChannels = append(allChannels, channels...)
		finalNextCursor = nextCursor

		// Stop only when no more pages
		if nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	contents := []mcp_golang.Content{}
	userNames := map[string]string{}
	for _, channel := range allChannels {
		var channelInfo string
		if channel.IsIM {
			// For DMs, you might want to fetch the user's name
			// This requires additional API calls and permissions

			if userName, ok := userNames[channel.User]; !ok {
				user, err := api.GetUserInfo(channel.User)
				if err != nil {
					fmt.Println("Error getting user info:", err)
					continue
				}
				userNames[channel.User] = user.Name
				channelInfo = fmt.Sprintf("Direct Message (ID: %s), members: %s", channel.ID, user.Name)
			} else {
				channelInfo = fmt.Sprintf("Direct Message (ID: %s), members: %s", channel.ID, userName)
			}
		} else if channel.IsMpIM {
			memberNames := []string{}
			ids, _, _ := api.GetUsersInConversationContext(
				ctx, &slack.GetUsersInConversationParameters{ChannelID: channel.ID})
			for _, member := range ids {
				if userName, ok := userNames[member]; !ok {
					user, err := api.GetUserInfo(member)
					if err != nil {
						fmt.Println("Error getting user info:", err)
						continue
					}
					userNames[member] = user.Name
					memberNames = append(memberNames, user.Name)
				} else {
					memberNames = append(memberNames, userName)
				}
			}

			channelInfo = fmt.Sprintf("Group Direct Message (ID: %s) with: %s", channel.ID, strings.Join(memberNames, ", "))
		}

		contents = append(contents, mcp_golang.NewTextContent(channelInfo))
	}

	// Append the next cursor information as a text content if it exists
	if finalNextCursor != "" {
		contents = append(contents, mcp_golang.NewTextContent(fmt.Sprintf("Next page cursor: %s", finalNextCursor)))
	}

	return contents, nil
}

func processListChannels(
	ctx context.Context,
	accessToken string,
	arguments ListChannelsArguments,
) ([]mcp_golang.Content, error) {
	api := slack.New(accessToken)

	allChannels := []slack.Channel{}
	cursor := arguments.Cursor
	var finalNextCursor string

	fmt.Println("Fetching all available channels")

	for {
		params := &slack.GetConversationsParameters{
			Cursor: cursor,
			Limit:  50, // Use max per call to minimize API calls
			Types: []string{
				"public_channel",
				"private_channel",
			}, // Adjust types as needed
		}

		fmt.Println("Getting channels with params", params)
		channels, nextCursor, err := api.GetConversationsContext(ctx, params)
		if err != nil {
			fmt.Println("Error getting channels:", err)
			return nil, err
		}

		allChannels = append(allChannels, channels...)
		fmt.Println("Total channels fetched so far:", len(allChannels))
		finalNextCursor = nextCursor

		// Stop only when no more pages
		if nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	contents := []mcp_golang.Content{}
	fmt.Println("Final number of channels:", len(allChannels))
	for _, channel := range allChannels {
		channelInfo := fmt.Sprintf("Channel: %s (ID: %s)", channel.Name, channel.ID)

		contents = append(contents, mcp_golang.NewTextContent(channelInfo))
	}

	// Append the next cursor information as a text content if it exists
	if finalNextCursor != "" {
		contents = append(contents, mcp_golang.NewTextContent(fmt.Sprintf("Next page cursor: %s", finalNextCursor)))
	}

	return contents, nil
}

func processPostMessage(
	ctx context.Context,
	accessToken string,
	arguments PostMessageArguments,
) ([]mcp_golang.Content, error) {
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

	return []mcp_golang.Content{
		mcp_golang.NewTextContent(fmt.Sprintf("Message posted successfully: %s", messageLink)),
	}, nil
}

func processSearchMessages(
	ctx context.Context,
	accessToken string,
	arguments SearchMessagesArguments,
) ([]mcp_golang.Content, error) {
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

	contents := []mcp_golang.Content{}

	if channelID == "" {
		messageResults, err := api.SearchMessagesContext(ctx, arguments.Query, searchParams)
		if err != nil {
			fmt.Println("Error searching messages:", err)
			return nil, err
		}

		for _, match := range messageResults.Matches {
			// Format the message content for display
			// Note: match.User might be a user ID. You might need another API call to get the username.
			messageInfo := fmt.Sprintf("Channel: %s\nUser: %s\nTimestamp: %s\nLink: %s\nText: %s\n---",
				match.Channel.Name, match.Username, match.Timestamp, match.Permalink, match.Text)
			contents = append(contents, mcp_golang.NewTextContent(messageInfo))
		}
	} else {
		params := &slack.GetConversationHistoryParameters{
			ChannelID: channelID, // e.g. "C01234567"
			Cursor:    "",        // first page
			Limit:     100,       // max 100; Slack may return fewer
			Inclusive: false,     // don’t include the message at oldest/latest
		}

		// one page
		messages, err := api.GetConversationHistoryContext(ctx, params)
		if err != nil {
			fmt.Println("Error getting conversation history:", err)
			return nil, err
		}

		fmt.Println("Message results length", len(messages.Messages))

		for _, message := range messages.Messages {
			// Note: match.User might be a user ID. You might need another API call to get the username.
			messageInfo := fmt.Sprintf("Channel: %s\nUser: %s\nTimestamp: %s\nLink: %s\nText: %s\n---",
				message.Channel, message.Username, message.Timestamp, message.Permalink, message.Text)
			contents = append(contents, mcp_golang.NewTextContent(messageInfo))
		}
	}

	// Determine the next page cursor (which is just the next page number as a string)
	// var nextCursor *string
	// if results.Paging.Page < results.Paging.Pages {
	// 	nextPageStr := fmt.Sprintf("%d", results.Paging.Page+1)
	// 	nextCursor = &nextPageStr
	// }

	// Append the next cursor information as a text content if it exists
	// if nextCursor != nil && *nextCursor != "" {
	// 	contents = append(contents, &mcp_golang.Content{
	// 		Type: "text",
	// 		TextContent: &mcp_golang.TextContent{
	// 			Text: fmt.Sprintf("Next page cursor: %s", *nextCursor),
	// 		},
	// 	})
	// }

	return contents, nil
}

func GenerateSlackTools() ([]mcp_golang.Tool, error) {
	var tools []mcp_golang.Tool

	// List channels tool
	listChannelsSchema, err := utils.ConverToInputSchema(ListChannelsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_channels: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:        LIST_CHANNELS_TOOL_NAME,
		Description: LIST_CHANNELS_TOOL_DESCRIPTION,
		InputSchema: mcp_golang.ToolInputSchema{
			Type:       "object",
			Properties: listChannelsSchema,
		},
	})

	// List direct message conversations tool
	listDMConversationsSchema, err := utils.ConverToInputSchema(ListDirectMessageConversationsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_direct_message_conversations: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:        LIST_DIRECT_MESSAGE_CONVERSATIONS_TOOL_NAME,
		Description: LIST_DIRECT_MESSAGE_CONVERSATIONS_TOOL_DESCRIPTION,
		InputSchema: mcp_golang.ToolInputSchema{
			Type:       "object",
			Properties: listDMConversationsSchema,
		},
	})

	// Post message tool
	postMessageSchema, err := utils.ConverToInputSchema(PostMessageArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for post_message: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:        POST_MESSAGE_TOOL_NAME,
		Description: POST_MESSAGE_TOOL_DESCRIPTION,
		InputSchema: mcp_golang.ToolInputSchema{
			Type:       "object",
			Properties: postMessageSchema,
		},
	})

	// Search messages tool
	searchMessagesSchema, err := utils.ConverToInputSchema(SearchMessagesArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for search_messages: %w", err)
	}
	tools = append(tools, mcp_golang.Tool{
		Name:        SEARCH_MESSAGES_TOOL_NAME,
		Description: SEARCH_MESSAGES_TOOL_DESCRIPTION,
		InputSchema: mcp_golang.ToolInputSchema{
			Type:       "object",
			Properties: searchMessagesSchema,
		},
	})

	return tools, nil
}
