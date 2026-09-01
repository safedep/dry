package endpointsync

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func hostObservationRule(window time.Duration) DedupRule {
	return DedupRule{
		Name:   "host-observation",
		Window: window,
		Key: func(e *servicev1.ToolEvent) ([]string, bool) {
			ho := e.GetPmgEvent().GetHostObservation()
			if ho == nil {
				return nil, false
			}
			return []string{ho.GetHostname(), ho.GetMethod()}, true
		},
	}
}

func newDedupEmitter(t *testing.T, walPath string, opts ...SyncOption) *EventEmitterClient {
	t.Helper()
	opts = append([]SyncOption{WithWALPath(walPath)}, opts...)
	client, err := NewEventEmitterClient("test-tool", "1.0.0", opts...)
	require.NoError(t, err)
	return client
}

type eventFactory interface {
	NewEvent() (*servicev1.ToolEvent, error)
}

func hostObsEvent(t *testing.T, client eventFactory, hostname string, method string) *servicev1.ToolEvent {
	t.Helper()
	event, err := client.NewEvent()
	require.NoError(t, err)
	event.PmgEvent = &controltowerv1.PmgEvent{
		EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_HOST_OBSERVATION,
		HostObservation: &controltowerv1.PmgHostObservation{
			Hostname: hostname,
			Method:   method,
		},
	}
	return event
}

func pendingToolEvents(t *testing.T, w *eventWriter) []*servicev1.ToolEvent {
	t.Helper()
	rows, err := w.store.readPending(1000)
	require.NoError(t, err)

	events := make([]*servicev1.ToolEvent, 0, len(rows))
	for _, r := range rows {
		var te servicev1.ToolEvent
		require.NoError(t, proto.Unmarshal(r.payload, &te))
		events = append(events, &te)
	}
	return events
}

func requirePendingCountInvariant(t *testing.T, w *eventWriter) {
	t.Helper()
	var meta, rows int
	require.NoError(t, w.store.db.QueryRow("SELECT pending_count FROM wal_meta WHERE id = 1").Scan(&meta))
	require.NoError(t, w.store.db.QueryRow("SELECT COUNT(*) FROM events WHERE status = 'pending'").Scan(&rows))
	assert.Equal(t, rows, meta, "pending_count must equal the pending rows")
}

func TestDedupRuleValidation(t *testing.T) {
	validKey := func(*servicev1.ToolEvent) ([]string, bool) { return nil, false }

	cases := []struct {
		name    string
		rules   []DedupRule
		wantErr string
	}{
		{
			name:    "empty name",
			rules:   []DedupRule{{Name: "", Key: validKey, Window: time.Minute}},
			wantErr: "dedup rule name is required",
		},
		{
			name: "duplicate name",
			rules: []DedupRule{
				{Name: "a", Key: validKey, Window: time.Minute},
				{Name: "a", Key: validKey, Window: time.Minute},
			},
			wantErr: "duplicate dedup rule name",
		},
		{
			name:    "nil key",
			rules:   []DedupRule{{Name: "a", Key: nil, Window: time.Minute}},
			wantErr: "requires a key function",
		},
		{
			name:    "zero window",
			rules:   []DedupRule{{Name: "a", Key: validKey, Window: 0}},
			wantErr: "requires a window of at least one millisecond",
		},
		{
			name:    "negative window",
			rules:   []DedupRule{{Name: "a", Key: validKey, Window: -time.Minute}},
			wantErr: "requires a window of at least one millisecond",
		},
		{
			name:    "sub-millisecond window",
			rules:   []DedupRule{{Name: "a", Key: validKey, Window: 500 * time.Microsecond}},
			wantErr: "requires a window of at least one millisecond",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEventEmitterClient("test-tool", "1.0.0",
				WithWALPath(filepath.Join(t.TempDir(), "test.db")),
				WithDedupRules(tc.rules...),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}

	t.Run("valid rules construct", func(t *testing.T) {
		client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
			WithDedupRules(hostObservationRule(15*time.Minute)))
		require.NoError(t, client.Close())
	})
}

func TestDedupFirstEventDelivers(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	event := hostObsEvent(t, client, "evil.example.com", "GET")
	require.NoError(t, client.Emit(context.Background(), event))

	pending := pendingToolEvents(t, client.eventWriter)
	require.Len(t, pending, 1)
	assert.Equal(t, event.GetEventId(), pending[0].GetEventId())
	assert.Nil(t, pending[0].GetDedupContext())
	requirePendingCountInvariant(t, client.eventWriter)
}

func TestDedupSuppressesInWindow(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	for range 5 {
		require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	}
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "other.example.com", "GET")))
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "POST")))

	pending := pendingToolEvents(t, client.eventWriter)
	assert.Len(t, pending, 3, "one delivered event per distinct key")
	requirePendingCountInvariant(t, client.eventWriter)
}

func TestDedupWindowExpiryFlushesCarrier(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	current := time.Now()
	client.now = func() time.Time { return current }

	first := hostObsEvent(t, client, "evil.example.com", "GET")
	require.NoError(t, client.Emit(context.Background(), first))

	var lastSuppressed *servicev1.ToolEvent
	for range 4 {
		lastSuppressed = hostObsEvent(t, client, "evil.example.com", "GET")
		require.NoError(t, client.Emit(context.Background(), lastSuppressed))
	}

	current = current.Add(16 * time.Minute)
	next := hostObsEvent(t, client, "evil.example.com", "GET")
	require.NoError(t, client.Emit(context.Background(), next))

	pending := pendingToolEvents(t, client.eventWriter)
	require.Len(t, pending, 3, "first event, carrier, and the next window's first event")

	byID := make(map[string]*servicev1.ToolEvent, len(pending))
	for _, e := range pending {
		byID[e.GetEventId()] = e
	}

	carrier, ok := byID[lastSuppressed.GetEventId()]
	require.True(t, ok, "the carrier is the last held-back event")
	require.NotNil(t, carrier.GetDedupContext())
	assert.Equal(t, uint64(4), carrier.GetDedupContext().GetRepeatCount())

	assert.Nil(t, byID[first.GetEventId()].GetDedupContext())
	assert.Nil(t, byID[next.GetEventId()].GetDedupContext())

	// Exactness: 6 raw events are 1 + 4 + 1 across the delivered rows.
	var total uint64
	for _, e := range pending {
		if dc := e.GetDedupContext(); dc != nil {
			total += dc.GetRepeatCount()
		} else {
			total++
		}
	}
	assert.Equal(t, uint64(6), total)
	requirePendingCountInvariant(t, client.eventWriter)
}

func TestDedupClockStepBackward(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	current := time.Now()
	client.now = func() time.Time { return current }

	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))

	current = current.Add(-time.Hour)
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))

	pending := pendingToolEvents(t, client.eventWriter)
	assert.Len(t, pending, 1, "a backward clock step counts as in-window")
}

func TestDedupUnmatchedEventPassesThrough(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	for range 3 {
		event, err := client.NewEvent()
		require.NoError(t, err)
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{},
		}
		require.NoError(t, client.Emit(context.Background(), event))
	}

	pending := pendingToolEvents(t, client.eventWriter)
	assert.Len(t, pending, 3, "unmatched events never dedup")

	var stateRows int
	require.NoError(t, client.store.db.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&stateRows))
	assert.Zero(t, stateRows)
}

func TestDedupSweepOnSync(t *testing.T) {
	var received []*servicev1.ToolEvent
	transport := &mockTransport{
		sendFunc: func(_ context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
			received = append(received, req.GetEvents()...)
			ids := make([]string, 0, len(req.GetEvents()))
			for _, e := range req.GetEvents() {
				ids = append(ids, e.GetEventId())
			}
			return &servicev1.SyncEventsResponse{ConfirmedEventIds: ids}, nil
		},
	}

	client, err := NewSyncClient("test-tool", "1.0.0", transport,
		NewEndpointIdentityResolver(WithEndpointID("test-endpoint")),
		WithWALPath(filepath.Join(t.TempDir(), "test.db")),
		WithDedupRules(hostObservationRule(15*time.Minute)),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	current := time.Now()
	client.now = func() time.Time { return current }

	for range 4 {
		require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	}

	synced, err := client.Sync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, synced, "the open window holds back everything but the first event")

	current = current.Add(16 * time.Minute)
	synced, err = client.Sync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, synced, "the sweep flushes the carrier with no new emit")

	require.Len(t, received, 2)
	carrier := received[1]
	require.NotNil(t, carrier.GetDedupContext())
	assert.Equal(t, uint64(3), carrier.GetDedupContext().GetRepeatCount())

	var stateRows int
	require.NoError(t, client.store.db.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&stateRows))
	assert.Zero(t, stateRows, "the sweep deletes the flushed window")
}

func TestDedupSweepOnCloseAndReopen(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "test.db")

	client := newDedupEmitter(t, walPath, WithDedupRules(hostObservationRule(15*time.Minute)))
	current := time.Now()
	client.now = func() time.Time { return current }

	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))

	current = current.Add(16 * time.Minute)
	require.NoError(t, client.Close())

	reopened := newDedupEmitter(t, walPath, WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = reopened.Close() }()

	pending := pendingToolEvents(t, reopened.eventWriter)
	require.Len(t, pending, 2, "Close flushed the carrier for the expired window")

	var repeats []uint64
	for _, e := range pending {
		if dc := e.GetDedupContext(); dc != nil {
			repeats = append(repeats, dc.GetRepeatCount())
		}
	}
	assert.Equal(t, []uint64{1}, repeats)
}

func TestDedupRemovedRuleFlushesOnSweep(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "test.db")

	client := newDedupEmitter(t, walPath, WithDedupRules(hostObservationRule(15*time.Minute)))
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	require.NoError(t, client.Close())

	// The window is still open, so the row survives the first Close.
	reopened := newDedupEmitter(t, walPath)
	require.NoError(t, reopened.Close())

	// The reopened client declared no rules, so its Close flushed the row.
	verify := newDedupEmitter(t, walPath)
	defer func() { _ = verify.Close() }()

	pending := pendingToolEvents(t, verify.eventWriter)
	require.Len(t, pending, 2)

	var repeats []uint64
	for _, e := range pending {
		if dc := e.GetDedupContext(); dc != nil {
			repeats = append(repeats, dc.GetRepeatCount())
		}
	}
	assert.Equal(t, []uint64{1}, repeats, "a removed rule never strands a count")
}

func TestDedupSweepPagesThroughManyKeys(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()

	current := time.Now()
	client.now = func() time.Time { return current }

	keys := dedupSweepBatch + 500
	for i := range keys {
		event := hostObsEvent(t, client, fmt.Sprintf("host-%d.example.com", i), "GET")
		require.NoError(t, client.Emit(context.Background(), event))
	}

	current = current.Add(16 * time.Minute)
	require.NoError(t, client.sweepDedup())

	var stateRows int
	require.NoError(t, client.store.db.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&stateRows))
	assert.Zero(t, stateRows, "the sweep pages through every state row")
	requirePendingCountInvariant(t, client.eventWriter)
}

func TestDedupKeyCapDeliversDirectly(t *testing.T) {
	client := newDedupEmitter(t, filepath.Join(t.TempDir(), "test.db"),
		WithDedupRules(hostObservationRule(15*time.Minute)))
	defer func() { _ = client.Close() }()
	client.store.maxDedupKeys = 2

	for i := range 4 {
		event := hostObsEvent(t, client, fmt.Sprintf("host-%d.example.com", i), "GET")
		require.NoError(t, client.Emit(context.Background(), event))
	}

	var stateRows int
	require.NoError(t, client.store.db.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&stateRows))
	assert.Equal(t, 2, stateRows, "the cap stops state growth")

	pending := pendingToolEvents(t, client.eventWriter)
	assert.Len(t, pending, 4, "an event past the cap delivers directly")
	requirePendingCountInvariant(t, client.eventWriter)
}

func TestDedupWALFullKeepsState(t *testing.T) {
	walPath := filepath.Join(t.TempDir(), "test.db")

	client := newDedupEmitter(t, walPath,
		WithDedupRules(hostObservationRule(15*time.Minute)),
		WithMaxPending(1),
	)
	current := time.Now()
	client.now = func() time.Time { return current }

	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")))
	require.NoError(t, client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET")),
		"a held-back event never touches the pending queue")

	current = current.Add(16 * time.Minute)
	err := client.Emit(context.Background(), hostObsEvent(t, client, "evil.example.com", "GET"))
	require.ErrorIs(t, err, ErrWALFull)
	requirePendingCountInvariant(t, client.eventWriter)
	require.NoError(t, client.Close())

	// With capacity restored, the sweep delivers the kept count.
	reopened := newDedupEmitter(t, walPath, WithDedupRules(hostObservationRule(15*time.Minute)))
	reopened.now = func() time.Time { return current }
	require.NoError(t, reopened.Close())

	verify := newDedupEmitter(t, walPath)
	defer func() { _ = verify.Close() }()

	pending := pendingToolEvents(t, verify.eventWriter)
	require.Len(t, pending, 2)

	var repeats []uint64
	for _, e := range pending {
		if dc := e.GetDedupContext(); dc != nil {
			repeats = append(repeats, dc.GetRepeatCount())
		}
	}
	assert.Equal(t, []uint64{1}, repeats, "the full queue delayed the count, never dropped it")
}
