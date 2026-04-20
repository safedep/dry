package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// devHandler emits a single compact human-readable line per record:
//
//	12:34:56.789 INFO  http.request   service=api env=prod http.status=200 duration_ms=3.41
//
// It is intended for local development; production uses the JSON handler.
type devHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
	group string
}

func newDevHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	lvl := slog.Leveler(slog.LevelInfo)
	if opts != nil && opts.Level != nil {
		lvl = opts.Level
	}
	return &devHandler{mu: &sync.Mutex{}, w: w, level: lvl}
}

func (h *devHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *devHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %-5s %s", r.Time.Format("15:04:05.000"), r.Level.String(), r.Message)

	writeAttr := func(a slog.Attr) bool {
		if a.Key == "" {
			return true
		}
		fmt.Fprintf(&b, " %s=%s", a.Key, formatAttr(a.Value))
		return true
	}

	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(writeAttr)
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write([]byte(b.String()))
	return err
}

func (h *devHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &nh
}

func (h *devHandler) WithGroup(name string) slog.Handler {
	nh := *h
	nh.group = name
	return &nh
}

func formatAttr(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, " \t\"") {
			return fmt.Sprintf("%q", s)
		}
		return s
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(v.Any())
	}
}
