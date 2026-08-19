package endpointsync

import (
	"context"

	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
)

// mockTransport implements EventTransport for testing
type mockTransport struct {
	sendFunc    func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error)
	checkInFunc func(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error)
	closed      bool
}

func (m *mockTransport) Send(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return &servicev1.SyncEventsResponse{}, nil
}

func (m *mockTransport) CheckIn(ctx context.Context, req *servicev1.CheckInRequest) (*servicev1.CheckInResponse, error) {
	if m.checkInFunc != nil {
		return m.checkInFunc(ctx, req)
	}
	return &servicev1.CheckInResponse{}, nil
}

func (m *mockTransport) Close() error {
	m.closed = true
	return nil
}
