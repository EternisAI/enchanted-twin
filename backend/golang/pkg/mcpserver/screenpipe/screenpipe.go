package screenpipe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

const (
	defaultTimeout = 90 * time.Second
	apiBaseURL     = "http://localhost:3030"
)

type ScreenpipeClient struct {
	httpClient *http.Client
	apiBaseURL string
}

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

		content, err = processClickElement(ctx, c, *arguments, isMac())
		if err != nil {
			return nil, err
		}
	case FillElementToolName:
		arguments := &FillElementArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processFillElement(ctx, c, *arguments, isMac())
		if err != nil {
			return nil, err
		}
	case ScrollElementToolName:
		arguments := &ScrollElementArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processScrollElement(ctx, c, *arguments, isMac())
		if err != nil {
			return nil, err
		}
	case OpenApplicationToolName:
		arguments := &OpenApplicationArguments{}
		err = json.Unmarshal(bytes, arguments)
		if err != nil {
			return nil, err
		}

		content, err = processOpenApplication(ctx, c, *arguments, isMac())
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

		content, err = processFindElements(ctx, c, *arguments, isMac())
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

func isMac() bool {
	switch runtime.GOOS {
	case "windows":
		return false
	default: // darwin (macOS), linux, etc.
		return true
	}
}
