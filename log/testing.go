package log

import (
	"io"
	"log/slog"
)

// SwapGlobalForTest installs a slog-backed JSON logger that writes to w
// and returns a restore function. Callers should defer the restore:
//
//	restore := drylog.SwapGlobalForTest(&buf)
//	defer restore()
//
// Not safe for parallel tests that each expect their own logger —
// globalLogger is a single slot. Use in serial tests.
func SwapGlobalForTest(w io.Writer) (restore func()) {
	prev := globalLogger
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler).With(
		slog.String(loggerKeyServiceName, "test"),
		slog.String(loggerKeyServiceEnv, "test"),
		slog.String(loggerKeyLoggerType, "slog"),
	)

	globalLogger = &slogLoggerWrapper{
		logger:          logger,
		captureMessages: true,
		devMode:         false,
	}

	return func() { globalLogger = prev }
}

// Global returns the current global logger. Exposed for tests.
func Global() Logger { return globalLogger }
