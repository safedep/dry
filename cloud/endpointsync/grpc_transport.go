package endpointsync

import (
	"context"

	"buf.build/gen/go/safedep/api/grpc/go/safedep/services/controltower/v1/controltowerv1grpc"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"google.golang.org/grpc"
)

type grpcTransport struct {
	client controltowerv1grpc.EndpointServiceClient
}

// NewGrpcTransport creates a transport that sends events via unary gRPC
// calls to EndpointService.SyncEvents.
func NewGrpcTransport(conn *grpc.ClientConn) EventTransport {
	return &grpcTransport{
		client: controltowerv1grpc.NewEndpointServiceClient(conn),
	}
}

func (t *grpcTransport) Send(ctx context.Context, req *servicev1.SyncEventsRequest) (*servicev1.SyncEventsResponse, error) {
	return t.client.SyncEvents(ctx, req)
}

func (t *grpcTransport) Close() error {
	return nil // Connection lifecycle managed by cloud.Client
}
