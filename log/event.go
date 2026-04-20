package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// EndFunc flushes a canonical event. Safe to call multiple times; only
// the first call emits a record.
type EndFunc func()

// EventOption configures a new event at BeginEvent time.
type EventOption func(*Event)

// WithEventAttrs pre-populates attributes at event start.
func WithEventAttrs(attrs map[string]any) EventOption {
	return func(e *Event) {
		for k, v := range attrs {
			e.attrs[k] = v
		}
	}
}

// WithEventLevel overrides the default event level. Err() still
// promotes the level to error regardless.
func WithEventLevel(level slog.Level) EventOption {
	return func(e *Event) { e.level = level }
}

// Event is a canonical-log-line accumulator. All methods are safe for
// concurrent use.
type Event struct {
	name      string
	startedAt time.Time

	mu    sync.Mutex
	attrs map[string]any
	level slog.Level
	ended bool
}

// eventSnapshot is a lock-free copy of an Event's state, suitable for
// emission by any canonicalEmitter.
type eventSnapshot struct {
	name       string
	level      slog.Level
	durationMs float64
	attrs      map[string]any
}

// snapshot takes a consistent copy of the event state under a single
// lock, so emitters can format the record without holding ev.mu across
// allocation or I/O.
func (e *Event) snapshot() eventSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	attrs := make(map[string]any, len(e.attrs))
	for k, v := range e.attrs {
		attrs[k] = v
	}
	return eventSnapshot{
		name:       e.name,
		level:      e.level,
		durationMs: float64(time.Since(e.startedAt).Microseconds()) / 1000.0,
		attrs:      attrs,
	}
}

// Name returns the event name, or "" if e is nil.
func (e *Event) Name() string {
	if e == nil {
		return ""
	}
	return e.name
}

// BeginEvent starts a canonical event scope bound to the returned
// context. Call the returned EndFunc (typically via defer) to flush the
// event as a single log record.
func BeginEvent(ctx context.Context, name string, opts ...EventOption) (context.Context, EndFunc) {
	if existing := fromContext(ctx); existing != nil {
		existing.mu.Lock()
		existing.attrs["nested_begin"] = true
		existing.mu.Unlock()
		return ctx, func() {}
	}

	ev := &Event{
		name:      name,
		startedAt: time.Now(),
		attrs:     make(map[string]any),
		level:     slog.LevelInfo,
	}
	for _, opt := range opts {
		opt(ev)
	}

	newCtx := withContext(ctx, ev)
	return newCtx, func() {
		// Recover any panic, attach it, then re-panic AFTER emission.
		r := recover()

		ev.mu.Lock()
		if ev.ended {
			ev.mu.Unlock()
			if r != nil {
				panic(r)
			}
			return
		}
		ev.ended = true
		if r != nil {
			ev.attrs["panic"] = fmt.Sprintf("%v", r)
			ev.attrs["stack"] = string(debug.Stack())
			ev.level = slog.LevelError
		}
		ev.mu.Unlock()

		if emitter, ok := globalLogger.(canonicalEmitter); ok {
			emitter.emitCanonical(ev)
		}
		if r != nil {
			panic(r)
		}
	}
}

// FromContext returns the active event from ctx, or nil if none.
func FromContext(ctx context.Context) *Event {
	return fromContext(ctx)
}

// Set records a single attribute on the event. No-op if e is nil.
func (e *Event) Set(key string, value any) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attrs[key] = value
}

// SetAttrs records multiple attributes in one call. No-op if e is nil.
func (e *Event) SetAttrs(attrs map[string]any) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range attrs {
		e.attrs[k] = v
	}
}

// Counter increments an integer attribute by delta. If the attribute
// is not yet set or is not an int64, it is (re)initialised. No-op if e
// is nil.
func (e *Event) Counter(key string, delta int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	cur, _ := e.attrs[key].(int64)
	e.attrs[key] = cur + delta
}

// Err records an error on the event and promotes its level to error.
// Later Err calls overwrite earlier ones. No-op if e is nil or err is nil.
func (e *Event) Err(err error) {
	if e == nil || err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attrs["error"] = err.Error()
	e.level = slog.LevelError
}

// Set records an attribute on the event bound to ctx. No-op if no event.
func Set(ctx context.Context, key string, value any) {
	if ev := fromContext(ctx); ev != nil {
		ev.Set(key, value)
	}
}

// SetAttrs records multiple attributes. No-op if no event.
func SetAttrs(ctx context.Context, attrs map[string]any) {
	if ev := fromContext(ctx); ev != nil {
		ev.SetAttrs(attrs)
	}
}

// Counter increments an integer attribute. No-op if no event.
func Counter(ctx context.Context, key string, delta int64) {
	if ev := fromContext(ctx); ev != nil {
		ev.Counter(key, delta)
	}
}

// Err records an error and promotes the event level. No-op if no event
// or if err is nil.
func Err(ctx context.Context, err error) {
	if ev := fromContext(ctx); ev != nil {
		ev.Err(err)
	}
}

