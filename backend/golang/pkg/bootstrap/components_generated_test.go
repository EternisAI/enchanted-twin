package bootstrap

import (
	"testing"

	"github.com/charmbracelet/log"
)

func TestRegisterAllKnownComponents(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	loggerFactory := NewLoggerFactory(baseLogger)

	// Call the generated registration function
	RegisterAllKnownComponents(loggerFactory)

	// Get the registry and validate components
	registry := loggerFactory.GetComponentRegistry()
	components := registry.ListComponents()

	// Validate we have components
	if len(components) == 0 {
		t.Fatal("No components registered")
	}

	// Validate each component has proper metadata
	for _, comp := range components {
		if comp.ID == "" {
			t.Errorf("Component has empty ID: %+v", comp)
		}

		if comp.Type == "" {
			t.Errorf("Component %s has empty Type", comp.ID)
		}

		// Validate component can be found in catalog
		if catalogType, exists := ComponentCatalog[comp.ID]; !exists {
			t.Errorf("Component %s not found in ComponentCatalog", comp.ID)
		} else if catalogType != comp.Type {
			t.Errorf("Component %s type mismatch: registry=%s, catalog=%s",
				comp.ID, comp.Type, catalogType)
		}

		// Validate component can be found in sources
		if source, exists := ComponentSources[comp.ID]; !exists {
			t.Errorf("Component %s not found in ComponentSources", comp.ID)
		} else if source == "" {
			t.Errorf("Component %s has empty source location", comp.ID)
		}

		// Validate component is enabled by default
		if !registry.IsComponentEnabled(comp.ID) {
			t.Errorf("Component %s should be enabled by default", comp.ID)
		}

		// Validate component has default log level
		level := registry.GetComponentLogLevel(comp.ID)
		if level != log.InfoLevel {
			t.Errorf("Component %s should have InfoLevel by default, got %s", comp.ID, level.String())
		}
	}

	t.Logf("Successfully validated %d components", len(components))
}

func TestComponentCatalogConsistency(t *testing.T) {
	// Ensure ComponentCatalog and ComponentSources have the same keys
	for id := range ComponentCatalog {
		if _, exists := ComponentSources[id]; !exists {
			t.Errorf("Component %s exists in ComponentCatalog but not in ComponentSources", id)
		}
	}

	for id := range ComponentSources {
		if _, exists := ComponentCatalog[id]; !exists {
			t.Errorf("Component %s exists in ComponentSources but not in ComponentCatalog", id)
		}
	}
}

func TestKnownComponentTypes(t *testing.T) {
	// Test that we have components of expected core types
	expectedTypes := []ComponentType{
		ComponentTypeService,
		ComponentTypeDatabase,
		ComponentTypeTemporal,
		ComponentTypeAI,
	}

	baseLogger := NewBootstrapLogger()
	loggerFactory := NewLoggerFactory(baseLogger)
	RegisterAllKnownComponents(loggerFactory)
	registry := loggerFactory.GetComponentRegistry()

	for _, expectedType := range expectedTypes {
		components := registry.ListComponentsByType(expectedType)
		if len(components) == 0 {
			t.Errorf("Expected to find components of type %s, but found none", expectedType)
		} else {
			t.Logf("Found %d components of type %s", len(components), expectedType)
		}
	}
}

func TestComponentRegistrationIsIdempotent(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	loggerFactory := NewLoggerFactory(baseLogger)

	// Register components twice
	RegisterAllKnownComponents(loggerFactory)
	componentsAfterFirst := loggerFactory.GetComponentRegistry().ListComponents()

	RegisterAllKnownComponents(loggerFactory)
	componentsAfterSecond := loggerFactory.GetComponentRegistry().ListComponents()

	// Should have the same number of components
	if len(componentsAfterFirst) != len(componentsAfterSecond) {
		t.Errorf("Component registration is not idempotent: first=%d, second=%d",
			len(componentsAfterFirst), len(componentsAfterSecond))
	}

	// Validate each component still exists and has the same metadata
	firstMap := make(map[string]*ComponentInfo)
	for _, comp := range componentsAfterFirst {
		firstMap[comp.ID] = comp
	}

	for _, comp := range componentsAfterSecond {
		if first, exists := firstMap[comp.ID]; !exists {
			t.Errorf("Component %s disappeared after second registration", comp.ID)
		} else if first.Type != comp.Type {
			t.Errorf("Component %s type changed: %s -> %s", comp.ID, first.Type, comp.Type)
		}
	}
}
