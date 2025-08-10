package bootstrap

import (
	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/logging"
)

// NewBootstrapLogger creates a bootstrap logger for early application startup.
func NewBootstrapLogger() *log.Logger {
	return logging.NewBootstrapLogger()
}

// NewLogger creates a configured logger based on the provided configuration.
func NewLogger(cfg *config.Config) *log.Logger {
	return logging.NewLogger(cfg)
}
