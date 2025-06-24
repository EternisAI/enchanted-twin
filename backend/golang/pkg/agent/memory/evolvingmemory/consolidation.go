package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// ConsolidationFact represents a high-level insight synthesized from multiple raw facts.
type ConsolidationFact struct {
	memory.MemoryFact
	ConsolidatedFrom []string `json:"consolidated_from"` // IDs of source facts
}

// ConsolidationReport contains the complete output of a consolidation process.
type ConsolidationReport struct {
	Topic             string               `json:"topic"`
	Summary           string               `json:"summary"` // Narrative summary (1-2 paragraphs)
	ConsolidatedFacts []*ConsolidationFact `json:"consolidated_facts"`
	SourceFactCount   int                  `json:"source_fact_count"`
	GeneratedAt       time.Time            `json:"generated_at"`
}

// ConsolidateByTag performs memory consolidation for a given tag.
func ConsolidateByTag(ctx context.Context, tag string, storage memory.Storage, ai *ai.Service, model string, logger *log.Logger) (*ConsolidationReport, error) {
	// Fetch raw facts
	facts, err := fetchFactsByTag(ctx, tag, storage)
	if err != nil {
		return nil, fmt.Errorf("fetching facts: %w", err)
	}

	if len(facts) == 0 {
		return createEmptyReport(tag), nil
	}

	// Generate consolidation via LLM
	consolidation, err := generateConsolidation(ctx, tag, facts, ai, model, logger)
	if err != nil {
		return nil, fmt.Errorf("generating consolidation: %w", err)
	}

	return consolidation, nil
}

// StoreConsolidationReport saves the consolidation to storage.
func StoreConsolidationReport(ctx context.Context, report *ConsolidationReport, storage ConsolidationStorage) error {
	// Store summary as special consolidation fact
	if err := storage.StoreSummary(ctx, report.Topic, report.Summary, report.GeneratedAt); err != nil {
		return fmt.Errorf("storing summary: %w", err)
	}

	// Store consolidation facts
	for _, fact := range report.ConsolidatedFacts {
		if err := storage.StoreConsolidationFact(ctx, fact); err != nil {
			return fmt.Errorf("storing consolidation fact: %w", err)
		}
	}

	return nil
}

// ExportToJSON writes the consolidation report to a JSON file.
func (r *ConsolidationReport) ExportToJSON(filepath string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	return writeFile(filepath, data)
}

// ConsolidationStorage abstracts storage operations for consolidations.
type ConsolidationStorage interface {
	StoreSummary(ctx context.Context, topic, summary string, generatedAt time.Time) error
	StoreConsolidationFact(ctx context.Context, fact *ConsolidationFact) error
}

// === PURE FUNCTIONS ===

// formatFactsForLLM converts raw facts into LLM input format.
func formatFactsForLLM(tag string, facts []*memory.MemoryFact) string {
	if len(facts) == 0 {
		return fmt.Sprintf("Topic: %s\nNo facts found for this topic.", tag)
	}

	content := fmt.Sprintf("Topic: %s\n\nRaw Facts:\n", tag)
	for i, fact := range facts {
		content += fmt.Sprintf("%d. %s\n", i+1, fact.GenerateContent())
	}

	return content
}

// parseConsolidationResponse extracts consolidation data from LLM response.
func parseConsolidationResponse(response openai.ChatCompletionMessage, sourceFactIDs []string) (*ConsolidationReport, error) {
	if len(response.ToolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls in LLM response")
	}

	var consolidatedFacts []*ConsolidationFact
	var summary string

	for _, toolCall := range response.ToolCalls {
		if toolCall.Function.Name != "CONSOLIDATE_MEMORIES" {
			continue
		}

		var args ConsolidateMemoriesToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return nil, fmt.Errorf("parsing tool arguments: %w", err)
		}

		summary = args.Summary

		// Convert to ConsolidationFacts with source tracking
		for _, fact := range args.ConsolidatedFacts {
			consolidatedFacts = append(consolidatedFacts, &ConsolidationFact{
				MemoryFact:       fact,
				ConsolidatedFrom: sourceFactIDs, // All facts contributed to this consolidation
			})
		}
	}

	return &ConsolidationReport{
		Summary:           summary,
		ConsolidatedFacts: consolidatedFacts,
		SourceFactCount:   len(sourceFactIDs),
		GeneratedAt:       time.Now(),
	}, nil
}

// createEmptyReport creates a report for when no facts are found.
func createEmptyReport(topic string) *ConsolidationReport {
	return &ConsolidationReport{
		Topic:             topic,
		Summary:           fmt.Sprintf("No memory facts found for topic '%s'.", topic),
		ConsolidatedFacts: []*ConsolidationFact{},
		SourceFactCount:   0,
		GeneratedAt:       time.Now(),
	}
}

// === SIDE EFFECT FUNCTIONS ===

// fetchFactsByTag retrieves all facts with the given tag.
func fetchFactsByTag(ctx context.Context, tag string, storage memory.Storage) ([]*memory.MemoryFact, error) {
	filter := &memory.Filter{
		Tags: &memory.TagsFilter{
			All: []string{tag},
		},
	}

	result, err := storage.Query(ctx, tag, filter)
	if err != nil {
		return nil, fmt.Errorf("querying storage: %w", err)
	}

	// Convert to pointers for consistent interface
	facts := make([]*memory.MemoryFact, len(result.Facts))
	for i := range result.Facts {
		facts[i] = &result.Facts[i]
	}
	return facts, nil
}

// generateConsolidation calls the LLM to create consolidated insights.
func generateConsolidation(ctx context.Context, tag string, facts []*memory.MemoryFact, ai *ai.Service, model string, logger *log.Logger) (*ConsolidationReport, error) {
	// Prepare LLM input
	userPrompt := formatFactsForLLM(tag, facts)
	sourceFactIDs := extractFactIDs(facts)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(MemoryConsolidationPrompt),
		openai.UserMessage(userPrompt),
	}

	tools := []openai.ChatCompletionToolParam{consolidateMemoriesTool}

	logger.Debug("Calling LLM for consolidation", "tag", tag, "fact_count", len(facts))

	// Call LLM
	response, err := ai.Completions(ctx, messages, tools, model)
	if err != nil {
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	// Parse response
	report, err := parseConsolidationResponse(response, sourceFactIDs)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	report.Topic = tag
	return report, nil
}

// === UTILITY FUNCTIONS ===

// extractFactIDs gets IDs from a slice of MemoryFacts.
func extractFactIDs(facts []*memory.MemoryFact) []string {
	ids := make([]string, len(facts))
	for i, fact := range facts {
		ids[i] = fact.ID
	}
	return ids
}

// writeFile is a testable file writing abstraction.
var writeFile = func(filepath string, data []byte) error {
	// This would typically be os.WriteFile, but abstracted for testing
	return fmt.Errorf("writeFile not implemented - use os.WriteFile")
}
