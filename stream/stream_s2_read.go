package stream

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/s2-streamstore/s2-sdk-go/s2"
)

type s2StreamReadSession struct {
	session *s2.ReadSession
	// ctx is the session-bound context. The S2 SDK can close its record channel
	// on context cancellation without surfacing the cause via ReadSession.Err(),
	// so Next consults ctx before falling back to io.EOF — otherwise a shutdown
	// reads as a clean end-of-stream.
	ctx context.Context //nolint:containedctx // bound to the read session lifetime
}

var _ StreamReadSession = &s2StreamReadSession{}

// NewS2StreamReadSession opens a blocking S2 read session bound to ctx, resuming
// from opts.StartPosition (or the stream start / tail per opts). The stream and
// basin must already exist in the S2 service.
//
// The session is the read-side dual of NewS2StreamWriter: same basin resolution
// and stream addressing, opposite direction. Reopening at a cursor (after a Nack
// or a transient transport error) is the caller's job — construct a fresh session
// with the persisted StartPosition; the session itself holds no cursor.
func NewS2StreamReadSession(ctx context.Context, config S2StreamProviderConfig,
	basinResolver S2BasinResolver, stream Stream, opts StreamReadOptions) (StreamReadSession, error) {

	if config.ApiKey == "" {
		return nil, fmt.Errorf("S2 API key is not set")
	}

	basin, err := basinResolver.GetBasin(ctx, stream.Namespace, stream.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get basin: %w", err)
	}

	streamID, err := stream.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream ID: %w", err)
	}

	readOpts, err := s2ReadOptionsFrom(opts)
	if err != nil {
		return nil, err
	}

	client := s2.New(config.ApiKey, nil).Basin(basin).Stream(s2.StreamName(streamID))
	session, err := client.ReadSession(ctx, readOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open read session: %w", err)
	}

	return &s2StreamReadSession{session: session, ctx: ctx}, nil
}

func (s *s2StreamReadSession) Next() (*StreamRecord, error) {
	if s.session.Next() {
		return streamRecordFromS2(s.session.Record()), nil
	}

	// Next returned false: the session hit a terminal transport error, the bound
	// context is done, or the stream is exhausted / closed.
	return resolveTerminalErr(s.session.Err(), s.ctx.Err())
}

// resolveTerminalErr picks the terminal error to report when a read session stops
// yielding records. The session's own error wins; otherwise the bound context's
// error (the SDK can cancel without setting Err); otherwise a clean end-of-stream.
func resolveTerminalErr(sessionErr, ctxErr error) (*StreamRecord, error) {
	switch {
	case sessionErr != nil:
		return nil, sessionErr
	case ctxErr != nil:
		return nil, ctxErr
	default:
		return nil, io.EOF
	}
}

func (s *s2StreamReadSession) Close() error {
	return s.session.Close()
}

// streamRecordFromS2 maps an S2 sequenced record onto the transport-neutral
// StreamRecord, deriving the resume cursor (Next = SeqNum+1) so a consumer that
// persists Next resumes one past this record.
func streamRecordFromS2(rec s2.SequencedRecord) *StreamRecord {
	headers := make(map[string]string, len(rec.Headers))
	for _, h := range rec.Headers {
		headers[string(h.Name)] = string(h.Value)
	}

	// Guard the +1 against uint64 wraparound: at the max sequence number Next
	// stays put rather than rewinding the cursor to 0. Unreachable in practice
	// (an S2 stream will not hit 2^64 records) but a silent rewind would be a
	// nasty failure mode, so pin it.
	next := rec.SeqNum
	if next < math.MaxUint64 {
		next++
	}

	return &StreamRecord{
		Body:     rec.Body,
		Headers:  headers,
		Position: encodeS2Position(rec.SeqNum),
		Next:     encodeS2Position(next),
	}
}

// s2ReadOptionsFrom maps the provider-agnostic StreamReadOptions onto S2 read
// options. A persisted cursor (StartPosition) takes precedence over the FromTail
// start intent.
func s2ReadOptionsFrom(opts StreamReadOptions) (*s2.ReadOptions, error) {
	// Filter S2 command records (trim/fence) client-side: they are stream
	// management artifacts, not events, and would otherwise be delivered as
	// payloads and poison a typed consumer's decode. The cursor still advances
	// past their sequence numbers.
	if opts.StartPosition == "" && opts.FromTail {
		return &s2.ReadOptions{TailOffset: s2.Int64(0), IgnoreCommandRecords: true}, nil
	}

	startSeq, err := decodeS2Position(opts.StartPosition)
	if err != nil {
		return nil, err
	}

	return &s2.ReadOptions{SeqNum: s2.Uint64(startSeq), IgnoreCommandRecords: true}, nil
}

// encodeS2Position serializes an S2 sequence number into the opaque position
// string carried through the consumer. The encoding is decimal text — reversible
// via decodeS2Position, stable across builds, comparable by equality only (not
// ordering; switch to fixed-width zero-padded if ordered comparison is needed).
func encodeS2Position(seqNum uint64) string {
	return strconv.FormatUint(seqNum, 10)
}

// decodeS2Position parses an opaque position back into an S2 sequence number.
// Empty means "from the beginning" and returns 0 (S2's lowest-retained sentinel).
func decodeS2Position(position string) (uint64, error) {
	if position == "" {
		return 0, nil
	}

	seq, err := strconv.ParseUint(position, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid s2 position %q: %w", position, err)
	}

	return seq, nil
}
