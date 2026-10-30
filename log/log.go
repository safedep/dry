package log

import "sync/atomic"

const (
	logLevelNameInfo  = "info"
	logLevelNameDebug = "debug"
	logLevelNameWarn  = "warn"
	logLevelNameError = "error"
)

// Values for APP_LOG_FORMAT.
const (
	logFormatNameText = "text"
	logFormatNameJSON = "json"
)

// Logger represents a contract for implementing a logging module
type Logger interface {
	Infof(msg string, args ...any)
	Warnf(msg string, args ...any)
	Errorf(msg string, args ...any)
	Debugf(msg string, args ...any)
	Fatalf(msg string, args ...any)

	With(args map[string]any) Logger
}

// loggerSlot holds the global logger behind an atomic pointer. A program can
// log from a goroutine while its main goroutine sets the logger, for example a
// telemetry client that starts before the CLI configures logging. A plain
// interface variable can tear in that race, and a call on the torn value
// crashes the process.
type loggerSlot struct{ Logger }

var globalLogger atomic.Pointer[loggerSlot]

var nopGlobal Logger = &nopLogger{}

// global returns the nop logger while the slot is empty, for example for a
// log call from a package-level initializer that runs before any logger is
// set.
func global() Logger {
	if slot := globalLogger.Load(); slot != nil {
		return slot.Logger
	}
	return nopGlobal
}

// swapGlobal sets the global logger and returns a function that restores the
// previous one. A nil logger becomes the nop logger, as no log call can use
// nil.
func swapGlobal(logger Logger) (restore func()) {
	if logger == nil {
		logger = nopGlobal
	}
	prev := globalLogger.Swap(&loggerSlot{logger})
	return func() { globalLogger.Store(prev) }
}

// SetGlobal sets the global logger instance. A nil logger is ignored
// to avoid leaving the package in a state where every log call panics.
func SetGlobal(logger Logger) {
	if logger == nil {
		return
	}
	globalLogger.Store(&loggerSlot{logger})
}

// Nop logger for internal modules (default)
type nopLogger struct{}

// NewNopLogger creates a No Operation (NOP) logger
func NewNopLogger() Logger {
	return &nopLogger{}
}

func (*nopLogger) Infof(msg string, args ...any)     {}
func (*nopLogger) Warnf(msg string, args ...any)     {}
func (*nopLogger) Errorf(msg string, args ...any)    {}
func (*nopLogger) Debugf(msg string, args ...any)    {}
func (*nopLogger) Fatalf(msg string, args ...any)    {}
func (n *nopLogger) With(args map[string]any) Logger { return n }
func (*nopLogger) emitCanonical(_ *Event)            {}

// Constants to standardise logger keys
const (
	loggerKeyServiceName      = "service"
	loggerKeyServiceEnv       = "env"
	loggerKeyLoggerType       = "l"
	loggerKeyEnvLogFileName   = "APP_LOG_FILE"
	loggerKeyEnvLogLevel      = "APP_LOG_LEVEL"
	loggerKeySkipStdoutLogger = "APP_LOG_SKIP_STDOUT_LOGGER"
	loggerKeyCliStdout        = "APP_LOG_CLI_STDOUT"
	loggerKeyEnvLogFormat     = "APP_LOG_FORMAT"
)

// Initialize logger for the given service name
// This is the contract for initializing logger. The actual implementation
// of the logger may vary based on runtime environment
func Init(name, env string) {
	InitZapLogger(name, env)
}
