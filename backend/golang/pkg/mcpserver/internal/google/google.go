package google

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type GoogleClient struct {
	Store *db.Store
}

func (c *GoogleClient) ListTools(
	ctx context.Context,
	request mcp_golang.ListToolsRequest,
) (*mcp_golang.ListToolsResult, error) {
	tools := []mcp_golang.Tool{}
	log.Info("Listing tools in google mcp server")
	gmailTools, err := GenerateGmailTools()
	if err != nil {
		return nil, err
	}
	tools = append(tools, gmailTools...)

	googleCalendarTools, err := GenerateGoogleCalendarTools()
	if err != nil {
		return nil, err
	}
	tools = append(tools, googleCalendarTools...)

	googleDriveTools, err := GenerateGoogleDriveTools()
	if err != nil {
		return nil, err
	}
	tools = append(tools, googleDriveTools...)
	log.Info("Returning tools", "tools", tools)
	return &mcp_golang.ListToolsResult{
		Tools: tools,
	}, nil
}

func (c *GoogleClient) CallTool(
	ctx context.Context,
	request mcp_golang.CallToolRequest,
) (*mcp_golang.CallToolResult, error) {
	// Convert generic arguments to the expected Go struct.
	name := request.Params.Name
	fmt.Println("Call tool GOOGLE", name, request.Params.Arguments)

	var content []mcp_golang.Content

	switch name {
	case LIST_EMAIL_ACCOUNTS_TOOL_NAME:
		result, err := processListEmailAccounts(ctx, c.Store)
		if err != nil {
			return nil, err
		}
		content = result
	case SEARCH_EMAILS_TOOL_NAME:
		var argumentsTyped SearchEmailsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processSearchEmails(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case SEND_EMAIL_TOOL_NAME:
		var argumentsTyped SendEmailArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processSendEmail(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case EMAIL_BY_ID_TOOL_NAME:
		var argumentsTyped EmailByIdArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processEmailById(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case REPLY_EMAIL_TOOL_NAME:
		var argumentsTyped ReplyEmailArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processReplyEmail(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case SEARCH_FILES_TOOL_NAME:
		var argumentsTyped SearchFilesArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processSearchFiles(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case READ_FILE_TOOL_NAME:
		var argumentsTyped ReadFileArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processReadFile(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case LIST_CALENDAR_EVENTS_TOOL_NAME:
		var argumentsTyped ListEventsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processListEvents(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case CREATE_CALENDAR_EVENT_TOOL_NAME:
		var argumentsTyped CreateEventArgs
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processCreateEvent(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	default:
		return nil, fmt.Errorf("tool not found")
	}

	return &mcp_golang.CallToolResult{
		Content: content,
	}, nil
}
