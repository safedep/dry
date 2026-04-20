package async

import (
	"context"

	drylog "github.com/safedep/dry/log"
)

// WithEventLogging wraps a MessageHandler so every invocation produces
// exactly one canonical log line including msg.subject, msg.reply_to,
// msg.bytes, handler.duration_ms, and any error.
//
// Opt-in per consumer: callers apply this wrapper when registering
// a QueueSubscribe callback.
func WithEventLogging(name string, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, data []byte, extra MessageExtra) error {
		ctx, end := drylog.BeginEvent(ctx, name,
			drylog.WithEventAttrs(map[string]any{
				"msg.subject":  extra.Subject,
				"msg.reply_to": extra.ReplyTo,
				"msg.bytes":    len(data),
			}),
		)
		defer end()

		err := handler(ctx, data, extra)
		if err != nil {
			drylog.Err(ctx, err)
		}
		return err
	}
}
