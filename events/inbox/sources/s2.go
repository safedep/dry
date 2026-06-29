// Package sources holds the inbound transport adapters for dry/events/inbox.
// It is split from inbox so the Consume core stays transport-free.
package sources

import (
	"context"
	"errors"
	"fmt"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/stream"
)

type s2Source struct {
	stream        stream.Stream
	config        stream.S2StreamProviderConfig
	basinResolver stream.S2BasinResolver
	cursors       inbox.CursorStore
	consumerName  string
	feed          string

	// session is the open read session, or nil when a (re)open is pending. A Nack
	// or a transport error closes it so the next Receive reopens at the persisted
	// cursor — the sequential S2 read model means redelivery is a reopen, never a
	// "rewind in place".
	session stream.StreamReadSession

	// lastOpenPosition + redeliverAttempts detect a stalled cursor: consecutive
	// reopens at the same position mean a record is being redelivered without ever
	// advancing (a poison record blocking the feed).
	lastOpenPosition  string
	redeliverAttempts int

	// newSession opens a read session at a position. Defaults to the real S2 read
	// session; tests inject a fake to exercise reopen/stall/cursor logic offline.
	newSession func(ctx context.Context, startPosition string) (stream.StreamReadSession, error)

	// leader, when set (WithLeader), gates reading so only one replica consumes
	// the feed — required because the S2 cursor is client-side and two readers
	// would clobber it. Nil for single-replica deployments.
	leader *advisoryLeader

	// leaderAdapter is staged by WithLeader and consumed by NewS2 once the cursor
	// key (consumer + feed) is known, then discarded.
	leaderAdapter db.SqlDataAdapter
}

var _ inbox.Source = &s2Source{}

// S2Option configures an S2 inbox Source.
type S2Option func(*s2Source)

// WithLeader makes the source safe to run on every replica: only the holder of a
// Postgres advisory lock (keyed on the consumer + feed) actually reads; the rest
// idle and take over if it dies. Requires a PostgreSQL adapter — NewS2 errors
// otherwise. Omit it for single-replica deployments (S2's server side does not
// elect for us, unlike a NATS durable consumer).
func WithLeader(adapter db.SqlDataAdapter) S2Option {
	return func(s *s2Source) { s.leaderAdapter = adapter }
}

// NewS2 builds an S2 inbox Source for one event feed. The caller resolves the
// stream identity — stream.StreamFor(routing) for a global feed, or
// stream.StreamForWithTenant(routing, tenant) for a tenant-scoped one — mirroring
// how the outbox publisher addresses the same feed; reading a different identity
// than the publisher wrote would silently see no records.
//
// The cursor is keyed by the full stream id (which encodes exposure and tenant),
// so feeds that differ only by exposure or tenant never share a cursor row. The
// cursor lives in the consumer's DB (via cursors); consumerName is the cursor key
// and identifies this consumer in stalled-cursor diagnostics. A nil basinResolver
// uses the default.
func NewS2(feedStream stream.Stream, config stream.S2StreamProviderConfig,
	basinResolver stream.S2BasinResolver, cursors inbox.CursorStore, consumerName string, opts ...S2Option) (inbox.Source, error) {

	if config.ApiKey == "" {
		return nil, fmt.Errorf("inbox/s2: S2 API key is not set")
	}
	if cursors == nil {
		return nil, fmt.Errorf("inbox/s2: cursor store is required")
	}
	if consumerName == "" {
		return nil, fmt.Errorf("inbox/s2: consumer name is required")
	}

	feed, err := feedStream.ID()
	if err != nil {
		return nil, fmt.Errorf("inbox/s2: invalid stream: %w", err)
	}

	if basinResolver == nil {
		basinResolver = stream.NewDefaultS2BasinResolver()
	}

	src := &s2Source{
		stream:        feedStream,
		config:        config,
		basinResolver: basinResolver,
		cursors:       cursors,
		consumerName:  consumerName,
		feed:          feed,
	}
	for _, opt := range opts {
		opt(src)
	}
	if src.leaderAdapter != nil {
		leader, err := newAdvisoryLeader(src.leaderAdapter, leaderKey(consumerName, feed))
		if err != nil {
			return nil, err
		}
		src.leader = leader
		src.leaderAdapter = nil
	}
	src.newSession = func(ctx context.Context, startPosition string) (stream.StreamReadSession, error) {
		return stream.NewS2StreamReadSession(ctx, src.config, src.basinResolver, src.stream,
			stream.StreamReadOptions{StartPosition: startPosition})
	}
	return src, nil
}

func (s *s2Source) Receive(ctx context.Context) (*inbox.Delivery, error) {
	// readCtx is the context the read session is bound to. With a leader it is the
	// leadership-term context, so a read blocked in Next() unwinds the instant
	// leadership is lost (preventing a former leader from reading alongside a new
	// one). Without a leader it is just the consumer's context.
	readCtx := ctx
	if s.leader != nil {
		// Block until we hold leadership. A standby parks here until the current
		// leader dies; a fresh acquisition (reacquired) means another replica may
		// have advanced the cursor, so drop our session and resume from it.
		leadCtx, reacquired, err := s.leader.ensureLeading(ctx)
		if err != nil {
			return nil, err
		}
		if reacquired {
			s.closeSession()
		}
		readCtx = leadCtx
	}

	if s.session == nil {
		if err := s.reopen(readCtx); err != nil {
			return nil, err
		}
	}

	rec, err := s.session.Next()
	if err != nil {
		// Drop the session so the next Receive reopens at the persisted cursor.
		s.closeSession()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr // the consumer's own context ended → terminal
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// The lease context (not the consumer's) was cancelled: leadership was
			// lost mid-read. Surface a non-terminal error so Consume backs off and
			// the next Receive re-acquires leadership and reopens at the cursor.
			return nil, errLeadershipLost
		}
		return nil, err // io.EOF / transport error: Consume backs off and retries
	}

	return &inbox.Delivery{
		Payload: rec.Body,
		// Advance under readCtx so a cursor write by a former leader fails once its
		// lease is cancelled, rather than racing the new leader's advance.
		Ack: func() error { return s.ack(readCtx, rec.Next) },
		Nack: func() error {
			// Tear down so the next Receive reopens at the (un-advanced) cursor,
			// redelivering this record and everything after it.
			s.closeSession()
			return nil
		},
	}, nil
}

// errLeadershipLost signals that the read stopped because this replica lost
// leadership (its lease context was cancelled) while the consumer is still
// running. It is non-terminal: Consume backs off and re-acquires.
var errLeadershipLost = errors.New("inbox/s2: leadership lost")

// reopen loads the persisted cursor and opens a fresh read session there, warning
// when consecutive reopens land on the same position (a stalled/poison record).
func (s *s2Source) reopen(ctx context.Context) error {
	position, err := s.cursors.Load(ctx, s.consumerName, s.feed)
	if err != nil && !errors.Is(err, inbox.ErrNoCursor) {
		return err
	}
	// ErrNoCursor leaves position == "" — the bootstrap signal that opens the
	// read session at the start of the stream (StreamReadOptions treats an empty
	// StartPosition as "from the beginning").

	if position == s.lastOpenPosition {
		s.redeliverAttempts++
	} else {
		s.lastOpenPosition = position
		s.redeliverAttempts = 1
	}
	if s.redeliverAttempts > 1 {
		log.Warnf("inbox/s2: stalled cursor consumer=%s feed=%s position=%q attempts=%d "+
			"(a record is repeatedly failing and blocking the feed; wire WithErrorHandler->Skip or a DLQ)",
			s.consumerName, s.feed, position, s.redeliverAttempts)
	}

	session, err := s.newSession(ctx, position)
	if err != nil {
		return err
	}
	s.session = session
	return nil
}

// ack persists the cursor at the record's Next position. A persist failure is a
// non-advance: tear down so the record redelivers rather than being skipped.
func (s *s2Source) ack(ctx context.Context, nextPosition string) error {
	if err := s.cursors.Advance(ctx, s.consumerName, s.feed, nextPosition); err != nil {
		s.closeSession()
		return err
	}
	return nil
}

func (s *s2Source) closeSession() {
	if s.session != nil {
		if err := s.session.Close(); err != nil {
			log.Warnf("inbox/s2: close read session: %v", err)
		}
		s.session = nil
	}
}
