package endpointsync

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, transport EventTransport) *SyncClient {
	t.Helper()
	client, err := NewSyncClient("test-tool", "1.0.0", transport,
		NewEndpointIdentityResolver(WithEndpointID("test-endpoint")),
		WithWALPath(filepath.Join(t.TempDir(), "test-sync.db")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestNewSyncClient(t *testing.T) {
	t.Run("missing transport returns error", func(t *testing.T) {
		_, err := NewSyncClient("test", "1.0.0", nil,
			NewEndpointIdentityResolver(WithEndpointID("ep")),
			WithWALPath(filepath.Join(t.TempDir(), "test.db")),
		)
		assert.ErrorIs(t, err, ErrMissingTransport)
	})

	t.Run("missing identity returns error", func(t *testing.T) {
		_, err := NewSyncClient("test", "1.0.0", &mockTransport{}, nil,
			WithWALPath(filepath.Join(t.TempDir(), "test.db")),
		)
		assert.ErrorIs(t, err, ErrMissingIdentity)
	})

	t.Run("valid config creates client", func(t *testing.T) {
		client := newTestClient(t, &mockTransport{})
		assert.NotNil(t, client)
	})
}

func TestNewEvent(t *testing.T) {
	client := newTestClient(t, &mockTransport{})

	event, err := client.NewEvent()
	require.NoError(t, err)

	assert.NotEmpty(t, event.GetEventId())
	assert.Equal(t, "test-tool", event.GetToolName())
	assert.Equal(t, "1.0.0", event.GetToolVersion())
	assert.NotNil(t, event.GetTimestamp())

	event2, _ := client.NewEvent()
	assert.NotEqual(t, event.GetEventId(), event2.GetEventId())
}

func TestEmitAndSync(t *testing.T) {
	t.Run("emit persists to WAL", func(t *testing.T) {
		client := newTestClient(t, &mockTransport{})

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{
				TotalAnalyzed: 10,
			},
		}

		err := client.Emit(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("sync delivers events", func(t *testing.T) {
		var received *servicev1.SyncEventsRequest
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				received = req
				ids := make([]string, len(req.GetEvents()))
				for i, e := range req.GetEvents() {
					ids[i] = e.GetEventId()
				}
				return &servicev1.SyncEventsResponse{
					ConfirmedEventIds: ids,
				}, nil
			},
		}

		client := newTestClient(t, transport)

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{
				TotalAnalyzed: 5,
			},
		}
		event.InvocationId = "inv-123"

		require.NoError(t, client.Emit(context.Background(), event))

		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, synced)

		require.NotNil(t, received)
		assert.Equal(t, "test-endpoint", received.GetEndpoint().GetIdentifier())
		assert.Len(t, received.GetEvents(), 1)
		assert.Equal(t, "inv-123", received.GetEvents()[0].GetInvocationId())
	})

	t.Run("sync with empty WAL returns zero", func(t *testing.T) {
		client := newTestClient(t, &mockTransport{})
		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
	})

	t.Run("sync respects context cancellation", func(t *testing.T) {
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				return nil, fmt.Errorf("should not be called")
			},
		}

		client := newTestClient(t, transport)

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_ERROR,
			Error:     &controltowerv1.PmgError{ErrorType: "test"},
		}
		require.NoError(t, client.Emit(context.Background(), event))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.Sync(ctx)
		assert.Error(t, err)
	})
}

func TestSyncFailedEvents(t *testing.T) {
	t.Run("duplicate error permanently removes event", func(t *testing.T) {
		var callCount int
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				callCount++
				eventID := req.GetEvents()[0].GetEventId()
				return &servicev1.SyncEventsResponse{
					FailedEvents: []*servicev1.EventError{
						{
							EventId:   eventID,
							ErrorCode: servicev1.EventErrorCode_EVENT_ERROR_CODE_DUPLICATE,
							Message:   "already processed",
						},
					},
				}, nil
			},
		}

		client := newTestClient(t, transport)

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{TotalAnalyzed: 1},
		}
		require.NoError(t, client.Emit(context.Background(), event))

		// First sync: event is a duplicate, gets dropped.
		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
		assert.Equal(t, 1, callCount)

		// Second sync: WAL is empty, no calls made.
		synced, err = client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
		assert.Equal(t, 1, callCount) // still 1
	})

	t.Run("invalid payload error permanently removes event", func(t *testing.T) {
		var callCount int
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				callCount++
				eventID := req.GetEvents()[0].GetEventId()
				return &servicev1.SyncEventsResponse{
					FailedEvents: []*servicev1.EventError{
						{
							EventId:   eventID,
							ErrorCode: servicev1.EventErrorCode_EVENT_ERROR_CODE_INVALID_PAYLOAD,
							Message:   "payload rejected",
						},
					},
				}, nil
			},
		}

		client := newTestClient(t, transport)

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{TotalAnalyzed: 1},
		}
		require.NoError(t, client.Emit(context.Background(), event))

		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)

		// Second sync must find an empty WAL.
		synced, err = client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
		assert.Equal(t, 1, callCount)
	})

	t.Run("quota exceeded leaves event for retry", func(t *testing.T) {
		callCount := 0
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				callCount++
				eventID := req.GetEvents()[0].GetEventId()
				if callCount == 1 {
					return &servicev1.SyncEventsResponse{
						FailedEvents: []*servicev1.EventError{
							{
								EventId:   eventID,
								ErrorCode: servicev1.EventErrorCode_EVENT_ERROR_CODE_QUOTA_EXCEEDED,
								Message:   "quota exceeded",
							},
						},
					}, nil
				}
				// Second call: confirm the event.
				return &servicev1.SyncEventsResponse{
					ConfirmedEventIds: []string{eventID},
				}, nil
			},
		}

		client := newTestClient(t, transport)

		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{TotalAnalyzed: 1},
		}
		require.NoError(t, client.Emit(context.Background(), event))

		// First sync: quota exceeded, event stays in WAL.
		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
		assert.Equal(t, 1, callCount)

		// Second sync: event is confirmed.
		synced, err = client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, synced)
		assert.Equal(t, 2, callCount)
	})

	t.Run("mix of confirmed and permanently failed events", func(t *testing.T) {
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				events := req.GetEvents()
				// Confirm first, reject second as duplicate.
				return &servicev1.SyncEventsResponse{
					ConfirmedEventIds: []string{events[0].GetEventId()},
					FailedEvents: []*servicev1.EventError{
						{
							EventId:   events[1].GetEventId(),
							ErrorCode: servicev1.EventErrorCode_EVENT_ERROR_CODE_DUPLICATE,
							Message:   "duplicate",
						},
					},
				}, nil
			},
		}

		client := newTestClient(t, transport)

		for i := 0; i < 2; i++ {
			event, _ := client.NewEvent()
			event.PmgEvent = &controltowerv1.PmgEvent{
				EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
				SessionSummary: &controltowerv1.PmgSessionSummary{TotalAnalyzed: uint32(i)},
			}
			require.NoError(t, client.Emit(context.Background(), event))
		}

		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		// Only confirmed event counts toward synced.
		assert.Equal(t, 1, synced)

		// WAL must be empty after: both events resolved.
		synced, err = client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
	})
}

func TestSyncCorruptedEvents(t *testing.T) {
	t.Run("corrupted WAL entries are dropped and do not block sync", func(t *testing.T) {
		var received *servicev1.SyncEventsRequest
		transport := &mockTransport{
			sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
				received = req
				ids := make([]string, len(req.GetEvents()))
				for i, e := range req.GetEvents() {
					ids[i] = e.GetEventId()
				}
				return &servicev1.SyncEventsResponse{ConfirmedEventIds: ids}, nil
			},
		}

		client := newTestClient(t, transport)

		// Insert a corrupted event directly into the WAL (invalid proto bytes).
		err := client.store.insert("corrupted-event-id", []byte("not-valid-proto"))
		require.NoError(t, err)

		// Insert a valid event via Emit.
		good, _ := client.NewEvent()
		good.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{TotalAnalyzed: 5},
		}
		require.NoError(t, client.Emit(context.Background(), good))

		synced, err := client.Sync(context.Background())
		require.NoError(t, err)
		// Only the good event is counted as synced.
		assert.Equal(t, 1, synced)

		// The server should have received exactly the good event.
		require.NotNil(t, received)
		assert.Len(t, received.GetEvents(), 1)
		assert.Equal(t, good.GetEventId(), received.GetEvents()[0].GetEventId())

		// Third sync: both events are gone, WAL is empty.
		synced, err = client.Sync(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, synced)
	})
}

func TestWithBatchSizeCap(t *testing.T) {
	t.Run("batch size above 100 is capped at 100", func(t *testing.T) {
		cfg := defaultSyncConfig("test")
		WithBatchSize(200)(cfg)
		assert.Equal(t, maxBatchSize, cfg.batchSize)
	})

	t.Run("batch size of 100 is accepted as-is", func(t *testing.T) {
		cfg := defaultSyncConfig("test")
		WithBatchSize(100)(cfg)
		assert.Equal(t, 100, cfg.batchSize)
	})

	t.Run("batch size below 100 is accepted as-is", func(t *testing.T) {
		cfg := defaultSyncConfig("test")
		WithBatchSize(10)(cfg)
		assert.Equal(t, 10, cfg.batchSize)
	})

	t.Run("batch size of zero is ignored", func(t *testing.T) {
		cfg := defaultSyncConfig("test")
		WithBatchSize(0)(cfg)
		assert.Equal(t, maxBatchSize, cfg.batchSize)
	})
}

func TestWALMarkDeliveredIdempotent(t *testing.T) {
	t.Run("pending count does not drift on double delivery", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		w.maxPending = 5
		require.NoError(t, w.insert("evt-1", []byte("p")))
		require.NoError(t, w.insert("evt-2", []byte("p")))

		// Mark evt-1 as delivered twice.
		delivered, err := w.markDelivered([]string{"evt-1"})
		require.NoError(t, err)
		assert.Equal(t, 1, delivered)

		delivered, err = w.markDelivered([]string{"evt-1"}) // already delivered
		require.NoError(t, err)
		assert.Equal(t, 0, delivered)

		// Now pending should be 1 (only evt-2).
		// If the count drifted to -1, inserting 5 more would incorrectly succeed.
		for i := 3; i <= 5; i++ {
			require.NoError(t, w.insert(fmt.Sprintf("evt-%d", i), []byte("p")))
		}

		// At this point: evt-2, evt-3, evt-4, evt-5 are pending (4 total).
		// maxPending=5, so one more should succeed.
		require.NoError(t, w.insert("evt-6", []byte("p")))

		// Now we are at capacity (5 pending).
		err = w.insert("evt-7", []byte("p"))
		assert.ErrorIs(t, err, ErrWALFull)
	})
}

func TestEmitWALFull(t *testing.T) {
	client, err := NewSyncClient("test", "1.0.0", &mockTransport{},
		NewEndpointIdentityResolver(WithEndpointID("ep")),
		WithWALPath(filepath.Join(t.TempDir(), "test.db")),
		WithMaxPending(2),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	for i := 0; i < 2; i++ {
		event, _ := client.NewEvent()
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{},
		}
		require.NoError(t, client.Emit(context.Background(), event))
	}

	event, _ := client.NewEvent()
	event.PmgEvent = &controltowerv1.PmgEvent{
		EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
		SessionSummary: &controltowerv1.PmgSessionSummary{},
	}
	err = client.Emit(context.Background(), event)
	assert.ErrorIs(t, err, ErrWALFull)
}
