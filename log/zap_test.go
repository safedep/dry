package log

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewZapLogger(t *testing.T) {
	logger, err := newZapLogger("TestSvc", "test", zapLoggerConfig{})

	assert.Nil(t, err)
	assert.NotNil(t, logger)

	zapLogger, typeCheck := logger.(*zapLoggerWrapper)
	assert.True(t, typeCheck)

	assert.NotNil(t, zapLogger.logger)
	assert.NotNil(t, zapLogger.sugaredLogger)
}

func TestProductionLoggerToFile(t *testing.T) {
	t.Run("ProductionLoggerToFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.log")
		assert.NoError(t, err)

		path := tmpFile.Name()
		require.NoError(t, tmpFile.Close())

		// Remove the file created by os.CreateTemp
		require.NoError(t, os.Remove(path))

		// Make sure the file created by the test is removed
		defer func() { _ = os.Remove(path) }()

		t.Setenv(loggerKeyEnvLogFileName, path)
		t.Setenv(loggerKeyEnvLogLevel, logLevelNameDebug)

		l, err := newZapLogger("TestSvc", "test", zapLoggerConfig{
			logLevel: logLevelNameDebug,
			logFile:  path,
		})
		assert.NoError(t, err)

		l.Infof("Test info log message")
		l.Debugf("Test debug log message")

		z, ok := l.(*zapLoggerWrapper)
		assert.True(t, ok)

		_ = z.logger.Sync()

		assert.FileExists(t, path)

		content, err := os.ReadFile(path)
		assert.NoError(t, err)

		assert.Contains(t, string(content), "Test info log message")
		assert.Contains(t, string(content), "Test debug log message")
	})
}

func TestZapWrapper_ImplementsCanonicalEmitter(t *testing.T) {
	logger, err := newZapLogger("TestSvc", "test", zapLoggerConfig{})
	assert.NoError(t, err)

	// Compile-time + runtime check that zap participates in canonical
	// event emission. Without this, BeginEvent would silently no-op for
	// services that call log.Init() (zap) instead of log.InitSlogLogger.
	_, ok := logger.(canonicalEmitter)
	assert.True(t, ok, "zapLoggerWrapper must implement canonicalEmitter")

	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = logger

	_, end := BeginEvent(context.Background(), "test.event")
	assert.NotPanics(t, func() { end() })
}

func TestZapWrapper_EmitCanonicalPreservesLevel(t *testing.T) {
	cases := []struct {
		name  string
		level slog.Level
		want  zapcore.Level
	}{
		{"debug", slog.LevelDebug, zapcore.DebugLevel},
		{"info", slog.LevelInfo, zapcore.InfoLevel},
		{"warn", slog.LevelWarn, zapcore.WarnLevel},
		{"error", slog.LevelError, zapcore.ErrorLevel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core, recorded := observer.New(zapcore.DebugLevel)
			wrapper := &zapLoggerWrapper{logger: zap.New(core)}

			prev := globalLogger
			defer func() { globalLogger = prev }()
			globalLogger = wrapper

			_, end := BeginEvent(context.Background(), "test.event", WithEventLevel(tc.level))
			end()

			entries := recorded.All()
			assert.Len(t, entries, 1)
			assert.Equal(t, tc.want, entries[0].Level,
				"canonical event level %v should map to zap %v", tc.level, tc.want)
		})
	}
}
