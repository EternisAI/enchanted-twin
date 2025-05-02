package google

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	mcp_golang "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

type GoogleClient struct {
	Store *db.Store
}

func (c *GoogleClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*mcp_golang.ToolsResponse, error) {
	tools := []mcp_golang.ToolRetType{}

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

	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *GoogleClient) CallTool(
	ctx context.Context,
	name string,
	arguments any,
) (*mcp_golang.ToolResponse, error) {
	// Convert generic arguments to the expected Go struct.
	fmt.Println("Call tool GOOGLE", name, arguments)

	bytes, err := helpers.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}

	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "google")
	if err != nil {
		return nil, err
	}

	logger := log.Default()
	if oauthTokens.ExpiresAt.Before(time.Now()) {
		fmt.Println("Refreshing token for google")
		_, err = auth.RefreshOAuthToken(ctx, logger, c.Store, "google")
		if err != nil {
			return nil, err
		}
		oauthTokens, err = c.Store.GetOAuthTokens(ctx, "google")
		if err != nil {
			return nil, err
		}
	}

	var content []*mcp_golang.Content

	switch name {
	case LIST_EMAIL_ACCOUNTS_TOOL_NAME:
		result, err := processListEmailAccounts(ctx, c.Store)
		if err != nil {
			return nil, err
		}
		content = result
	case SEARCH_EMAILS_TOOL_NAME:
		var argumentsTyped SearchEmailsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processSearchEmails(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case SEND_EMAIL_TOOL_NAME:
		var argumentsTyped SendEmailArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processSendEmail(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case EMAIL_BY_ID_TOOL_NAME:
		var argumentsTyped EmailByIdArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processEmailById(ctx, c.Store, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case SEARCH_FILES_TOOL_NAME:
		var argumentsTyped SearchFilesArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processSearchFiles(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case READ_FILE_TOOL_NAME:
		var argumentsTyped ReadFileArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processReadFile(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case LIST_CALENDAR_EVENTS_TOOL_NAME:
		var argumentsTyped ListEventsArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processListEvents(ctx, oauthTokens.AccessToken, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = result
	case CREATE_CALENDAR_EVENT_TOOL_NAME:
		var argumentsTyped CreateEventArgs
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		result, err := processCreateEvent(ctx, oauthTokens.AccessToken, argumentsTyped)
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
