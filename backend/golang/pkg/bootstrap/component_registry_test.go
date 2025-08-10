package bootstrap

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
)

func TestComponentRegistry_FlatHierarchy(t *testing.T) {
	registry := NewComponentRegistry()

	// Test registering components with flat IDs
	err := registry.RegisterComponent("holon.manager", ComponentTypeManager, map[string]interface{}{
		"package": "holon",
		"type":    "manager",
	})
	assert.NoError(t, err)

	err = registry.RegisterComponent("ai.anonymizer.gpt4", ComponentTypeAnonymizer, map[string]interface{}{
		"package": "ai",
		"type":    "anonymizer",
		"model":   "gpt4",
	})
	assert.NoError(t, err)

	err = registry.RegisterComponent("whatsapp.service", ComponentTypeService, map[string]interface{}{
		"package": "whatsapp",
		"type":    "service",
	})
	assert.NoError(t, err)

	// Test component retrieval
	info, exists := registry.GetComponentInfo("holon.manager")
	assert.True(t, exists)
	assert.Equal(t, "holon.manager", info.ID)
	assert.Equal(t, ComponentTypeManager, info.Type)
	assert.True(t, info.Enabled)

	// Test duplicate registration
	err = registry.RegisterComponent("holon.manager", ComponentTypeManager, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Test component listing
	components := registry.ListComponents()
	assert.Len(t, components, 3)

	// Test components by type
	managerComponents := registry.GetComponentsByType(ComponentTypeManager)
	assert.Len(t, managerComponents, 1)
	assert.Contains(t, managerComponents, "holon.manager")

	serviceComponents := registry.GetComponentsByType(ComponentTypeService)
	assert.Len(t, serviceComponents, 1)
	assert.Contains(t, serviceComponents, "whatsapp.service")

	// Test component types
	types := registry.ListComponentTypes()
	assert.Contains(t, types, ComponentTypeManager)
	assert.Contains(t, types, ComponentTypeAnonymizer)
	assert.Contains(t, types, ComponentTypeService)
}

func TestComponentRegistry_LogLevels(t *testing.T) {
	registry := NewComponentRegistry()

	// Register components
	_ = registry.RegisterComponent("test.component1", ComponentTypeUtility, nil)
	_ = registry.RegisterComponent("test.component2", ComponentTypeUtility, nil)

	// Test setting log levels
	err := registry.SetComponentLogLevel("test.component1", log.DebugLevel)
	assert.NoError(t, err)

	err = registry.SetComponentLogLevel("test.component2", log.WarnLevel)
	assert.NoError(t, err)

	// Test getting log levels
	level1 := registry.GetComponentLogLevel("test.component1")
	assert.Equal(t, log.DebugLevel, level1)

	level2 := registry.GetComponentLogLevel("test.component2")
	assert.Equal(t, log.WarnLevel, level2)

	// Test non-existent component
	level3 := registry.GetComponentLogLevel("non.existent")
	assert.Equal(t, log.InfoLevel, level3) // Default level
}

func TestComponentRegistry_EnableDisable(t *testing.T) {
	registry := NewComponentRegistry()

	// Register component
	_ = registry.RegisterComponent("test.component", ComponentTypeUtility, nil)

	// Test enable/disable
	err := registry.EnableComponent("test.component", false)
	assert.NoError(t, err)

	enabled := registry.IsComponentEnabled("test.component")
	assert.False(t, enabled)

	err = registry.EnableComponent("test.component", true)
	assert.NoError(t, err)

	enabled = registry.IsComponentEnabled("test.component")
	assert.True(t, enabled)

	// Test non-existent component
	err = registry.EnableComponent("non.existent", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestComponentRegistry_EnvironmentVariables(t *testing.T) {
	registry := NewComponentRegistry()

	// Set environment variables
	t.Setenv("LOG_LEVEL_test.component1", "debug")
	t.Setenv("LOG_LEVEL_test.component2", "warn")
	t.Setenv("LOG_LEVEL_test.component3", "error")

	// Load from environment
	registry.LoadLogLevelsFromEnv()

	// Verify levels were loaded
	level1 := registry.GetComponentLogLevel("test.component1")
	assert.Equal(t, log.DebugLevel, level1)

	level2 := registry.GetComponentLogLevel("test.component2")
	assert.Equal(t, log.WarnLevel, level2)

	level3 := registry.GetComponentLogLevel("test.component3")
	assert.Equal(t, log.ErrorLevel, level3)
}

func TestComponentRegistry_ConfigLoading(t *testing.T) {
	registry := NewComponentRegistry()

	// Test config loading
	config := map[string]string{
		"test.component1": "debug",
		"test.component2": "warn",
		"test.component3": "error",
	}

	registry.LoadLogLevelsFromConfig(config)

	// Verify levels were loaded
	level1 := registry.GetComponentLogLevel("test.component1")
	assert.Equal(t, log.DebugLevel, level1)

	level2 := registry.GetComponentLogLevel("test.component2")
	assert.Equal(t, log.WarnLevel, level2)

	level3 := registry.GetComponentLogLevel("test.component3")
	assert.Equal(t, log.ErrorLevel, level3)
}

func TestComponentRegistry_Statistics(t *testing.T) {
	registry := NewComponentRegistry()

	// Register components of different types
	_ = registry.RegisterComponent("test.manager1", ComponentTypeManager, nil)
	_ = registry.RegisterComponent("test.manager2", ComponentTypeManager, nil)
	_ = registry.RegisterComponent("test.service1", ComponentTypeService, nil)

	// Get statistics
	stats := registry.GetComponentStats()

	// Verify statistics
	assert.Equal(t, 3, stats["total_components"])
	assert.Equal(t, 2, stats["total_types"])

	byType, _ := stats["by_type"].(map[ComponentType]int)
	assert.Equal(t, 2, byType[ComponentTypeManager])
	assert.Equal(t, 1, byType[ComponentTypeService])
}

func TestComponentRegistry_LoggerCreation(t *testing.T) {
	registry := NewComponentRegistry()
	baseLogger := log.New(nil) // Use nil writer for testing

	// Register component with specific log level
	_ = registry.RegisterComponent("test.component", ComponentTypeUtility, nil)
	_ = registry.SetComponentLogLevel("test.component", log.DebugLevel)

	// Create logger for component
	logger := registry.GetLoggerForComponent(baseLogger, "test.component")

	// Verify logger has correct level
	assert.Equal(t, log.DebugLevel, logger.GetLevel())
}

func TestComponentRegistry_InvalidLogLevels(t *testing.T) {
	registry := NewComponentRegistry()

	// Test invalid log level parsing
	registry.LoadLogLevelsFromConfig(map[string]string{
		"test.component": "invalid_level",
	})

	// Should default to InfoLevel
	level := registry.GetComponentLogLevel("test.component")
	assert.Equal(t, log.InfoLevel, level)
}

func TestComponentRegistry_ComponentTypes(t *testing.T) {
	registry := NewComponentRegistry()

	// Test all component types
	types := []ComponentType{
		ComponentTypeService,
		ComponentTypeManager,
		ComponentTypeHandler,
		ComponentTypeResolver,
		ComponentTypeRepository,
		ComponentTypeWorker,
		ComponentTypeClient,
		ComponentTypeServer,
		ComponentTypeMiddleware,
		ComponentTypeUtility,
		ComponentTypeAI,
		ComponentTypeAnonymizer,
		ComponentTypeEmbedding,
		ComponentTypeCompletions,
		ComponentTypeProcessor,
		ComponentTypeWorkflow,
		ComponentTypeIntegration,
		ComponentTypeParser,
		ComponentTypeTelegram,
		ComponentTypeWhatsApp,
		ComponentTypeSlack,
		ComponentTypeGmail,
		ComponentTypeMCP,
		ComponentTypeDatabase,
		ComponentTypeNATS,
		ComponentTypeTemporal,
		ComponentTypeDirectory,
		ComponentTypeIdentity,
		ComponentTypeAuth,
		ComponentTypeOAuth,
		ComponentTypeChat,
		ComponentTypeMemory,
		ComponentTypeTwinChat,
		ComponentTypeTTS,
	}

	// Register one component of each type
	for i, componentType := range types {
		componentID := fmt.Sprintf("test.%s.%d", componentType, i)
		err := registry.RegisterComponent(componentID, componentType, nil)
		assert.NoError(t, err)
	}

	// Verify all types are registered
	registeredTypes := registry.ListComponentTypes()
	assert.Len(t, registeredTypes, len(types))

	for _, componentType := range types {
		assert.Contains(t, registeredTypes, componentType)
	}
}
