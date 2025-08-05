package google

import (
	"context"
	"fmt"
	"time"

	mcp_golang "github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
)

const (
	LIST_CALENDAR_EVENTS_TOOL_NAME  = "list_calendar_events"
	CREATE_CALENDAR_EVENT_TOOL_NAME = "create_calendar_event"
	UPDATE_CALENDAR_EVENT_TOOL_NAME = "update_calendar_event"
	DELETE_CALENDAR_EVENT_TOOL_NAME = "delete_calendar_event"
)

const (
	LIST_CALENDAR_EVENTS_TOOL_DESCRIPTION  = "List events from a specified Google Calendar within a time range. Returns event details including event_id needed for updates/deletes."
	CREATE_CALENDAR_EVENT_TOOL_DESCRIPTION = "Create a new event in a specified Google Calendar and return its event_id. Use the event_id to update the event using the update_calendar_event tool and delete the event using the delete_calendar_event tool."
	UPDATE_CALENDAR_EVENT_TOOL_DESCRIPTION = "Update an existing calendar event. Only provided fields will be updated, others remain unchanged. Requires event_id from list_calendar_events or create_calendar_event."
	DELETE_CALENDAR_EVENT_TOOL_DESCRIPTION = "Delete a calendar event by its ID. Requires event_id from list_calendar_events or create_calendar_event."
)

type EventReminder struct {
	Method  string `json:"method" jsonschema:"required,description=Reminder method: 'email' or 'popup'"`
	Minutes int    `json:"minutes" jsonschema:"required,description=Number of minutes before the event when the reminder should trigger"`
}

type ListEventsArguments struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to list events from"`
	CalendarID   string `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'. Use 'primary' for the primary calendar of the authenticated user."`
	TimeMin      string `json:"time_min,omitempty"    jsonschema:"description=Start time (RFC3339 format) for the query. Example: '2024-01-01T00:00:00Z'"`
	TimeMax      string `json:"time_max,omitempty"    jsonschema:"description=End time (RFC3339 format) for the query. Example: '2024-01-02T00:00:00Z'"`
	MaxResults   int    `json:"max_results,omitempty" jsonschema:"description=Maximum number of events returned on one result page. Default is 10, minimum 10, maximum 50."`
	PageToken    string `json:"page_token,omitempty"  jsonschema:"description=The page token to get the next page of events obtained from the previous list_calendar_events call."`
	Query        string `json:"q,omitempty"           jsonschema:"description=Free text search query terms to find events that match these terms in any field, except for extended properties."`
}

type CreateEventArgs struct {
	EmailAccount string          `json:"email_account" jsonschema:"required,description=The email account to create events from"`
	CalendarID   string          `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'."`
	Summary      string          `json:"summary"               jsonschema:"required,description=Title of the event."`
	Description  string          `json:"description,omitempty" jsonschema:"description=Description of the event."`
	StartTime    string          `json:"start_time"            jsonschema:"required,description=The start time of the event (RFC3339 format). Example: '2024-01-01T10:00:00Z'"`
	EndTime      string          `json:"end_time"              jsonschema:"required,description=The end time of the event (RFC3339 format). Example: '2024-01-01T11:00:00Z'"`
	Attendees    []string        `json:"attendees,omitempty"   jsonschema:"description=List of email addresses of attendees."`
	Location     string          `json:"location,omitempty"    jsonschema:"description=Geographic location of the event as free-form text."`
	Reminders    []EventReminder `json:"reminders,omitempty"   jsonschema:"description=Custom reminders for the event. If empty, uses calendar default reminders. Providing custom reminders will disable defaults. Example: [{'method': 'popup', 'minutes': 10}]"`
}

type UpdateEventArgs struct {
	EmailAccount string          `json:"email_account" jsonschema:"required,description=The email account to update events from"`
	CalendarID   string          `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'."`
	EventId      string          `json:"event_id" jsonschema:"required,description=The ID of the event to update"`
	Summary      string          `json:"summary,omitempty" jsonschema:"description=New title of the event. If empty, keeps current title."`
	Description  string          `json:"description,omitempty" jsonschema:"description=New description of the event. If empty, keeps current description."`
	StartTime    string          `json:"start_time,omitempty" jsonschema:"description=New start time (RFC3339 format). If empty, keeps current start time."`
	EndTime      string          `json:"end_time,omitempty" jsonschema:"description=New end time (RFC3339 format). If empty, keeps current end time."`
	Attendees    []string        `json:"attendees,omitempty" jsonschema:"description=New list of attendee email addresses. If provided (even if empty array), replaces all current attendees. If omitted completely, keeps current attendees."`
	Location     string          `json:"location,omitempty" jsonschema:"description=New location. If empty, keeps current location."`
	Reminders    []EventReminder `json:"reminders,omitempty" jsonschema:"description=New reminders. If empty, keeps current reminders. Providing custom reminders will disable defaults."`
}

type DeleteEventArgs struct {
	EmailAccount string `json:"email_account" jsonschema:"required,description=The email account to delete events from"`
	CalendarID   string `json:"calendar_id,omitempty" jsonschema:"description=Calendar identifier. Default is 'primary'."`
	EventId      string `json:"event_id" jsonschema:"required,description=The ID of the event to delete"`
	SendUpdates  string `json:"send_updates,omitempty" jsonschema:"description=Whether to send cancellation notifications: 'all' (default), 'externalOnly', or 'none'"`
}

func processListEvents(
	ctx context.Context,
	store *db.Store,
	args ListEventsArguments,
) ([]mcp_golang.Content, error) {
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

	if maxResults > 50 {
		maxResults = 50
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
		return nil, fmt.Errorf("error listing calendar events: %w", err)
	}

	contents := []mcp_golang.Content{}

	if events.NextPageToken != "" {
		paginationText := fmt.Sprintf("NextPageToken: %s\n(Use this token in page_token parameter for your next list_calendar_events call to get more events or mention explicitly that you can get more events)\n",
			events.NextPageToken)
		paginationContent := mcp_golang.NewTextContent(paginationText)
		contents = append(contents, paginationContent)
	}

	for _, event := range events.Items {
		// Handle both timed events (DateTime) and all-day events (Date)
		var startTime, endTime string
		if event.Start.DateTime != "" {
			startTime = event.Start.DateTime
		} else {
			startTime = event.Start.Date + " (All-day)"
		}

		if event.End.DateTime != "" {
			endTime = event.End.DateTime
		} else {
			endTime = event.End.Date + " (All-day)"
		}

		eventText := fmt.Sprintf(
			"Event: %s\nStart: %s\nEnd: %s\nEvent ID: %s\nTo modify this event, use 'update_calendar_event' with this Event ID",
			event.Summary,
			startTime,
			endTime,
			event.Id,
		)
		textContent := mcp_golang.NewTextContent(eventText)
		contents = append(contents, textContent)
	}

	return contents, nil
}

func processCreateEvent(
	ctx context.Context,
	store *db.Store,
	args CreateEventArgs,
) ([]mcp_golang.Content, error) {
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

	startTime, err := time.Parse(time.RFC3339, args.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time format, requires RFC3339: %w", err)
	}
	endTime, err := time.Parse(time.RFC3339, args.EndTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end time format, requires RFC3339: %w", err)
	}

	if endTime.Before(startTime) || endTime.Equal(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
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

	if len(args.Reminders) > 0 {
		overrides := make([]*calendar.EventReminder, len(args.Reminders))
		for i, reminder := range args.Reminders {
			if reminder.Method != "email" && reminder.Method != "popup" {
				return nil, fmt.Errorf("invalid reminder method '%s', must be 'email' or 'popup'", reminder.Method)
			}
			if reminder.Minutes < 0 {
				return nil, fmt.Errorf("reminder minutes must be non-negative, got %d", reminder.Minutes)
			}

			overrides[i] = &calendar.EventReminder{
				Method:  reminder.Method,
				Minutes: int64(reminder.Minutes),
			}
		}

		event.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  overrides,
		}
	} else {
		// If no custom reminders, use default reminders.
		event.Reminders = &calendar.EventReminders{
			UseDefault: true,
		}
	}

	createdEvent, err := calendarService.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("error creating calendar event: %w", err)
	}

	var reminderInfo string
	if len(args.Reminders) > 0 {
		reminderInfo = fmt.Sprintf(" with %d custom reminder(s) (default reminders disabled)", len(args.Reminders))
	} else {
		reminderInfo = " with calendar default reminders"
	}

	successText := fmt.Sprintf(
		"Event created: %s\nEvent ID: %s%s\n\nTo modify this event (add attendees, change time, location, etc.), use the 'update_calendar_event' tool with this Event ID.",
		createdEvent.Summary,
		createdEvent.Id,
		reminderInfo,
	)
	textContent := mcp_golang.NewTextContent(successText)
	return []mcp_golang.Content{textContent}, nil
}

func processUpdateEvent(
	ctx context.Context,
	store *db.Store,
	args UpdateEventArgs,
) ([]mcp_golang.Content, error) {
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

	if args.EventId == "" {
		return nil, fmt.Errorf("event_id is required")
	}

	// Get the current event details to preserve existing fields if not being updated.
	currentEvent, err := calendarService.Events.Get(calendarID, args.EventId).Do()
	if err != nil {
		return nil, fmt.Errorf("error getting current event: %w", err)
	}

	// Update only provided fields.
	if args.Summary != "" {
		currentEvent.Summary = args.Summary
	}
	if args.Description != "" {
		currentEvent.Description = args.Description
	}
	if args.Location != "" {
		currentEvent.Location = args.Location
	}

	if args.StartTime != "" {
		startTime, err := time.Parse(time.RFC3339, args.StartTime)
		if err != nil {
			return nil, fmt.Errorf("invalid start time format, requires RFC3339: %w", err)
		}
		currentEvent.Start = &calendar.EventDateTime{DateTime: args.StartTime}

		if args.EndTime != "" {
			endTime, err := time.Parse(time.RFC3339, args.EndTime)
			if err != nil {
				return nil, fmt.Errorf("invalid end time format, requires RFC3339: %w", err)
			}
			if endTime.Before(startTime) || endTime.Equal(startTime) {
				return nil, fmt.Errorf("end time must be after start time")
			}
			currentEvent.End = &calendar.EventDateTime{DateTime: args.EndTime}
		}
	} else if args.EndTime != "" {
		_, err = time.Parse(time.RFC3339, args.EndTime)
		if err != nil {
			return nil, fmt.Errorf("invalid end time format, requires RFC3339: %w", err)
		}
		currentEvent.End = &calendar.EventDateTime{DateTime: args.EndTime}
	}

	if len(args.Attendees) > 0 {
		attendees := make([]*calendar.EventAttendee, len(args.Attendees))
		for i, email := range args.Attendees {
			attendees[i] = &calendar.EventAttendee{Email: email}
		}
		currentEvent.Attendees = attendees
	} else if len(args.Attendees) == 0 && len(currentEvent.Attendees) > 0 {
		// Remove all attendees if empty attendees list provided.
		currentEvent.Attendees = nil
	}

	if len(args.Reminders) > 0 {
		overrides := make([]*calendar.EventReminder, len(args.Reminders))
		for i, reminder := range args.Reminders {
			if reminder.Method != "email" && reminder.Method != "popup" {
				return nil, fmt.Errorf("invalid reminder method '%s', must be 'email' or 'popup'", reminder.Method)
			}
			if reminder.Minutes < 0 {
				return nil, fmt.Errorf("reminder minutes must be non-negative, got %d", reminder.Minutes)
			}

			overrides[i] = &calendar.EventReminder{
				Method:  reminder.Method,
				Minutes: int64(reminder.Minutes),
			}
		}

		currentEvent.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  overrides,
		}
	}

	updatedEvent, err := calendarService.Events.Update(calendarID, args.EventId, currentEvent).Do()
	if err != nil {
		return nil, fmt.Errorf("error updating calendar event: %w", err)
	}

	var updateInfo string
	if len(args.Reminders) > 0 {
		updateInfo = fmt.Sprintf(" - reminders updated to %d custom reminder(s) (default reminders disabled)", len(args.Reminders))
	}

	successText := fmt.Sprintf(
		"Event updated: %s\nEvent ID: %s%s\n\nUse this same Event ID for any future modifications to this event.",
		updatedEvent.Summary,
		updatedEvent.Id,
		updateInfo,
	)
	textContent := mcp_golang.NewTextContent(successText)
	return []mcp_golang.Content{textContent}, nil
}

func processDeleteEvent(
	ctx context.Context,
	store *db.Store,
	args DeleteEventArgs,
) ([]mcp_golang.Content, error) {
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

	if args.EventId == "" {
		return nil, fmt.Errorf("event_id is required")
	}

	// Get the details of the event to be deleted to provide context in the response.
	eventToDelete, err := calendarService.Events.Get(calendarID, args.EventId).Do()
	if err != nil {
		return nil, fmt.Errorf("error getting event to delete: %w", err)
	}

	deleteCall := calendarService.Events.Delete(calendarID, args.EventId)

	sendUpdates := args.SendUpdates
	if sendUpdates == "" {
		sendUpdates = "all"
	}
	if sendUpdates == "all" || sendUpdates == "externalOnly" || sendUpdates == "none" {
		deleteCall = deleteCall.SendUpdates(sendUpdates)
	} else {
		return nil, fmt.Errorf("invalid send_updates value '%s', must be 'all', 'externalOnly', or 'none'", sendUpdates)
	}

	err = deleteCall.Do()
	if err != nil {
		return nil, fmt.Errorf("error deleting calendar event: %w", err)
	}

	successText := fmt.Sprintf(
		"Successfully deleted event: %s (ID: %s)",
		eventToDelete.Summary,
		eventToDelete.Id,
	)
	textContent := mcp_golang.NewTextContent(successText)
	return []mcp_golang.Content{textContent}, nil
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

func GenerateGoogleCalendarTools() ([]mcp_golang.Tool, error) {
	var tools []mcp_golang.Tool

	listEventsSchema, err := utils.ConverToInputSchema(ListEventsArguments{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for list_calendar_events: %w", err)
	}
	desc := LIST_CALENDAR_EVENTS_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.Tool{
		Name:           LIST_CALENDAR_EVENTS_TOOL_NAME,
		Description:    desc,
		RawInputSchema: listEventsSchema,
	})

	createEventSchema, err := utils.ConverToInputSchema(CreateEventArgs{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for create_calendar_event: %w", err)
	}
	desc = CREATE_CALENDAR_EVENT_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.Tool{
		Name:           CREATE_CALENDAR_EVENT_TOOL_NAME,
		Description:    desc,
		RawInputSchema: createEventSchema,
	})

	updateEventSchema, err := utils.ConverToInputSchema(UpdateEventArgs{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for update_calendar_event: %w", err)
	}
	desc = UPDATE_CALENDAR_EVENT_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.Tool{
		Name:           UPDATE_CALENDAR_EVENT_TOOL_NAME,
		Description:    desc,
		RawInputSchema: updateEventSchema,
	})

	deleteEventSchema, err := utils.ConverToInputSchema(DeleteEventArgs{})
	if err != nil {
		return nil, fmt.Errorf("error generating schema for delete_calendar_event: %w", err)
	}
	desc = DELETE_CALENDAR_EVENT_TOOL_DESCRIPTION
	tools = append(tools, mcp_golang.Tool{
		Name:           DELETE_CALENDAR_EVENT_TOOL_NAME,
		Description:    desc,
		RawInputSchema: deleteEventSchema,
	})

	return tools, nil
}
