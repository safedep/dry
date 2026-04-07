package endpointsync

import (
	"context"
	"testing"

	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
)

// mockTransport implements EventTransport for testing
type mockTransport struct {
	sendFunc func(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error)
	closed   bool
}

func (m *mockTransport) Send(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return &servicev1.SyncEventsResponse{}, nil
}

func (m *mockTransport) Close() error {
	m.closed = true
	return nil
}

func TestMockTransportImplementsInterface(t *testing.T) {
	var _ EventTransport = &mockTransport{}
	assert.True(t, true)
}
