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

	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
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
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results to return,default=10,minimum=10,maximum=50"`
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

// Process Functions (Implementations to follow using ScreenpipeClient).
func processSearchContent(
	ctx context.Context,
	client *ScreenpipeClient,
	arguments SearchContentArguments,
) ([]mcp_golang.Content, error) {
	if arguments.Limit < 10 {
		arguments.Limit = 10
	}
	if arguments.Limit > 50 {
		arguments.Limit = 50
	}
	resp, err := client.SearchContent(ctx, arguments)
	content := []mcp_golang.Content{}
	if err != nil {
		textContent := mcp_golang.NewTextContent(fmt.Sprintf("failed to search screenpipe: %v", err))
		content = append(content, textContent)
		return content, nil
	}

	if len(resp.Data) == 0 {
		textContent := mcp_golang.NewTextContent("no results found")
		return []mcp_golang.Content{textContent}, nil
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

	textContent := mcp_golang.NewTextContent("Search Results:\n\n" + strings.Join(formattedResults, "\n"))
	return []mcp_golang.Content{textContent}, nil
}

func GetInputSchema(args any) map[string]any {
	inputSchema, err := utils.ConverToInputSchema(args)
	if err != nil {
		return nil
	}

	return inputSchema
}

func GetScreenpipeTools(client *ScreenpipeClient, isMacOS bool) (*mcp_golang.ListToolsResult, error) {
	searchContentDesc := SearchContentToolDescription

	tools := []mcp_golang.Tool{
		{
			Name:        SearchContentToolName,
			Description: searchContentDesc,
			InputSchema: mcp_golang.ToolInputSchema{
				Type:       "object",
				Properties: GetInputSchema(SearchContentArguments{}),
			},
		},
	}

	return &mcp_golang.ListToolsResult{
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
