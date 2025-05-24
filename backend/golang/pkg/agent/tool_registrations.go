package agent

import (
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
)

type ToolProvider interface {
	Tools() []tools.Tool
}

func RegisterMCPTools(registry tools.ToolRegistry, mcpTools []tools.Tool) []tools.Tool {
	registeredTools := []tools.Tool{}

	for _, tool := range mcpTools {
		if err := registry.Register(tool); err == nil {
			registeredTools = append(registeredTools, tool)
		}
	}

	return registeredTools
}
