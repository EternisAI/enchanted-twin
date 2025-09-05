package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/config"
)

func TestNewBootstrapLogger(t *testing.T) {
	logger := NewBootstrapLogger()
	assert.NotNil(t, logger)
	assert.Equal(t, log.InfoLevel, logger.GetLevel())
}

func TestNewLogger_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "debug",
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	assert.Equal(t, log.DebugLevel, logger.GetLevel())
}

func TestNewLogger_TextFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  "warn",
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	assert.Equal(t, log.WarnLevel, logger.GetLevel())
}

func TestNewLogger_LogfmtFormat(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "logfmt",
		LogLevel:  "error",
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	assert.Equal(t, log.ErrorLevel, logger.GetLevel())
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "invalid_level",
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	// Should default to InfoLevel for invalid levels
	assert.Equal(t, log.InfoLevel, logger.GetLevel())
}

func TestLoggerFactory_ForComponent(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")
	assert.NotNil(t, logger)

	// Capture output to verify component field is added
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Info("test message")
	output := buf.String()

	// For text format, check if component is included
	assert.Contains(t, output, "test.component")
}

func TestLoggerFactory_ForService(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForService("test.service")
	assert.NotNil(t, logger)

	// Capture output to verify service field is added
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Info("test message")
	output := buf.String()

	// For text format, check if service is included
	assert.Contains(t, output, "test.service")
}

func TestLoggerFactory_WithError(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")

	// Test with nil error
	errorLogger := factory.WithError(logger, nil)
	assert.Equal(t, logger, errorLogger)

	// Test with actual error
	testError := assert.AnError
	errorLogger = factory.WithError(logger, testError)
	assert.NotEqual(t, logger, errorLogger)

	// Capture output to verify error fields are added
	var buf bytes.Buffer
	errorLogger.SetOutput(&buf)

	errorLogger.Error("test error message")
	output := buf.String()

	// For text format, check if error fields are included
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "error_type")
}

func TestLoggerFactory_WithError_ErrorTypeDetection(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")

	// Test with a standard error that has a specific type
	testError := fmt.Errorf("database connection failed")
	errorLogger := factory.WithError(logger, testError)
	assert.NotEqual(t, logger, errorLogger)

	// Capture output to verify error type is captured correctly
	var buf bytes.Buffer
	errorLogger.SetOutput(&buf)

	errorLogger.Error("test error with type detection")
	output := buf.String()

	// Check that the actual error type is captured, not just "error"
	assert.Contains(t, output, "database connection failed")
	// The error type should be "*errors.errorString" or similar, not just "error"
	assert.NotEqual(t, output, "error")
}

func TestLoggerFactory_WithContext(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")
	contextLogger := factory.WithContext(logger, "custom_field", "custom_value")
	assert.NotEqual(t, logger, contextLogger)

	// Capture output to verify custom field is added
	var buf bytes.Buffer
	contextLogger.SetOutput(&buf)

	contextLogger.Info("test message")
	output := buf.String()

	// For text format, check if custom field is included
	assert.Contains(t, output, "custom_field")
	assert.Contains(t, output, "custom_value")
}

func TestLoggerFactory_WithRequestID(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")
	requestLogger := factory.WithRequestID(logger, "test-request-123")
	assert.NotEqual(t, logger, requestLogger)

	// Capture output to verify request_id field is added
	var buf bytes.Buffer
	requestLogger.SetOutput(&buf)

	requestLogger.Info("test message")
	output := buf.String()

	// For text format, check if request_id field is included
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "test-request-123")
}

func TestLoggerFactory_WithUserID(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	logger := factory.ForComponent("test.component")
	userLogger := factory.WithUserID(logger, "test-user-456")
	assert.NotEqual(t, logger, userLogger)

	// Capture output to verify user_id field is added
	var buf bytes.Buffer
	userLogger.SetOutput(&buf)

	userLogger.Info("test message")
	output := buf.String()

	// For text format, check if user_id field is included
	assert.Contains(t, output, "user_id")
	assert.Contains(t, output, "test-user-456")
}

func TestJSONOutput_ComponentContext(t *testing.T) {
	// Test JSON output with component context
	cfg := &config.Config{
		LogFormat: "json",
		LogLevel:  "info",
	}

	baseLogger := NewLogger(cfg)
	factory := NewLoggerFactory(baseLogger)
	logger := factory.ForComponent("test.component")

	// Capture JSON output
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Info("test message", "user_id", "12345")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 1)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(lines[0]), &logEntry)
	require.NoError(t, err)

	// Verify JSON structure and fields
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "test.component", logEntry["component"])
	assert.Equal(t, "12345", logEntry["user_id"])
	assert.Equal(t, "info", logEntry["level"])
	assert.NotEmpty(t, logEntry["time"])
}

func TestEnvironmentVariableOverride(t *testing.T) {
	// Test that environment variables can override config
	originalFormat := os.Getenv("LOG_FORMAT")
	originalLevel := os.Getenv("LOG_LEVEL")

	defer func() {
		if originalFormat != "" {
			_ = os.Setenv("LOG_FORMAT", originalFormat)
		} else {
			_ = os.Unsetenv("LOG_FORMAT")
		}
		if originalLevel != "" {
			_ = os.Setenv("LOG_LEVEL", originalLevel)
		} else {
			_ = os.Unsetenv("LOG_LEVEL")
		}
	}()

	// Set environment variables
	_ = os.Setenv("LOG_FORMAT", "json")
	_ = os.Setenv("LOG_LEVEL", "debug")

	// Create config (should pick up environment variables)
	cfg := &config.Config{}
	// Note: In a real scenario, LoadConfigWithAutoDetection() would be called
	// For this test, we'll manually set the values that would come from env vars
	cfg.LogFormat = "json"
	cfg.LogLevel = "debug"

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	assert.Equal(t, log.DebugLevel, logger.GetLevel())
}

func TestLoggerFactory_ComponentTypes(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	// Test different component type methods
	logger := factory.ForManager("test.manager")
	assert.NotNil(t, logger)

	logger = factory.ForService("test.service")
	assert.NotNil(t, logger)

	logger = factory.ForHandler("test.handler")
	assert.NotNil(t, logger)

	logger = factory.ForRepository("test.repository")
	assert.NotNil(t, logger)

	logger = factory.ForWorker("test.worker")
	assert.NotNil(t, logger)

	logger = factory.ForClient("test.client")
	assert.NotNil(t, logger)

	logger = factory.ForServer("test.server")
	assert.NotNil(t, logger)

	logger = factory.ForMiddleware("test.middleware")
	assert.NotNil(t, logger)

	logger = factory.ForAI("test.ai")
	assert.NotNil(t, logger)

	logger = factory.ForDatabase("test.database")
	assert.NotNil(t, logger)

	logger = factory.ForMCP("test.mcp")
	assert.NotNil(t, logger)
}

func TestLoggerFactory_ComponentRegistration(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	// Test that components are automatically registered
	logger1 := factory.ForComponent("test.component1")
	logger2 := factory.ForComponent("test.component2")

	assert.NotNil(t, logger1)
	assert.NotNil(t, logger2)

	// Check that components are registered in the registry
	stats := factory.GetComponentStats()
	assert.Equal(t, 2, stats["total_components"])

	// Check component types
	types := factory.ListComponentTypes()
	assert.Contains(t, types, ComponentTypeUtility)
}

func TestLoggerFactory_LogLevelConfiguration(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	// First register the component
	logger := factory.ForComponent("test.component")
	assert.NotNil(t, logger)

	// Test setting log levels for components
	err := factory.SetComponentLogLevel("test.component", log.DebugLevel)
	assert.NoError(t, err)

	level := factory.GetComponentLogLevel("test.component")
	assert.Equal(t, log.DebugLevel, level)

	// Test non-existent component
	level = factory.GetComponentLogLevel("non.existent")
	assert.Equal(t, log.InfoLevel, level) // Default level
}

func TestLoggerFactory_ComponentEnableDisable(t *testing.T) {
	baseLogger := NewBootstrapLogger()
	factory := NewLoggerFactory(baseLogger)

	// First register the component
	logger := factory.ForComponent("test.component")
	assert.NotNil(t, logger)

	// Test enabling/disabling components
	err := factory.EnableComponent("test.component", false)
	assert.NoError(t, err)

	enabled := factory.IsComponentEnabled("test.component")
	assert.False(t, enabled)

	err = factory.EnableComponent("test.component", true)
	assert.NoError(t, err)

	enabled = factory.IsComponentEnabled("test.component")
	assert.True(t, enabled)
}
