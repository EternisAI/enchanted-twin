package google

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"golang.org/x/oauth2"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	LIST_EMAILS_TOOL_NAME = "list_emails"
	SEND_EMAIL_TOOL_NAME  = "send_email"
)

const (
	LIST_EMAILS_TOOL_DESCRIPTION = "List the emails from the user's inbox"
	SEND_EMAIL_TOOL_DESCRIPTION  = "Send an email to recipient email address"
)

type ListEmailsArguments struct {
	PageToken string `json:"page_token" jsonschema:"required,description=The page token to list, default is empty"`
	Limit     int    `json:"limit" jsonschema:"required,description=The number of emails to list"`
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
	request := gmailService.Users.Messages.List("me").MaxResults(int64(arguments.Limit))
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

		// // Convert message to JSON
		// msgJSON, err := json.Marshal(msg)
		// if err != nil {
		// 	fmt.Println("Error marshaling message to JSON:", err)
		// 	continue
		// }

		var subject, from, date string
		for _, header := range msg.Payload.Headers {
			switch header.Name {
			case "Subject":
				subject = header.Value
			case "From":
				from = header.Value
			case "Date":
				date = header.Value
			}
		}

		formattedText := fmt.Sprintf("From: %s\nSubject: %s\nDate: %s\nID: %s",
			from, subject, date, msg.Id)

		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: formattedText,
			},
		})

	}

	return contents, nil
}

func processSendEmail(ctx context.Context, accessToken string, arguments SendEmailArguments) ([]*mcp_golang.Content, error) {
	token := &oauth2.Token{
		AccessToken: accessToken,
	}

	config := oauth2.Config{}
	client := config.Client(ctx, token)

	gmailService, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		fmt.Println("Error initializing Gmail service:", err)
		return nil, err
	}

	profile, err := gmailService.Users.GetProfile("me").Do()
	if err != nil {
		fmt.Println("Error retrieving user profile:", err)
		return nil, err
	}

	message := createMessage(profile.EmailAddress, arguments.To, arguments.Subject, arguments.Body)

	_, err = gmailService.Users.Messages.Send("me", message).Do()
	if err != nil {
		fmt.Printf("Error sending email: %s\n", err)
		return nil, err
	}

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: "Successfully sent email",
			},
		},
	}, nil
}

func createMessage(from, to, subject, bodyContent string) *gmail.Message {
	// Compose email
	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	var message string
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + bodyContent

	// Encode as base64
	return &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(message)),
	}
}

func GenerateGmailTools() ([]mcp_golang.ToolRetType, error) {
	var tools []mcp_golang.ToolRetType

	listEmailsSchema, err := helpers.ConverToInputSchema(ListEmailsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_emails: %w", err)
	}
	desc := LIST_EMAILS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        LIST_EMAILS_TOOL_NAME,
		Description: &desc,
		InputSchema: listEmailsSchema,
	})

	sendEmailSchema, err := helpers.ConverToInputSchema(SendEmailArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for send_email: %w", err)
	}
	desc = SEND_EMAIL_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        SEND_EMAIL_TOOL_NAME,
		Description: &desc,
		InputSchema: sendEmailSchema,
	})

	return tools, nil
}
