package screenpipe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcp_golang "github.com/mark3labs/mcp-go/mcp"

	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
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

	bytes, err := utils.ConvertToBytes(request.Params.Arguments)
	if err != nil {
		return nil, err
	}
	var content []mcp_golang.Content
	switch request.Params.Name {
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
	default:
		return nil, fmt.Errorf("unknown tool: %s", request.Params.Name)
	}

	return &mcp_golang.CallToolResult{
		Content: content,
	}, nil
}
