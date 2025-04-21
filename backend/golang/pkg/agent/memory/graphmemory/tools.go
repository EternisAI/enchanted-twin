package graphmemory

import (
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

func extractSQLQueriesToolDefinition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "submit_sql_queries",
			Description: param.NewOpt("Return one or more SQL statements plus the aliases of every column they select"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"queries": map[string]any{
						"type":        "array",
						"description": "Each item is an object {query: string, columns: []string}",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{"type": "string"},
								"columns": map[string]any{
									"type":  "array",
									"items": map[string]any{"type": "string"},
								},
							},
							"required": []string{"query", "columns"},
						},
					},
				},
				"required": []string{"queries"},
			},
		},
	}
}
