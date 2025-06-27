package searchtools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// QueryAnalyzerTool implements the QueryAnalyzer spec as an OpenAI function tool.
// It converts a natural-language query into a structured QueryIntent.
type QueryAnalyzerTool struct {
	Logger *log.Logger
	AI     *ai.Service
	Model  string
}

// NewQueryAnalyzerTool returns a new instance ready for registration.
func NewQueryAnalyzerTool(logger *log.Logger, aiSvc *ai.Service, model string) *QueryAnalyzerTool {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &QueryAnalyzerTool{Logger: logger, AI: aiSvc, Model: model}
}

// Execute analyses the provided natural language query and returns a QueryIntent.
func (t *QueryAnalyzerTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	raw, ok := inputs["query"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, errors.New("query must be a non-empty string")
	}

	intent, err := t.analyzeWithLLM(ctx, raw)
	if err != nil {
		return nil, err
	}

	out, _ := json.Marshal(intent)

	return &types.StructuredToolResult{
		ToolName:   "query_analyzer",
		ToolParams: inputs,
		Output: map[string]any{
			"intent": json.RawMessage(out),
		},
	}, nil
}

// analyzeWithLLM uses the injected ai.Service to get structured JSON from the LLM.
func (t *QueryAnalyzerTool) analyzeWithLLM(ctx context.Context, query string) (QueryIntent, error) {
	if t.AI == nil {
		return QueryIntent{}, errors.New("ai service not configured")
	}

	schemaDesc := `Respond with valid JSON that matches this Go struct (snake_case keys acceptable):
 {
     "intent_type": "pending_action | content_search | relationship_query",
     "temporal_context": {
         "reference": string,
         "start": RFC3339 timestamp or null,
         "end": RFC3339 timestamp or null
     },
     "entities": [
         {"type": "person|topic|organization", "value": string, "role": string}
     ],
     "content_type": "link|document|message",
     "action_required": bool
 }`

	messages := []ai.Message{
		{Role: ai.MessageRoleSystem, Content: "You are a query intent extraction function."},
		{Role: ai.MessageRoleSystem, Content: schemaDesc},
		{Role: ai.MessageRoleUser, Content: query},
	}

	resp, err := t.AI.CompletionsWithMessages(ctx, messages, nil, t.Model)
	if err != nil {
		return QueryIntent{}, err
	}

	// Extract JSON from the response content (strip code fences if any)
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var intent QueryIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return QueryIntent{}, err
	}
	return intent, nil
}

// Definition returns the OpenAI function definition for the tool.
func (t *QueryAnalyzerTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "query_analyzer",
			Description: param.NewOpt("Analyzes a natural language user query and returns structured intent metadata used for downstream search planning."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The natural language question coming from the user.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}
