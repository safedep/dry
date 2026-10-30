package log

// Infof logs an info msg using global logger
func Infof(msg string, args ...any) {
	global().Infof(msg, args...)
}

// Warnf logs a warning msg using global logger
func Warnf(msg string, args ...any) {
	global().Warnf(msg, args...)
}

// Errorf logs an error msg using global logger
func Errorf(msg string, args ...any) {
	global().Errorf(msg, args...)
}

// Errorf logs a debug msg using global logger
func Debugf(msg string, args ...any) {
	global().Debugf(msg, args...)
}

// Fatalf logs a fatal msg using global logger
func Fatalf(msg string, args ...any) {
	global().Fatalf(msg, args...)
}

// With returns a logger instance with args attributes
func With(args map[string]any) Logger {
	return global().With(args)
}
