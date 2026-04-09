package endpointsync

import (
	"context"

	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
)

// EventTransport abstracts the sync delivery mechanism.
type EventTransport interface {
	Send(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error)
	Close() error
}
