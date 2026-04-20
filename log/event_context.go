package log

import "context"

type eventContextKeyT struct{}

var eventContextKey = eventContextKeyT{}

func fromContext(ctx context.Context) *Event {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(eventContextKey).(*Event)
	return v
}

func withContext(ctx context.Context, ev *Event) context.Context {
	return context.WithValue(ctx, eventContextKey, ev)
}
