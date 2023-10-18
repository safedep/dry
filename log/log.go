package log

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
	loggerKeyServiceName = "service"
	loggerKeyLoggerType  = "l"
)

// Initialize logger for the given service name
func Init(name string) {
	InitZapLogger(name)
}
