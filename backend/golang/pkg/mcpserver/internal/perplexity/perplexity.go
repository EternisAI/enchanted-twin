package perplexity

import (
	"context"
	"encoding/json"
	"fmt"

	mcp_golang "github.com/metoro-io/mcp-golang"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver/internal/utils"
)

type PerplexityClient struct {
	Store            *db.Store
	PerplexityAPIKey string
}

func NewClient(store *db.Store, perplexityAPIKey string) *PerplexityClient {
	return &PerplexityClient{
		Store:            store,
		PerplexityAPIKey: perplexityAPIKey,
	}
}

func (c *PerplexityClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*mcp_golang.ToolsResponse, error) {
	toolDescription := PERPLEXITY_ASK_TOOL_DESCRIPTION
	inputSchema, err := utils.ConverToInputSchema(PerplexityAskArguments{})
	if err != nil {
		return nil, err
	}
	tool := mcp_golang.ToolRetType{
		Name:        PERPLEXITY_ASK_TOOL_NAME,
		Description: &toolDescription,
		InputSchema: inputSchema,
	}
	tools := []mcp_golang.ToolRetType{tool}
	return &mcp_golang.ToolsResponse{
		Tools: tools,
	}, nil
}

func (c *PerplexityClient) CallTool(
	ctx context.Context,
	name string,
	arguments any,
) (*mcp_golang.ToolResponse, error) {
	bytes, err := utils.ConvertToBytes(arguments)
	if err != nil {
		return nil, err
	}

	var content []*mcp_golang.Content

	switch name {
	case PERPLEXITY_ASK_TOOL_NAME:
		var argumentsTyped PerplexityAskArguments
		if err := json.Unmarshal(bytes, &argumentsTyped); err != nil {
			return nil, err
		}
		results, err := processPerplexityAsk(ctx, argumentsTyped)
		if err != nil {
			return nil, err
		}
		content = results
	default:
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return &mcp_golang.ToolResponse{
		Content: content,
	}, nil
}
