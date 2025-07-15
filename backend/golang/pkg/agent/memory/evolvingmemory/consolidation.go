package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
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

// StoreConsolidationReports saves multiple consolidation reports using the modular storage approach.
// This is optimized for batch processing of multiple reports (e.g., comprehensive consolidation).
func StoreConsolidationReports(ctx context.Context, reports []*ConsolidationReport, memoryStorage MemoryStorage, progressCallback memory.ProgressCallback) error {
	if len(reports) == 0 {
		return nil
	}

	// Convert all consolidated facts to regular MemoryFacts for storage
	var allFacts []*memory.MemoryFact
	totalSourceFacts := 0
	totalConsolidatedFacts := 0

	for _, report := range reports {
		// Add summary as a special fact
		summaryFact := &memory.MemoryFact{
			ID:                 uuid.New().String(),
			Content:            report.Summary,
			Category:           "summary",
			Subject:            report.Topic,
			Attribute:          "consolidated_summary",
			Value:              report.Summary,
			Sensitivity:        "low",
			Importance:         3,
			Source:             "consolidation",
			Timestamp:          report.GeneratedAt,
			Tags:               []string{"consolidated", "summary", report.Topic},
			DocumentReferences: []string{},
			Metadata: map[string]string{
				"consolidation_type":  "summary",
				"source_fact_count":   fmt.Sprintf("%d", report.SourceFactCount),
				"consolidation_topic": report.Topic,
			},
		}
		allFacts = append(allFacts, summaryFact)

		// Add all consolidated facts from this report
		for _, consolidatedFact := range report.ConsolidatedFacts {
			// Create a copy of the MemoryFact
			fact := consolidatedFact.MemoryFact

			// Update metadata to track source facts
			if fact.Metadata == nil {
				fact.Metadata = make(map[string]string)
			}
			fact.Metadata["consolidation_type"] = "fact"
			fact.Metadata["consolidated_from_count"] = fmt.Sprintf("%d", len(consolidatedFact.ConsolidatedFrom))
			fact.Metadata["consolidation_topic"] = report.Topic

			// Store source fact IDs in metadata
			for idx, sourceID := range consolidatedFact.ConsolidatedFrom {
				fact.Metadata[fmt.Sprintf("source_fact_%d", idx)] = sourceID
			}

			// Ensure tags include consolidation markers
			fact.Tags = append(fact.Tags, "consolidated", report.Topic)

			allFacts = append(allFacts, &fact)
		}

		totalSourceFacts += report.SourceFactCount
		totalConsolidatedFacts += len(report.ConsolidatedFacts)
	}

	// Use the modular StoreFactsDirectly function for batch storage! 🚀
	return memoryStorage.StoreFactsDirectly(ctx, allFacts, progressCallback)
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

		// Convert to ConsolidationFacts with proper field population
		for _, rawFact := range args.ConsolidatedFacts {
			// Generate unique ID for consolidated fact
			consolidatedID := uuid.New().String()

			// Create properly populated MemoryFact
			fact := memory.MemoryFact{
				ID:              consolidatedID,
				Category:        rawFact.Category,
				Subject:         rawFact.Subject,
				Attribute:       rawFact.Attribute,
				Value:           rawFact.Value,
				TemporalContext: rawFact.TemporalContext,
				Sensitivity:     rawFact.Sensitivity,
				Importance:      rawFact.Importance,
				Source:          "consolidation",                            // Mark as consolidated fact
				Timestamp:       time.Now(),                                 // Current timestamp for consolidation
				Tags:            []string{"consolidated", rawFact.Category}, // Add consolidation tags
			}

			// Generate searchable content from structured fields
			fact.Content = fact.GenerateContent()

			// Map LLM-provided indices to actual source fact IDs (convert from 1-based to 0-based)
			var relevantSourceIDs []string
			for _, index := range rawFact.SourceFactIndices {
				if index >= 1 && index <= len(sourceFactIDs) {
					relevantSourceIDs = append(relevantSourceIDs, sourceFactIDs[index-1]) // Convert to 0-based
				}
			}

			// Fallback to all source facts if no specific indices provided
			if len(relevantSourceIDs) == 0 {
				relevantSourceIDs = sourceFactIDs // Use all as fallback
			}

			// Create ConsolidationFact with intelligent source tracking
			consolidatedFacts = append(consolidatedFacts, &ConsolidationFact{
				MemoryFact:       fact,
				ConsolidatedFrom: relevantSourceIDs, // Only facts that actually contributed to this insight
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

// === UTILITY FUNCTIONS ===

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

	result, err := deps.CompletionsService.Completions(ctx, llmMessages, consolidationTools, deps.CompletionsModel, ai.Background)
	if err != nil {
		logger.Error("LLM consolidation failed", "error", err)
		return nil, fmt.Errorf("LLM consolidation: %w", err)
	}
	llmResponse := result.Message

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
