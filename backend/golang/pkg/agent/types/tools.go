package types

import (
	"context"

	"github.com/openai/openai-go"
)

// ToolResult defines the interface for results returned by tools.
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

// StructuredToolResult is a standard implementation of ToolResult.
type StructuredToolResult struct {
	ToolName   string         `json:"tool"`
	ToolParams map[string]any `json:"params"`
	Output     map[string]any `json:"data,omitempty"`
	ToolError  string         `json:"error,omitempty"`
}

func (t *StructuredToolResult) Tool() string {
	return t.ToolName
}

func (t *StructuredToolResult) Content() string {
	if content, ok := t.Output["content"].(string); ok {
		return content
	}

	return ""
}

func (t *StructuredToolResult) Data() any {
	if data, ok := t.Output["data"]; ok {
		return data
	}
	return t.Output
}

func (t *StructuredToolResult) ImageURLs() []string {
	imageURLs := make([]string, 0)

	if images, ok := t.Output["images"].([]string); ok {
		imageURLs = append(imageURLs, images...)
	}

	if content, ok := t.Output["content"].(string); ok {
		imageURLs = append(imageURLs, extractImageURLs(content)...)
	}

	return imageURLs
}

func (t *StructuredToolResult) Error() string {
	return t.ToolError
}

func (t *StructuredToolResult) Params() map[string]any {
	return t.ToolParams
}

// SimpleToolResult creates a minimal tool result with just content.
func SimpleToolResult(content string) *StructuredToolResult {
	return &StructuredToolResult{
		Output: map[string]any{
			"content": content,
		},
		ToolParams: make(map[string]any),
	}
}

// ImageToolResult creates a tool result with image URLs.
func ImageToolResult(content string, imageURLs []string) *StructuredToolResult {
	return &StructuredToolResult{
		Output: map[string]any{
			"content": content,
			"images":  imageURLs,
		},
		ToolParams: make(map[string]any),
	}
}

// Tool defines the interface for all executable tools.
type Tool interface {
	// Definition returns the tool metadata
	Definition() openai.ChatCompletionToolParam

	// Execute runs the tool with given inputs
	Execute(ctx context.Context, inputs map[string]any) (ToolResult, error)
}
