package log

import "context"

// RunCommand runs fn inside a canonical event named "cmd.<name>",
// recording any returned error and emitting exactly one log line.
// Intended for CLI entry points (cobra.Command.RunE adapters or plain
// scripts).
func RunCommand(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	ctx, end := BeginEvent(ctx, "cmd."+name)
	defer end()

	err := fn(ctx)
	if err != nil {
		Err(ctx, err)
	}
	return err
}
