package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

	if err := os.WriteFile(filepath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
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

// fetchFactsByCategory retrieves all facts with the given category.
func fetchFactsByCategory(ctx context.Context, category string, storage memory.Storage) ([]*memory.MemoryFact, error) {
	filter := &memory.Filter{
		FactCategory: &category,
		Limit:        func() *int { limit := 100; return &limit }(), // Reasonable limit
	}

	result, err := storage.Query(ctx, fmt.Sprintf("%s related facts", category), filter)
	if err != nil {
		return nil, fmt.Errorf("querying storage by category: %w", err)
	}

	// Convert to pointers for consistent interface
	facts := make([]*memory.MemoryFact, len(result.Facts))
	for i := range result.Facts {
		facts[i] = &result.Facts[i]
	}
	return facts, nil
}

// fetchFactsBySemantic retrieves facts using semantic similarity search.
func fetchFactsBySemantic(ctx context.Context, topic string, filter *memory.Filter, storage memory.Storage) ([]*memory.MemoryFact, error) {
	// Set default distance threshold if not provided
	if filter.Distance == 0 {
		filter.Distance = 0.8 // Allow fairly broad semantic matches
	}

	// Set default limit if not provided
	if filter.Limit == nil {
		limit := 50
		filter.Limit = &limit
	}

	result, err := storage.Query(ctx, topic, filter)
	if err != nil {
		return nil, fmt.Errorf("semantic querying storage: %w", err)
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

// ExportJSON exports the consolidation report to a JSON file.
func (cr *ConsolidationReport) ExportJSON(filepath string) error {
	data, err := json.MarshalIndent(cr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ConsolidationReport: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// ConsolidateMemoriesByCategory performs memory consolidation for a given category.
// It fetches facts by category and runs LLM consolidation.
func ConsolidateMemoriesByCategory(ctx context.Context, category string, deps ConsolidationDependencies) (*ConsolidationReport, error) {
	logger := deps.Logger.With("category", category)
	logger.Info("Starting category-based memory consolidation")

	// Step 1: Fetch facts by category
	facts, err := fetchFactsByCategory(ctx, category, deps.Storage)
	if err != nil {
		logger.Error("Failed to fetch facts by category", "error", err)
		return nil, fmt.Errorf("fetching facts by category: %w", err)
	}

	if len(facts) == 0 {
		logger.Warn("No facts found for category")
		return &ConsolidationReport{
			Topic:             category,
			Summary:           fmt.Sprintf("No memories found for category: %s", category),
			ConsolidatedFacts: []*ConsolidationFact{},
			SourceFactCount:   0,
			GeneratedAt:       time.Now(),
		}, nil
	}

	logger.Info("Found facts for consolidation", "count", len(facts))
	return processConsolidation(ctx, category, facts, deps)
}

// ConsolidateMemoriesBySemantic performs memory consolidation using semantic similarity.
// It searches for facts semantically related to the topic and consolidates them.
func ConsolidateMemoriesBySemantic(ctx context.Context, topic string, filter *memory.Filter, deps ConsolidationDependencies) (*ConsolidationReport, error) {
	logger := deps.Logger.With("topic", topic)
	logger.Info("Starting semantic memory consolidation")

	// Step 1: Fetch facts by semantic similarity
	facts, err := fetchFactsBySemantic(ctx, topic, filter, deps.Storage)
	if err != nil {
		logger.Error("Failed to fetch facts by semantic search", "error", err)
		return nil, fmt.Errorf("fetching facts by semantic search: %w", err)
	}

	if len(facts) == 0 {
		logger.Warn("No facts found for semantic topic")
		return &ConsolidationReport{
			Topic:             topic,
			Summary:           fmt.Sprintf("No memories found semantically related to: %s", topic),
			ConsolidatedFacts: []*ConsolidationFact{},
			SourceFactCount:   0,
			GeneratedAt:       time.Now(),
		}, nil
	}

	logger.Info("Found semantically related facts for consolidation", "count", len(facts))
	return processConsolidation(ctx, topic, facts, deps)
}

// processConsolidation performs the common consolidation logic for any set of facts.
func processConsolidation(ctx context.Context, topic string, facts []*memory.MemoryFact, deps ConsolidationDependencies) (*ConsolidationReport, error) {
	logger := deps.Logger.With("topic", topic)

	// Step 2: Prepare LLM input
	factsContent := buildFactsContent(facts)
	logger.Debug("Built facts content", "length", len(factsContent))

	// Step 3: Run LLM consolidation
	consolidationTools := []openai.ChatCompletionToolParam{consolidateMemoriesTool}

	llmMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(MemoryConsolidationPrompt),
		openai.UserMessage(fmt.Sprintf("Topic: %s\n\nFacts to consolidate:\n%s", topic, factsContent)),
	}

	logger.Debug("Sending consolidation request to LLM", "facts_count", len(facts))

	llmResponse, err := deps.CompletionsService.Completions(ctx, llmMessages, consolidationTools, deps.CompletionsModel)
	if err != nil {
		logger.Error("LLM consolidation failed", "error", err)
		return nil, fmt.Errorf("LLM consolidation: %w", err)
	}

	logger.Debug("LLM consolidation completed", "tool_calls", len(llmResponse.ToolCalls))

	// Step 4: Parse LLM response
	sourceFactIDs := make([]string, len(facts))
	for i, fact := range facts {
		sourceFactIDs[i] = fact.ID
	}

	report, err := parseConsolidationResponse(llmResponse, sourceFactIDs)
	if err != nil {
		logger.Error("Failed to parse consolidation response", "error", err)
		return nil, fmt.Errorf("parsing consolidation response: %w", err)
	}

	report.Topic = topic
	logger.Info("Memory consolidation completed", "consolidated_facts", len(report.ConsolidatedFacts))

	return report, nil
}

// ConsolidateMemoriesByTag performs memory consolidation for a given tag.
// It fetches relevant facts, runs LLM consolidation, and returns a comprehensive report.
func ConsolidateMemoriesByTag(ctx context.Context, tag string, deps ConsolidationDependencies) (*ConsolidationReport, error) {
	logger := deps.Logger.With("tag", tag)
	logger.Info("Starting memory consolidation")

	// Step 1: Fetch facts by tag
	facts, err := fetchFactsByTag(ctx, tag, deps.Storage)
	if err != nil {
		logger.Error("Failed to fetch facts by tag", "error", err)
		return nil, fmt.Errorf("fetching facts by tag: %w", err)
	}

	if len(facts) == 0 {
		logger.Warn("No facts found for tag")
		return &ConsolidationReport{
			Topic:             tag,
			Summary:           fmt.Sprintf("No memories found for topic: %s", tag),
			ConsolidatedFacts: []*ConsolidationFact{},
			SourceFactCount:   0,
			GeneratedAt:       time.Now(),
		}, nil
	}

	logger.Info("Found facts for consolidation", "count", len(facts))

	// Step 2: Prepare LLM input
	factsContent := buildFactsContent(facts)
	logger.Debug("Built facts content", "length", len(factsContent))

	// Step 3: Run LLM consolidation
	consolidationTools := []openai.ChatCompletionToolParam{consolidateMemoriesTool}

	llmMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(MemoryConsolidationPrompt),
		openai.UserMessage(fmt.Sprintf("Topic: %s\n\nFacts to consolidate:\n%s", tag, factsContent)),
	}

	logger.Debug("Sending consolidation request to LLM", "facts_count", len(facts))

	llmResponse, err := deps.CompletionsService.Completions(ctx, llmMessages, consolidationTools, deps.CompletionsModel)
	if err != nil {
		logger.Error("LLM consolidation failed", "error", err)
		return nil, fmt.Errorf("LLM consolidation: %w", err)
	}

	logger.Debug("LLM consolidation completed", "tool_calls", len(llmResponse.ToolCalls))

	// Step 4: Parse LLM response
	sourceFactIDs := make([]string, len(facts))
	for i, fact := range facts {
		sourceFactIDs[i] = fact.ID
	}

	report, err := parseConsolidationResponse(llmResponse, sourceFactIDs)
	if err != nil {
		logger.Error("Failed to parse consolidation response", "error", err)
		return nil, fmt.Errorf("parsing consolidation response: %w", err)
	}

	report.Topic = tag
	logger.Info("Memory consolidation completed", "consolidated_facts", len(report.ConsolidatedFacts))

	return report, nil
}

// ConsolidationDependencies contains all dependencies needed for consolidation.
type ConsolidationDependencies struct {
	Logger             *log.Logger
	Storage            memory.Storage
	CompletionsService *ai.Service
	CompletionsModel   string
}

// buildFactsContent creates a formatted string of facts for LLM input.
func buildFactsContent(facts []*memory.MemoryFact) string {
	var builder strings.Builder

	for i, fact := range facts {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s - %s: %s",
			i+1,
			fact.Category,
			fact.Subject,
			fact.Attribute,
			fact.Value))

		if fact.TemporalContext != nil {
			builder.WriteString(fmt.Sprintf(" (%s)", *fact.TemporalContext))
		}

		builder.WriteString(fmt.Sprintf(" [Importance: %d, Sensitivity: %s]\n",
			fact.Importance,
			fact.Sensitivity))
	}

	return builder.String()
}
