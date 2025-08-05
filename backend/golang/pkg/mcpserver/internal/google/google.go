package google

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
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
	for _, tool := range tools {
		log.Info("Google tool", "name", tool.Name)
	}
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

	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "google")
	if err != nil {
		return nil, err
	}

	// TODO: @pottekkat Pass loggers properly.
	// Refresh the token, it is actually retrieved by individual tools.
	logger := log.Default()
	if oauthTokens.ExpiresAt.Before(time.Now()) || oauthTokens.Error {
		logger.Debug("Refreshing token for google")
		_, err = auth.RefreshOAuthToken(ctx, logger, c.Store, "google")
		if err != nil {
			return nil, err
		}
		_, err = c.Store.GetOAuthTokens(ctx, "google")
		if err != nil {
			return nil, err
		}
	}

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
	case GET_LABELS_TOOL_NAME:
		var argumentsTyped GetLabelsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processGetLabels(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case MODIFY_EMAIL_LABELS_TOOL_NAME:
		var argumentsTyped ModifyEmailLabelsArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processModifyEmailLabels(ctx, c.Store, argumentsTyped)
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
	case UPDATE_CALENDAR_EVENT_TOOL_NAME:
		var argumentsTyped UpdateEventArgs
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processUpdateEvent(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case DELETE_CALENDAR_EVENT_TOOL_NAME:
		var argumentsTyped DeleteEventArgs
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		result, err := processDeleteEvent(ctx, c.Store, argumentsTyped)
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
