package google

import (
	"context"
	"fmt"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

const (
	LIST_CALENDAR_EVENTS_TOOL_NAME  = "list_calendar_events"
	CREATE_CALENDAR_EVENT_TOOL_NAME = "create_calendar_event"
)

const (
	LIST_CALENDAR_EVENTS_TOOL_DESCRIPTION  = "List events from a specified Google Calendar within a time range."
	CREATE_CALENDAR_EVENT_TOOL_DESCRIPTION = "Create a new event in a specified Google Calendar."
)

type ListEventsArguments struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to list events from"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'. Use 'primary' for the primary calendar of the authenticated user."`
	TimeMin    string `json:"time_min,omitempty"    jsonschema:"description=Start time (RFC3339 format) for the query. Example: '2024-01-01T00:00:00Z'"`
	TimeMax    string `json:"time_max,omitempty"    jsonschema:"description=End time (RFC3339 format) for the query. Example: '2024-01-02T00:00:00Z'"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=Maximum number of events returned on one result page. Default is 10, minimum 10, maximum 50."`
	PageToken  string `json:"page_token,omitempty"  jsonschema:"description=Token specifying which result page to return."`
	Query      string `json:"q,omitempty"           jsonschema:"description=Free text search query terms to find events that match these terms in any field, except for extended properties."`
}

type CreateEventArgs struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to create events from"`
	CalendarID  string   `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'."`
	Summary     string   `json:"summary"               jsonschema:"required,description=Title of the event."`
	Description string   `json:"description,omitempty" jsonschema:"description=Description of the event."`
	StartTime   string   `json:"start_time"            jsonschema:"required,description=The start time of the event (RFC3339 format). Example: '2024-01-01T10:00:00Z'"`
	EndTime     string   `json:"end_time"              jsonschema:"required,description=The end time of the event (RFC3339 format). Example: '2024-01-01T11:00:00Z'"`
	Attendees   []string `json:"attendees,omitempty"   jsonschema:"description=List of email addresses of attendees."`
	Location    string   `json:"location,omitempty"    jsonschema:"description=Geographic location of the event as free-form text."`
}

func processListEvents(
	ctx context.Context,
	store *db.Store,
	args ListEventsArguments,
) ([]*mcp_golang.Content, error) {
	accessToken, err := GetAccessToken(ctx, store, args.EmailAccount)
	if err != nil {
		return nil, err
	}

	calendarService, err := getCalendarService(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error initializing Calendar service: %w", err)
	}

	calendarID := args.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	eventsCall := calendarService.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true). // Expand recurring events
		MaxResults(int64(maxResults)).
		OrderBy("startTime")

	if args.TimeMin != "" {
		eventsCall = eventsCall.TimeMin(args.TimeMin)
	} else {
		// Default to start of today if not specified
		eventsCall = eventsCall.TimeMin(time.Now().Format(time.RFC3339))
	}

	if args.TimeMax != "" {
		eventsCall = eventsCall.TimeMax(args.TimeMax)
	}
	if args.PageToken != "" {
		eventsCall = eventsCall.PageToken(args.PageToken)
	}
	if args.Query != "" {
		eventsCall = eventsCall.Q(args.Query)
	}

	events, err := eventsCall.Do()
	if err != nil {
		fmt.Println("Error listing calendar events", err)
		return nil, fmt.Errorf("error listing calendar events: %w", err)
	}

	contents := []*mcp_golang.Content{}

	for _, event := range events.Items {
		sourceTitle := "Unknown"
		if event.Source != nil && event.Source.Title != "" {
			sourceTitle = event.Source.Title
		}

		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf(
					"Event: %s - %s, Start: %s, End: %s",
					sourceTitle,
					event.Summary,
					event.Start.DateTime,
					event.End.DateTime,
				),
			},
		})
	}

	return contents, nil
}

func processCreateEvent(
	ctx context.Context,
	store *db.Store,
	args CreateEventArgs,
) ([]*mcp_golang.Content, error) {
	accessToken, err := GetAccessToken(ctx, store, args.EmailAccount)
	if err != nil {
		return nil, err
	}

	calendarService, err := getCalendarService(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error initializing Calendar service: %w", err)
	}

	calendarID := args.CalendarID
	if calendarID == "" {
		calendarID = "primary"
	}

	if args.Summary == "" || args.StartTime == "" || args.EndTime == "" {
		return nil, fmt.Errorf("summary, start time, and end time are required")
	}

	_, err = time.Parse(time.RFC3339, args.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time format, requires RFC3339: %w", err)
	}
	_, err = time.Parse(time.RFC3339, args.EndTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end time format, requires RFC3339: %w", err)
	}

	event := &calendar.Event{
		Summary:     args.Summary,
		Location:    args.Location,
		Description: args.Description,
		Start: &calendar.EventDateTime{
			DateTime: args.StartTime,
		},
		End: &calendar.EventDateTime{
			DateTime: args.EndTime,
		},
	}

	if len(args.Attendees) > 0 {
		attendees := make([]*calendar.EventAttendee, len(args.Attendees))
		for i, email := range args.Attendees {
			attendees[i] = &calendar.EventAttendee{Email: email}
		}
		event.Attendees = attendees
	}

	createdEvent, err := calendarService.Events.Insert(calendarID, event).Do()
	if err != nil {
		fmt.Println("Error creating calendar event", err)
		return nil, fmt.Errorf("error creating calendar event: %w", err)
	}

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf(
					"Successfully created event: %s (ID: %s)",
					createdEvent.Summary,
					createdEvent.Id,
				),
			},
		},
	}, nil
}

func getCalendarService(ctx context.Context, accessToken string) (*calendar.Service, error) {
	token := &oauth2.Token{
		AccessToken: accessToken,
	}
	config := oauth2.Config{} // No need for client ID/secret here, just using the token
	client := config.Client(ctx, token)

	calendarService, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("error initializing Calendar service: %w", err)
	}
	return calendarService, nil
}

func GenerateGoogleCalendarTools() ([]mcp_golang.ToolRetType, error) {
	var tools []mcp_golang.ToolRetType

	listEventsSchema, err := helpers.ConverToInputSchema(ListEventsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_calendar_events: %w", err)
	}
	desc := LIST_CALENDAR_EVENTS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        LIST_CALENDAR_EVENTS_TOOL_NAME,
		Description: &desc,
		InputSchema: listEventsSchema,
	})

	createEventSchema, err := helpers.ConverToInputSchema(CreateEventArgs{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for create_calendar_event: %w", err)
	}
	desc = CREATE_CALENDAR_EVENT_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.ToolRetType{
		Name:        CREATE_CALENDAR_EVENT_TOOL_NAME,
		Description: &desc,
		InputSchema: createEventSchema,
	})

	return tools, nil
}
