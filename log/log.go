package log

const (
	logLevelNameInfo  = "info"
	logLevelNameDebug = "debug"
	logLevelNameWarn  = "warn"
	logLevelNameError = "error"
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

// Global logger instance
var globalLogger Logger

func init() {
	globalLogger = NewNopLogger()
}

// SetGlobal sets the global logger instance
func SetGlobal(logger Logger) {
	globalLogger = logger
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

// Constants to standardise logger keys
const (
	loggerKeyServiceName      = "service"
	loggerKeyServiceEnv       = "env"
	loggerKeyLoggerType       = "l"
	loggerKeyEnvLogFileName   = "APP_LOG_FILE"
	loggerKeyEnvLogLevel      = "APP_LOG_LEVEL"
	loggerKeySkipStdoutLogger = "APP_LOG_SKIP_STDOUT_LOGGER"
)

// Initialize logger for the given service name
// This is the contract for initializing logger. The actual implementation
// of the logger may vary based on runtime environment
func Init(name, env string) {
	InitZapLogger(name, env)
}
