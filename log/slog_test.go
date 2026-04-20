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
	return &slogLoggerWrapper{logger: logger, captureMessages: true}
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

func TestSlogWrapper_InfofInsideEvent_CapturesToMessages(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	_, end := BeginEvent(context.Background(), "req")
	Infof("step one: %d", 1)
	Errorf("something failed: %s", "oops")
	end()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1, "only the canonical line should be emitted")

	var got map[string]any
	_ = json.Unmarshal(lines[0], &got)

	msgs, ok := got["messages"].([]any)
	assert.True(t, ok, "messages should be an array")
	assert.Len(t, msgs, 2)
	assert.Equal(t, "ERROR", got["level"], "Errorf inside event promotes level")
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

func TestSlogWrapper_MessageCaptureCap(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	_, end := BeginEvent(context.Background(), "req")
	for i := 0; i < logCaptureMessagesCap+10; i++ {
		Infof("msg %d", i)
	}
	end()

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)

	msgs := got["messages"].([]any)
	assert.Len(t, msgs, logCaptureMessagesCap)
	assert.Equal(t, float64(10), got["messages_dropped"])
}

func TestSlogWrapper_CaptureDisabledByEnv(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	l := newSlogTestLogger(t, &buf)
	l.captureMessages = false // simulate APP_LOG_CAPTURE_MESSAGES=false resolved at init
	globalLogger = l

	_, end := BeginEvent(context.Background(), "req")
	Infof("dropped")
	end()

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	_, hasMessages := got["messages"]
	assert.False(t, hasMessages)
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
	globalLogger = &slogLoggerWrapper{logger: logger, captureMessages: true}

	_, end := BeginEvent(context.Background(), "http.request")
	end()

	out := buf.String()
	assert.Contains(t, out, "http.request")
	assert.Contains(t, out, "event=http.request")
	assert.Contains(t, out, "duration_ms=")
}
