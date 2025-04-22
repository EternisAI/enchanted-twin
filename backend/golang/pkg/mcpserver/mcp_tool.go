package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type MCPTool struct {
	Client *mcp_golang.Client
	Tool mcp_golang.ToolRetType
}

func (t *MCPTool) Execute(ctx context.Context, inputs map[string]any) (tools.ToolResult, error) {
	

	if t.Client == nil {
		fmt.Println("Client not found")
		return tools.ToolResult{}, errors.New("client not found")
	}

	response, err := t.Client.CallTool(ctx, t.Tool.Name, inputs)
	if err != nil {
		return tools.ToolResult{}, err
	}

	if len(response.Content) == 0 {
		return tools.ToolResult{
			Content: "No content returned from tool",
		}, nil
	}
	result := tools.ToolResult{}
	if response.Content[0].ImageContent != nil {
		result.ImageURLs = []string{response.Content[0].ImageContent.Data}
	}
	result.Content = response.Content[0].TextContent.Text
	return result, nil
}

func (t *MCPTool) Definition() openai.ChatCompletionToolParam {

	params := make(openai.FunctionParameters)

	if inputSchemaMap, ok := t.Tool.InputSchema.(map[string]any); ok && inputSchemaMap != nil {
		maps.Copy(params, inputSchemaMap)
	} else if t.Tool.InputSchema != nil {
		fmt.Printf("Warning: tool.InputSchema for tool %s is not a map[string]any or is nil, type is %T\n", t.Tool.Name, t.Tool.InputSchema)
	}

	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Tool.Name,
			Description: param.NewOpt(*t.Tool.Description),
			Parameters: params,
		},
	}
}
