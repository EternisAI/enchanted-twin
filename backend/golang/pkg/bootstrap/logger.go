package bootstrap

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type customLogWriter struct{}

func (w *customLogWriter) Write(p []byte) (n int, err error) {
	logContent := strings.ToLower(string(p))
	if strings.Contains(logContent, "err") || strings.Contains(logContent, "error") || strings.Contains(logContent, "failed") {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func NewLogger() *log.Logger {
	logger := log.NewWithOptions(&customLogWriter{}, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	logger.SetColorProfile(lipgloss.ColorProfile())

	return logger
}
