package screenpipe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mcp_golang "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

const (
	// Tool Names.
	SearchContentToolName   = "search_content"
	PixelControlToolName    = "pixel_control"
	FindElementsToolName    = "find_elements"
	ClickElementToolName    = "click_element"
	FillElementToolName     = "fill_element"
	ScrollElementToolName   = "scroll_element"
	OpenApplicationToolName = "open_application"
	OpenURLToolName         = "open_url"

	// Tool Descriptions.
	SearchContentToolDescription   = "Search through screenpipe recorded content (OCR text, audio transcriptions, UI elements). Use this to find specific content that has appeared on your screen or been spoken. Results include timestamps, app context, and the content itself."
	PixelControlToolDescription    = "Control mouse and keyboard at the pixel level. This is a cross-platform tool that works on all operating systems. Use this to type text, press keys, move the mouse, and click buttons."
	FindElementsToolDescription    = "Find UI elements with a specific role in an application. MacOS specific roles: 'AXButton', 'AXTextField', etc. Use MacOS Accessibility Inspector app to identify exact roles."
	ClickElementToolDescription    = "Click an element in an application using its id (MacOS only)"
	FillElementToolDescription     = "Type text into an element in an application (MacOS only)"
	ScrollElementToolDescription   = "Scroll an element in a specific direction (MacOS only)"
	OpenApplicationToolDescription = "Open an application by name"
	OpenURLToolDescription         = "Open a URL in a browser"
)

// Argument Structs

type SearchContentArguments struct {
	Query       string `json:"q" jsonschema:"description=Search query to find in recorded content"`
	ContentType string `json:"content_type,omitempty" jsonschema:"enum=all|ocr|audio|ui,description=Type of content to search: 'ocr' for screen text, 'audio' for spoken words, 'ui' for UI elements, or 'all' for everything,default=all"`
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results to return,default=10"`
	Offset      int    `json:"offset,omitempty" jsonschema:"description=Number of results to skip (for pagination),default=0"`
	StartTime   string `json:"start_time,omitempty" jsonschema:"format=date-time,description=Start time in ISO format UTC (e.g. 2024-01-01T00:00:00Z). Filter results from this time onward."`
	EndTime     string `json:"end_time,omitempty" jsonschema:"format=date-time,description=End time in ISO format UTC (e.g. 2024-01-01T00:00:00Z). Filter results up to this time."`
	AppName     string `json:"app_name,omitempty" jsonschema:"description=Filter by application name (e.g. 'Chrome', 'Safari', 'Terminal')"`
	WindowName  string `json:"window_name,omitempty" jsonschema:"description=Filter by window name or title"`
	MinLength   int    `json:"min_length,omitempty" jsonschema:"description=Minimum content length in characters"`
	MaxLength   int    `json:"max_length,omitempty" jsonschema:"description=Maximum content length in characters"`
}

type PixelControlData struct {
	// For WriteText and KeyPress
	Text string `json:"text,omitempty"`
	// For MouseMove
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
	// For MouseClick
	Button string `json:"button,omitempty" jsonschema:"enum=left|right|middle"`
}

type PixelControlArguments struct {
	ActionType string           `json:"action_type" jsonschema:"required,enum=WriteText|KeyPress|MouseMove|MouseClick,description=Type of input action to perform"`
	Data       PixelControlData `json:"data" jsonschema:"required,description=Action-specific data"` // Note: oneOf is tricky in Go struct tags directly, usually handled by custom unmarshalling or using interface{}
}

type FindElementsArguments struct {
	App               string `json:"app" jsonschema:"required,description=The name of the application (e.g., 'Chrome', 'Finder', 'Terminal')"`
	Window            string `json:"window,omitempty" jsonschema:"description=The window name or title (optional)"`
	Role              string `json:"role" jsonschema:"required,description=The role to search for (e.g., 'button', 'textfield', 'AXButton', 'AXTextField'). For best results, use MacOS AX prefixed roles."`
	MaxResults        int    `json:"max_results,omitempty" jsonschema:"description=Maximum number of elements to return,default=10"`
	MaxDepth          *int   `json:"max_depth,omitempty" jsonschema:"description=Maximum depth of element tree to search"`
	UseBackgroundApps *bool  `json:"use_background_apps,omitempty" jsonschema:"description=Whether to look in background apps,default=true"`
	ActivateApp       *bool  `json:"activate_app,omitempty" jsonschema:"description=Whether to activate the app before searching,default=true"`
}

type ClickElementArguments struct {
	App               string `json:"app" jsonschema:"required,description=The name of the application"`
	Window            string `json:"window,omitempty" jsonschema:"description=The window name (optional)"`
	ID                string `json:"id" jsonschema:"required,description=The id of the element to click"`
	UseBackgroundApps *bool  `json:"use_background_apps,omitempty" jsonschema:"description=Whether to look in background apps,default=true"`
	ActivateApp       *bool  `json:"activate_app,omitempty" jsonschema:"description=Whether to activate the app before clicking,default=true"`
}

type FillElementArguments struct {
	App               string `json:"app" jsonschema:"required,description=The name of the application"`
	Window            string `json:"window,omitempty" jsonschema:"description=The window name (optional)"`
	ID                string `json:"id" jsonschema:"required,description=The id of the element to fill"`
	Text              string `json:"text" jsonschema:"required,description=The text to type into the element"`
	UseBackgroundApps *bool  `json:"use_background_apps,omitempty" jsonschema:"description=Whether to look in background apps,default=true"`
	ActivateApp       *bool  `json:"activate_app,omitempty" jsonschema:"description=Whether to activate the app before typing,default=true"`
}

type ScrollElementArguments struct {
	App               string `json:"app" jsonschema:"required,description=The name of the application"`
	Window            string `json:"window,omitempty" jsonschema:"description=The window name (optional)"`
	ID                string `json:"id" jsonschema:"required,description=The id of the element to scroll"`
	Direction         string `json:"direction" jsonschema:"required,enum=up|down|left|right,description=The direction to scroll"`
	Amount            int    `json:"amount" jsonschema:"required,description=The amount to scroll in pixels"`
	UseBackgroundApps *bool  `json:"use_background_apps,omitempty" jsonschema:"description=Whether to look in background apps,default=true"`
	ActivateApp       *bool  `json:"activate_app,omitempty" jsonschema:"description=Whether to activate the app before scrolling,default=true"`
}

type OpenApplicationArguments struct {
	AppName string `json:"app_name" jsonschema:"required,description=The name of the application to open"`
}

type OpenURLArguments struct {
	URL     string `json:"url" jsonschema:"required,description=The URL to open"`
	Browser string `json:"browser,omitempty" jsonschema:"description=The browser to use (optional)"`
}

// Process Functions (Implementations to follow using ScreenpipeClient)

func processSearchContent(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments SearchContentArguments,
) ([]*mcp_golang.Content, error) {
	if arguments.Limit < 10 || arguments.Limit > 100 {
		arguments.Limit = 10
	}
	resp, err := client.SearchContent(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to search screenpipe: %v", err)},
		}}, nil
	}

	if len(resp.Data) == 0 {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: "no results found"},
		}}, nil
	}

	var formattedResults []string
	for _, result := range resp.Data {
		var textOutput string
		content := result.Content
		switch result.Type {
		case "OCR":
			textOutput = fmt.Sprintf(
				"OCR Text: %s\nApp: %s\nWindow: %s\nTime: %s\n---",
				content.Text, content.AppName, content.WindowName, content.Timestamp,
			)
		case "Audio":
			textOutput = fmt.Sprintf(
				"Audio Transcription: %s\nDevice: %s\nTime: %s\n---",
				content.Transcription, content.DeviceName, content.Timestamp,
			)
		case "UI":
			textOutput = fmt.Sprintf(
				"UI Text: %s\nApp: %s\nWindow: %s\nTime: %s\n---",
				content.Text, content.AppName, content.WindowName, content.Timestamp,
			)
		default:
			continue // Skip unknown types
		}
		formattedResults = append(formattedResults, textOutput)
	}

	fmt.Println("Search content response:", formattedResults)

	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: "Search Results:\n\n" + strings.Join(formattedResults, "\n")},
	}}, nil
}

func processPixelControl(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments PixelControlArguments,
) ([]*mcp_golang.Content, error) {
	_, err := client.PixelControl(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to perform input control: %v", err)},
		}}, nil
	}

	var resultText string
	actionType := arguments.ActionType
	actionData := arguments.Data

	switch actionType {
	case "WriteText":
		resultText = fmt.Sprintf("successfully typed text: '%s'", actionData.Text)
	case "KeyPress":
		resultText = fmt.Sprintf("successfully pressed key: '%s'", actionData.Text)
	case "MouseMove":
		if actionData.X != nil && actionData.Y != nil {
			resultText = fmt.Sprintf("successfully moved mouse to coordinates: x=%d, y=%d", *actionData.X, *actionData.Y)
		} else {
			resultText = "successfully performed mouse move (coordinates not provided in standard way)" // Should not happen if schema is followed
		}
	case "MouseClick":
		resultText = fmt.Sprintf("successfully clicked %s mouse button", actionData.Button)
	default:
		resultText = "successfully performed input control action"
	}

	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: resultText},
	}}, nil
}

func processFindElements(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments FindElementsArguments,
	isMacOS bool,
) ([]*mcp_golang.Content, error) {
	if !isMacOS {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("the '%s' tool is only available on MacOS.", FindElementsToolName)},
		}}, nil
	}
	resp, err := client.FindElements(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to find elements: %v", err)},
		}}, nil
	}
	if !resp.Success {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to find elements: %s", resp.Error)},
		}}, nil
	}
	if len(resp.Data) == 0 {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("no elements found matching role '%s' in app '%s'", arguments.Role, arguments.App)},
		}}, nil
	}

	var resultLines []string
	resultLines = append(resultLines, fmt.Sprintf("found %d elements matching role '%s' in app '%s':\n", len(resp.Data), arguments.Role, arguments.App))

	for i, element := range resp.Data {
		elementInfo := fmt.Sprintf(
			"Element %d:\nID: %s\nRole: %s\nText: %s\nDescription: %s\n---",
			i+1, element.ID, element.Role, element.Text, element.Description,
		)
		resultLines = append(resultLines, elementInfo)
	}

	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: strings.Join(resultLines, "\n")},
	}}, nil
}

func processClickElement(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments ClickElementArguments,
	isMacOS bool,
) ([]*mcp_golang.Content, error) {
	if !isMacOS {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("the '%s' tool is only available on MacOS.", ClickElementToolName)},
		}}, nil
	}
	resp, err := client.ClickElement(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to click element: %v", err)},
		}}, nil
	}
	if !resp.Success { // Redundant if client method guarantees success or error
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to click element: %s", resp.Error)},
		}}, nil
	}
	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("successfully clicked element using %s. %s", resp.Result.Method, resp.Result.Details)},
	}}, nil
}

func processFillElement(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments FillElementArguments,
	isMacOS bool,
) ([]*mcp_golang.Content, error) {
	if !isMacOS {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("the '%s' tool is only available on MacOS.", FillElementToolName)},
		}}, nil
	}
	_, err := client.FillElement(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to fill element: %v", err)},
		}}, nil
	}

	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: "successfully filled element with text"},
	}}, nil
}

func processScrollElement(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments ScrollElementArguments,
	isMacOS bool,
) ([]*mcp_golang.Content, error) {
	if !isMacOS {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("the '%s' tool is only available on MacOS.", ScrollElementToolName)},
		}}, nil
	}

	// TODO: Implement client.ScrollElement call and response handling

	fmt.Printf("Processing ScrollElement with args: %+v (Not implemented yet)\n", arguments)
	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: "ScrollElement success placeholder (Not implemented)."},
	}}, nil
}

func processOpenApplication(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments OpenApplicationArguments,
	isMacOS bool,
) ([]*mcp_golang.Content, error) {
	if !isMacOS {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("the '%s' tool for direct operation is primarily for MacOS. Attempting on non-MacOS may have different behavior or be a no-op.", OpenApplicationToolName)},
		}}, nil
	}

	_, err := client.OpenApplication(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to open application: %v", err)},
		}}, nil
	}
	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("successfully opened application '%s'", arguments.AppName)},
	}}, nil
}

func processOpenURL(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments OpenURLArguments,
) ([]*mcp_golang.Content, error) {
	_, err := client.OpenURL(ctx, arguments)
	if err != nil {
		return []*mcp_golang.Content{{
			Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("failed to open url: %v", err)},
		}}, nil
	}
	browserInfo := ""
	if arguments.Browser != "" {
		browserInfo = fmt.Sprintf(" using %s", arguments.Browser)
	}
	return []*mcp_golang.Content{{
		Type: "text", TextContent: &mcp_golang.TextContent{Text: fmt.Sprintf("successfully opened url '%s'%s", arguments.URL, browserInfo)},
	}}, nil
}

func GetInputSchema(args any) map[string]any {
	inputSchema, err := helpers.ConverToInputSchema(args)
	if err != nil {
		return nil
	}

	return inputSchema
}

func GetScreenpipeTools(client *ScreenpipeClient, isMacOS bool) (*mcp_golang.ToolsResponse, error) {
	searchContentDesc := SearchContentToolDescription
	pixelControlDesc := PixelControlToolDescription
	findElementsDesc := FindElementsToolDescription
	clickElementDesc := ClickElementToolDescription
	fillElementDesc := FillElementToolDescription
	scrollElementDesc := ScrollElementToolDescription
	openApplicationDesc := OpenApplicationToolDescription
	openURLDesc := OpenURLToolDescription

	tools := []mcp_golang.ToolRetType{
		{
			Name:        SearchContentToolName,
			Description: &searchContentDesc,
			InputSchema: GetInputSchema(SearchContentArguments{}),
		},
		{
			Name:        PixelControlToolName,
			Description: &pixelControlDesc,
			InputSchema: GetInputSchema(PixelControlArguments{}),
		},
		{
			Name:        FindElementsToolName,
			Description: &findElementsDesc,
			InputSchema: GetInputSchema(FindElementsArguments{}),
		},
		{
			Name:        ClickElementToolName,
			Description: &clickElementDesc,
			InputSchema: GetInputSchema(ClickElementArguments{}),
		},
		{
			Name:        FillElementToolName,
			Description: &fillElementDesc,
			InputSchema: GetInputSchema(FillElementArguments{}),
		},
		{
			Name:        ScrollElementToolName,
			Description: &scrollElementDesc,
			InputSchema: GetInputSchema(ScrollElementArguments{}),
		},
		{
			Name:        OpenApplicationToolName,
			Description: &openApplicationDesc,
			InputSchema: GetInputSchema(OpenApplicationArguments{}),
		},
		{
			Name:        OpenURLToolName,
			Description: &openURLDesc,
			InputSchema: GetInputSchema(OpenURLArguments{}),
		},
	}

	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

// --- Common API Structures ---

// ScreenpipeBaseResponse is a common structure for simple success/failure responses.
type ScreenpipeBaseResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ScreenpipeAPIError represents an error returned from the Screenpipe API.
type ScreenpipeAPIError struct {
	Message    string
	StatusCode int
	Body       string
}

func (e *ScreenpipeAPIError) Error() string {
	return fmt.Sprintf("Screenpipe API error: %s (status: %d, body: %s)", e.Message, e.StatusCode, e.Body)
}

func (c *ScreenpipeClient) makeRequest(
	ctx context.Context,
	method string,
	endpoint string,
	queryParams url.Values,
	requestBody interface{},
	responseDest interface{},
) error {
	fullURL := c.apiBaseURL + endpoint
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	var reqBodyReader io.Reader
	if requestBody != nil {
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var baseResp ScreenpipeBaseResponse
		if json.Unmarshal(bodyBytes, &baseResp) == nil && baseResp.Error != "" {
			return &ScreenpipeAPIError{Message: baseResp.Error, StatusCode: resp.StatusCode, Body: string(bodyBytes)}
		}
		return &ScreenpipeAPIError{Message: fmt.Sprintf("request failed with status %d", resp.StatusCode), StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	if responseDest != nil {
		if err := json.Unmarshal(bodyBytes, responseDest); err != nil {
			// If the primary unmarshal fails, try to unmarshal into ScreenpipeBaseResponse
			// to see if it's a success:false with an error message that wasn't caught by status code.
			var baseResp ScreenpipeBaseResponse
			if json.Unmarshal(bodyBytes, &baseResp) == nil && !baseResp.Success && baseResp.Error != "" {
				return fmt.Errorf("operation failed: %s. Body: %s", baseResp.Error, string(bodyBytes))
			}
			return fmt.Errorf("failed to unmarshal response body into target: %w. Body: %s", err, string(bodyBytes))
		}
	}

	err = resp.Body.Close()
	if err != nil {
		fmt.Println("failed to close response body: %w", err)
	}

	return nil
}

// --- API Specific Structs ---

// SearchContent.
type ScreenpipeSearchResultContent struct {
	Text          string `json:"text,omitempty"`
	AppName       string `json:"app_name,omitempty"`
	WindowName    string `json:"window_name,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Transcription string `json:"transcription,omitempty"`
	DeviceName    string `json:"device_name,omitempty"`
}

type ScreenpipeSearchResultItem struct {
	Type    string                        `json:"type"` // "OCR", "Audio", "UI"
	Content ScreenpipeSearchResultContent `json:"content"`
}

type ScreenpipeSearchResponse struct {
	Data []ScreenpipeSearchResultItem `json:"data"`
}

// PixelControl.
type PixelControlAction struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
type PixelControlRequest struct {
	Action PixelControlAction `json:"action"`
}

// ElementSelector (common for click, fill, find).
type ElementSelector struct {
	AppName           string `json:"app_name"`
	WindowName        string `json:"window_name,omitempty"`
	Locator           string `json:"locator"`
	UseBackgroundApps *bool  `json:"use_background_apps,omitempty"`
	ActivateApp       *bool  `json:"activate_app,omitempty"`
}

// ClickElement.
type ClickElementRequest struct {
	Selector ElementSelector `json:"selector"`
}
type ClickElementResult struct {
	Method  string `json:"method"`
	Details string `json:"details"`
}
type ClickElementResponse struct {
	ScreenpipeBaseResponse
	Result ClickElementResult `json:"result,omitempty"`
}

// FillElement.
type FillElementRequest struct {
	Selector ElementSelector `json:"selector"`
	Text     string          `json:"text"`
}

// FindElements.
type FindElementsRequest struct {
	Selector   ElementSelector `json:"selector"`
	MaxResults int             `json:"max_results,omitempty"`
	MaxDepth   *int            `json:"max_depth,omitempty"`
}
type ScreenpipeUIElement struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Text        string `json:"text"`
	Description string `json:"description"`
}
type FindElementsResponse struct {
	ScreenpipeBaseResponse
	Data []ScreenpipeUIElement `json:"data,omitempty"`
}

// OpenApplication.
type OpenApplicationRequest struct {
	AppName string `json:"app_name"`
}

// OpenURL.
type OpenURLRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// --- Client Methods ---

func (c *ScreenpipeClient) SearchContent(ctx context.Context, args SearchContentArguments) (*ScreenpipeSearchResponse, error) {
	params := url.Values{}
	if args.Query != "" {
		params.Set("q", args.Query)
	}
	if args.ContentType != "" && args.ContentType != "all" {
		params.Set("content_type", args.ContentType)
	}
	if args.Limit > 0 {
		params.Set("limit", strconv.Itoa(args.Limit))
	}
	if args.Offset > 0 {
		params.Set("offset", strconv.Itoa(args.Offset))
	}
	if args.StartTime != "" {
		params.Set("start_time", args.StartTime)
	}
	if args.EndTime != "" {
		params.Set("end_time", args.EndTime)
	}
	if args.AppName != "" {
		params.Set("app_name", args.AppName)
	}
	if args.WindowName != "" {
		params.Set("window_name", args.WindowName)
	}
	if args.MinLength > 0 {
		params.Set("min_length", strconv.Itoa(args.MinLength))
	}
	if args.MaxLength > 0 {
		params.Set("max_length", strconv.Itoa(args.MaxLength))
	}

	var resp ScreenpipeSearchResponse
	err := c.makeRequest(ctx, http.MethodGet, "/search", params, nil, &resp)
	if err != nil {
		if _, ok := err.(*json.SyntaxError); ok {
			return nil, fmt.Errorf("failed to parse JSON response from /search: %w", err)
		}
		return nil, fmt.Errorf("failed to search screenpipe (/search): %w", err)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) PixelControl(ctx context.Context, args PixelControlArguments) (*ScreenpipeBaseResponse, error) {
	payload := PixelControlRequest{
		Action: PixelControlAction{
			Type: args.ActionType,
			Data: args.Data,
		},
	}
	var resp ScreenpipeBaseResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator/pixel", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("pixel control API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to perform input control: %s", resp.Error)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) ClickElement(ctx context.Context, args ClickElementArguments) (*ClickElementResponse, error) {
	selector := ElementSelector{
		AppName:           args.App,
		WindowName:        args.Window,
		Locator:           "#" + args.ID,
		UseBackgroundApps: args.UseBackgroundApps,
		ActivateApp:       args.ActivateApp,
	}

	payload := ClickElementRequest{Selector: selector}
	var resp ClickElementResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator/click", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("click element API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to click element: %s", resp.Error)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) FillElement(ctx context.Context, args FillElementArguments) (*ScreenpipeBaseResponse, error) {
	selector := ElementSelector{
		AppName:           args.App,
		WindowName:        args.Window,
		Locator:           "#" + args.ID,
		UseBackgroundApps: args.UseBackgroundApps,
		ActivateApp:       args.ActivateApp,
	}

	payload := FillElementRequest{
		Selector: selector,
		Text:     args.Text,
	}
	var resp ScreenpipeBaseResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator/type", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("fill element API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to fill element: %s", resp.Error)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) FindElements(ctx context.Context, args FindElementsArguments) (*FindElementsResponse, error) {
	selector := ElementSelector{
		AppName:           args.App,
		WindowName:        args.Window,
		Locator:           args.Role,
		UseBackgroundApps: args.UseBackgroundApps,
		ActivateApp:       args.ActivateApp,
	}

	payload := FindElementsRequest{
		Selector:   selector,
		MaxResults: args.MaxResults,
		MaxDepth:   args.MaxDepth,
	}

	var resp FindElementsResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("find elements API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to find elements: %s", resp.Error)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) OpenApplication(ctx context.Context, args OpenApplicationArguments) (*ScreenpipeBaseResponse, error) {
	payload := OpenApplicationRequest(args)
	var resp ScreenpipeBaseResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator/open-application", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("open application API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to open application: %s", resp.Error)
	}
	return &resp, nil
}

func (c *ScreenpipeClient) OpenURL(ctx context.Context, args OpenURLArguments) (*ScreenpipeBaseResponse, error) {
	payload := OpenURLRequest(args)
	var resp ScreenpipeBaseResponse
	err := c.makeRequest(ctx, http.MethodPost, "/experimental/operator/open-url", nil, payload, &resp)
	if err != nil {
		return nil, fmt.Errorf("open URL API call failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to open URL: %s", resp.Error)
	}
	return &resp, nil
}
