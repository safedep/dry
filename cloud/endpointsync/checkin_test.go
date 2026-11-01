package endpointsync

import (
	"context"
	"path/filepath"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	recordv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/record/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func newCheckInTestClient(t *testing.T, transport EventTransport) *SyncClient {
	t.Helper()
	client, err := NewSyncClient("pmg", "1.2.3", transport,
		NewEndpointIdentityResolver(WithEndpointID("checkin-test")),
		WithWALPath(filepath.Join(t.TempDir(), "checkin.db")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCheckIn_RequestFields(t *testing.T) {
	var got *servicev1.CheckInRequest
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			got = req
			return &servicev1.CheckInResponse{}, nil
		},
	}

	client := newCheckInTestClient(t, transport)
	require.NoError(t, client.CheckIn(context.Background()))

	require.NotNil(t, got)
	assert.Equal(t, "checkin-test", got.GetEndpoint().GetIdentifier())
	assert.Equal(t, "pmg", got.GetToolName())
	assert.Equal(t, "1.2.3", got.GetToolVersion())
}

func TestCheckIn_ErrorPropagates(t *testing.T) {
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			return nil, status.Error(codes.Unavailable, "server down")
		},
	}

	client := newCheckInTestClient(t, transport)
	err := client.CheckIn(context.Background())
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestCheckIn_UnimplementedDoesNotTripBreaker(t *testing.T) {
	calls := 0
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			calls++
			return nil, status.Error(codes.Unimplemented, "old server")
		},
	}

	client := newCheckInTestClient(t, transport)
	for i := 0; i < 10; i++ {
		err := client.CheckIn(context.Background())
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	}

	// Every call reached the transport: the breaker never opened.
	assert.Equal(t, 10, calls)
}

func TestCheckIn_RealFailuresTripBreaker(t *testing.T) {
	calls := 0
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			calls++
			return nil, status.Error(codes.Unavailable, "server down")
		},
	}

	client := newCheckInTestClient(t, transport)
	for i := 0; i < 10; i++ {
		require.Error(t, client.CheckIn(context.Background()))
	}

	// The breaker opens after 5 consecutive failures and short-circuits
	// the rest.
	assert.Equal(t, 5, calls)
}

func TestCheckIn_BreakerIsolatedFromSync(t *testing.T) {
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			return nil, status.Error(codes.Unavailable, "server down")
		},
		sendFunc: func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
			ids := make([]string, len(req.GetEvents()))
			for i, e := range req.GetEvents() {
				ids[i] = e.GetEventId()
			}
			return &servicev1.SyncEventsResponse{ConfirmedEventIds: ids}, nil
		},
	}

	client := newCheckInTestClient(t, transport)
	ctx := context.Background()

	// Open the check-in breaker.
	for i := 0; i < 6; i++ {
		require.Error(t, client.CheckIn(ctx))
	}

	// Event sync still works: it uses its own breaker.
	event, err := client.NewEvent()
	require.NoError(t, err)
	event.InvocationId = "inv-1"
	require.NoError(t, client.Emit(ctx, event))

	synced, err := client.Sync(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, synced)
}

func TestCheckInWithOptions(t *testing.T) {
	pmgState := servicev1.ToolPolicyState_builder{
		ToolName:    "pmg",
		ToolVersion: "1.2.3",
		Policy:      recordv1.ApplyReport_builder{Revision: 48, Digest: "sha256:9f2c"}.Build(),
	}.Build()
	daemonState := servicev1.ToolPolicyState_builder{ToolName: "safedep-daemon", ToolVersion: "0.1.0"}.Build()
	labels := reportedLabels("linux", "amd64")

	cases := []struct {
		name       string
		opts       []CheckInOption
		wantTools  []*servicev1.ToolPolicyState
		wantLabels []*controltowerv1.Label
	}{
		{name: "no option"},
		{
			name:      "tool policy",
			opts:      []CheckInOption{WithToolPolicy(pmgState)},
			wantTools: []*servicev1.ToolPolicyState{pmgState},
		},
		{
			name:      "each tool policy adds one state",
			opts:      []CheckInOption{WithToolPolicy(pmgState), WithToolPolicy(daemonState)},
			wantTools: []*servicev1.ToolPolicyState{pmgState, daemonState},
		},
		{
			name: "nil tool policy adds nothing",
			opts: []CheckInOption{WithToolPolicy(nil)},
		},
		{
			name:       "labels",
			opts:       []CheckInOption{WithLabels(labels)},
			wantLabels: labels,
		},
		{
			name:       "tool policy and labels",
			opts:       []CheckInOption{WithLabels(labels), WithToolPolicy(pmgState)},
			wantTools:  []*servicev1.ToolPolicyState{pmgState},
			wantLabels: labels,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got *servicev1.CheckInRequest
			transport := &mockTransport{
				checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
					got = req
					return &servicev1.CheckInResponse{}, nil
				},
			}

			client := newCheckInTestClient(t, transport)
			require.NoError(t, client.CheckInWithOptions(context.Background(), tc.opts...))

			want := &servicev1.CheckInRequest{
				Endpoint:    client.identity,
				ToolName:    "pmg",
				ToolVersion: "1.2.3",
				Tools:       tc.wantTools,
				Labels:      tc.wantLabels,
			}
			assert.True(t, proto.Equal(want, got), "got %v", got)
		})
	}
}

// A caller that passes no option sends the request of CheckIn, and CheckIn
// stays a func(context.Context) error value for its callers.
func TestCheckIn_SameRequestAsNoOption(t *testing.T) {
	var reqs []*servicev1.CheckInRequest
	transport := &mockTransport{
		checkInFunc: func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
			reqs = append(reqs, req)
			return &servicev1.CheckInResponse{}, nil
		},
	}
	client := newCheckInTestClient(t, transport)

	callAsValue := func(checkIn func(context.Context) error) error {
		return checkIn(context.Background())
	}
	require.NoError(t, callAsValue(client.CheckIn))
	require.NoError(t, client.CheckInWithOptions(context.Background()))

	require.Len(t, reqs, 2)
	assert.True(t, proto.Equal(reqs[0], reqs[1]))
	assert.Empty(t, reqs[0].GetTools())
	assert.Empty(t, reqs[0].GetLabels())
}
