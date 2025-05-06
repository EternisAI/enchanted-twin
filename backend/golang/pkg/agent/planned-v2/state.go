package plannedv2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// ToolCall represents a tool call in a serializable format.
// It embeds the ai.ToolCall struct and adds the execution result.
type ToolCall struct {
	ai.ToolCall
	// Result of the tool execution (nil if not yet executed)
	Result types.ToolResult `json:"result,omitempty"`
}

// PlanState represents the unified state for planned agent execution.
type PlanState struct {
	// Name of the task this plan will address
	Name string `json:"name"`

	// The plan text that the agent should follow
	Plan string `json:"plan"`

	// RRULE-formatted schedule (optional)
	Schedule string `json:"schedule,omitempty"`

	// Current execution progress
	CurrentStep int `json:"current_step"`

	// Flag indicating if execution is complete
	Complete bool `json:"complete"`

	// Final output when plan is complete
	Output string `json:"output"`

	// Error message if execution failed
	Error string `json:"error,omitempty"`

	// Message history in serializable format
	Messages []ai.Message `json:"messages"`

	// SelectedTools represents the tools available for execution
	SelectedTools []string `json:"selected_tools,omitempty"`

	// Tool calls made in a serializable format
	ToolCalls []ToolCall `json:"tool_calls"`

	// Tool results (results from executed tools)
	ToolResults []types.ToolResult `json:"tool_results"`

	// Typed history entries (for structured logging and UI)
	// NOTE: currently this mostly duplicates the Messages field
	// it may be useful for future UI or logging needs
	History []HistoryEntry `json:"history"`

	// Available tools for the agent
	// Tools []types.ToolDef `json:"tools"`

	// Image URLs generated (if any)
	ImageURLs []string `json:"image_urls"`

	// Execution metadata
	StartTime time.Time `json:"start_time"`
}

// HistoryEntry represents a single entry in the agent's execution history.
type HistoryEntry struct {
	// Type of entry: "thought", "action", "observation", "system", "error"
	Type string `json:"type"`

	// Content of the entry
	Content string `json:"content"`

	// Timestamp when the entry was created
	Timestamp time.Time `json:"timestamp"`
}

// ActionRequest represents a request to execute a tool.
type ActionRequest struct {
	// Name of the tool to use
	Tool string `json:"tool"`

	// Parameters for the tool
	Params map[string]any `json:"params"`
}

// PlanInput represents the input for the planned agent workflow.
type PlanInput struct {
	// Origin of the tool call
	Origin map[string]any `json:"origin"`

	// Name of the task this plan will address
	Name string `json:"name"`

	// RRULE-formatted schedule (optional)
	Schedule string `json:"schedule,omitempty"`

	// The plan text that the agent should follow
	Plan string `json:"plan"`

	// List of tool names that the agent can use
	ToolNames []string `json:"tool_names,omitempty"`

	// LLM model to use for completions
	Model string `json:"model"`

	// Maximum number of steps to execute
	MaxSteps int `json:"max_steps"`

	// System prompt to use (optional)
	SystemPrompt string `json:"system_prompt,omitempty"`
}

// UnmarshalJSON custom unmarshaler for PlanState.
func (ps *PlanState) UnmarshalJSON(data []byte) error {
	// Alias type to avoid recursion during unmarshaling
	type Alias PlanState
	aux := &struct {
		ToolResults []json.RawMessage `json:"tool_results"`
		*Alias
	}{
		Alias: (*Alias)(ps),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	ps.ToolResults = make([]types.ToolResult, len(aux.ToolResults))
	for i, rawMsg := range aux.ToolResults {
		// Attempt to unmarshal into the most common concrete type
		// In a real scenario with multiple concrete types, you might need
		// a "type" field in the JSON to decide which struct to unmarshal into.
		var structuredResult types.StructuredToolResult
		if err := json.Unmarshal(rawMsg, &structuredResult); err == nil {
			ps.ToolResults[i] = &structuredResult
		} else {
			return fmt.Errorf("failed to unmarshal tool_results[%d] into StructuredToolResult: %w. JSON: %s", i, err, string(rawMsg))
		}
	}
	return nil
}

// ToolCallsFromOpenAI converts OpenAI tool calls to our custom format.
func ToolCallsFromOpenAI(openaiToolCalls []openai.ChatCompletionMessageToolCall) []ToolCall {
	customToolCalls := make([]ToolCall, 0, len(openaiToolCalls))
	for _, tc := range openaiToolCalls {
		customToolCalls = append(customToolCalls, ToolCall{
			ToolCall: ai.FromOpenAIToolCall(tc),
		})
	}
	return customToolCalls
}
