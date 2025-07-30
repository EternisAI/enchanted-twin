package bootstrap

import (
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type customLogWriter struct{}

func (w *customLogWriter) Write(p []byte) (n int, err error) {
	// Route all logs to stdout - let the log consumer handle filtering
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
