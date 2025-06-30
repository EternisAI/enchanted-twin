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

// IntelligentMemoryStorage defines the enhanced memory interface with intelligent querying
// This interface mirrors evolvingmemory.MemoryStorage to avoid circular imports.
type IntelligentMemoryStorage interface {
	Storage // Embed the basic storage interface

	// IntelligentQuery executes 3-stage intelligent query (consolidation-first approach)
	IntelligentQuery(ctx context.Context, queryText string, filter *Filter) (*IntelligentQueryResult, error)
}

// IntelligentQueryResult represents the structured result of intelligent querying.
// This mirrors the structure from evolvingmemory to avoid circular imports.
type IntelligentQueryResult struct {
	Query                string        `json:"query"`
	ConsolidatedInsights []MemoryFact  `json:"consolidated_insights"`
	CitedEvidence        []MemoryFact  `json:"cited_evidence"`
	AdditionalContext    []MemoryFact  `json:"additional_context"`
	Metadata             QueryMetadata `json:"metadata"`
}

// QueryMetadata provides information about the query execution.
type QueryMetadata struct {
	QueriedAt                string `json:"queried_at"`
	ConsolidatedInsightCount int    `json:"consolidated_insight_count"`
	CitedEvidenceCount       int    `json:"cited_evidence_count"`
	AdditionalContextCount   int    `json:"additional_context_count"`
	TotalResults             int    `json:"total_results"`
	QueryStrategy            string `json:"query_strategy"`
}

// MemorySearchTool implements a tool for searching agent memory.
type MemorySearchTool struct {
	Logger *log.Logger
	Memory Storage
}

// NewMemorySearchTool creates a new memory search tool.
func NewMemorySearchTool(logger *log.Logger, memory Storage) *MemorySearchTool {
	return &MemorySearchTool{Logger: logger, Memory: memory}
}

// Execute runs the memory search using intelligent querying.
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
		var source string
		source, ok = sourceVal.(string)
		if ok {
			sourcePtr = &source
		}
	}

	var subjectPtr *string
	subjectVal, exists := input["subject"]
	if exists {
		var subject string
		subject, ok = subjectVal.(string)
		if ok {
			subjectPtr = &subject
		}
	}

	t.Logger.Info("Memory query", "query", query, "source", helpers.SafeDeref(sourcePtr), "subject", helpers.SafeDeref(subjectPtr))

	// 🚀 ALWAYS use intelligent query system (no backward compatibility)
	intelligentMemory, ok := t.Memory.(IntelligentMemoryStorage)
	if !ok {
		return nil, errors.New("memory storage does not support intelligent querying - this should not happen in production")
	}

	t.Logger.Info("Using intelligent query system", "query", query)
	return t.executeIntelligentQuery(ctx, intelligentMemory, query, sourcePtr, subjectPtr)
}

// executeIntelligentQuery uses the 3-stage intelligent querying system.
func (t *MemorySearchTool) executeIntelligentQuery(ctx context.Context, intelligentMemory IntelligentMemoryStorage, query string, sourcePtr, subjectPtr *string) (types.ToolResult, error) {
	// Build filter for intelligent query
	filter := &Filter{
		Subject:  subjectPtr,
		Source:   sourcePtr,
		Distance: 0.7, // Good balance between precision and recall
	}

	intelligentResult, err := intelligentMemory.IntelligentQuery(ctx, query, filter)
	if err != nil {
		t.Logger.Error("Intelligent memory query failed", "error", err, "query", query)
		return nil, err
	}

	t.Logger.Info("Intelligent memory query completed",
		"query", query,
		"total_results", intelligentResult.Metadata.TotalResults,
		"insights", intelligentResult.Metadata.ConsolidatedInsightCount,
		"evidence", intelligentResult.Metadata.CitedEvidenceCount,
		"context", intelligentResult.Metadata.AdditionalContextCount)

	// Format results for LLM with clear structure
	resultText := t.formatIntelligentResults(intelligentResult)

	t.Logger.Debug("Intelligent memory tool result", "response", resultText)
	t.Logger.Info("=== INTELLIGENT MEMORY TOOL QUERY END ===")

	return types.SimpleToolResult(resultText), nil
}

// formatIntelligentResults formats the intelligent query results for LLM consumption.
func (t *MemorySearchTool) formatIntelligentResults(result *IntelligentQueryResult) string {
	resultText := fmt.Sprintf("🧠 Intelligent Memory Results for: \"%s\"\n", result.Query)
	resultText += fmt.Sprintf("📊 Total: %d | 🔗 Insights: %d | 📋 Evidence: %d | 📄 Context: %d\n\n",
		result.Metadata.TotalResults,
		result.Metadata.ConsolidatedInsightCount,
		result.Metadata.CitedEvidenceCount,
		result.Metadata.AdditionalContextCount)

	// Section 1: Consolidated Insights (highest priority)
	if len(result.ConsolidatedInsights) > 0 {
		resultText += "🔗 CONSOLIDATED INSIGHTS (High-level synthesized knowledge):\n"
		for i, fact := range result.ConsolidatedInsights {
			resultText += fmt.Sprintf("  %d. %s - %s [%s]\n",
				i+1,
				fact.Subject,
				fact.GenerateContentForLLM(),
				fact.Timestamp.Format("Jan 2006"))
		}
		resultText += "\n"
	}

	// Section 2: Supporting Evidence (medium priority)
	if len(result.CitedEvidence) > 0 {
		resultText += "📋 SUPPORTING EVIDENCE (Source facts that led to insights):\n"
		for i, fact := range result.CitedEvidence {
			if i >= 5 { // Limit evidence for brevity
				resultText += fmt.Sprintf("  ... and %d more supporting facts\n", len(result.CitedEvidence)-i)
				break
			}
			resultText += fmt.Sprintf("  %d. %s - %s [%s]\n",
				i+1,
				fact.Subject,
				fact.GenerateContentForLLM(),
				fact.Timestamp.Format("Jan 2006"))
		}
		resultText += "\n"
	}

	// Section 3: Additional Context (lower priority)
	if len(result.AdditionalContext) > 0 {
		resultText += "📄 ADDITIONAL CONTEXT (Related raw facts):\n"
		for i, fact := range result.AdditionalContext {
			if i >= 3 { // Limit context for brevity
				resultText += fmt.Sprintf("  ... and %d more contextual facts\n", len(result.AdditionalContext)-i)
				break
			}
			resultText += fmt.Sprintf("  %d. %s - %s [%s]\n",
				i+1,
				fact.Subject,
				fact.GenerateContentForLLM(),
				fact.Timestamp.Format("Jan 2006"))
		}
		resultText += "\n"
	}

	if result.Metadata.TotalResults == 0 {
		resultText += "No relevant memories found for this query.\n"
	}

	return resultText
}

// Definition returns the OpenAI tool definition.
func (t *MemorySearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "memory_tool",
			Description: param.NewOpt(
				"Search the user's memory for relevant information. Uses intelligent 3-stage querying that prioritizes consolidated insights over raw facts. Returns structured results: insights (synthesized knowledge), evidence (supporting facts), and context (related information).",
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
