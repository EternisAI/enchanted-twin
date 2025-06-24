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
