package bootstrap

import (
	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/logging"
)

// LoggerFactory provides component-aware loggers with consistent field naming.
// Deprecated: Use logging.Factory directly instead.
type LoggerFactory = logging.Factory

// NewLoggerFactory creates a new logger factory.
// Deprecated: Use logging.NewFactory instead.
func NewLoggerFactory(baseLogger *log.Logger) *LoggerFactory {
	return logging.NewFactory(baseLogger)
}

// NewLoggerFactoryWithConfig creates a new logger factory and loads component log levels from config.
// Deprecated: Use logging.NewFactoryWithConfig instead.
func NewLoggerFactoryWithConfig(baseLogger *log.Logger, componentLogLevels map[string]string) *LoggerFactory {
	return logging.NewFactoryWithConfig(baseLogger, componentLogLevels)
}
