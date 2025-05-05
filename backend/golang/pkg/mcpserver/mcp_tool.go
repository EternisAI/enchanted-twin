package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"maps"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type MCPClient interface {
	CallTool(ctx context.Context, name string, arguments any) (*mcp_golang.ToolResponse, error)
	ListTools(ctx context.Context, cursor *string) (*mcp_golang.ToolsResponse, error)
}

type MCPTool struct {
	Client MCPClient
	Tool   mcp_golang.ToolRetType
}

func (t *MCPTool) Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error) {
	if t.Client == nil {
		fmt.Println("Client not found")
		return &agenttypes.StructuredToolResult{
			ToolName:   t.Tool.Name,
			ToolParams: inputs,
			ToolError:  "client not found",
		}, errors.New("client not found")
	}

	response, err := t.Client.CallTool(ctx, t.Tool.Name, inputs)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   t.Tool.Name,
			ToolParams: inputs,
			ToolError:  err.Error(),
		}, err
	}

	if len(response.Content) == 0 {
		return &agenttypes.StructuredToolResult{
			ToolName:   t.Tool.Name,
			ToolParams: inputs,
			Output: map[string]any{
				"content": "No content returned from tool",
			},
		}, nil
	}

	resultText := ""
	resultImages := []string{}
	for _, content := range response.Content {
		if content.ImageContent != nil {
			resultImages = append(resultImages, content.ImageContent.Data)
		}
		if content.TextContent != nil {
			resultText = fmt.Sprintf("%s\n%s", resultText, content.TextContent.Text)
		}
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   t.Tool.Name,
		ToolParams: inputs,
		Output: map[string]any{
			"content": resultText,
			"images":  resultImages,
		},
	}, nil
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
			Parameters:  params,
		},
	}
}
