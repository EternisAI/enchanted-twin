package screenpipe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	mcp_golang "github.com/metoro-io/mcp-golang"
	// "github.com/metoro-io/mcp-golang" // Unused in this file
)

const (
	defaultTimeout = 90 * time.Second
	apiBaseURL   = "http://localhost:3030"
)

// ScreenpipeClient interacts with the Screenpipe API.
type ScreenpipeClient struct {
	httpClient *http.Client
	apiBaseURL string
	// authToken string // If API requires auth token
}

// NewClient creates a new ScreenpipeClient.
// baseURL should be the root URL of the Screenpipe API (e.g., "http://localhost:8000").
func NewClient() *ScreenpipeClient {
	client := &http.Client{Timeout: defaultTimeout}
	return &ScreenpipeClient{
		httpClient: client,
		apiBaseURL: apiBaseURL,
	}
}

func (c *ScreenpipeClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*mcp_golang.ToolsResponse, error) {
	return GetScreenpipeTools(c, false)
}

func (c *ScreenpipeClient) CallTool(
	ctx context.Context,
	name string,
	arguments any,
) (*mcp_golang.ToolResponse, error) {

	fmt.Println("Call tool SCREENPIPE", name, arguments)

	bytes, err := helpers.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}
	var content []*mcp_golang.Content
	switch name {
	case SearchContentToolName:
		arguments := &SearchContentArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}
		content, err = processSearchContent(ctx, c, *arguments)
		if err != nil {
			return nil, err
		}
	case ClickElementToolName:
		arguments := &ClickElementArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processClickElement(ctx, c, *arguments, false)
		if err != nil {
			return nil, err
		}
	case FillElementToolName:
		arguments := &FillElementArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processFillElement(ctx, c, *arguments, false)
		if err != nil {
			return nil, err
		}
	case ScrollElementToolName:
		arguments := &ScrollElementArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processScrollElement(ctx, c, *arguments, false)
		if err != nil {
			return nil, err
		}
	case OpenApplicationToolName:
		arguments := &OpenApplicationArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processOpenApplication(ctx, c, *arguments, false)
		if err != nil {
			return nil, err
		}
	case OpenURLToolName:
		arguments := &OpenURLArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processOpenURL(ctx, c, *arguments)
		if err != nil {
			return nil, err
		}
	case FindElementsToolName:
		arguments := &FindElementsArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processFindElements(ctx, c, *arguments, true)
		if err != nil {
			return nil, err
		}
	case PixelControlToolName:
		arguments := &PixelControlArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processPixelControl(ctx, c, *arguments)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return &mcp_golang.ToolResponse{
		Content: content,
	}, nil
}

// Note: The `formatResultsToContent` helper was a placeholder.
// Specific formatting logic will be in each `process...` function in tools.go.

// To handle the default true for UseBackgroundApps and ActivateApp correctly,
// their respective argument structs in tools.go (e.g. ClickElementArguments)
// should be modified. For example, by adding `UseBackgroundAppsIsSet bool` and `ActivateAppIsSet bool`
// which would be populated by a custom UnmarshalJSON or by the tool registration logic if it can detect presence.
// Alternatively, make UseBackgroundApps and ActivateApp *bool in the arg structs.
// For now, I've added explicit IsSet checks in the client methods, assuming these IsSet fields will be added to tools.go arg structs.
// If not, the logic `valUseBackground := args.UseBackgroundApps` then `selector.UseBackgroundApps = &valUseBackground`
// would pass the Go default `false` if not set by user, and the API would rely on its schema default of `true` when `false` is omitted by `omitempty`.
// The current explicit `IsSet` approach in client methods is safer if we want to ensure `true` is sent when not specified by user.
// I will assume for now that `tools.go` arg structs will be updated to include these `IsSet` fields.
// If not, the `selector.UseBackgroundApps = &args.UseBackgroundApps` (and similar) would be used,
// relying on `*bool` in ElementSelector and `omitempty`.
