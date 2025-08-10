package bootstrap

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// ComponentType represents the type of a component based on actual project components.
type ComponentType string

const (
	// Core service types.
	ComponentTypeService    ComponentType = "service"
	ComponentTypeManager    ComponentType = "manager"
	ComponentTypeHandler    ComponentType = "handler"
	ComponentTypeResolver   ComponentType = "resolver"
	ComponentTypeRepository ComponentType = "repository"
	ComponentTypeWorker     ComponentType = "worker"
	ComponentTypeClient     ComponentType = "client"
	ComponentTypeServer     ComponentType = "server"
	ComponentTypeMiddleware ComponentType = "middleware"
	ComponentTypeUtility    ComponentType = "utility"

	// AI and ML components.
	ComponentTypeAI          ComponentType = "ai"
	ComponentTypeAnonymizer  ComponentType = "anonymizer"
	ComponentTypeEmbedding   ComponentType = "embedding"
	ComponentTypeCompletions ComponentType = "completions"

	// Data processing components.
	ComponentTypeProcessor   ComponentType = "processor"
	ComponentTypeWorkflow    ComponentType = "workflow"
	ComponentTypeIntegration ComponentType = "integration"
	ComponentTypeParser      ComponentType = "parser"

	// Communication components.
	ComponentTypeTelegram ComponentType = "telegram"
	ComponentTypeWhatsApp ComponentType = "whatsapp"
	ComponentTypeSlack    ComponentType = "slack"
	ComponentTypeGmail    ComponentType = "gmail"
	ComponentTypeMCP      ComponentType = "mcp"

	// Infrastructure components.
	ComponentTypeDatabase  ComponentType = "database"
	ComponentTypeNATS      ComponentType = "nats"
	ComponentTypeTemporal  ComponentType = "temporal"
	ComponentTypeDirectory ComponentType = "directory"

	// Identity and auth components.
	ComponentTypeIdentity ComponentType = "identity"
	ComponentTypeAuth     ComponentType = "auth"
	ComponentTypeOAuth    ComponentType = "oauth"

	// Chat and memory components.
	ComponentTypeChat     ComponentType = "chat"
	ComponentTypeMemory   ComponentType = "memory"
	ComponentTypeTwinChat ComponentType = "twinchat"
	ComponentTypeTTS      ComponentType = "tts"
)

// ComponentInfo contains information about a registered component.
type ComponentInfo struct {
	ID        string // Unique component identifier
	Type      ComponentType
	LogLevel  log.Level
	Enabled   bool
	Metadata  map[string]interface{}
	CreatedAt int64 // Unix timestamp
}

// ComponentRegistry manages all registered components and their logging configuration.
type ComponentRegistry struct {
	mu         sync.RWMutex
	components map[string]*ComponentInfo
	logLevels  map[string]log.Level
	types      map[ComponentType][]string // Track components by type
}

// NewComponentRegistry creates a new component registry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		components: make(map[string]*ComponentInfo),
		logLevels:  make(map[string]log.Level),
		types:      make(map[ComponentType][]string),
	}
}

// RegisterComponent registers a new component with the registry.
func (cr *ComponentRegistry) RegisterComponent(id string, componentType ComponentType, metadata map[string]interface{}) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if _, exists := cr.components[id]; exists {
		return fmt.Errorf("component already registered: %s", id)
	}

	info := &ComponentInfo{
		ID:        id,
		Type:      componentType,
		LogLevel:  log.InfoLevel, // Default log level
		Enabled:   true,
		Metadata:  metadata,
		CreatedAt: getCurrentTimestamp(),
	}

	cr.components[id] = info

	// Track components by type
	if cr.types[componentType] == nil {
		cr.types[componentType] = make([]string, 0)
	}
	cr.types[componentType] = append(cr.types[componentType], id)

	return nil
}

// SetComponentLogLevel sets the logging level for a specific component.
func (cr *ComponentRegistry) SetComponentLogLevel(id string, level log.Level) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if info, exists := cr.components[id]; exists {
		info.LogLevel = level
		cr.logLevels[id] = level
		return nil
	}

	return fmt.Errorf("component not found: %s", id)
}

// LoadLogLevelsFromEnv loads component-specific log levels from environment variables.
func (cr *ComponentRegistry) LoadLogLevelsFromEnv() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Look for environment variables with pattern LOG_LEVEL_<COMPONENT_ID>
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "LOG_LEVEL_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				// Extract component identifier from LOG_LEVEL_<COMPONENT_ID>
				componentID := strings.TrimPrefix(key, "LOG_LEVEL_")
				cr.logLevels[componentID] = parseLogLevel(value)
			}
		}
	}
}

// LoadLogLevelsFromConfig loads component-specific log levels from the config.
func (cr *ComponentRegistry) LoadLogLevelsFromConfig(componentLogLevels map[string]string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for componentID, levelStr := range componentLogLevels {
		level := parseLogLevel(levelStr)
		cr.logLevels[componentID] = level
	}
}

// GetComponentLogLevel gets the logging level for a specific component.
func (cr *ComponentRegistry) GetComponentLogLevel(id string) log.Level {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if level, exists := cr.logLevels[id]; exists {
		return level
	}

	// Return default level if not configured
	return log.InfoLevel
}

// GetComponentInfo gets information about a registered component.
func (cr *ComponentRegistry) GetComponentInfo(id string) (*ComponentInfo, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	info, exists := cr.components[id]
	return info, exists
}

// ListComponents returns all registered components.
func (cr *ComponentRegistry) ListComponents() []*ComponentInfo {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	components := make([]*ComponentInfo, 0, len(cr.components))
	for _, info := range cr.components {
		components = append(components, info)
	}
	return components
}

// ListComponentsByType returns all components of a specific type.
func (cr *ComponentRegistry) ListComponentsByType(componentType ComponentType) []*ComponentInfo {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	var components []*ComponentInfo
	for _, info := range cr.components {
		if info.Type == componentType {
			components = append(components, info)
		}
	}
	return components
}

// ListComponentTypes returns all registered component types.
func (cr *ComponentRegistry) ListComponentTypes() []ComponentType {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	types := make([]ComponentType, 0, len(cr.types))
	for componentType := range cr.types {
		types = append(types, componentType)
	}
	return types
}

// GetComponentsByType returns all component IDs of a specific type.
func (cr *ComponentRegistry) GetComponentsByType(componentType ComponentType) []string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if components, exists := cr.types[componentType]; exists {
		result := make([]string, len(components))
		copy(result, components)
		return result
	}
	return []string{}
}

// EnableComponent enables or disables a component.
func (cr *ComponentRegistry) EnableComponent(id string, enabled bool) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if info, exists := cr.components[id]; exists {
		info.Enabled = enabled
		return nil
	}

	return fmt.Errorf("component not found: %s", id)
}

// IsComponentEnabled checks if a component is enabled.
func (cr *ComponentRegistry) IsComponentEnabled(id string) bool {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if info, exists := cr.components[id]; exists {
		return info.Enabled
	}

	return false
}

// GetComponentStats returns statistics about registered components.
func (cr *ComponentRegistry) GetComponentStats() map[string]interface{} {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_components"] = len(cr.components)
	stats["total_types"] = len(cr.types)

	// Count by type
	typeCounts := make(map[ComponentType]int)
	for _, info := range cr.components {
		typeCounts[info.Type]++
	}
	stats["by_type"] = typeCounts

	return stats
}

// GetLoggerForComponent creates a logger for a specific component with proper level and context.
func (cr *ComponentRegistry) GetLoggerForComponent(baseLogger *log.Logger, id string) *log.Logger {
	level := cr.GetComponentLogLevel(id)
	enabled := cr.IsComponentEnabled(id)

	logger := baseLogger.With("component", id)

	if !enabled {
		// Set to highest level to effectively disable logging
		logger.SetLevel(log.ErrorLevel)
	} else {
		logger.SetLevel(level)
	}

	return logger
}

// parseLogLevel safely parses a log level string.
func parseLogLevel(levelStr string) log.Level {
	level, err := log.ParseLevel(levelStr)
	if err != nil {
		// Default to InfoLevel for invalid levels
		return log.InfoLevel
	}
	return level
}

// getCurrentTimestamp returns the current Unix timestamp.
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
