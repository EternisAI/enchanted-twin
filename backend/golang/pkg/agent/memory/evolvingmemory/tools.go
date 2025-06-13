package evolvingmemory

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

var extractFactsTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "EXTRACT_FACTS",
		Description: param.NewOpt(
			"Extract ONLY high-quality, memorable facts with confidence 7+ (on 1-10 scale) that pass strict quality filters. ALWAYS extract major life milestones (moving, job changes, health developments, major purchases, relationship changes, family events). NEVER extract routine daily activities, temporary mood states, one-off experiences, or vague considerations. PRIORITIZE QUALITY OVER QUANTITY - better to extract 2-3 excellent facts than 10 mediocre ones. Facts must be worth remembering in medium to long term and have clear practical value.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"facts": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"category": map[string]any{
								"type": "string",
								"enum": []string{
									"profile_stable",
									"preference",
									"goal_plan",
									"routine",
									"skill",
									"relationship",
									"health",
									"context_env",
									"affective_marker",
									"event",
									"conversation_context",
								},
								"description": "Category of the fact",
							},
							"subject": map[string]any{
								"type":        "string",
								"description": "Subject of the fact - typically 'user' or specific entity name",
							},
							"attribute": map[string]any{
								"type":        "string",
								"description": "Specific property or attribute being described",
							},
							"value": map[string]any{
								"type":        "string",
								"description": "Descriptive phrase with context (aim for 8-30 words)",
							},
							"temporal_context": map[string]any{
								"type":        "string",
								"description": "YYYY-MM-DD format, relative time, or descriptive time reference (optional)",
							},
							"sensitivity": map[string]any{
								"type":        "string",
								"enum":        []string{"high", "medium", "low"},
								"description": "Holistic life sensitivity assessment across all life domains (personal, professional, social, health, financial)",
							},
							"importance": map[string]any{
								"type":        "integer",
								"minimum":     1,
								"maximum":     3,
								"description": "Life significance score: 1=minor detail worth noting, 2=meaningful information affecting decisions/relationships, 3=major life factor with significant ongoing impact",
							},
						},
						"required":             []string{"category", "subject", "attribute", "value", "sensitivity", "importance"},
						"additionalProperties": false,
					},
					"description": "Array of extracted facts - keep this array small and focused on high-value facts only (confidence 7+ on 1-10 scale)",
				},
			},
			"required":             []string{"facts"},
			"additionalProperties": false,
		},
	},
}

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
