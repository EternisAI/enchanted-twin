package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// MemorySearchTool implements a tool for searching agent memory.
type MemorySearchTool struct {
	Logger *log.Logger
	Memory Storage
}

// NewMemorySearchTool creates a new memory search tool.
func NewMemorySearchTool(logger *log.Logger, memory Storage) *MemorySearchTool {
	return &MemorySearchTool{Logger: logger, Memory: memory}
}

// Execute runs the memory search.
func (t *MemorySearchTool) Execute(ctx context.Context, input map[string]any) (types.ToolResult, error) {
	queryVal, exists := input["query"]
	if !exists {
		return nil, errors.New("query is required")
	}

	query, ok := queryVal.(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	var sourcePtr *string
	sourceVal, exists := input["source"]
	if exists {
		source, ok := sourceVal.(string)
		if ok {
			sourcePtr = &source
		}
	}

	var subjectPtr *string
	subjectVal, exists := input["subject"]
	if exists {
		subject, ok := subjectVal.(string)
		if ok {
			subjectPtr = &subject
		}
	}

	t.Logger.Info("Memory query", "query", query, "source", helpers.SafeDeref(sourcePtr), "subject", helpers.SafeDeref(subjectPtr))
	result, err := t.Memory.Query(ctx, query, &Filter{Subject: subjectPtr, Source: sourcePtr})
	if err != nil {
		t.Logger.Error("Memory query failed", "error", err, "query", query)
		return nil, err
	}

	t.Logger.Info("Memory query completed",
		"query", query,
		"facts_found", len(result.Facts))

	if len(result.Facts) == 0 {
		t.Logger.Warn("No facts found for query - this suggests a semantic search issue", "query", query)

		// Try some test queries to debug
		testQueries := []string{"WhatsApp", "contact", "Ornella", "name"}
		for _, testQuery := range testQueries {
			testResult, testErr := t.Memory.Query(ctx, testQuery, nil)
			if testErr == nil {
				t.Logger.Info("Test query result",
					"test_query", testQuery,
					"facts_found", len(testResult.Facts))
			}
		}
	}

	resultText := ""
	for i, fact := range result.Facts {
		// Extract document references from metadata if present
		docRefs := ""

		if fact.Metadata != nil && fact.Metadata["documentReferences"] != "" {
			docRefs = fmt.Sprintf(", DocRefs: %s", fact.Metadata["documentReferences"])
		}

		resultText += fmt.Sprintf(
			"Memory %d [ID: %s]: %s - %s (Source: %s, Time: %s%s)\n",
			i+1,
			fact.ID,
			fact.Subject,
			fact.Content,
			fact.Source,
			fact.Timestamp.Format("2006-01-02 15:04:05"),
			docRefs,
		)
	}

	t.Logger.Debug(
		"Memory tool result",
		"facts_count",
		len(result.Facts),
		"response",
		resultText,
	)

	t.Logger.Info("=== MEMORY TOOL QUERY END ===")

	return types.SimpleToolResult(resultText), nil
}

// Definition returns the OpenAI tool definition.
func (t *MemorySearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "memory_tool",
			Description: param.NewOpt(
				"Information that should be retrieved from the user's memory.",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type":        "string",
						"description": "The query to search for in the memory",
					},
					"source": map[string]any{
						"type":        "string",
						"enum":        []string{"chat", "telegram", "whatsapp", "gmail", "x"},
						"description": "The source to search for in the memory",
					},
					"subject": map[string]string{
						"type":        "string",
						"description": "The subject to search for in the memory",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}
