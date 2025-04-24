package gmail

import (
	"context"
	"encoding/json"
	"fmt"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"golang.org/x/oauth2"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)


const LIST_EMAILS_TOOL_NAME = "list_emails"
const SEND_EMAIL_TOOL_NAME = "send_email"

const LIST_EMAILS_TOOL_DESCRIPTION = "List the emails from the user's inbox"
const SEND_EMAIL_TOOL_DESCRIPTION = "Send an email"



type ListEmailsArguments struct {
	PageToken string 	`json:"page_token" jsonschema:"required,description=The page token to list, default is empty"`
	Limit     int 		`json:"limit" jsonschema:"required,description=The number of emails to list"`
}


type SendEmailArguments struct {
	To      string `json:"to" jsonschema:"required,description=The email address to send the email to"`
	Subject string `json:"subject" jsonschema:"required,description=The subject of the email"`
	Body    string `json:"body" jsonschema:"required,description=The body of the email"`
}


func processListEmails(ctx context.Context, accessToken string, arguments ListEmailsArguments) ([]*mcp_golang.Content, error) {

	// Configure OAuth2 token
	token := &oauth2.Token{
		AccessToken: accessToken,
	}

	// Create an HTTP client with the access token
	config := oauth2.Config{}
	client := config.Client(ctx, token)

	// Initialize Gmail service
	gmailService, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		fmt.Println("Error initializing Gmail service:", err)
		return nil, err
	}
	fmt.Println("Gmail service initialized")
	request := gmailService.Users.Messages.List("me").MaxResults(int64(arguments.Limit))
	fmt.Println("Request:", request)
	if arguments.PageToken != "" {
		request = request.PageToken(arguments.PageToken)
	}
	response, err := request.Do()
	if err != nil {
		fmt.Println("Error listing emails:", err)
		return nil, err
	}
	
	contents := make([]*mcp_golang.Content, 0)

	for _, message := range response.Messages {
		// Get the message details
		msg, err := gmailService.Users.Messages.Get("me", message.Id).Do()
		if err != nil {
			fmt.Println("Error getting message details:", err)
			continue
		}
		
		// Convert message to JSON
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message to JSON:", err)
			continue
		}

		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: string(msgJSON),
			},
		})
	}

	return contents, nil
}



func processSendEmail(ctx context.Context, accessToken string, arguments SendEmailArguments) ([]*mcp_golang.Content, error) {

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: "Sent email to " + arguments.To,
			},
		},
	}, nil
}

