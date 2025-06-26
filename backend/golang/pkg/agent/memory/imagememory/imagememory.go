package imagememory

import (
	"context"
	"errors"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type ImageSearchTool struct {
	Logger      *log.Logger
	ClipService *ai.ClipEmbeddingService
}

func (t *ImageSearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "image_search",
			Description: param.NewOpt("Search for images in the image memory"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type":        "string",
						"description": "The query to search for in the image memory",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (t *ImageSearchTool) Execute(ctx context.Context, input map[string]any) (types.ToolResult, error) {
	queryVal, exists := input["query"]
	if !exists {
		return nil, errors.New("query is required")
	}

	query, ok := queryVal.(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	t.Logger.Info("query", "llm_query", query)

	images, err := t.ClipService.ImageSearch(ctx, query, 1)
	if err != nil {
		return nil, err
	}

	t.Logger.Info("images", "images", images)

	return &types.StructuredToolResult{
		ToolName: t.Definition().Function.Name,
		Output: map[string]any{
			"images": images.ImagePaths,
		},
	}, nil
}
