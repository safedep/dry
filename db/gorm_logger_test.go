package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/safedep/dry/log"
	"github.com/stretchr/testify/assert"
	gormlogger "gorm.io/gorm/logger"
)

// captureLogger records every log call so tests can assert on level and fields.
type logEntry struct {
	level  string
	msg    string
	args   []any
	fields map[string]any
}

type captureLogger struct {
	entries *[]logEntry
	fields  map[string]any
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{entries: &[]logEntry{}, fields: map[string]any{}}
}

func (c *captureLogger) record(level, msg string, args []any) {
	*c.entries = append(*c.entries, logEntry{level: level, msg: msg, args: args, fields: c.fields})
}

func (c *captureLogger) Infof(msg string, args ...any)  { c.record("info", msg, args) }
func (c *captureLogger) Warnf(msg string, args ...any)  { c.record("warn", msg, args) }
func (c *captureLogger) Errorf(msg string, args ...any) { c.record("error", msg, args) }
func (c *captureLogger) Debugf(msg string, args ...any) { c.record("debug", msg, args) }
func (c *captureLogger) Fatalf(msg string, args ...any) { c.record("fatal", msg, args) }

func (c *captureLogger) With(args map[string]any) log.Logger {
	merged := map[string]any{}
	for k, v := range c.fields {
		merged[k] = v
	}
	for k, v := range args {
		merged[k] = v
	}
	return &captureLogger{entries: c.entries, fields: merged}
}

func (c *captureLogger) all() []logEntry { return *c.entries }

func recordingTrace(sql string, rows int64) func() (string, int64) {
	return func() (string, int64) { return sql, rows }
}

func TestDefaultGormLogConfig(t *testing.T) {
	cfg := DefaultGormLogConfig()
	assert.Equal(t, "warn", cfg.Level)
	assert.Equal(t, 200*time.Millisecond, cfg.SlowThreshold)
	assert.True(t, cfg.IgnoreRecordNotFound)
}

func TestGormLoggerInfoRespectsLevel(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "info"})
	l.Info(context.Background(), "hello %s", "world")

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "info", entries[0].level)
	assert.Equal(t, "hello %s", entries[0].msg)
	assert.Equal(t, []any{"world"}, entries[0].args)
}

func TestGormLoggerInfoSuppressedBelowLevel(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn"})
	l.Info(context.Background(), "should not appear")
	assert.Empty(t, cap.all())
}

func TestGormLoggerWarnSuppressedAtErrorLevel(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "error"})
	l.Warn(context.Background(), "nope")
	assert.Empty(t, cap.all())
}

func TestGormLoggerErrorSuppressedAtSilent(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "silent"})
	l.Error(context.Background(), "nope")
	assert.Empty(t, cap.all())
}

func TestGormLoggerTraceErrorLogsWithFields(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn"})

	err := errors.New("boom")
	l.Trace(context.Background(), time.Now(), recordingTrace("SELECT 1", 0), err)

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "error", entries[0].level)
	assert.Equal(t, "SELECT 1", entries[0].fields["sql"])
	assert.Equal(t, int64(0), entries[0].fields["rows"])
	assert.Equal(t, "boom", entries[0].fields["error"])
	assert.Contains(t, entries[0].fields, "elapsed_ms")
}

func TestGormLoggerTraceIgnoresRecordNotFound(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn", IgnoreRecordNotFound: true})

	l.Trace(context.Background(), time.Now(),
		recordingTrace("SELECT 1", 0), gormlogger.ErrRecordNotFound)

	assert.Empty(t, cap.all())
}

func TestGormLoggerTraceLogsRecordNotFoundWhenNotIgnored(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn", IgnoreRecordNotFound: false})

	l.Trace(context.Background(), time.Now(),
		recordingTrace("SELECT 1", 0), gormlogger.ErrRecordNotFound)

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "error", entries[0].level)
}

func TestGormLoggerTraceSlowQuery(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn", SlowThreshold: 100 * time.Millisecond})

	begin := time.Now().Add(-500 * time.Millisecond)
	l.Trace(context.Background(), begin, recordingTrace("SELECT slow", 3), nil)

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "warn", entries[0].level)
	assert.Equal(t, "SELECT slow", entries[0].fields["sql"])
	assert.Equal(t, int64(3), entries[0].fields["rows"])
}

func TestGormLoggerTraceNormalQueryLoggedAtInfo(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "info"})

	l.Trace(context.Background(), time.Now(), recordingTrace("SELECT ok", 1), nil)

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "info", entries[0].level)
	assert.Equal(t, "SELECT ok", entries[0].fields["sql"])
}

func TestGormLoggerTraceNormalQuerySuppressedBelowInfo(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "warn"})

	l.Trace(context.Background(), time.Now(), recordingTrace("SELECT ok", 1), nil)

	assert.Empty(t, cap.all())
}

func TestGormLoggerTraceSilentLogsNothing(t *testing.T) {
	cap := newCaptureLogger()
	l := NewGormLogger(cap, GormLogConfig{Level: "silent"})

	l.Trace(context.Background(), time.Now().Add(-time.Second),
		recordingTrace("SELECT x", 1), errors.New("boom"))

	assert.Empty(t, cap.all())
}

func TestGormLoggerLogModeReturnsCopyWithNewLevel(t *testing.T) {
	cap := newCaptureLogger()
	base := NewGormLogger(cap, GormLogConfig{Level: "warn"})

	verbose := base.LogMode(gormlogger.Info)
	verbose.Info(context.Background(), "from verbose")

	base.Info(context.Background(), "from base")

	entries := cap.all()
	assert.Len(t, entries, 1)
	assert.Equal(t, "from verbose", entries[0].msg)
}

func TestNewGormLoggerNilLoggerDoesNotPanic(t *testing.T) {
	l := NewGormLogger(nil, DefaultGormLogConfig())
	assert.NotPanics(t, func() {
		l.Error(context.Background(), "routed to global")
	})
}

func TestResolveGormLoggerNilConfigUsesDefaults(t *testing.T) {
	l := resolveGormLogger(nil)
	assert.NotNil(t, l)

	gl, ok := l.(*gormLogger)
	assert.True(t, ok)
	assert.Equal(t, gormlogger.Warn, gl.level)
	assert.True(t, gl.ignoreRNF)
}

func TestResolveGormLoggerHonoursSuppliedConfig(t *testing.T) {
	l := resolveGormLogger(&GormLogConfig{Level: "silent"})
	gl, ok := l.(*gormLogger)
	assert.True(t, ok)
	assert.Equal(t, gormlogger.Silent, gl.level)
}
