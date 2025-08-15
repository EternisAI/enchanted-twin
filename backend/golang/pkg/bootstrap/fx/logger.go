package fx

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"go.uber.org/fx/fxevent"
)

// CharmLogger adapts charmbracelet log.Logger to fx's fxevent.Logger interface.
type CharmLogger struct {
	logger *log.Logger
}

// NewCharmLoggerWithComponent creates a new fx logger with a specific component.
func NewCharmLoggerWithComponent(logger *log.Logger, component string) fxevent.Logger {
	componentLogger := logger.With("component", component)
	return &CharmLogger{logger: componentLogger}
}

// LogEvent implements fxevent.Logger interface.
func (l *CharmLogger) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		l.logger.Info("[Fx] HOOK OnStart",
			"function", e.FunctionName,
			"caller", e.CallerName)
	case *fxevent.OnStartExecuted:
		if e.Err != nil {
			l.logger.Error("[Fx] HOOK OnStart FAILED",
				"function", e.FunctionName,
				"caller", e.CallerName,
				"error", e.Err,
				"runtime", e.Runtime)
		} else {
			l.logger.Info("[Fx] HOOK OnStart SUCCESS",
				"function", e.FunctionName,
				"caller", e.CallerName,
				"runtime", e.Runtime)
		}
	case *fxevent.OnStopExecuting:
		l.logger.Info("[Fx] HOOK OnStop",
			"function", e.FunctionName,
			"caller", e.CallerName)
	case *fxevent.OnStopExecuted:
		if e.Err != nil {
			l.logger.Error("[Fx] HOOK OnStop FAILED",
				"function", e.FunctionName,
				"caller", e.CallerName,
				"error", e.Err,
				"runtime", e.Runtime)
		} else {
			l.logger.Info("[Fx] HOOK OnStop SUCCESS",
				"function", e.FunctionName,
				"caller", e.CallerName,
				"runtime", e.Runtime)
		}
	case *fxevent.Supplied:
		// Skip supply events as they're too verbose
		return
	case *fxevent.Provided:
		// Show all provided services
		l.logger.Info("[Fx] PROVIDE",
			"constructor", e.ConstructorName,
			"module", e.ModuleName,
			"type", e.OutputTypeNames)
	case *fxevent.Invoked:
		l.logger.Info("[Fx] INVOKE",
			"function", e.FunctionName,
			"module", e.ModuleName)
	case *fxevent.Started:
		l.logger.Info("[Fx] RUNNING")
	case *fxevent.LoggerInitialized:
		l.logger.Info("[Fx] LOGGER", "constructor", e.ConstructorName)
	case *fxevent.Stopping:
		l.logger.Info("[Fx] STOPPING")
	case *fxevent.Stopped:
		if e.Err != nil {
			l.logger.Error("[Fx] STOP FAILED", "error", e.Err)
		} else {
			l.logger.Info("[Fx] STOPPED")
		}
	default:
		// Log unhandled events for debugging
		l.logger.Debug("[Fx] EVENT", "type", strings.TrimPrefix(fmt.Sprintf("%T", e), "*fxevent."))
	}
}
