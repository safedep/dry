package endpointsync

import (
	"context"
	"path/filepath"
	"testing"

	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
