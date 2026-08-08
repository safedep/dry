package inbox

import "time"

// redeliveryBackoff paces repeated redeliveries of one failing record, keyed by
// the record's payload fingerprint. The first redelivery is immediate, so a
// one-off transient failure costs no latency; consecutive redeliveries of the
// same record then wait base, 2*base, ... capped at max. A different record or
// a reset clears the escalation.
//
// This bounds poison-record read amplification at the source: on S2 every
// redelivery reopens a streaming read session at the stalled cursor and the
// server re-pushes the backlog behind it, so unpaced retries of a permanently
// failing record turn into unbounded metered reads.
type redeliveryBackoff struct {
	base, max time.Duration
	key       string
	delay     time.Duration
}

// next reports how long to wait before redelivering the record identified by
// key: zero for its first redelivery, then base doubling up to max.
func (b *redeliveryBackoff) next(key string) time.Duration {
	if key != b.key {
		b.key = key
		b.delay = 0
		return 0
	}

	switch {
	case b.delay == 0:
		b.delay = b.base
	case b.delay >= b.max/2:
		// Doubling would overshoot the cap (or overflow the duration).
		b.delay = b.max
	default:
		b.delay *= 2
	}

	return b.delay
}

func (b *redeliveryBackoff) reset() {
	b.key = ""
	b.delay = 0
}
