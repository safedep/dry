package log

// Logger represents a contract for implementing a logging module
type Logger interface {
	Infof(msg string, kv ...any)
	Warnf(msg string, kv ...any)
	Errorf(msg string, kv ...any)
	Debugf(msg string, kv ...any)

	With(kv ...any) Logger
}

type nopLogger struct{}

// NewNopLogger creates a No Operation (NOP) logger
func NewNopLogger() Logger {
	return &nopLogger{}
}

func (*nopLogger) Infof(msg string, kv ...any)  {}
func (*nopLogger) Warnf(msg string, kv ...any)  {}
func (*nopLogger) Errorf(msg string, kv ...any) {}
func (*nopLogger) Debugf(msg string, kv ...any) {}
func (n *nopLogger) With(kv ...any) Logger      { return n }
