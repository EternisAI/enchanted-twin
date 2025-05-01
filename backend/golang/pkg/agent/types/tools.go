package types

import (
	"context"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// ToolResult defines the interface for results returned by tools
type ToolResult interface {
	// Tool returns the name of the tool that was executed
	Tool() string

	// Content returns the text content result from the tool
	Content() string

	// Data returns any structured data from the tool
	Data() any

	// ImageURLs returns any image URLs produced by the tool
	ImageURLs() []string

	// Error returns any error message if execution failed
	Error() string

	// Params returns the parameters used for execution
	Params() map[string]any
}

// StructuredToolResult is a standard implementation of ToolResult
type StructuredToolResult struct {
	ToolName string         `json:"tool"`
	Params   map[string]any `json:"params"`
	Output   map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func (t *StructuredToolResult) Content() string {
	if content, ok := t.Output["content"].(string); ok {
		return content
	}

	return ""
}

// SimpleToolResult creates a minimal tool result with just content
func SimpleToolResult(content string) *StructuredToolResult {
	return &StructuredToolResult{
		ToolContent: content,
		Params:      make(map[string]any),
	}
}

// ImageToolResult creates a tool result with image URLs
func ImageToolResult(content string, imageURLs []string) *StructuredToolResult {
	return &StructuredToolResult{
		ToolContent: content,
		ToolImages:  imageURLs,
		Params:      make(map[string]any),
	}
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

// Tool defines the interface for all executable tools
type Tool interface {
	// Definition returns the tool metadata
	Definition() openai.ChatCompletionToolParam

	// Execute runs the tool with given inputs
	Execute(ctx context.Context, inputs map[string]any) (ToolResult, error)
}
