package screenpipe

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcp_golang "github.com/mark3labs/mcp-go/mcp"
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
	request mcp_golang.ListToolsRequest,
) (*mcp_golang.ListToolsResult, error) {
	return GetScreenpipeTools(c, false)
}

func (c *ScreenpipeClient) CallTool(
	ctx context.Context,
	request mcp_golang.CallToolRequest,
) (*mcp_golang.CallToolResult, error) {
	fmt.Println("Call tool SCREENPIPE", request.Params.Name, request.Params.Arguments)

	var content []mcp_golang.Content
	switch request.Params.Name {
	case SearchContentToolName:
		var argumentsTyped SearchContentArguments
		err := request.BindArguments(&argumentsTyped)
		if err != nil {
			return nil, err
		}
		contentResp, err := processSearchContent(ctx, c, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = contentResp
	default:
		return nil, fmt.Errorf("unknown tool: %s", request.Params.Name)
	}

	return &mcp_golang.CallToolResult{
		Content: content,
	}, nil
}
