package searchtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	mem "github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	em "github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// SearchExecutorTool executes a SearchPlan and returns raw step results.
type SearchExecutorTool struct {
	Logger        *log.Logger
	MemoryStore   em.MemoryStorage
	DocumentStore mem.Storage
}

func NewSearchExecutorTool(logger *log.Logger, memStore em.MemoryStorage, docStore mem.Storage) *SearchExecutorTool {
	return &SearchExecutorTool{Logger: logger, MemoryStore: memStore, DocumentStore: docStore}
}

func (t *SearchExecutorTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	rawPlan, ok := inputs["plan"].(string)
	if !ok || strings.TrimSpace(rawPlan) == "" {
		return nil, errors.New("plan must be provided as JSON string")
	}

	var plan SearchPlan
	if err := json.Unmarshal([]byte(rawPlan), &plan); err != nil {
		return nil, err
	}

	stepOutputs := make(map[int]any)

	for idx, step := range plan.Steps {
		t.Logger.Debug("Executing search step", "step_index", idx, "type", step.Type)

		switch step.Type {
		case StepFilterMemories:
			// Build query string fallback.
			query := step.Query
			if query == "" {
				query = "*"
			}

			res, err := t.MemoryStore.Query(ctx, query, step.Filter)
			if err != nil {
				t.Logger.Error("memory query failed", "error", err)
				return nil, err
			}
			stepOutputs[idx] = res.Facts
		case StepCheckConversations:
			docIDs := gatherDocumentIDs(step.DependsOn, stepOutputs)
			if len(docIDs) == 0 {
				stepOutputs[idx] = []mem.Document{}
				break
			}

			// Deduplicate IDs
			docIDs = dedupStrings(docIDs)

			// Try to fetch actual documents via DocumentStore if possible.
			filter := &mem.Filter{DocumentReferences: docIDs}

			if t.DocumentStore != nil {
				res, err := t.DocumentStore.Query(ctx, "*", filter)
				if err != nil {
					return nil, err
				}
				stepOutputs[idx] = res.Facts
			} else {
				// Fallback to MemoryStore
				res, err := t.MemoryStore.Query(ctx, "*", filter)
				if err != nil {
					return nil, err
				}
				stepOutputs[idx] = res.Facts
			}
		default:
			return nil, fmt.Errorf("unsupported step type: %s", step.Type)
		}
	}

	resultBytes, _ := json.Marshal(SearchExecutionResult{StepResults: stepOutputs})

	return &types.StructuredToolResult{
		ToolName:   "search_executor",
		ToolParams: inputs,
		Output: map[string]any{
			"results": json.RawMessage(resultBytes),
		},
	}, nil
}

func (t *SearchExecutorTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "search_executor",
			Description: param.NewOpt("Executes a memory search plan previously produced by the search_plan tool and returns per-step data."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"plan": map[string]any{
						"type":        "string",
						"description": "JSON string representation of a SearchPlan returned by the search_plan tool.",
					},
				},
				"required": []string{"plan"},
			},
		},
	}
}

// gatherDocumentIDs collects document reference IDs from previous fact results.
func gatherDocumentIDs(depIdx []int, outputs map[int]any) []string {
	var ids []string
	for _, i := range depIdx {
		if v, ok := outputs[i]; ok {
			switch vv := v.(type) {
			case []memory.MemoryFact:
				for _, f := range vv {
					ids = append(ids, f.DocumentReferences...)
				}
			}
		}
	}
	return ids
}

func dedupStrings(arr []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range arr {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
