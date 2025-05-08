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

	// TODO: Schedule will be handled by the parent workflow
	// RRULE-formatted schedule (optional) -- handled by the parent
	Schedule string `json:"schedule,omitempty"`

	// Current execution progress
	CurrentStep int `json:"current_step"`

	// Execution metadata
	StartedAt time.Time `json:"started_at"`

	// Flag indicating if execution is complete
	CompletedAt time.Time `json:"completed_at"`

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
	// except it tracks the type of each entry (thought, action, etc.)
	// and the timestamp of each entry. (used for `UpdatedAt`)
	History []HistoryEntry `json:"history"`

	// Image URLs generated (if any)
	ImageURLs []string `json:"image_urls"`
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
	// Schedule string `json:"schedule,omitempty"`

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
		// Use our robust helper function to create a structured result
		result, err := createStructuredToolResult(rawMsg)
		if err != nil {
			return fmt.Errorf("failed to create structured tool result for index %d: %w", i, err)
		}
		ps.ToolResults[i] = result
	}
	return nil
}

// This function is robust to different JSON formats that might come from different tool implementations.
func createStructuredToolResult(rawMsg json.RawMessage) (*types.StructuredToolResult, error) {
	// First, attempt to unmarshal to see what type of structure it has
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawMsg, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into map: %w. JSON: %s", err, string(rawMsg))
	}

	// Create a structured result regardless of the input format
	structuredResult := &types.StructuredToolResult{
		ToolParams: make(map[string]interface{}),
		Output:     make(map[string]interface{}),
	}

	// Try to populate from the raw data
	if toolName, ok := rawData["tool"].(string); ok {
		structuredResult.ToolName = toolName
	}

	// Handle params field
	if params, ok := rawData["params"].(map[string]interface{}); ok {
		structuredResult.ToolParams = params
	}

	// Handle content in different formats
	if content, ok := rawData["content"].(string); ok {
		structuredResult.Output["content"] = content
	} else if data, ok := rawData["data"].(map[string]interface{}); ok {
		structuredResult.Output = data
		// Make sure there's a content field
		if _, hasContent := data["content"]; !hasContent {
			if dataStr, err := json.Marshal(data); err == nil {
				structuredResult.Output["content"] = string(dataStr)
			}
		}
	} else if output, ok := rawData["output"].(map[string]interface{}); ok {
		structuredResult.Output = output
	} else {
		// As a fallback, include the entire raw data as content
		if dataStr, err := json.Marshal(rawData); err == nil {
			structuredResult.Output["content"] = string(dataStr)
		}
	}

	// Handle error field
	if errorMsg, ok := rawData["error"].(string); ok {
		structuredResult.ToolError = errorMsg
	}

	return structuredResult, nil
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
