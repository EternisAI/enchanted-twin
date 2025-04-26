package types

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// ToolResult contains the result of tool execution.
type ToolResult struct {
	// Name of the tool that was executed
	Tool string `json:"tool"`

	// Parameters used for the execution
	Params map[string]any `json:"params"`

	// Content result from the tool
	Content string `json:"content"`

	// Structured result data (if any)
	Data any `json:"data,omitempty"`

	// Image URLs produced (if any)
	ImageURLs []string `json:"image_urls,omitempty"`

	// Error message (if execution failed)
	Error string `json:"error,omitempty"`
}

// ToOpenAIToolParam converts a ToolDef to OpenAI tool format.
func (t ToolDef) ToOpenAIToolParam() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: param.NewOpt(t.Description),
			Parameters:  map[string]any(t.Parameters),
		},
	}
}

// GetParameters returns the Parameters field as a map[string]any.
func (t ToolDef) GetParameters() map[string]any {
	return map[string]any(t.Parameters)
}

// GetReturns returns the Returns field as a map[string]any.
func (t ToolDef) GetReturns() map[string]any {
	return map[string]any(t.Returns)
}
