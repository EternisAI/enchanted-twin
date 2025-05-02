package google

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

const (
	SEARCH_EMAILS_TOOL_NAME       = "search_emails"
	SEND_EMAIL_TOOL_NAME          = "send_email"
	EMAIL_BY_ID_TOOL_NAME         = "email_by_id"
	LIST_EMAIL_ACCOUNTS_TOOL_NAME = "list_email_accounts"
)

const (
	SEARCH_EMAILS_TOOL_DESCRIPTION       = "Search the emails from the user's inbox, returns subject, from, date and id"
	SEND_EMAIL_TOOL_DESCRIPTION          = "Send an email to recipient email address"
	EMAIL_BY_ID_TOOL_DESCRIPTION         = "Get the email by id, returns subject, from, date and body"
	LIST_EMAIL_ACCOUNTS_TOOL_DESCRIPTION = "List the email accounts the user has"
)

type EmailQuery struct {
	In         string     `json:"in"         jsonschema:"description=The inbox to list emails from, default is 'inbox'"`
	TimeFilter TimeFilter `json:"time_filter" jsonschema:"description=The time filter to list emails, default is empty"`
	From       string     `json:"from"       jsonschema:"description=The sender of the emails to list, default is empty"`
	To         string     `json:"to"         jsonschema:"description=The recipient of the emails to list, default is empty"`
	Subject    string     `json:"subject"    jsonschema:"description=The text to search for in the subject of the emails, default is empty"`
	Body       string     `json:"body"       jsonschema:"description=The text to search for in the body of the emails, default is empty"`
	Label      string     `json:"label"      jsonschema:"description=The label of the emails to list, default is empty"`
}

type SearchEmailsArguments struct {
	EmailAccount string     `json:"email_account" jsonschema:"required,description=The email account to list emails from"`
	Query        EmailQuery `json:"query"      jsonschema:"description=The query to list emails, default is 'in:inbox'"`
	PageToken    string     `json:"page_token" jsonschema:"description=The page token to list, default is empty"`
	Limit        int        `json:"limit"      jsonschema:"required,description=The number of emails to list, minimum 10, maximum 50"`
}

type SendEmailArguments struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to send the email from"`
	To           string `json:"to"  jsonschema:"required,description=The email address to send the email to"`
	Subject      string `json:"subject" jsonschema:"required,description=The subject of the email"`
	Body         string `json:"body"    jsonschema:"required,description=The body of the email"`
}

type EmailByIdArguments struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to get the email from"`
	Id           string `json:"id" jsonschema:"required,description=The id of the email"`
}

func (q *EmailQuery) ToQuery() (string, error) {
	query := "in:inbox"
	if q.In != "" {
		query = "in:" + q.In
	}

	start, end, err := q.TimeFilter.ToUnixRange(time.Now())
	if err != nil {
		fmt.Println("Error converting time filter to unix range:", err)
		return "", err
	}

	if start != 0 {
		query += fmt.Sprintf(" after:%d", start)
	}

	if end != 0 {
		query += fmt.Sprintf(" before:%d", end)
	}

	if q.From != "" {
		query += fmt.Sprintf(" from:%s", q.From)
	}
	if q.To != "" {
		query += fmt.Sprintf(" to:%s", q.To)
	}
	if q.Subject != "" {
		query += fmt.Sprintf(" subject:%s", q.Subject)
	}
	if q.Body != "" {
		query += fmt.Sprintf(" \"%s\"", q.Body)
	}
	if q.Label != "" {
		query += fmt.Sprintf(" label:%s", q.Label)
	}
	return query, nil
}

func processSearchEmails(
	ctx context.Context,
	store *db.Store,
	arguments SearchEmailsArguments,
) ([]*mcp_golang.Content, error) {
	accessToken, err := GetAccessToken(ctx, store, arguments.EmailAccount)
	if err != nil {
		return nil, err
	}

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

	maxResults := 10
	if arguments.Limit > maxResults {
		maxResults = arguments.Limit
	}

	if maxResults <= 0 {
		// default limit
		maxResults = 10
	}

	query, err := arguments.Query.ToQuery()
	if err != nil {
		fmt.Println("Error converting query to string:", err)
		return nil, err
	}

	request := gmailService.Users.Messages.List("me").Q(query).MaxResults(int64(maxResults))
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

func processSendEmail(
	ctx context.Context,
	store *db.Store,
	arguments SendEmailArguments,
) ([]*mcp_golang.Content, error) {
	accessToken, err := GetAccessToken(ctx, store, arguments.EmailAccount)
	if err != nil {
		return nil, err
	}

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

func processEmailById(
	ctx context.Context,
	store *db.Store,
	arguments EmailByIdArguments,
) ([]*mcp_golang.Content, error) {
	accessToken, err := GetAccessToken(ctx, store, arguments.EmailAccount)
	if err != nil {
		return nil, err
	}

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

	if arguments.Id == "" {
		return nil, errors.New("id is required")
	}

	msg, err := gmailService.Users.Messages.Get("me", arguments.Id).Do()
	if err != nil {
		fmt.Println("Error getting message details:", err)
		return nil, err
	}

	body, err := getBody(msg.Payload)
	if err != nil {
		fmt.Println("Error getting body:", err)
		return nil, err
	}

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

	formattedText := fmt.Sprintf("From: %s\nSubject: %s\nDate: %s\nID: %s\nBody: %s",
		from, subject, date, msg.Id, body)

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: formattedText,
			},
		},
	}, nil
}

func processListEmailAccounts(
	ctx context.Context,
	store *db.Store,
) ([]*mcp_golang.Content, error) {
	oauthTokens, err := store.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return nil, err
	}

	emailAccounts := make([]string, 0)
	for _, oauthToken := range oauthTokens {
		emailAccounts = append(emailAccounts, oauthToken.Username)
	}

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: "Email accounts: " + strings.Join(emailAccounts, ", "),
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

func getBody(p *gmail.MessagePart) (string, error) {
	if p == nil {
		return "", errors.New("empty payload")
	}

	if p.Body != nil && len(p.Body.Data) > 0 &&
		(p.MimeType == "text/plain") {
		data, err := base64.URLEncoding.DecodeString(p.Body.Data) // URL-safe base64
		if err != nil {
			return "", err
		}
		return string(data), nil
	} else if p.Body != nil && len(p.Body.Data) > 0 && p.MimeType == "text/html" {
		data, err := base64.URLEncoding.DecodeString(p.Body.Data) // URL-safe base64
		if err != nil {
			return "", err
		}
		html, _ := html2text.FromString(string(data), html2text.Options{OmitLinks: true, TextOnly: true})
		return html, nil
	}

	for _, part := range p.Parts { // recurse into multipart/*
		if body, _ := getBody(part); body != "" {
			return body, nil
		}
	}

	return "", errors.New("no body found")
}

func GenerateGmailTools() ([]mcp_golang.ToolRetType, error) {
	var tools []mcp_golang.ToolRetType

	searchEmailsSchema, err := helpers.ConverToInputSchema(SearchEmailsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for search_emails: %w", err)
	}
	desc := SEARCH_EMAILS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        SEARCH_EMAILS_TOOL_NAME,
		Description: &desc,
		InputSchema: searchEmailsSchema,
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

	emailByIdSchema, err := helpers.ConverToInputSchema(EmailByIdArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for email_by_id: %w", err)
	}
	desc = EMAIL_BY_ID_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        EMAIL_BY_ID_TOOL_NAME,
		Description: &desc,
		InputSchema: emailByIdSchema,
	})

	desc = LIST_EMAIL_ACCOUNTS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        LIST_EMAIL_ACCOUNTS_TOOL_NAME,
		Description: &desc,
		InputSchema: "{}",
	})

	return tools, nil
}
