package gmail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	mcp_golang "github.com/metoro-io/mcp-golang"
)

type GmailClient struct {
	Store *db.Store
}

func (c *GmailClient) ListTools(ctx context.Context, cursor *string) (*mcp_golang.ToolsResponse, error) {

	tools := []mcp_golang.ToolRetType{}

	inputSchema, err := helpers.ConverToInputSchema(ListEmailsArguments{})
	if err != nil {
		return nil, err
	}
	desc := LIST_EMAILS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name: LIST_EMAILS_TOOL_NAME,
		Description: &desc,
		InputSchema: inputSchema,
	})

	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *GmailClient) CallTool(ctx context.Context, name string, arguments any) (*mcp_golang.ToolResponse, error) {
	// Convert generic arguments to the expected Go struct.
	fmt.Println("Call tool GMAIL", name, arguments)


	bytes, err := helpers.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}


	oauthTokens, err := c.Store.GetOAuthTokens(ctx, "google")

	if err != nil {
		return nil, err
	}

	var content []*mcp_golang.Content

	switch name {
		case "list_emails":
			var argumentsTyped ListEmailsArguments
			if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
				return nil, err
			}
			result, err := processListEmails(ctx, oauthTokens.AccessToken, argumentsTyped)
			if err != nil {
				return nil, err
			}
			content = result
		case "send_email":
			var argumentsTyped SendEmailArguments
			if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
				return nil, err
			}
			result, err := processSendEmail(ctx, oauthTokens.AccessToken, argumentsTyped)
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

