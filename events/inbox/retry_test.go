package inbox_test

import (
	"context"
	"errors"
	"testing"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDLQ struct {
	stored  []inbox.DeadLetterRecord
	failErr error
}

func (f *fakeDLQ) Store(_ context.Context, rec inbox.DeadLetterRecord) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.stored = append(f.stored, rec)
	return nil
}

func delivery(payload string) *inbox.Delivery {
	return &inbox.Delivery{Payload: []byte(payload)}
}

func TestRetryPolicy_RetriesUntilBudgetThenDeadLetters(t *testing.T) {
	dlq := &fakeDLQ{}
	policy := inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(3))
	cause := errors.New("boom")
	d := delivery("poison")

	// Same record fails repeatedly. First two attempts retry; the third exhausts
	// the budget and dead-letters.
	assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
	assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
	assert.Empty(t, dlq.stored, "not dead-lettered before the budget is exhausted")

	assert.Equal(t, inbox.Skip, policy(t.Context(), d, cause))
	require.Len(t, dlq.stored, 1)
	assert.Equal(t, []byte("poison"), dlq.stored[0].Payload)
	assert.Equal(t, "boom", dlq.stored[0].Error)
	assert.Equal(t, 3, dlq.stored[0].Attempts)
}

func TestRetryPolicy_PermanentClassifierDeadLettersImmediately(t *testing.T) {
	dlq := &fakeDLQ{}
	permanent := func(err error) bool { return err.Error() == "permanent" }
	policy := inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(5), inbox.WithPermanentClassifier(permanent))

	assert.Equal(t, inbox.Skip, policy(t.Context(), delivery("p"), errors.New("permanent")))
	require.Len(t, dlq.stored, 1, "permanent error skips the retry budget")
	assert.Equal(t, 1, dlq.stored[0].Attempts)
}

func TestRetryPolicy_TransientErrorRetriesUnderClassifier(t *testing.T) {
	dlq := &fakeDLQ{}
	permanent := func(err error) bool { return err.Error() == "permanent" }
	policy := inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(3), inbox.WithPermanentClassifier(permanent))

	// A non-permanent error still gets the full retry budget.
	assert.Equal(t, inbox.Retry, policy(t.Context(), delivery("p"), errors.New("transient")))
	assert.Empty(t, dlq.stored)
}

func TestRetryPolicy_CounterResetsOnDifferentRecord(t *testing.T) {
	dlq := &fakeDLQ{}
	policy := inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(2))
	cause := errors.New("boom")

	// Interleave two distinct records: each must accrue its own attempt count, so
	// neither dead-letters on its first failure.
	assert.Equal(t, inbox.Retry, policy(t.Context(), delivery("a"), cause))
	assert.Equal(t, inbox.Retry, policy(t.Context(), delivery("b"), cause))
	assert.Empty(t, dlq.stored)

	// Record "a" fails a second consecutive time → budget (2) exhausted.
	assert.Equal(t, inbox.Retry, policy(t.Context(), delivery("a"), cause)) // resets: a != last(b)
	assert.Equal(t, inbox.Skip, policy(t.Context(), delivery("a"), cause))
	require.Len(t, dlq.stored, 1)
	assert.Equal(t, []byte("a"), dlq.stored[0].Payload)
}

func TestRetryPolicy_NilDLQNeverSkips(t *testing.T) {
	policy := inbox.NewRetryPolicy(nil, inbox.WithMaxAttempts(2))
	cause := errors.New("boom")
	d := delivery("poison")

	// Without a sink, an exhausted budget must retry, never drop the record.
	assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
	assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
	assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
}

func TestRetryPolicy_StoreFailureRetries(t *testing.T) {
	dlq := &fakeDLQ{failErr: errors.New("db down")}
	policy := inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(1))

	// Budget is 1, so the first failure would dead-letter — but the store fails, so
	// the record is retried rather than skipped-and-lost.
	assert.Equal(t, inbox.Retry, policy(t.Context(), delivery("p"), errors.New("boom")))
}

func TestRetryPolicy_DefaultBudget(t *testing.T) {
	dlq := &fakeDLQ{}
	policy := inbox.NewRetryPolicy(dlq) // DefaultMaxAttempts
	cause := errors.New("boom")
	d := delivery("poison")

	for i := 1; i < inbox.DefaultMaxAttempts; i++ {
		assert.Equal(t, inbox.Retry, policy(t.Context(), d, cause))
	}
	assert.Equal(t, inbox.Skip, policy(t.Context(), d, cause))
	assert.Len(t, dlq.stored, 1)
}

// End-to-end: a poison record redelivered through Consume is retried up to the
// budget, dead-lettered, and acked so the cursor can advance past it.
func TestConsume_RetryPolicyDeadLettersPoisonRecord(t *testing.T) {
	poison := eventBytes(t, "evt-poison")
	src := &fakeSource{steps: []step{
		{payload: poison},
		{payload: poison},
		{payload: poison},
	}}
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return errors.New("boom")
	}

	dlq := &fakeDLQ{}
	err := inbox.Consume(t.Context(), src, newObservation, handler,
		inbox.WithErrorHandler(inbox.NewRetryPolicy(dlq, inbox.WithMaxAttempts(3))),
	)
	require.ErrorIs(t, err, context.Canceled)

	assert.Equal(t, 2, src.nacks, "first two attempts retry")
	assert.Equal(t, 1, src.acks, "third attempt is dead-lettered and acked")
	require.Len(t, dlq.stored, 1)
	assert.Equal(t, 3, dlq.stored[0].Attempts)
}
