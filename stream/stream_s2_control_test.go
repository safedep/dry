package stream

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/s2-streamstore/s2-sdk-go/s2"
)

// ptrEq returns true iff both pointers are nil, or both are non-nil and
// point to equal values. Used for comparing *string fields of s2.ResourceSet.
func ptrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestS2StreamMatcher(t *testing.T) {
	tests := []struct {
		name string
		req  StreamAccessRequest
		want *s2.ResourceSet
	}{
		{
			name: "single stream exact",
			req: StreamAccessRequest{
				Stream: Stream{Namespace: "ns1", Name: "s1"},
			},
			want: &s2.ResourceSet{Exact: s2.Ptr("ns1:s1")},
		},
		{
			name: "single multi-tenant stream exact",
			req: StreamAccessRequest{
				Stream: Stream{TenantID: "t1", Namespace: "ns1", Name: "s1", IsMultiTenant: true},
			},
			want: &s2.ResourceSet{Exact: s2.Ptr("t1:ns1:s1")},
		},
		{
			name: "scope tenant only",
			req: StreamAccessRequest{
				Scope: &StreamScope{TenantID: "t1"},
			},
			want: &s2.ResourceSet{Prefix: s2.Ptr("t1:")},
		},
		{
			name: "scope tenant + namespace",
			req: StreamAccessRequest{
				Scope: &StreamScope{TenantID: "t1", Namespace: "ns1"},
			},
			want: &s2.ResourceSet{Prefix: s2.Ptr("t1:ns1:")},
		},
		{
			name: "scope tenant + namespace + name prefix",
			req: StreamAccessRequest{
				Scope: &StreamScope{TenantID: "t1", Namespace: "ns1", NamePrefix: "foo"},
			},
			want: &s2.ResourceSet{Prefix: s2.Ptr("t1:ns1:foo")},
		},
		{
			name: "scope namespace only",
			req: StreamAccessRequest{
				Scope: &StreamScope{Namespace: "ns1"},
			},
			want: &s2.ResourceSet{Prefix: s2.Ptr("ns1:")},
		},
		{
			name: "scope namespace + name prefix",
			req: StreamAccessRequest{
				Scope: &StreamScope{Namespace: "ns1", NamePrefix: "foo"},
			},
			want: &s2.ResourceSet{Prefix: s2.Ptr("ns1:foo")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s2StreamMatcher(tt.req)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got == nil {
				t.Fatalf("got nil matcher")
			}
			if !ptrEq(got.Exact, tt.want.Exact) || !ptrEq(got.Prefix, tt.want.Prefix) {
				t.Fatalf("want {Exact:%v Prefix:%v}, got {Exact:%v Prefix:%v}",
					deref(tt.want.Exact), deref(tt.want.Prefix),
					deref(got.Exact), deref(got.Prefix))
			}
		})
	}
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestS2OpsForRole(t *testing.T) {
	tests := []struct {
		role StreamAccessRole
		want []string
	}{
		{StreamAccessRead, []string{s2.OperationRead, s2.OperationCheckTail}},
		{StreamAccessWrite, []string{s2.OperationAppend}},
		{StreamAccessReadWrite, []string{s2.OperationRead, s2.OperationCheckTail, s2.OperationAppend}},
	}
	for _, tt := range tests {
		t.Run(tt.role.String(), func(t *testing.T) {
			got := s2OpsForRole(tt.role)
			if len(got) != len(tt.want) {
				t.Fatalf("want %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("want %v, got %v", tt.want, got)
				}
			}
		})
	}
}

// fakeS2Tokens implements s2AccessTokenClient for unit tests.
type fakeS2Tokens struct {
	issueIn   *s2IssueInput
	issueOut  *s2IssueOutput
	issueErr  error
	revokeID  string
	revokeErr error
}

func (f *fakeS2Tokens) IssueAccessToken(_ context.Context, in *s2IssueInput) (*s2IssueOutput, error) {
	f.issueIn = in
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issueOut, nil
}

func (f *fakeS2Tokens) RevokeAccessToken(_ context.Context, id string) error {
	f.revokeID = id
	return f.revokeErr
}

// stubBasinResolver returns a fixed basin name.
type stubBasinResolver struct{ basin string }

func (r *stubBasinResolver) GetBasin(_ context.Context, _, _ string) (string, error) {
	return r.basin, nil
}

func newTestControlPlane(t *testing.T, fake *fakeS2Tokens, fixedNow time.Time) *s2StreamControlPlane {
	t.Helper()
	return &s2StreamControlPlane{
		tokens:        fake,
		basinResolver: &stubBasinResolver{basin: "test-basin"},
		now:           func() time.Time { return fixedNow },
	}
}

func TestCreateStreamAccess_SingleStream(t *testing.T) {
	fixedNow := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	expiresAt := fixedNow.Add(time.Hour)
	fake := &fakeS2Tokens{
		issueOut: &s2IssueOutput{Token: "sk_abc", ExpiresAt: expiresAt},
	}
	cp := newTestControlPlane(t, fake, fixedNow)

	got, err := cp.CreateStreamAccess(context.Background(), StreamAccessRequest{
		Stream: Stream{Namespace: "ns1", Name: "s1"},
		Access: StreamAccessWrite,
		Expiry: time.Hour,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Token != "sk_abc" {
		t.Fatalf("want token sk_abc, got %q", got.Token)
	}
	if got.AccessID == "" {
		t.Fatalf("AccessID should be populated")
	}
	if !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("want ExpiresAt %v, got %v", expiresAt, got.ExpiresAt)
	}
	if fake.issueIn == nil {
		t.Fatal("fake not called")
	}
	if fake.issueIn.ID != got.AccessID {
		t.Fatalf("AccessID must match issued ID; got %s vs %s", got.AccessID, fake.issueIn.ID)
	}
	if !fake.issueIn.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("want ExpiresAt %v passed to s2, got %v", expiresAt, fake.issueIn.ExpiresAt)
	}
	if len(fake.issueIn.Ops) != 1 {
		t.Fatalf("write role should produce one op, got %v", fake.issueIn.Ops)
	}
}

func TestCreateStreamAccess_ScopedPrefix(t *testing.T) {
	fixedNow := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	fake := &fakeS2Tokens{
		issueOut: &s2IssueOutput{Token: "sk_xyz", ExpiresAt: fixedNow.Add(time.Hour)},
	}
	cp := newTestControlPlane(t, fake, fixedNow)

	_, err := cp.CreateStreamAccess(context.Background(), StreamAccessRequest{
		Scope:  &StreamScope{TenantID: "t1", Namespace: "ns1"},
		Access: StreamAccessReadWrite,
		Expiry: 0,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fake.issueIn.StreamMatch == nil || fake.issueIn.StreamMatch.Prefix == nil {
		t.Fatalf("want Prefix matcher, got %+v", fake.issueIn.StreamMatch)
	}
	if *fake.issueIn.StreamMatch.Prefix != "t1:ns1:" {
		t.Fatalf("want prefix t1:ns1:, got %q", *fake.issueIn.StreamMatch.Prefix)
	}
	if !fake.issueIn.ExpiresAt.Equal(fixedNow.Add(StreamAccessDefaultExpiry)) {
		t.Fatalf("expected default expiry applied")
	}
}

func TestCreateStreamAccess_ValidationErrors(t *testing.T) {
	cp := newTestControlPlane(t, &fakeS2Tokens{}, time.Now())
	cases := []struct {
		name string
		req  StreamAccessRequest
		want error
	}{
		{"role none", StreamAccessRequest{Stream: Stream{Namespace: "n", Name: "s"}, Access: StreamAccessNone}, ErrInvalidAccessRole},
		{"expiry too short", StreamAccessRequest{Stream: Stream{Namespace: "n", Name: "s"}, Access: StreamAccessRead, Expiry: time.Second}, ErrExpiryTooShort},
		{"empty scope", StreamAccessRequest{Scope: &StreamScope{}, Access: StreamAccessRead}, ErrInvalidScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cp.CreateStreamAccess(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCreateStreamAccess_PropagatesS2Error(t *testing.T) {
	boom := errors.New("boom")
	fake := &fakeS2Tokens{issueErr: boom}
	cp := newTestControlPlane(t, fake, time.Now())

	_, err := cp.CreateStreamAccess(context.Background(), StreamAccessRequest{
		Stream: Stream{Namespace: "n", Name: "s"},
		Access: StreamAccessRead,
		Expiry: time.Hour,
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want wrapped boom, got %v", err)
	}
}

func TestDeleteStreamAccess_Success(t *testing.T) {
	fake := &fakeS2Tokens{}
	cp := newTestControlPlane(t, fake, time.Now())
	err := cp.DeleteStreamAccess(context.Background(), "tok-123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fake.revokeID != "tok-123" {
		t.Fatalf("want revoke id tok-123, got %q", fake.revokeID)
	}
}

func TestDeleteStreamAccess_EmptyIDRejected(t *testing.T) {
	cp := newTestControlPlane(t, &fakeS2Tokens{}, time.Now())
	err := cp.DeleteStreamAccess(context.Background(), "")
	if !errors.Is(err, ErrInvalidAccessID) {
		t.Fatalf("want ErrInvalidAccessID, got %v", err)
	}
}

func TestDeleteStreamAccess_IdempotentOnNotFound(t *testing.T) {
	fake := &fakeS2Tokens{revokeErr: errS2AccessTokenNotFound}
	cp := newTestControlPlane(t, fake, time.Now())
	err := cp.DeleteStreamAccess(context.Background(), "already-gone")
	if err != nil {
		t.Fatalf("not-found should be treated as success, got %v", err)
	}
}

func TestDeleteStreamAccess_PropagatesOtherErrors(t *testing.T) {
	boom := errors.New("transport boom")
	fake := &fakeS2Tokens{revokeErr: boom}
	cp := newTestControlPlane(t, fake, time.Now())
	err := cp.DeleteStreamAccess(context.Background(), "tok-x")
	if !errors.Is(err, boom) {
		t.Fatalf("want wrapped boom, got %v", err)
	}
}

type fakeS2Streams struct {
	createBasin, createName string
	createErr               error
	deleteBasin, deleteName string
	deleteErr               error
}

func (f *fakeS2Streams) CreateStream(_ context.Context, basin, name string) error {
	f.createBasin, f.createName = basin, name
	return f.createErr
}

func (f *fakeS2Streams) DeleteStream(_ context.Context, basin, name string) error {
	f.deleteBasin, f.deleteName = basin, name
	return f.deleteErr
}

func newTestControlPlaneWithStreams(t *testing.T, tokens *fakeS2Tokens, streams *fakeS2Streams) *s2StreamControlPlane {
	t.Helper()
	return &s2StreamControlPlane{
		tokens:        tokens,
		streams:       streams,
		basinResolver: &stubBasinResolver{basin: "test-basin"},
		now:           time.Now,
	}
}

func TestCreateStream_Success(t *testing.T) {
	streams := &fakeS2Streams{}
	cp := newTestControlPlaneWithStreams(t, &fakeS2Tokens{}, streams)
	err := cp.CreateStream(context.Background(), Stream{Namespace: "ns1", Name: "s1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if streams.createName != "ns1:s1" || streams.createBasin != "test-basin" {
		t.Fatalf("wrong basin/name: %q/%q", streams.createBasin, streams.createName)
	}
}

func TestCreateStream_IdempotentOnAlreadyExists(t *testing.T) {
	streams := &fakeS2Streams{createErr: errS2StreamAlreadyExists}
	cp := newTestControlPlaneWithStreams(t, &fakeS2Tokens{}, streams)
	err := cp.CreateStream(context.Background(), Stream{Namespace: "ns1", Name: "s1"})
	if err != nil {
		t.Fatalf("want nil on already-exists, got %v", err)
	}
}

func TestDeleteStream_IdempotentOnNotFound(t *testing.T) {
	streams := &fakeS2Streams{deleteErr: errS2StreamNotFound}
	cp := newTestControlPlaneWithStreams(t, &fakeS2Tokens{}, streams)
	err := cp.DeleteStream(context.Background(), Stream{Namespace: "ns1", Name: "s1"})
	if err != nil {
		t.Fatalf("want nil on not-found, got %v", err)
	}
}

func TestCreateStream_InvalidStreamRejected(t *testing.T) {
	streams := &fakeS2Streams{}
	cp := newTestControlPlaneWithStreams(t, &fakeS2Tokens{}, streams)
	err := cp.CreateStream(context.Background(), Stream{})
	if err == nil {
		t.Fatal("want error on empty stream")
	}
}

func requireS2Admin(t *testing.T) S2ControlPlaneConfig {
	t.Helper()
	cfg := DefaultS2ControlPlaneConfig()
	if cfg.AdminApiKey == "" {
		t.Skip("STREAM_PROVIDER_S2_ADMIN_API_KEY not set; skipping S2 integration test")
	}
	return cfg
}

func TestIntegration_IssueAndRevokeSingleStreamToken(t *testing.T) {
	cfg := requireS2Admin(t)
	cp, err := NewS2StreamControlPlane(cfg, NewDefaultS2BasinResolver())
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	ctx := context.Background()

	// Use a throwaway stream name to avoid collisions.
	stream := Stream{
		Namespace: "integration-test",
		Name:      fmt.Sprintf("scoped-token-%d", time.Now().UnixNano()),
	}
	if err := cp.CreateStream(ctx, stream); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	t.Cleanup(func() { _ = cp.DeleteStream(context.Background(), stream) })

	access, err := cp.CreateStreamAccess(ctx, StreamAccessRequest{
		Stream: stream,
		Access: StreamAccessWrite,
		Expiry: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if access.Token == "" || access.AccessID == "" {
		t.Fatalf("empty access response: %+v", access)
	}
	if access.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expiry in the past: %v", access.ExpiresAt)
	}

	// Revoke.
	if err := cp.DeleteStreamAccess(ctx, access.AccessID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// Second revoke: idempotent.
	if err := cp.DeleteStreamAccess(ctx, access.AccessID); err != nil {
		t.Fatalf("idempotent revoke: %v", err)
	}
}

func TestIntegration_IssuePrefixScopedToken(t *testing.T) {
	cfg := requireS2Admin(t)
	cp, err := NewS2StreamControlPlane(cfg, NewDefaultS2BasinResolver())
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	ctx := context.Background()

	access, err := cp.CreateStreamAccess(ctx, StreamAccessRequest{
		Scope:  &StreamScope{Namespace: "integration-test", NamePrefix: "scoped-token-"},
		Access: StreamAccessReadWrite,
		Expiry: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	t.Cleanup(func() { _ = cp.DeleteStreamAccess(context.Background(), access.AccessID) })

	if access.Token == "" {
		t.Fatal("empty token")
	}
}
