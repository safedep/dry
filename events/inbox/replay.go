package inbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/safedep/dry/events"
	"google.golang.org/protobuf/proto"
)

// ReplayOutcome is the result for one dead-letter record. Err is nil when the
// record was reprocessed and removed; otherwise it is the decode / envelope /
// handler failure that kept the record parked.
type ReplayOutcome struct {
	ID      uint64
	EventID string
	Err     error
}

// ReplayResult summarises a Replay run. Outcomes is per-record, in id order.
type ReplayResult struct {
	Succeeded int
	Failed    int
	Outcomes  []ReplayOutcome
}

// Replay feeds each parked record's raw bytes back through the original decode →
// envelope → handler path — the same ladder Consume uses — and deletes the record
// when the handler succeeds. It drains every record matching f, paging internally
// (f.Limit is the page size), and returns a per-record summary.
//
// A record only clears if the handler can now process bytes it previously could
// not: the earlier failure was transient (e.g. the store was down), or a code
// change now handles them (e.g. a consumer that now sanitizes a byte its schema
// rejects). A record that is permanently poison — no code change makes it
// processable — fails replay again and stays parked; purge it via
// DeadLetterReader.Delete. Replay never re-parks a failed record it already saw:
// it pages by advancing past the highest id seen, failures included, so the drain
// always terminates.
//
// The handler runs with the same at-least-once, must-be-idempotent contract as in
// Consume. A List error aborts and is returned; a single record's failure is
// recorded in its outcome and does not stop the run.
func Replay[T proto.Message](
	ctx context.Context,
	reader DeadLetterReader,
	newEvent func() T,
	handler Handler[T],
	f DeadLetterFilter,
) (ReplayResult, error) {
	if reader == nil {
		return ReplayResult{}, errors.New("inbox: replay: reader is required")
	}
	if newEvent == nil {
		return ReplayResult{}, errors.New("inbox: replay: newEvent constructor is required")
	}
	if handler == nil {
		return ReplayResult{}, errors.New("inbox: replay: handler is required")
	}
	// Same fail-fast as Consume: a nil message would panic proto.Unmarshal.
	if nilMessage(newEvent()) {
		return ReplayResult{}, errors.New("inbox: replay: newEvent must return a non-nil message")
	}

	var res ReplayResult
	afterID := f.AfterID
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		page := f
		page.AfterID = afterID
		rows, err := reader.List(ctx, page)
		if err != nil {
			return res, fmt.Errorf("inbox: replay: %w", err)
		}
		if len(rows) == 0 {
			return res, nil
		}
		for i := range rows {
			// Stop before touching the next buffered row once the caller cancels:
			// otherwise a context-unaware handler runs side effects past the cancel
			// while the context-aware delete fails and re-parks the row.
			if err := ctx.Err(); err != nil {
				return res, err
			}
			row := rows[i]
			// Advance the cursor past every row we touch, including failures, so a
			// permanently-failing record is not re-listed on the next page — that
			// would loop forever.
			if row.ID > afterID {
				afterID = row.ID
			}
			out := ReplayOutcome{ID: row.ID, EventID: row.EventID}
			if err := replayOne(ctx, reader, row, newEvent, handler); err != nil {
				out.Err = err
				res.Failed++
			} else {
				res.Succeeded++
			}
			res.Outcomes = append(res.Outcomes, out)
		}
	}
}

func replayOne[T proto.Message](
	ctx context.Context,
	reader DeadLetterReader,
	row DeadLetter,
	newEvent func() T,
	handler Handler[T],
) error {
	event := newEvent()
	if err := proto.Unmarshal(row.Payload, event); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	meta, err := events.MetaOf(event)
	if err != nil {
		return fmt.Errorf("envelope: %w", err)
	}
	if err := handler(ctx, event, meta); err != nil {
		return fmt.Errorf("handler: %w", err)
	}
	if err := reader.Delete(ctx, row.ID); err != nil {
		return fmt.Errorf("delete after replay: %w", err)
	}
	return nil
}
