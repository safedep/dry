package endpointsync

import (
	"context"
	"path/filepath"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_MultiBatch(t *testing.T) {
	const batchSize = 5
	const totalEvents = 13 // more than two full batches

	var batches []*servicev1.SyncEventsRequest

	transport := &mockTransport{
		sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
			batches = append(batches, req)
			ids := make([]string, len(req.GetEvents()))
			for i, e := range req.GetEvents() {
				ids[i] = e.GetEventId()
			}
			return &servicev1.SyncEventsResponse{ConfirmedEventIds: ids}, nil
		},
	}

	client, err := NewSyncClient("pmg", transport,
		NewEndpointIdentityResolver(WithEndpointID("multi-batch-test")),
		WithWALPath(filepath.Join(t.TempDir(), "multi-batch.db")),
		WithBatchSize(batchSize),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	for i := 0; i < totalEvents; i++ {
		event, _ := client.NewEvent()
		event.InvocationId = "inv-multi-batch"
		event.PmgEvent = &controltowerv1.PmgEvent{
			EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
			SessionSummary: &controltowerv1.PmgSessionSummary{
				TotalAnalyzed: uint32(i),
			},
		}
		require.NoError(t, client.Emit(ctx, event))
	}

	synced, err := client.Sync(ctx)
	require.NoError(t, err)
	assert.Equal(t, totalEvents, synced)

	// Verify the batch loop ran ceil(totalEvents/batchSize) times.
	expectedBatches := (totalEvents + batchSize - 1) / batchSize
	assert.Len(t, batches, expectedBatches)

	// Verify batch sizes: all but the last should be batchSize.
	for i, batch := range batches {
		if i < len(batches)-1 {
			assert.Len(t, batch.GetEvents(), batchSize,
				"batch %d should have %d events", i, batchSize)
		} else {
			assert.Len(t, batch.GetEvents(), totalEvents%batchSize,
				"last batch should have remainder events")
		}
	}

	// Second sync must be empty.
	synced, err = client.Sync(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, synced)
}

func TestIntegration_PMGWorkflow(t *testing.T) {
	var batches []*servicev1.SyncEventsRequest

	transport := &mockTransport{
		sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
			batches = append(batches, req)
			ids := make([]string, len(req.GetEvents()))
			for i, e := range req.GetEvents() {
				ids[i] = e.GetEventId()
			}
			return &servicev1.SyncEventsResponse{ConfirmedEventIds: ids}, nil
		},
	}

	client, err := NewSyncClient("pmg", transport,
		NewEndpointIdentityResolver(WithEndpointID("dev-machine-1")),
		WithWALPath(filepath.Join(t.TempDir(), "integration.db")),
		WithToolVersion("1.2.3"),
		WithBatchSize(10),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	invocationID := "inv-test-001"

	// Emit session summary
	event1, _ := client.NewEvent()
	event1.InvocationId = invocationID
	event1.PmgEvent = &controltowerv1.PmgEvent{
		EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
		SessionSummary: &controltowerv1.PmgSessionSummary{
			PackageManager: controltowerv1.PmgPackageManager_PMG_PACKAGE_MANAGER_NPM,
			FlowType:       controltowerv1.PmgFlowType_PMG_FLOW_TYPE_GUARD,
			TotalAnalyzed:  150,
			AllowedCount:   149,
			BlockedCount:   1,
			Outcome:        controltowerv1.PmgSessionOutcome_PMG_SESSION_OUTCOME_BLOCKED,
		},
	}
	require.NoError(t, client.Emit(ctx, event1))

	// Emit blocked package decision
	event2, _ := client.NewEvent()
	event2.InvocationId = invocationID
	event2.PmgEvent = &controltowerv1.PmgEvent{
		EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_PACKAGE_DECISION,
		PackageDecision: &controltowerv1.PmgPackageDecision{
			PackageVersion: &packagev1.PackageVersion{
				Package: &packagev1.Package{
					Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
					Name:      "evil-package",
				},
				Version: "1.0.0",
			},
			Action:     controltowerv1.PmgPackageAction_PMG_PACKAGE_ACTION_BLOCKED,
			AnalysisId: "analysis-abc-123",
			IsMalware:  true,
			IsVerified: true,
		},
	}
	require.NoError(t, client.Emit(ctx, event2))

	// Sync
	synced, err := client.Sync(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, synced)

	// Verify batches
	require.Len(t, batches, 1)
	batch := batches[0]
	assert.Equal(t, "dev-machine-1", batch.GetEndpoint().GetIdentifier())
	assert.Len(t, batch.GetEvents(), 2)

	// Verify events have correct tool metadata
	for _, e := range batch.GetEvents() {
		assert.Equal(t, "pmg", e.GetToolName())
		assert.Equal(t, "1.2.3", e.GetToolVersion())
		assert.Equal(t, invocationID, e.GetInvocationId())
		assert.True(t, e.HasPmgEvent())
	}

	// Second sync should be empty
	synced, err = client.Sync(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, synced)
}
