package bootstrap

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/config"
)

type customLogWriter struct{}

func (w *customLogWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func NewBootstrapLogger() *log.Logger {
	logger := log.NewWithOptions(&customLogWriter{}, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
		TimeFormat:      time.RFC3339,
		Formatter:       log.JSONFormatter, // Use JSON format to match main logger
	})

	// Only set color profile for text formatter
	// JSON formatter doesn't use colors
	return logger
}

func NewLogger(cfg *config.Config) *log.Logger {
	var writer io.Writer
	if cfg.LogOutput == "file" {
		// TODO: Implement file writer with rotation
		writer = &customLogWriter{}
	} else {
		writer = &customLogWriter{}
	}

	level := log.InfoLevel
	if cfg.LogLevel != "" {
		if parsedLevel, err := log.ParseLevel(cfg.LogLevel); err == nil {
			level = parsedLevel
		}
	}

	formatter := log.TextFormatter
	switch cfg.LogFormat {
	case "json":
		formatter = log.JSONFormatter
	case "logfmt":
		formatter = log.LogfmtFormatter
	}

	logger := log.NewWithOptions(writer, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           level,
		TimeFormat:      time.RFC3339,
		Formatter:       formatter,
	})

	if formatter == log.TextFormatter {
		logger.SetColorProfile(lipgloss.ColorProfile())
	}

	return logger
}
