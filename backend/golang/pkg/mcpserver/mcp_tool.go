package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type MCPClient interface {
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
}

type MCPTool struct {
	Client MCPClient
	Tool   mcp.Tool
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
	fmt.Println("Executing tool in execute", t.Tool.Name, inputs)
	request := mcp.CallToolRequest{}
	request.Params.Name = t.Tool.GetName()
	request.Params.Arguments = inputs
	response, err := t.Client.CallTool(ctx, request)
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
		if textContent, ok := content.(mcp.TextContent); ok {
			resultText = fmt.Sprintf("%s\n%s", resultText, textContent.Text)
		} else if imageContent, ok := content.(mcp.ImageContent); ok {
			resultImages = append(resultImages, imageContent.Data)
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

type EmptyParams struct{}

func (t *MCPTool) Definition() openai.ChatCompletionToolParam {
	params := make(openai.FunctionParameters)

	if t.Tool.InputSchema.Properties != nil {
		// See: https://platform.openai.com/docs/guides/function-calling
		params["type"] = t.Tool.InputSchema.Type
		params["properties"] = t.Tool.InputSchema.Properties
		if len(t.Tool.InputSchema.Required) > 0 {
			params["required"] = t.Tool.InputSchema.Required
		}
	}

	if len(params) == 1 && params["type"] == "object" {
		params = openai.FunctionParameters{}
	}

	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Tool.GetName(),
			Description: param.NewOpt(t.Tool.Description),
			Parameters:  params,
		},
	}
}
