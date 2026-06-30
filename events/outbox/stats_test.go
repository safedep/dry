package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRecord inserts one outbox record with a controlled FQN, emission time, and
// delivery state directly (dry owns the model), so a stats scenario can be built
// without driving the drain into each state.
func seedRecord(t *testing.T, a testAdapter, id, fqn string, createdAt time.Time, delivered, stuck bool) {
	t.Helper()
	gdb, err := a.GetDB()
	require.NoError(t, err)

	rec := &Record{EventID: id, FQN: fqn, Subject: "pkg:npm/x", CreatedAt: createdAt}
	if delivered {
		d := createdAt.Add(time.Minute)
		rec.DeliveredAt = &d
	}
	require.NoError(t, gdb.Create(rec).Error)

	del := &Delivery{OutboxID: rec.ID, Destination: "s2", Subject: rec.Subject}
	if delivered {
		p := createdAt.Add(time.Minute)
		del.PublishedAt = &p
	}
	if stuck {
		s := createdAt.Add(2 * time.Minute)
		del.StuckSince = &s
	}
	require.NoError(t, gdb.Create(del).Error)
}

// statsFixture seeds a known mix of records across two feeds and delivery states.
//
//	A: r1 delivered | r2 pending | r3 stuck
//	B: r4 delivered | r5 pending
func statsFixture(t *testing.T, now time.Time) *StatsReader {
	t.Helper()
	a := newStore(t)
	seedRecord(t, a, "r1", "feed.A", now.Add(-10*time.Minute), true, false)
	seedRecord(t, a, "r2", "feed.A", now.Add(-5*time.Minute), false, false)
	seedRecord(t, a, "r3", "feed.A", now.Add(-1*time.Minute), false, true)
	seedRecord(t, a, "r4", "feed.B", now.Add(-3*time.Minute), true, false)
	seedRecord(t, a, "r5", "feed.B", now.Add(-20*time.Minute), false, false)

	r, err := NewStatsReader(a)
	require.NoError(t, err)
	return r
}

func TestNewStatsReader_NilStore(t *testing.T) {
	_, err := NewStatsReader(nil)
	require.Error(t, err)
}

func TestStatsReader_StateTotals(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	r := statsFixture(t, now)

	tests := []struct {
		name  string
		since time.Time
		want  StateTotals
	}{
		{
			name:  "all history partitions into delivered + pending, stuck is a subset",
			since: time.Time{},
			want:  StateTotals{Emitted: 5, Delivered: 2, Pending: 3, Stuck: 1},
		},
		{
			name:  "since window drops the two oldest records",
			since: now.Add(-6 * time.Minute), // keeps r2, r3, r4; drops r1, r5
			want:  StateTotals{Emitted: 3, Delivered: 1, Pending: 2, Stuck: 1},
		},
		{
			name:  "future since yields an empty window",
			since: now.Add(time.Hour),
			want:  StateTotals{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.StateTotals(context.Background(), tt.since)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			// Delivered + Pending always reconstruct Emitted.
			assert.Equal(t, got.Emitted, got.Delivered+got.Pending)
		})
	}
}

func TestStatsReader_PerFQN(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	r := statsFixture(t, now)

	t.Run("all history, busiest feed first", func(t *testing.T) {
		got, err := r.PerFQN(context.Background(), time.Time{})
		require.NoError(t, err)
		require.Len(t, got, 2)

		assert.Equal(t, "feed.A", got[0].FQN)
		assert.Equal(t, int64(3), got[0].Emitted)
		assert.Equal(t, int64(1), got[0].Delivered)
		assert.Equal(t, int64(2), got[0].Pending)
		assert.WithinDuration(t, now.Add(-1*time.Minute), got[0].LastEmitted, time.Second)

		assert.Equal(t, "feed.B", got[1].FQN)
		assert.Equal(t, int64(2), got[1].Emitted)
		assert.Equal(t, int64(1), got[1].Delivered)
		assert.Equal(t, int64(1), got[1].Pending)
		assert.WithinDuration(t, now.Add(-3*time.Minute), got[1].LastEmitted, time.Second)
	})

	t.Run("since window narrows per-feed counts", func(t *testing.T) {
		got, err := r.PerFQN(context.Background(), now.Add(-6*time.Minute))
		require.NoError(t, err)
		require.Len(t, got, 2)

		byFQN := map[string]FQNStat{got[0].FQN: got[0], got[1].FQN: got[1]}
		assert.Equal(t, int64(2), byFQN["feed.A"].Emitted) // r2, r3
		assert.Equal(t, int64(0), byFQN["feed.A"].Delivered)
		assert.Equal(t, int64(1), byFQN["feed.B"].Emitted) // r4
		assert.Equal(t, int64(1), byFQN["feed.B"].Delivered)
	})

	t.Run("empty store yields no rows", func(t *testing.T) {
		empty, err := NewStatsReader(newStore(t))
		require.NoError(t, err)
		got, err := empty.PerFQN(context.Background(), time.Time{})
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
