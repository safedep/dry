package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"

	"gopkg.in/natefinch/lumberjack.v2"
)

// InitSlogLogger initializes a slog-backed logger and sets it as the
// global logger. Opt-in; Init() still uses zap.
func InitSlogLogger(name, env string) {
	skipStdout, _ := strconv.ParseBool(os.Getenv(loggerKeySkipStdoutLogger))

	cfg := slogLoggerConfig{
		name:      name,
		env:       env,
		level:     parseLogLevel(os.Getenv(loggerKeyEnvLogLevel)),
		format:    os.Getenv(loggerKeyEnvLogFormat),
		logFile:   os.Getenv(loggerKeyEnvLogFileName),
		logStdout: !skipStdout,
	}

	logger, err := newSlogLogger(cfg)
	if err != nil {
		panic(err)
	}

	SetGlobal(logger)
}

type slogLoggerConfig struct {
	name      string
	env       string
	level     slog.Level
	format    string // "", "text", "json"
	logFile   string
	logStdout bool
}

func newSlogLogger(cfg slogLoggerConfig) (Logger, error) {
	writers := []slogWriterSpec{}
	if cfg.logStdout {
		writers = append(writers, slogWriterSpec{w: os.Stdout, format: cfg.resolvedFormat()})
	}

	if cfg.logFile != "" {
		file := &lumberjack.Logger{
			Filename:   cfg.logFile,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
		}

		writers = append(writers, slogWriterSpec{w: file, format: "json"})
	}

	handlers := make([]slog.Handler, 0, len(writers))
	for _, spec := range writers {
		handlers = append(handlers, spec.build(cfg.level))
	}

	root := slog.New(teeHandler(handlers))
	root = root.With(
		slog.String(loggerKeyServiceName, cfg.name),
		slog.String(loggerKeyServiceEnv, cfg.env),
		slog.String(loggerKeyLoggerType, "slog"),
	)

	return &slogLoggerWrapper{logger: root}, nil
}

// resolvedFormat returns "text" or "json", honouring APP_LOG_FORMAT and
// falling back to "text" for dev-ish envs, "json" otherwise.
func (c slogLoggerConfig) resolvedFormat() string {
	switch c.format {
	case "text", "json":
		return c.format
	}

	switch c.env {
	case "", "dev", "development", "local":
		return "text"
	}

	return "json"
}

type slogWriterSpec struct {
	w      io.Writer
	format string
}

func (s slogWriterSpec) build(level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	if s.format == "text" {
		return newDevHandler(s.w, opts)
	}

	return slog.NewJSONHandler(s.w, opts)
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case logLevelNameDebug:
		return slog.LevelDebug
	case logLevelNameWarn:
		return slog.LevelWarn
	case logLevelNameError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// slogLoggerWrapper implements the existing Logger interface over slog.
type slogLoggerWrapper struct {
	logger *slog.Logger
}

func (z *slogLoggerWrapper) Infof(msg string, args ...any) {
	z.logger.Info(fmt.Sprintf(msg, args...))
}

func (z *slogLoggerWrapper) Warnf(msg string, args ...any) {
	z.logger.Warn(fmt.Sprintf(msg, args...))
}

func (z *slogLoggerWrapper) Errorf(msg string, args ...any) {
	z.logger.Error(fmt.Sprintf(msg, args...))
}

func (z *slogLoggerWrapper) Debugf(msg string, args ...any) {
	z.logger.Debug(fmt.Sprintf(msg, args...))
}

func (z *slogLoggerWrapper) Fatalf(msg string, args ...any) {
	z.logger.Error(fmt.Sprintf(msg, args...))
	os.Exit(1)
}

func (z *slogLoggerWrapper) With(args map[string]any) Logger {
	attrs := make([]any, 0, len(args)*2)
	for k, v := range args {
		attrs = append(attrs, slog.Any(k, v))
	}
	return &slogLoggerWrapper{logger: z.logger.With(attrs...)}
}

// --- multiHandler fans one record out to multiple handlers ---------

type multiHandler struct{ handlers []slog.Handler }

func teeHandler(hs []slog.Handler) slog.Handler {
	if len(hs) == 1 {
		return hs[0]
	}

	return &multiHandler{handlers: hs}
}

func (m *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}

	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: out}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: out}
}

// canonicalEmitter is the internal hook used by BeginEvent's EndFunc to
// emit the canonical record. Implementations are the slog wrapper (real)
// and nop logger (no-op).
type canonicalEmitter interface {
	emitCanonical(ev *Event)
}

func (z *slogLoggerWrapper) emitCanonical(ev *Event) {
	if ev == nil {
		return
	}
	snap := ev.snapshot()

	attrs := make([]slog.Attr, 0, len(snap.attrs)+2)
	attrs = append(attrs,
		slog.String("event", snap.name),
		slog.Float64("duration_ms", snap.durationMs),
	)
	for k, v := range snap.attrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	z.logger.LogAttrs(context.Background(), snap.level, snap.name, attrs...)
}
