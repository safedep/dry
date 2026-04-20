package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

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

	captureMessages := true
	if v := os.Getenv(loggerKeyEnvCaptureMessages); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			captureMessages = b
		}
	}

	return &slogLoggerWrapper{
		logger:          root,
		captureMessages: captureMessages,
		devMode:         cfg.resolvedFormat() == "text",
	}, nil
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

// logCaptureMessagesCap bounds the messages[] array on a canonical
// event. Beyond this, calls are dropped and counted.
const logCaptureMessagesCap = 50

// slogLoggerWrapper implements the existing Logger interface over slog.
type slogLoggerWrapper struct {
	logger          *slog.Logger
	captureMessages bool
	devMode         bool
}

func (z *slogLoggerWrapper) Infof(msg string, args ...any) {
	z.log(slog.LevelInfo, "info", msg, args...)
}

func (z *slogLoggerWrapper) Warnf(msg string, args ...any) {
	z.log(slog.LevelWarn, "warn", msg, args...)
}

func (z *slogLoggerWrapper) Errorf(msg string, args ...any) {
	z.log(slog.LevelError, "error", msg, args...)
}

func (z *slogLoggerWrapper) Debugf(msg string, args ...any) {
	z.log(slog.LevelDebug, "debug", msg, args...)
}

func (z *slogLoggerWrapper) Fatalf(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)

	if ev := getActiveEvent(); ev != nil {
		// Inside an active event: flush the canonical line (with the fatal
		// message attached) before os.Exit kills the deferred EndFunc.
		// The fatal message is carried on the canonical event as a "fatal"
		// attr; no second standalone record is emitted.
		ev.mu.Lock()
		ev.attrs["fatal"] = formatted
		ev.level = slog.LevelError
		wasEnded := ev.ended
		if !wasEnded {
			ev.ended = true
		}

		ev.mu.Unlock()
		if !wasEnded {
			z.emitCanonical(ev)
		}
		clearActiveEvent()
	} else {
		// No active event: emit as a standalone error record.
		z.logger.Log(context.Background(), slog.LevelError, formatted)
	}

	os.Exit(1)
}

func (z *slogLoggerWrapper) With(args map[string]any) Logger {
	attrs := make([]any, 0, len(args)*2)
	for k, v := range args {
		attrs = append(attrs, slog.Any(k, v))
	}

	return &slogLoggerWrapper{
		logger:          z.logger.With(attrs...),
		captureMessages: z.captureMessages,
		devMode:         z.devMode,
	}
}

func (z *slogLoggerWrapper) log(level slog.Level, levelName, msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if ev := getActiveEvent(); ev != nil {
		if !z.captureMessages {
			// Capture disabled. In dev, emit a standalone line so local
			// debugging still prints. In prod, drop silently.
			if z.devMode {
				z.logger.Log(context.Background(), level, formatted)
			}
			return
		}

		ev.captureMessage(levelName, formatted)
		if level == slog.LevelError {
			ev.mu.Lock()
			ev.level = slog.LevelError
			ev.mu.Unlock()
		}

		return
	}

	z.logger.Log(context.Background(), level, formatted)
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
	ev.mu.Lock()
	attrs := make([]slog.Attr, 0, len(ev.attrs)+3)
	attrs = append(attrs,
		slog.String("event", ev.name),
		slog.Float64("duration_ms", float64(time.Since(ev.startedAt).Microseconds())/1000.0),
	)

	for k, v := range ev.attrs {
		attrs = append(attrs, slog.Any(k, v))
	}

	if len(ev.messages) > 0 {
		attrs = append(attrs, slog.Any("messages", ev.messages))
	}

	if ev.messagesDropped > 0 {
		attrs = append(attrs, slog.Int("messages_dropped", ev.messagesDropped))
	}

	level := ev.level
	ev.mu.Unlock()

	z.logger.LogAttrs(context.Background(), level, ev.name, attrs...)
}
