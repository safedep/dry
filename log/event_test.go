package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBeginEvent_StoresEventInContext(t *testing.T) {
	ctx := context.Background()

	newCtx, end := BeginEvent(ctx, "test.event")
	defer end()

	ev := FromContext(newCtx)
	assert.NotNil(t, ev)
	assert.Equal(t, "test.event", ev.Name())
}

func TestFromContext_ReturnsNilWhenNoEvent(t *testing.T) {
	ev := FromContext(context.Background())
	assert.Nil(t, ev)
}

func TestEvent_Set(t *testing.T) {
	ctx, end := BeginEvent(context.Background(), "x")
	defer end()

	Set(ctx, "user.id", "u1")
	ev := FromContext(ctx)

	ev.mu.Lock()
	defer ev.mu.Unlock()
	assert.Equal(t, "u1", ev.attrs["user.id"])
}

func TestEvent_SetAttrs(t *testing.T) {
	ctx, end := BeginEvent(context.Background(), "x")
	defer end()

	SetAttrs(ctx, map[string]any{"a": 1, "b": "two"})
	ev := FromContext(ctx)

	ev.mu.Lock()
	defer ev.mu.Unlock()
	assert.Equal(t, 1, ev.attrs["a"])
	assert.Equal(t, "two", ev.attrs["b"])
}

func TestEvent_Counter_ConcurrentIncrements(t *testing.T) {
	ctx, end := BeginEvent(context.Background(), "x")
	defer end()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Counter(ctx, "db.queries", 1)
		}()
	}
	wg.Wait()

	ev := FromContext(ctx)
	ev.mu.Lock()
	defer ev.mu.Unlock()
	assert.Equal(t, int64(100), ev.attrs["db.queries"])
}

func TestEvent_Err_SetsErrorAttrAndLevel(t *testing.T) {
	ctx, end := BeginEvent(context.Background(), "x")
	defer end()

	Err(ctx, errors.New("boom"))
	ev := FromContext(ctx)

	ev.mu.Lock()
	defer ev.mu.Unlock()
	assert.Equal(t, "boom", ev.attrs["error"])
	assert.Equal(t, slog.LevelError, ev.level)
}

func TestEvent_Helpers_NoopWhenNoEvent(t *testing.T) {
	ctx := context.Background()
	// Must not panic.
	Set(ctx, "k", "v")
	SetAttrs(ctx, map[string]any{"a": 1})
	Counter(ctx, "c", 1)
	Err(ctx, errors.New("x"))
}

func TestBeginEvent_EmitsOneRecordOnEnd(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	ctx, end := BeginEvent(context.Background(), "http.request",
		WithEventAttrs(map[string]any{"http.method": "GET"}))
	Set(ctx, "http.status", 200)
	end()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1)

	var got map[string]any
	err := json.Unmarshal(lines[0], &got)
	assert.NoError(t, err)
	assert.Equal(t, "http.request", got["msg"])
	assert.Equal(t, "http.request", got["event"])
	assert.Equal(t, "GET", got["http.method"])
	assert.Equal(t, float64(200), got["http.status"])
	assert.Contains(t, got, "duration_ms")
}

func TestBeginEvent_PanicInHandlerIsCapturedAndRethrown(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	defer func() {
		r := recover()
		assert.NotNil(t, r)
		assert.Equal(t, "boom", r)

		var got map[string]any
		_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
		assert.Equal(t, "ERROR", got["level"])
		assert.Equal(t, "boom", got["panic"])
		assert.Contains(t, got, "stack")
	}()

	func() {
		ctx, end := BeginEvent(context.Background(), "task")
		defer end()
		_ = ctx
		panic("boom")
	}()
}

func TestEndFunc_IsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	_, end := BeginEvent(context.Background(), "x")
	end()
	end()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1)
}

func TestBeginEvent_NestedReturnsExistingAndMarksMisuse(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	outerCtx, endOuter := BeginEvent(context.Background(), "outer")
	innerCtx, endInner := BeginEvent(outerCtx, "inner")

	// Inner ctx must carry the outer event (nested not allowed).
	assert.Equal(t, "outer", FromContext(innerCtx).Name())

	endInner() // no-op
	endOuter() // emits once

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1)

	var got map[string]any
	_ = json.Unmarshal(lines[0], &got)
	assert.Equal(t, true, got["nested_begin"])
	assert.Equal(t, "outer", got["msg"])
}

func TestEvent_NilReceiverMethodsAreSafe(t *testing.T) {
	var ev *Event // nil

	assert.NotPanics(t, func() { _ = ev.Name() })
	assert.NotPanics(t, func() { ev.Set("k", "v") })
	assert.NotPanics(t, func() { ev.SetAttrs(map[string]any{"a": 1}) })
	assert.NotPanics(t, func() { ev.Counter("c", 1) })
	assert.NotPanics(t, func() { ev.Err(errors.New("x")) })
	assert.NotPanics(t, func() { ev.Err(nil) })

	assert.Equal(t, "", ev.Name())
}

func TestFromContext_ResultMethodsAreNilSafe(t *testing.T) {
	// FromContext returns nil when no event is active; callers should be
	// able to chain methods without nil checks for convenience.
	ctx := context.Background()

	assert.NotPanics(t, func() {
		FromContext(ctx).Set("k", "v")
		FromContext(ctx).Counter("c", 1)
		FromContext(ctx).Err(errors.New("x"))
	})
}

func TestEvent_SnapshotOnNilReceiver(t *testing.T) {
	var ev *Event
	assert.NotPanics(t, func() { _ = ev.snapshot() })
}

func TestBeginEvent_NilContextIsTreatedAsBackground(t *testing.T) {
	assert.NotPanics(t, func() {
		//nolint:staticcheck // intentionally passing nil to exercise the guard
		ctx, end := BeginEvent(nil, "test.event")
		defer end()

		assert.NotNil(t, ctx)
		assert.NotNil(t, FromContext(ctx))
	})
}

func TestSetGlobal_IgnoresNil(t *testing.T) {
	prev := globalLogger
	defer func() { globalLogger = prev }()

	SetGlobal(nil)
	assert.NotNil(t, globalLogger, "SetGlobal(nil) must not leave the global in a nil state")

	// Subsequent calls must not panic.
	assert.NotPanics(t, func() { Infof("still works") })
}

func TestSetEventName_OverridesCanonicalLine(t *testing.T) {
	var buf bytes.Buffer
	prev := globalLogger
	defer func() { globalLogger = prev }()
	globalLogger = newSlogTestLogger(t, &buf)

	ctx, end := BeginEvent(context.Background(), "http.request")
	SetEventName(ctx, "controltower.v1.PingService/Ping")
	end()

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "controltower.v1.PingService/Ping", got["msg"])
	assert.Equal(t, "controltower.v1.PingService/Ping", got["event"])
}

func TestEvent_SetName_OnInstance(t *testing.T) {
	ctx, end := BeginEvent(context.Background(), "old")
	defer end()

	FromContext(ctx).SetName("new")
	assert.Equal(t, "new", FromContext(ctx).Name())
}

func TestEvent_SetName_NilSafe(t *testing.T) {
	var ev *Event
	assert.NotPanics(t, func() { ev.SetName("x") })

	// Top-level helper with no active event is also a no-op.
	assert.NotPanics(t, func() { SetEventName(context.Background(), "x") })
}

func TestEmitCanonical_NilEventIsSafe(t *testing.T) {
	// Both slog and zap wrappers implement canonicalEmitter; a defensive
	// check lives in each. Verify via the nopLogger path and both real
	// wrappers through a shared assertion.
	nop := NewNopLogger().(canonicalEmitter)
	assert.NotPanics(t, func() { nop.emitCanonical(nil) })

	slogW := &slogLoggerWrapper{logger: slog.Default()}
	assert.NotPanics(t, func() { slogW.emitCanonical(nil) })
}
