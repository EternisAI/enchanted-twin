package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

// ToolRegistry defines the contract for tool registries.
type ToolRegistry interface {
	// Register adds a tool to the registry.
	Register(tools ...Tool) error

	// Get retrieves a tool by name.
	Get(name string) (Tool, bool)

	// Execute runs a tool by name with the given parameters.
	Execute(ctx context.Context, name string, params map[string]interface{}) (ToolResult, error)

	// Definitions returns OpenAI-compatible tool definitions for all registered tools.
	Definitions() []openai.ChatCompletionToolParam

	// List returns a list of all registered tool names.
	List() []string
}

// ToolMapRegistry manages the registration and retrieval of tools.
type ToolMapRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new tool registry instance.
func NewRegistry() *ToolMapRegistry {
	return &ToolMapRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds tools to the registry.
func (r *ToolMapRegistry) Register(tools ...Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, tool := range tools {
		// Get the tool's definition
		def := tool.Definition()
		if def.Type != "function" {
			return fmt.Errorf("only function tools are supported, got %s", def.Type)
		}

		toolName := def.Function.Name
		if toolName == "" {
			return fmt.Errorf("tool name cannot be empty")
		}

		if _, exists := r.tools[toolName]; exists {
			return fmt.Errorf("tool '%s' is already registered", toolName)
		}

		// Store the tool
		r.tools[toolName] = tool
	}

	return nil
}

// Get retrieves a tool by name.
func (r *ToolMapRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// Execute runs a tool by name with the given parameters.
func (r *ToolMapRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (ToolResult, error) {
	tool, exists := r.Get(name)
	if !exists {
		return ToolResult{}, fmt.Errorf("tool '%s' not found", name)
	}

	return tool.Execute(ctx, params)
}

// Definitions returns OpenAI-compatible tool definitions for all registered tools.
func (r *ToolMapRegistry) Definitions() []openai.ChatCompletionToolParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]openai.ChatCompletionToolParam, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

// List returns a list of all registered tool names.
func (r *ToolMapRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Global is the default registry instance shared across the application.
var Global *ToolMapRegistry
var initOnce sync.Once

// GetGlobal returns the global registry, creating it if needed.
func GetGlobal(logger *log.Logger) *ToolMapRegistry {
	initOnce.Do(func() {
		Global = NewRegistry()
	})
	return Global
}
