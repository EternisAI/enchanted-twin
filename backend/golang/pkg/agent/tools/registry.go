package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

// Registry manages the registration and retrieval of tools.
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	logger *log.Logger
}

// NewRegistry creates a new tool registry instance.
func NewRegistry(logger *log.Logger) *Registry {
	return &Registry{
		tools:  make(map[string]Tool),
		logger: logger,
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

	if r.logger != nil {
		r.logger.Debug("Registered tool", "name", toolName)
	}

	return nil
}

// RegisterBulk adds multiple tools at once.
func (r *Registry) RegisterBulk(tools []Tool) error {
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// Execute runs a tool by name with the given parameters.
func (r *Registry) Execute(
	ctx context.Context,
	name string,
	params map[string]interface{},
) (ToolResult, error) {
	tool, exists := r.Get(name)
	if !exists {
		return ToolResult{}, fmt.Errorf("tool '%s' not found", name)
	}

	return tool.Execute(ctx, params)
}

// Definitions returns OpenAI-compatible tool definitions for all registered tools.
func (r *Registry) Definitions() []openai.ChatCompletionToolParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]openai.ChatCompletionToolParam, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

// List returns a list of all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Global is the default registry instance shared across the application.
var (
	Global   *Registry
	initOnce sync.Once
)

// GetGlobal returns the global registry, creating it if needed.
func GetGlobal(logger *log.Logger) *Registry {
	initOnce.Do(func() {
		Global = NewRegistry(logger)
	})
	return Global
}
