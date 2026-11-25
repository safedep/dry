package cloud

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	testTenant = "acme.safedep.io"

	// tlsHandshakeRecord is the first byte of a TLS ClientHello. A client
	// without TLS starts with the HTTP/2 preface "PRI * HTTP/2.0".
	tlsHandshakeRecord = 0x16
)

func TestNewDataPlaneClient(t *testing.T) {
	t.Run("rejects control plane credentials", func(t *testing.T) {
		cred, _ := NewTokenCredential("jwt", "refresh", "tenant")
		_, err := NewDataPlaneClient("test", cred)
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})

	t.Run("rejects nil credentials", func(t *testing.T) {
		_, err := NewDataPlaneClient("test", nil)
		assert.Error(t, err)
	})
}

func TestNewControlPlaneClient(t *testing.T) {
	t.Run("rejects data plane credentials", func(t *testing.T) {
		cred, _ := NewAPIKeyCredential("sk-key", "tenant")
		_, err := NewControlPlaneClient("test", cred)
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})

	t.Run("rejects nil credentials", func(t *testing.T) {
		_, err := NewControlPlaneClient("test", nil)
		assert.Error(t, err)
	})
}

type testPlane struct {
	name     string
	addrEnv  string
	wantAuth string
	creds    func(t *testing.T, exchangeAddr string) *Credentials
	connect  func(name string, creds *Credentials, opts ...ClientOption) (*Client, error)
}

func testPlanes() []testPlane {
	return []testPlane{
		{
			name:     "data plane with API key",
			addrEnv:  envDataPlaneAddr,
			wantAuth: "sk-test",
			creds: func(t *testing.T, _ string) *Credentials {
				creds, err := NewAPIKeyCredential("sk-test", testTenant)
				require.NoError(t, err)
				return creds
			},
			connect: NewDataPlaneClientWithOptions,
		},
		{
			name:     "data plane with workload identity",
			addrEnv:  envDataPlaneAddr,
			wantAuth: "wc-1",
			creds:    newTestWorkloadCredentials,
			connect:  NewDataPlaneClientWithOptions,
		},
		{
			name:     "control plane with token",
			addrEnv:  envControlPlaneAddr,
			wantAuth: "jwt-test",
			creds: func(t *testing.T, _ string) *Credentials {
				creds, err := NewTokenCredential("jwt-test", "refresh-test", testTenant)
				require.NoError(t, err)
				return creds
			},
			connect: NewControlPlaneClientWithOptions,
		},
	}
}

func newTestWorkloadCredentials(t *testing.T, exchangeAddr string) *Credentials {
	newFakeGitHubTokenEndpoint(t).setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	exchange := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, exchangeAddr))
	creds, err := NewWorkloadIdentityCredential(
		NewRefreshingWorkloadCredential(exchange, source, WorkloadExchangeRequest{TenantDomain: testTenant}))
	require.NoError(t, err)
	return creds
}

// staticPerRPCCredentials stands in for a caller credential that tries to
// replace the headers of the client.
type staticPerRPCCredentials map[string]string

func (c staticPerRPCCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c, nil
}

func (c staticPerRPCCredentials) RequireTransportSecurity() bool {
	return false
}

func TestClientDialOptions(t *testing.T) {
	cases := []struct {
		name                string
		opts                []ClientOption
		wantUserAgentPrefix string
		wantExtra           map[string]string
	}{
		{
			name:                "no options",
			wantUserAgentPrefix: "grpc-go/",
		},
		{
			name:                "user agent",
			opts:                []ClientOption{WithDialOptions(grpc.WithUserAgent("pmg-test"))},
			wantUserAgentPrefix: "pmg-test grpc-go/",
		},
		{
			name:                "keepalive option keeps the dial working",
			opts:                []ClientOption{WithKeepalive()},
			wantUserAgentPrefix: "grpc-go/",
		},
		{
			name: "caller credential adds a header and does not replace auth or tenant",
			opts: []ClientOption{WithDialOptions(grpc.WithPerRPCCredentials(staticPerRPCCredentials{
				"authorization": "caller-token",
				tenantIDHeader:  "other.safedep.io",
				"x-caller":      "pmg",
			}))},
			wantUserAgentPrefix: "grpc-go/",
			wantExtra:           map[string]string{"x-caller": "pmg"},
		},
		{
			name: "options from two calls add up",
			opts: []ClientOption{
				WithDialOptions(grpc.WithUserAgent("pmg-test")),
				WithDialOptions(grpc.WithPerRPCCredentials(staticPerRPCCredentials{"x-caller": "pmg"})),
			},
			wantUserAgentPrefix: "pmg-test grpc-go/",
			wantExtra:           map[string]string{"x-caller": "pmg"},
		},
	}

	for _, plane := range testPlanes() {
		for _, tc := range cases {
			t.Run(plane.name+"/"+tc.name, func(t *testing.T) {
				fake := &fakeWorkloadIdentityServer{clock: &fakeClock{now: time.Now()}, life: time.Hour}
				addr := startFakeWorkloadIdentityServer(t, fake)
				t.Setenv(plane.addrEnv, addr)
				t.Setenv(insecureTransportEnvVar, "true")

				client, err := plane.connect("test", plane.creds(t, addr), tc.opts...)
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, client.Close()) })

				err = client.Connection().Invoke(context.Background(), "/test.v1.TestService/Call", &emptypb.Empty{}, &emptypb.Empty{})
				require.Equal(t, codes.Unimplemented, status.Code(err))

				require.Len(t, fake.calls, 1)
				md := fake.calls[0]
				assert.Equal(t, []string{plane.wantAuth}, md.Get("authorization"))
				assert.Equal(t, []string{testTenant}, md.Get(tenantIDHeader))
				userAgent := md.Get("user-agent")
				require.Len(t, userAgent, 1)
				assert.Truef(t, strings.HasPrefix(userAgent[0], tc.wantUserAgentPrefix),
					"user agent %q starts with %q", userAgent[0], tc.wantUserAgentPrefix)
				for k, v := range tc.wantExtra {
					assert.Equal(t, []string{v}, md.Get(k))
				}
			})
		}
	}
}

// pipeDialer gives the server end of each connection the client dials to
// conns, so a test reads the first bytes that the client sends.
func pipeDialer(conns chan<- net.Conn) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		client, server := net.Pipe()
		select {
		case conns <- server:
			return client, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
}

func TestClientKeepsTransportSecurity(t *testing.T) {
	cases := []struct {
		name          string
		insecureEnv   string
		opts          []grpc.DialOption
		wantFirstByte byte
	}{
		{
			name:          "TLS by default",
			wantFirstByte: tlsHandshakeRecord,
		},
		{
			name:          "TLS wins over caller transport credentials",
			opts:          []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			wantFirstByte: tlsHandshakeRecord,
		},
		{
			name:          "insecure transport switch sends plain HTTP/2",
			insecureEnv:   "true",
			wantFirstByte: 'P',
		},
	}

	for _, plane := range testPlanes() {
		for _, tc := range cases {
			t.Run(plane.name+"/"+tc.name, func(t *testing.T) {
				const unusedAddr = "127.0.0.1:1"
				creds := plane.creds(t, unusedAddr)
				t.Setenv(plane.addrEnv, unusedAddr)
				t.Setenv(insecureTransportEnvVar, tc.insecureEnv)

				conns := make(chan net.Conn, 1)
				opts := append([]grpc.DialOption{pipeDialer(conns)}, tc.opts...)
				client, err := plane.connect("test", creds, WithDialOptions(opts...))
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, client.Close()) })
				client.Connection().Connect()

				var server net.Conn
				select {
				case server = <-conns:
				case <-time.After(5 * time.Second):
					require.FailNow(t, "the client did not use the caller dialer")
				}
				defer func() { assert.NoError(t, server.Close()) }()

				require.NoError(t, server.SetReadDeadline(time.Now().Add(5*time.Second)))
				first := make([]byte, 1)
				_, err = io.ReadFull(server, first)
				require.NoError(t, err)
				assert.Equal(t, tc.wantFirstByte, first[0])
			})
		}
	}
}
