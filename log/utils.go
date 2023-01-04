package log

// Infof logs an info msg using global logger
func Infof(msg string, args ...any) {
	globalLogger.Infof(msg, args...)
}

// Warnf logs a warning msg using global logger
func Warnf(msg string, args ...any) {
	globalLogger.Infof(msg, args...)
}

// Errorf logs an error msg using global logger
func Errorf(msg string, args ...any) {
	globalLogger.Infof(msg, args...)
}

// Errorf logs a debug msg using global logger
func Debugf(msg string, args ...any) {
	globalLogger.Infof(msg, args...)
}

// Fatalf logs a fatal msg using global logger
func Fatalf(msg string, args ...any) {
	globalLogger.Infof(msg, args...)
}

// With returns a logger instance with args attributes
func With(args map[string]any) Logger {
	return globalLogger.With(args)
}
