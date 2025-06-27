package searchtools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// SearchPlanTool converts a QueryIntent into a concrete SearchPlan consisting
// of executable steps that the SearchExecutor tool can run.
type SearchPlanTool struct {
	Logger *log.Logger
}

func NewSearchPlanTool(logger *log.Logger) *SearchPlanTool {
	return &SearchPlanTool{Logger: logger}
}

func (t *SearchPlanTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	// Expect the "intent" parameter containing a JSON-serialised QueryIntent object
	rawIntent, ok := inputs["intent"].(string)
	if !ok || strings.TrimSpace(rawIntent) == "" {
		return nil, errors.New("intent must be a JSON string parameter")
	}

	var intent QueryIntent
	if err := json.Unmarshal([]byte(rawIntent), &intent); err != nil {
		return nil, err
	}

	plan := t.buildPlan(intent)

	planBytes, _ := json.Marshal(plan)

	return &types.StructuredToolResult{
		ToolName:   "search_plan",
		ToolParams: inputs,
		Output: map[string]any{
			"plan": json.RawMessage(planBytes),
		},
	}, nil
}

// buildPlan converts intent to a basic plan. Rules can be enhanced incrementally.
func (t *SearchPlanTool) buildPlan(intent QueryIntent) SearchPlan {
	switch intent.IntentType {
	case "pending_action":
		return t.buildPendingActionPlan(intent)
	default:
		return t.buildContentSearchPlan(intent)
	}
}

func (t *SearchPlanTool) buildPendingActionPlan(intent QueryIntent) SearchPlan {
	steps := []SearchStep{
		{
			Type:  StepFilterMemories,
			Query: "messages with unanswered questions or requests",
			Filter: &memory.Filter{
				TimestampAfter:  intent.TemporalContextStartPointer(),
				TimestampBefore: intent.TemporalContextEndPointer(),
			},
			ResultField: "facts",
		},
		{
			Type:        StepCheckConversations,
			DependsOn:   []int{0},
			ResultField: "facts",
		},
	}
	return SearchPlan{Steps: steps}
}

func (t *SearchPlanTool) buildContentSearchPlan(intent QueryIntent) SearchPlan {
	// Build basic filter with entity (if any) and time.
	var filter memory.Filter
	if len(intent.Entities) > 0 {
		filter.Subject = ptr(strings.Join([]string{intent.Entities[0].Value}, " "))
	}
	if intent.TemporalContext != nil {
		filter.TimestampAfter = intent.TemporalContext.Start
		filter.TimestampBefore = intent.TemporalContext.End
	}

	steps := []SearchStep{
		{
			Type:        StepFilterMemories,
			Query:       "",
			Filter:      &filter,
			ResultField: "facts",
		},
		{
			Type:        StepCheckConversations,
			DependsOn:   []int{0},
			ResultField: "facts",
		},
	}

	return SearchPlan{Steps: steps}
}

// Definition of OpenAI tool.
func (t *SearchPlanTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "search_plan",
			Description: param.NewOpt("Generates a multi-step memory search plan from a structured query intent produced by the query_analyzer tool."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"intent": map[string]any{
						"type":        "string",
						"description": "JSON string representation of a QueryIntent struct returned by the query_analyzer tool.",
					},
				},
				"required": []string{"intent"},
			},
		},
	}
}

// Helper returning pointer for string
func ptr[T any](v T) *T {
	return &v
}

// Helper functions to safely get pointer fields from TemporalContext
func (qi QueryIntent) TemporalContextStartPointer() *time.Time {
	if qi.TemporalContext != nil {
		return qi.TemporalContext.Start
	}
	return nil
}

func (qi QueryIntent) TemporalContextEndPointer() *time.Time {
	if qi.TemporalContext != nil {
		return qi.TemporalContext.End
	}
	return nil
}
