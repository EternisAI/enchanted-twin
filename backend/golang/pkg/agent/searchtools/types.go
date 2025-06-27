package searchtools

import (
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// TemporalContext captures parsed temporal references in a query.
// For example, "last week" will be converted to explicit start / end bounds.
type TemporalContext struct {
	Reference string     `json:"reference,omitempty"` // e.g. "last_week"
	Start     *time.Time `json:"start,omitempty"`
	End       *time.Time `json:"end,omitempty"`
}

// Entity represents a person, topic or organisation referenced in a query.
type Entity struct {
	Type  string `json:"type,omitempty"`  // person | topic | organisation
	Value string `json:"value,omitempty"` // "Jon"
	Role  string `json:"role,omitempty"`  // sender | recipient | subject
}

// QueryIntent is the structured interpretation of the natural language query
// produced by the QueryAnalyzer tool.
type QueryIntent struct {
	IntentType      string           `json:"intent_type"`                // e.g. pending_action | content_search
	TemporalContext *TemporalContext `json:"temporal_context,omitempty"` // parsed temporal information
	Entities        []Entity         `json:"entities,omitempty"`
	ContentType     string           `json:"content_type,omitempty"` // link | document | message
	ActionRequired  bool             `json:"action_required"`
}

// SearchStepType enumerates supported executor step kinds.
type SearchStepType string

const (
	StepFilterMemories     SearchStepType = "filter_memories"
	StepCheckConversations SearchStepType = "check_conversations"
)

// SearchStep describes an individual step in a SearchPlan.
type SearchStep struct {
	Type        SearchStepType `json:"type"`
	Query       string         `json:"query,omitempty"`
	Filter      *memory.Filter `json:"filter,omitempty"`
	DependsOn   []int          `json:"depends_on,omitempty"`
	ResultField string         `json:"result_field,omitempty"`
}

// SearchPlan is produced by the SearchPlan tool and ultimately executed by SearchExecutor.
type SearchPlan struct {
	Steps []SearchStep `json:"steps"`
}

// SearchExecutionResult bundles per-step outputs returned by SearchExecutor.
type SearchExecutionResult struct {
	StepResults map[int]any `json:"step_results"`
}
