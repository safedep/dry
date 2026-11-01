package configsync

import (
	"context"

	"buf.build/gen/go/safedep/api/grpc/go/safedep/services/controltower/policy/v1/policyv1grpc"
	policyv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/policy/v1"
	"google.golang.org/grpc"
)

// Transport sends GetResolvedPolicy to SafeDep Cloud. Tests give the Fetcher
// a stub, so they need no network.
type Transport interface {
	GetResolvedPolicy(ctx context.Context, req *policyv1.GetResolvedPolicyRequest) (*policyv1.GetResolvedPolicyResponse, error)
}

type grpcTransport struct {
	client policyv1grpc.PolicySyncServiceClient
}

// NewGrpcTransport returns a Transport over a gRPC connection, as the
// data-plane client of dry/cloud gives it:
//
//	client, err := cloud.NewDataPlaneClient("pmg", creds)
//	transport := configsync.NewGrpcTransport(client.Connection())
//
// The call keeps the default receive limit of gRPC, 4 MiB. A resolved policy
// is at most 2 MiB.
func NewGrpcTransport(conn grpc.ClientConnInterface) Transport {
	return &grpcTransport{client: policyv1grpc.NewPolicySyncServiceClient(conn)}
}

func (t *grpcTransport) GetResolvedPolicy(ctx context.Context, req *policyv1.GetResolvedPolicyRequest) (*policyv1.GetResolvedPolicyResponse, error) {
	return t.client.GetResolvedPolicy(ctx, req)
}
