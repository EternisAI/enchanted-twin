package logging

import (
	"github.com/charmbracelet/log"
)

// Factory provides component-aware loggers with consistent field naming.
type Factory struct {
	baseLogger        *log.Logger
	componentRegistry *ComponentRegistry
}

// NewFactory creates a new logger factory.
func NewFactory(baseLogger *log.Logger) *Factory {
	return &Factory{
		baseLogger:        baseLogger,
		componentRegistry: NewComponentRegistry(),
	}
}

// NewFactoryWithConfig creates a new logger factory and loads component log levels from config.
func NewFactoryWithConfig(baseLogger *log.Logger, componentLogLevels map[string]string) *Factory {
	registry := NewComponentRegistry()
	registry.LoadLogLevelsFromConfig(componentLogLevels)

	return &Factory{
		baseLogger:        baseLogger,
		componentRegistry: registry,
	}
}

// getLoggerForComponent is a private helper that centralizes component registration
// and logger retrieval logic, ensuring consistent error handling.
func (lf *Factory) getLoggerForComponent(id string, componentType ComponentType, metadata map[string]interface{}) *log.Logger {
	if err := lf.componentRegistry.RegisterComponent(id, componentType, metadata); err != nil {
		// Log the registration error but continue to provide a logger
		// This ensures the application doesn't crash due to registration issues
		lf.baseLogger.Error("failed to register component", "component_id", id, "component_type", componentType, "error", err)
	}

	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForComponent creates a logger for a specific component.
func (lf *Factory) ForComponent(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeUtility, nil)
}

// ForComponentWithMetadata creates a logger for a specific component with metadata.
func (lf *Factory) ForComponentWithMetadata(id string, metadata map[string]interface{}) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeUtility, metadata)
}

// ForService creates a logger for service components.
func (lf *Factory) ForService(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeService, nil)
}

// ForServiceWithMetadata creates a logger for service components with metadata.
func (lf *Factory) ForServiceWithMetadata(id string, metadata map[string]interface{}) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeService, metadata)
}

// ForManager creates a logger for manager components.
func (lf *Factory) ForManager(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeManager, nil)
}

// ForManagerWithMetadata creates a logger for manager components with metadata.
func (lf *Factory) ForManagerWithMetadata(id string, metadata map[string]interface{}) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeManager, metadata)
}

// ForHandler creates a logger for handler components.
func (lf *Factory) ForHandler(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeHandler, nil)
}

// ForResolver creates a logger for GraphQL resolver components.
func (lf *Factory) ForResolver(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeResolver, nil)
}

// ForRepository creates a logger for repository components.
func (lf *Factory) ForRepository(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeRepository, nil)
}

// ForWorker creates a logger for worker components.
func (lf *Factory) ForWorker(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeWorker, nil)
}

// ForClient creates a logger for client components.
func (lf *Factory) ForClient(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeClient, nil)
}

// ForServer creates a logger for server components.
func (lf *Factory) ForServer(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeServer, nil)
}

// ForMiddleware creates a logger for middleware components.
func (lf *Factory) ForMiddleware(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeMiddleware, nil)
}

// AI and ML specific loggers.
func (lf *Factory) ForAI(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeAI, nil)
}

func (lf *Factory) ForAnonymizer(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeAnonymizer, nil)
}

func (lf *Factory) ForEmbedding(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeEmbedding, nil)
}

func (lf *Factory) ForCompletions(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeCompletions, nil)
}

// Data processing specific loggers.
func (lf *Factory) ForProcessor(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeProcessor, nil)
}

func (lf *Factory) ForWorkflow(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeWorkflow, nil)
}

func (lf *Factory) ForIntegration(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeIntegration, nil)
}

func (lf *Factory) ForParser(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeParser, nil)
}

// Communication specific loggers.
func (lf *Factory) ForTelegram(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeTelegram, nil)
}

func (lf *Factory) ForWhatsApp(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeWhatsApp, nil)
}

func (lf *Factory) ForSlack(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeSlack, nil)
}

func (lf *Factory) ForGmail(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeGmail, nil)
}

func (lf *Factory) ForMCP(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeMCP, nil)
}

// Infrastructure specific loggers.
func (lf *Factory) ForDatabase(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeDatabase, nil)
}

func (lf *Factory) ForNATS(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeNATS, nil)
}

func (lf *Factory) ForTemporal(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeTemporal, nil)
}

func (lf *Factory) ForDirectory(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeDirectory, nil)
}

// Identity and auth specific loggers.
func (lf *Factory) ForIdentity(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeIdentity, nil)
}

func (lf *Factory) ForAuth(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeAuth, nil)
}

func (lf *Factory) ForOAuth(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeOAuth, nil)
}

// Chat and memory specific loggers.
func (lf *Factory) ForChat(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeChat, nil)
}

func (lf *Factory) ForMemory(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeMemory, nil)
}

func (lf *Factory) ForTwinChat(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeTwinChat, nil)
}

func (lf *Factory) ForTTS(id string) *log.Logger {
	return lf.getLoggerForComponent(id, ComponentTypeTTS, nil)
}

// WithContext adds additional context to a logger.
func (lf *Factory) WithContext(logger *log.Logger, key string, value interface{}) *log.Logger {
	return logger.With(key, value)
}

// WithRequestID adds request correlation ID to a logger.
func (lf *Factory) WithRequestID(logger *log.Logger, requestID string) *log.Logger {
	return logger.With("request_id", requestID)
}

// WithUserID adds user context to a logger.
func (lf *Factory) WithUserID(logger *log.Logger, userID string) *log.Logger {
	return logger.With("user_id", userID)
}

// WithError adds error context to a logger.
func (lf *Factory) WithError(logger *log.Logger, err error) *log.Logger {
	if err != nil {
		return logger.With("error", err.Error(), "error_type", "error")
	}
	return logger
}

// WithOperation adds operation context to a logger.
func (lf *Factory) WithOperation(logger *log.Logger, operation string) *log.Logger {
	return logger.With("operation", operation)
}

// GetComponentRegistry returns the component registry for configuration.
func (lf *Factory) GetComponentRegistry() *ComponentRegistry {
	return lf.componentRegistry
}

// SetComponentLogLevel sets the logging level for a specific component.
func (lf *Factory) SetComponentLogLevel(id string, level log.Level) error {
	return lf.componentRegistry.SetComponentLogLevel(id, level)
}

// GetComponentLogLevel gets the logging level for a specific component.
func (lf *Factory) GetComponentLogLevel(id string) log.Level {
	return lf.componentRegistry.GetComponentLogLevel(id)
}

// EnableComponent enables or disables a component.
func (lf *Factory) EnableComponent(id string, enabled bool) error {
	return lf.componentRegistry.EnableComponent(id, enabled)
}

// IsComponentEnabled checks if a component is enabled.
func (lf *Factory) IsComponentEnabled(id string) bool {
	return lf.componentRegistry.IsComponentEnabled(id)
}

// GetComponentStats returns statistics about registered components.
func (lf *Factory) GetComponentStats() map[string]interface{} {
	return lf.componentRegistry.GetComponentStats()
}

// ListComponentTypes returns all registered component types.
func (lf *Factory) ListComponentTypes() []ComponentType {
	return lf.componentRegistry.ListComponentTypes()
}

// ListComponentsByType returns all components of a specific type.
func (lf *Factory) ListComponentsByType(componentType ComponentType) []*ComponentInfo {
	return lf.componentRegistry.ListComponentsByType(componentType)
}

// LoadLogLevelsFromEnv loads component-specific log levels from environment variables.
func (lf *Factory) LoadLogLevelsFromEnv() {
	lf.componentRegistry.LoadLogLevelsFromEnv()
}
