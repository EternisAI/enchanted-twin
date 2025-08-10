package bootstrap

import (
	"github.com/charmbracelet/log"
)

// LoggerFactory provides component-aware loggers with consistent field naming.
type LoggerFactory struct {
	baseLogger        *log.Logger
	componentRegistry *ComponentRegistry
}

// NewLoggerFactory creates a new logger factory.
func NewLoggerFactory(baseLogger *log.Logger) *LoggerFactory {
	return &LoggerFactory{
		baseLogger:        baseLogger,
		componentRegistry: NewComponentRegistry(),
	}
}

// NewLoggerFactoryWithConfig creates a new logger factory and loads component log levels from config.
func NewLoggerFactoryWithConfig(baseLogger *log.Logger, componentLogLevels map[string]string) *LoggerFactory {
	registry := NewComponentRegistry()
	registry.LoadLogLevelsFromConfig(componentLogLevels)

	return &LoggerFactory{
		baseLogger:        baseLogger,
		componentRegistry: registry,
	}
}

// ForComponent creates a logger for a specific component.
func (lf *LoggerFactory) ForComponent(id string) *log.Logger {
	// Register the component if not already registered
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeUtility, nil)

	// Get logger with proper level and context
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForService creates a logger for service components.
func (lf *LoggerFactory) ForService(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeService, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForManager creates a logger for manager components.
func (lf *LoggerFactory) ForManager(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeManager, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForHandler creates a logger for handler components.
func (lf *LoggerFactory) ForHandler(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeHandler, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForResolver creates a logger for GraphQL resolver components.
func (lf *LoggerFactory) ForResolver(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeResolver, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForRepository creates a logger for repository components.
func (lf *LoggerFactory) ForRepository(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeRepository, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForWorker creates a logger for worker components.
func (lf *LoggerFactory) ForWorker(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWorker, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForClient creates a logger for client components.
func (lf *LoggerFactory) ForClient(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeClient, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForServer creates a logger for server components.
func (lf *LoggerFactory) ForServer(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeServer, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// ForMiddleware creates a logger for middleware components.
func (lf *LoggerFactory) ForMiddleware(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMiddleware, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// AI and ML specific loggers.
func (lf *LoggerFactory) ForAI(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAI, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForAnonymizer(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAnonymizer, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForEmbedding(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeEmbedding, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForCompletions(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeCompletions, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Data processing specific loggers.
func (lf *LoggerFactory) ForProcessor(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeProcessor, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForWorkflow(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWorkflow, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForIntegration(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeIntegration, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForParser(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeParser, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Communication specific loggers.
func (lf *LoggerFactory) ForTelegram(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTelegram, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForWhatsApp(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeWhatsApp, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForSlack(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeSlack, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForGmail(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeGmail, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForMCP(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMCP, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Infrastructure specific loggers.
func (lf *LoggerFactory) ForDatabase(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeDatabase, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForNATS(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeNATS, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForTemporal(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTemporal, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForDirectory(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeDirectory, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Identity and auth specific loggers.
func (lf *LoggerFactory) ForIdentity(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeIdentity, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForAuth(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeAuth, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForOAuth(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeOAuth, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// Chat and memory specific loggers.
func (lf *LoggerFactory) ForChat(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeChat, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForMemory(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeMemory, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForTwinChat(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTwinChat, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

func (lf *LoggerFactory) ForTTS(id string) *log.Logger {
	_ = lf.componentRegistry.RegisterComponent(id, ComponentTypeTTS, nil)
	return lf.componentRegistry.GetLoggerForComponent(lf.baseLogger, id)
}

// WithContext adds additional context to a logger.
func (lf *LoggerFactory) WithContext(logger *log.Logger, key string, value interface{}) *log.Logger {
	return logger.With(key, value)
}

// WithRequestID adds request correlation ID to a logger.
func (lf *LoggerFactory) WithRequestID(logger *log.Logger, requestID string) *log.Logger {
	return logger.With("request_id", requestID)
}

// WithUserID adds user context to a logger.
func (lf *LoggerFactory) WithUserID(logger *log.Logger, userID string) *log.Logger {
	return logger.With("user_id", userID)
}

// WithError adds error context to a logger.
func (lf *LoggerFactory) WithError(logger *log.Logger, err error) *log.Logger {
	if err != nil {
		return logger.With("error", err.Error(), "error_type", "error")
	}
	return logger
}

// WithOperation adds operation context to a logger.
func (lf *LoggerFactory) WithOperation(logger *log.Logger, operation string) *log.Logger {
	return logger.With("operation", operation)
}

// GetComponentRegistry returns the component registry for configuration.
func (lf *LoggerFactory) GetComponentRegistry() *ComponentRegistry {
	return lf.componentRegistry
}

// SetComponentLogLevel sets the logging level for a specific component.
func (lf *LoggerFactory) SetComponentLogLevel(id string, level log.Level) error {
	return lf.componentRegistry.SetComponentLogLevel(id, level)
}

// GetComponentLogLevel gets the logging level for a specific component.
func (lf *LoggerFactory) GetComponentLogLevel(id string) log.Level {
	return lf.componentRegistry.GetComponentLogLevel(id)
}

// EnableComponent enables or disables a component.
func (lf *LoggerFactory) EnableComponent(id string, enabled bool) error {
	return lf.componentRegistry.EnableComponent(id, enabled)
}

// IsComponentEnabled checks if a component is enabled.
func (lf *LoggerFactory) IsComponentEnabled(id string) bool {
	return lf.componentRegistry.IsComponentEnabled(id)
}

// GetComponentStats returns statistics about registered components.
func (lf *LoggerFactory) GetComponentStats() map[string]interface{} {
	return lf.componentRegistry.GetComponentStats()
}

// ListComponentTypes returns all registered component types.
func (lf *LoggerFactory) ListComponentTypes() []ComponentType {
	return lf.componentRegistry.ListComponentTypes()
}

// ListComponentsByType returns all components of a specific type.
func (lf *LoggerFactory) ListComponentsByType(componentType ComponentType) []*ComponentInfo {
	return lf.componentRegistry.ListComponentsByType(componentType)
}

// LoadLogLevelsFromEnv loads component-specific log levels from environment variables.
func (lf *LoggerFactory) LoadLogLevelsFromEnv() {
	lf.componentRegistry.LoadLogLevelsFromEnv()
}
