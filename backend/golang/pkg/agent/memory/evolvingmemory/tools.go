package evolvingmemory

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

var addMemoryTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "ADD",
		Description: param.NewOpt(
			"Add a new memory fact to the memory store.",
		),
		Parameters: openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
	},
}

var updateMemoryTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "UPDATE",
		Description: param.NewOpt(
			"If the new fact provides additional details or an update to an existing memory.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The ID of the memory to update.",
				},
				"updated_content": map[string]any{
					"type":        "string",
					"description": "The new content for the memory.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "The reason for updating the memory.",
				},
			},
			"required": []string{"id", "updated_content", "reason"},
		},
	},
}

var deleteMemoryTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "DELETE",
		Description: param.NewOpt(
			" If the new fact explicitly invalidates or marks an existing memory as obsolete. If there is contradicting information, delete the memory.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The ID of the memory to delete.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "The reason for deleting the memory.",
				},
			},
			"required": []string{"id", "reason"},
		},
	},
}

var noneMemoryTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "NONE",
		Description: param.NewOpt(
			"Take no action for the given memory (explicitly choose to do nothing).",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"reason": map[string]any{
					"type":        "string",
					"description": "The reason for choosing to do nothing.",
				},
			},
			"required": []string{"reason"},
		},
	},
}

var extractFactsTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "EXTRACT_FACTS",
		Description: param.NewOpt(
			"Extract facts from the given text.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"facts": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "List of extracted facts from the text.",
				},
			},
			"required": []string{"facts"},
		},
	},
}
