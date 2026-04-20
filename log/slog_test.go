package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// newSlogTestLogger captures output into a buffer so tests can assert
// on the emitted JSON.
func newSlogTestLogger(t *testing.T, w io.Writer) *slogLoggerWrapper {
	t.Helper()
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler).With(
		slog.String(loggerKeyServiceName, "TestSvc"),
		slog.String(loggerKeyServiceEnv, "test"),
		slog.String(loggerKeyLoggerType, "slog"),
	)
	return &slogLoggerWrapper{logger: logger}
}

func TestSlogWrapper_Infof_EmitsJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := newSlogTestLogger(t, &buf)

	l.Infof("hello %s", "world")

	var got map[string]any
	err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", got["msg"])
	assert.Equal(t, "INFO", got["level"])
	assert.Equal(t, "TestSvc", got[loggerKeyServiceName])
}

func TestSlogWrapper_With_AddsAttrs(t *testing.T) {
	var buf bytes.Buffer
	l := newSlogTestLogger(t, &buf)

	l.With(map[string]any{"k": "v"}).Infof("x")

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "v", got["k"])
}

// Infof calls emit their own standalone line regardless of whether an
// event is active. Canonical events carry attributes via log.Set(ctx,...)
// only — legacy Infof is intentionally NOT captured into the canonical
// line (this keeps the Logger interface ctx-free and avoids goroutine
// tracking).
func TestSlogWrapper_InfofInsideEvent_EmitsStandaloneLine(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	_, end := BeginEvent(context.Background(), "req")
	Infof("step one")
	end()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 2, "one standalone Infof line + one canonical line")
}

func TestSlogWrapper_InfofOutsideEvent_EmitsStandaloneLine(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	Infof("startup: port %d", 8080)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "startup: port 8080", got["msg"])
}

func TestDevHandler_PrintsHumanReadableLine(t *testing.T) {
	var buf bytes.Buffer
	handler := newDevHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	logger.Info("hello", slog.String("k", "v"))

	out := buf.String()
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "k=v")
}

func TestDevHandler_CanonicalEventPrintedAsOneBlock(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()

	logger := slog.New(newDevHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).With(
		slog.String(loggerKeyServiceName, "TestSvc"),
		slog.String(loggerKeyServiceEnv, "test"),
		slog.String(loggerKeyLoggerType, "slog"),
	)
	globalLogger = &slogLoggerWrapper{logger: logger}

	_, end := BeginEvent(context.Background(), "http.request")
	end()

	out := buf.String()
	assert.Contains(t, out, "http.request")
	assert.Contains(t, out, "event=http.request")
	assert.Contains(t, out, "duration_ms=")
}
