package testing

import (
	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/logging"
)

// TestLoggerFactory provides a test-scoped logger factory for testing purposes.
var TestLoggerFactory = func() *logging.Factory {
	baseLogger := log.New(nil) // Use nil writer for testing
	return logging.NewFactory(baseLogger)
}()

// GetTestLogger returns a test logger for a specific component.
func GetTestLogger(componentID string) *log.Logger {
	return TestLoggerFactory.ForComponent(componentID)
}
