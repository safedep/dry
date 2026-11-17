package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"buf.build/gen/go/safedep/api/grpc/go/safedep/services/controltower/v1/controltowerv1grpc"
	messagesv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const testIDTokenRequestToken = "gh-request-token"

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeGitHubTokenEndpoint serves ACTIONS_ID_TOKEN_REQUEST_URL. Each request
// gets a new token, as GitHub gives.
type fakeGitHubTokenEndpoint struct {
	*httptest.Server

	mu        sync.Mutex
	served    int
	failing   bool
	audiences []string
}

func newFakeGitHubTokenEndpoint(t *testing.T) *fakeGitHubTokenEndpoint {
	f := &fakeGitHubTokenEndpoint{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.failing || r.Header.Get("Authorization") != "Bearer "+testIDTokenRequestToken {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		f.served++
		f.audiences = append(f.audiences, r.URL.Query().Get("audience"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"count": 1,
			"value": fmt.Sprintf("id-token-%d", f.served),
		}))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGitHubTokenEndpoint) setFailing(failing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = failing
}

func (f *fakeGitHubTokenEndpoint) servedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.served
}

// setGitHubActionsEnv makes the environment of a GitHub Actions job with
// "id-token: write" that gets its tokens from f.
func (f *fakeGitHubTokenEndpoint) setGitHubActionsEnv(t *testing.T) {
	t.Setenv(githubActionsIDTokenRequestURLEnvVar, f.URL+"/token?api-version=2.0")
	t.Setenv(githubActionsIDTokenRequestTokenEnvVar, testIDTokenRequestToken)
}

// fakeWorkloadIdentityServer stands in for control-tower. It issues a new
// credential for each exchange, with a life from the shared clock. Any other
// RPC records its metadata and fails with Unimplemented.
type fakeWorkloadIdentityServer struct {
	controltowerv1grpc.UnimplementedWorkloadIdentityServiceServer

	clock *fakeClock
	life  time.Duration

	mu        sync.Mutex
	exchanges []*controltowerv1.ExchangeTokenRequest
	revokes   []*controltowerv1.RevokeTokenRequest
	calls     []metadata.MD
}

func (s *fakeWorkloadIdentityServer) ExchangeToken(_ context.Context, req *controltowerv1.ExchangeTokenRequest) (*controltowerv1.ExchangeTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.exchanges = append(s.exchanges, req)
	n := len(s.exchanges)
	return &controltowerv1.ExchangeTokenResponse{
		AccessToken:     fmt.Sprintf("wc-%d", n),
		ExpiresAt:       timestamppb.New(s.clock.Now().Add(s.life)),
		Grant:           req.GetRequestedGrant(),
		TenantDomain:    req.GetTenantDomain(),
		EndpointId:      "gha/acme/web",
		CredentialId:    fmt.Sprintf("exchange-%d", n),
		DisplayIdentity: "github:acme/web@refs/heads/main run 987/1",
	}, nil
}

func (s *fakeWorkloadIdentityServer) RevokeToken(_ context.Context, req *controltowerv1.RevokeTokenRequest) (*controltowerv1.RevokeTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokes = append(s.revokes, req)
	return &controltowerv1.RevokeTokenResponse{}, nil
}

func (s *fakeWorkloadIdentityServer) recordCall(_ any, stream grpc.ServerStream) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, md)
	return status.Error(codes.Unimplemented, "test service")
}

func (s *fakeWorkloadIdentityServer) subjectTokens() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tokens []string
	for _, req := range s.exchanges {
		tokens = append(tokens, req.GetSubjectToken())
	}
	return tokens
}

// startFakeWorkloadIdentityServer serves gRPC over HTTP/2 without TLS on an
// httptest server, and returns its host:port.
func startFakeWorkloadIdentityServer(t *testing.T, fake *fakeWorkloadIdentityServer) string {
	srv := grpc.NewServer(grpc.UnknownServiceHandler(fake.recordCall))
	controltowerv1grpc.RegisterWorkloadIdentityServiceServer(srv, fake)

	hs := httptest.NewUnstartedServer(srv)
	hs.Config.Protocols = new(http.Protocols)
	hs.Config.Protocols.SetUnencryptedHTTP2(true)
	hs.Start()
	t.Cleanup(hs.Close)

	return hs.Listener.Addr().String()
}

func dialFakeWorkloadIdentityServer(t *testing.T, addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient("passthrough:///"+addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })
	return conn
}

func TestWorkloadExchangeClient_Exchange(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	clock := &fakeClock{now: time.Unix(1_900_000_000, 0)}
	fake := &fakeWorkloadIdentityServer{clock: clock, life: time.Hour}
	client := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, startFakeWorkloadIdentityServer(t, fake)))

	cred, err := client.Exchange(context.Background(), source, WorkloadExchangeRequest{
		TenantDomain: "acme.safedep.io",
		Lifetime:     2 * time.Hour,
		ToolName:     "vet",
		ToolVersion:  "2.0.0",
	})
	require.NoError(t, err)

	require.Len(t, fake.exchanges, 1)
	req := fake.exchanges[0]
	assert.Equal(t, "id-token-1", req.GetSubjectToken())
	assert.Equal(t, controltowerv1.SubjectTokenType_SUBJECT_TOKEN_TYPE_GITHUB_ACTIONS_OIDC, req.GetSubjectTokenType())
	assert.Equal(t, "acme.safedep.io", req.GetTenantDomain())
	assert.Equal(t, messagesv1.CredentialGrant_CREDENTIAL_GRANT_TOOL, req.GetRequestedGrant())
	assert.Equal(t, 2*time.Hour, req.GetRequestedLifetime().AsDuration())
	assert.Equal(t, "vet", req.GetToolName())
	assert.Equal(t, "2.0.0", req.GetToolVersion())
	assert.Equal(t, []string{WorkloadIdentityAudience}, tokens.audiences)

	assert.Equal(t, WorkloadCredential{
		AccessToken:     "wc-1",
		ExpiresAt:       clock.Now().Add(time.Hour).UTC(),
		TenantDomain:    "acme.safedep.io",
		EndpointID:      "gha/acme/web",
		CredentialID:    "exchange-1",
		DisplayIdentity: "github:acme/web@refs/heads/main run 987/1",
	}, cred)
	assert.NotContains(t, cred.String(), "wc-1")
	assert.NotContains(t, fmt.Sprintf("%v", cred), "wc-1")
}

func TestWorkloadExchangeClient_FreshIDTokenForEachExchange(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	fake := &fakeWorkloadIdentityServer{clock: &fakeClock{now: time.Now()}, life: time.Hour}
	client := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, startFakeWorkloadIdentityServer(t, fake)))

	for range 2 {
		_, err := client.Exchange(context.Background(), source, WorkloadExchangeRequest{TenantDomain: "acme.safedep.io"})
		require.NoError(t, err)
	}

	assert.Equal(t, []string{"id-token-1", "id-token-2"}, fake.subjectTokens())
	assert.Nil(t, fake.exchanges[0].GetRequestedLifetime(), "zero lifetime takes the rule maximum")
}

func TestRefreshingWorkloadCredential_RefreshesBeforeExpiry(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	clock := &fakeClock{now: time.Unix(1_900_000_000, 0)}
	fake := &fakeWorkloadIdentityServer{clock: clock, life: time.Hour}
	client := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, startFakeWorkloadIdentityServer(t, fake)))

	w := NewRefreshingWorkloadCredential(client, source, WorkloadExchangeRequest{TenantDomain: "acme.safedep.io"})
	w.now = clock.Now
	ctx := context.Background()

	md, err := w.GetRequestMetadata(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"authorization": "wc-1", "x-tenant-id": "acme.safedep.io"}, md)

	clock.Advance(time.Hour - workloadRefreshMargin - time.Second)
	token, err := w.AccessToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "wc-1", token, "the credential is far from expiry")

	clock.Advance(2 * time.Second)
	token, err = w.AccessToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "wc-2", token, "the credential is within the refresh margin")

	assert.Equal(t, []string{"id-token-1", "id-token-2"}, fake.subjectTokens())
	assert.Equal(t, 2, tokens.servedCount())
}

func TestRefreshingWorkloadCredential_KeepsCredentialWhenRefreshFails(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	clock := &fakeClock{now: time.Unix(1_900_000_000, 0)}
	fake := &fakeWorkloadIdentityServer{clock: clock, life: time.Hour}
	client := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, startFakeWorkloadIdentityServer(t, fake)))

	w := NewRefreshingWorkloadCredential(client, source, WorkloadExchangeRequest{TenantDomain: "acme.safedep.io"})
	w.now = clock.Now
	ctx := context.Background()

	token, err := w.AccessToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "wc-1", token)

	// The job can no longer get an ID token, as in the post step of a job.
	tokens.setFailing(true)

	clock.Advance(time.Hour - time.Minute)
	token, err = w.AccessToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "wc-1", token, "the current credential stays until it expires")

	clock.Advance(time.Minute)
	_, err = w.AccessToken(ctx)
	assert.ErrorContains(t, err, "GitHub Actions ID token request returned status 403")
	assert.Len(t, fake.exchanges, 1)
}

func TestRefreshingWorkloadCredential_Revoke(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)
	source, err := NewGitHubActionsSource()
	require.NoError(t, err)

	fake := &fakeWorkloadIdentityServer{clock: &fakeClock{now: time.Now()}, life: time.Hour}
	client := NewWorkloadExchangeClient(dialFakeWorkloadIdentityServer(t, startFakeWorkloadIdentityServer(t, fake)))

	w := NewRefreshingWorkloadCredential(client, source, WorkloadExchangeRequest{TenantDomain: "acme.safedep.io"})
	ctx := context.Background()

	_, err = w.AccessToken(ctx)
	require.NoError(t, err)
	require.NoError(t, w.Revoke(ctx))

	require.Len(t, fake.revokes, 1)
	assert.Equal(t, "wc-1", fake.revokes[0].GetAccessToken())
	assert.Equal(t, "acme.safedep.io", fake.revokes[0].GetTenantDomain())

	_, err = w.AccessToken(ctx)
	assert.ErrorIs(t, err, ErrWorkloadCredentialRevoked)
	assert.Len(t, fake.exchanges, 1, "a revoked credential does not exchange again")
}
