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

// ForComponent creates a logger for a specific component.
func (lf *Factory) ForComponent(id string) *log.Logger {
	// Register the component if not already registered
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeUtility, nil)

	// Get logger with proper level and context
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForService creates a logger for service components.
func (lf *Factory) ForService(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeService, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForManager creates a logger for manager components.
func (lf *Factory) ForManager(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeManager, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForHandler creates a logger for handler components.
func (lf *Factory) ForHandler(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeHandler, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForResolver creates a logger for GraphQL resolver components.
func (lf *Factory) ForResolver(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeResolver, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForRepository creates a logger for repository components.
func (lf *Factory) ForRepository(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeRepository, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForWorker creates a logger for worker components.
func (lf *Factory) ForWorker(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWorker, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForClient creates a logger for client components.
func (lf *Factory) ForClient(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeClient, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForServer creates a logger for server components.
func (lf *Factory) ForServer(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeServer, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForMiddleware creates a logger for middleware components.
func (lf *Factory) ForMiddleware(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMiddleware, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// AI and ML specific loggers.
func (lf *Factory) ForAI(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAI, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForAnonymizer(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAnonymizer, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForEmbedding(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeEmbedding, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForCompletions(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeCompletions, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Data processing specific loggers.
func (lf *Factory) ForProcessor(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeProcessor, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForWorkflow(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWorkflow, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForIntegration(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeIntegration, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForParser(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeParser, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Communication specific loggers.
func (lf *Factory) ForTelegram(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTelegram, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForWhatsApp(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWhatsApp, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForSlack(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeSlack, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForGmail(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeGmail, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForMCP(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMCP, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Infrastructure specific loggers.
func (lf *Factory) ForDatabase(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeDatabase, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForNATS(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeNATS, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForTemporal(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTemporal, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForDirectory(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeDirectory, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Identity and auth specific loggers.
func (lf *Factory) ForIdentity(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeIdentity, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForAuth(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAuth, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForOAuth(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeOAuth, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Chat and memory specific loggers.
func (lf *Factory) ForChat(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeChat, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForMemory(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMemory, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForTwinChat(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTwinChat, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *Factory) ForTTS(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTTS, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
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
