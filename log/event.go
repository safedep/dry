package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// EndFunc flushes a canonical event. Safe to call multiple times; only
// the first call emits a record.
//
// Must be called from the same goroutine that called BeginEvent — the
// per-goroutine active-event tracker (used by Infof/Errorf capture)
// assumes this. If you need to hand a context off to a worker goroutine,
// emit the event on the original goroutine before dispatch.
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

	// messages captured from per-event Infof/Errorf calls inside the scope.
	// Bounded; see logCaptureMessagesCap.
	messages        []capturedMessage
	messagesDropped int
}

type capturedMessage struct {
	Time  time.Time
	Level string
	Msg   string
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

	setActiveEvent(ev)
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
		clearActiveEvent()
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

// goroutineActiveEvent is set by BeginEvent and cleared by EndFunc. It
// lets the slog wrapper capture per-event log calls without threading a
// ctx through the existing Logger interface.
//
// Concurrency: one event per goroutine. Nested BeginEvent is already
// disallowed (Task 6), so this map cannot have stacked entries.
var goroutineActiveEvent sync.Map // map[uint64]*Event

func setActiveEvent(ev *Event) { goroutineActiveEvent.Store(goroutineID(), ev) }
func clearActiveEvent()        { goroutineActiveEvent.Delete(goroutineID()) }

func getActiveEvent() *Event {
	if v, ok := goroutineActiveEvent.Load(goroutineID()); ok {
		return v.(*Event)
	}
	return nil
}

// goroutineID returns the calling goroutine's ID. Used only for
// in-process, per-goroutine event lookup.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Format: "goroutine N [status]:\n..."
	s := buf[:n]
	i := 10 // after "goroutine "
	var id uint64
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		id = id*10 + uint64(s[i]-'0')
	}
	return id
}

func (e *Event) captureMessage(level, msg string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.messages) >= logCaptureMessagesCap {
		e.messagesDropped++
		return
	}
	e.messages = append(e.messages, capturedMessage{
		Time: time.Now(), Level: level, Msg: msg,
	})
}
