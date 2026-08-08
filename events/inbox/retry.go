package inbox

import (
	"context"

	"github.com/safedep/dry/log"
)

// DefaultMaxAttempts is the retry budget NewRetryPolicy uses when the caller does
// not set one: a record is retried this many times before it is dead-lettered.
const DefaultMaxAttempts = 4

type retryConfig struct {
	maxAttempts int
	permanent   func(error) bool
}

// RetryOption configures NewRetryPolicy.
type RetryOption func(*retryConfig)

// WithMaxAttempts sets the retry budget before a record is dead-lettered. Values
// below 1 are ignored (the default stands), so a record is always tried at least
// once.
func WithMaxAttempts(n int) RetryOption {
	return func(c *retryConfig) {
		if n >= 1 {
			c.maxAttempts = n
		}
	}
}

// WithPermanentClassifier supplies a consumer-owned predicate that reports
// whether an error is permanent (a poison record that will never succeed). When
// it returns true the record is dead-lettered immediately, skipping the retry
// budget. This is the seam for transport- or storage-specific knowledge (e.g. a
// Postgres data-exception SQLSTATE) so the generic policy stays free of it.
func WithPermanentClassifier(f func(error) bool) RetryOption {
	return func(c *retryConfig) { c.permanent = f }
}

// NewRetryPolicy returns an ErrorHandler that retries a failing record a bounded
// number of times and then dead-letters it, so one poison record can never
// head-of-line-block the feed forever (the default Retry policy's failure mode).
//
// A record's identity is a fingerprint of its raw bytes. Consume is single-
// threaded and a Nack redelivers the same record as the very next Receive, so a
// consecutive-failure counter over the last-seen fingerprint is sufficient — no
// per-record persistence. The counter resets when a different record arrives or
// the process restarts; a restart therefore re-grants the budget, which still
// bounds retries per run and eventually dead-letters a durable poison record.
//
// A record is dead-lettered when the classifier marks it permanent, or the retry
// budget is exhausted. If dlq is nil, or the dead-letter Store fails, the record
// is retried instead of skipped — the feed never drops data it could not first
// persist.
func NewRetryPolicy(dlq DeadLetterQueue, opts ...RetryOption) ErrorHandler {
	cfg := retryConfig{maxAttempts: DefaultMaxAttempts}
	for _, o := range opts {
		o(&cfg)
	}

	var (
		lastKey  string
		attempts int
	)

	return func(ctx context.Context, d *Delivery, cause error) Disposition {
		key := payloadFingerprint(d.Payload)
		if key == lastKey {
			attempts++
		} else {
			lastKey = key
			attempts = 1
		}

		permanent := cfg.permanent != nil && cfg.permanent(cause)
		if !permanent && attempts < cfg.maxAttempts {
			return Retry
		}

		if dlq == nil {
			// No sink wired: preserve the record by retrying rather than dropping
			// it. This keeps the zero-DLQ configuration as safe as the legacy
			// infinite-Retry default.
			return Retry
		}

		if err := dlq.Store(ctx, DeadLetterRecord{
			Payload:  d.Payload,
			Error:    cause.Error(),
			Attempts: attempts,
		}); err != nil {
			log.Warnf("inbox: dead-letter store failed (record will be retried): %v", err)
			return Retry
		}

		log.Warnf("inbox: dead-lettered record after %d attempt(s): %v", attempts, cause)
		return Skip
	}
}
