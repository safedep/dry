package configsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	policyv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/policy/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tenantA = "a.example.safedep.io"
	tenantB = "b.example.safedep.io"
)

var (
	policyA = []byte("policy A")
	policyB = []byte("policy B")
	policyC = []byte("policy C")
	policyD = []byte("policy D")
)

type fixture struct {
	t         *testing.T
	dir       string
	store     *Store
	transport *MockTransport
	fetcher   *Fetcher
	clock     time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cloud-policy")
	store, err := NewStore(dir)
	require.NoError(t, err)
	transport := NewMockTransport(t)
	fetcher, err := NewFetcher(store, transport)
	require.NoError(t, err)

	f := &fixture{
		t:         t,
		dir:       dir,
		store:     store,
		transport: transport,
		fetcher:   fetcher,
		clock:     time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC),
	}
	fetcher.now = func() time.Time { return f.clock }
	return f
}

func (f *fixture) tick() time.Time {
	f.clock = f.clock.Add(time.Minute)
	return f.clock
}

// reply makes the next call of the transport return resp and err, and
// returns the request that it receives.
func (f *fixture) reply(resp *policyv1.GetResolvedPolicyResponse, err error) **policyv1.GetResolvedPolicyRequest {
	got := new(*policyv1.GetResolvedPolicyRequest)
	f.transport.EXPECT().GetResolvedPolicy(mock.Anything, mock.Anything).
		Run(func(_ context.Context, req *policyv1.GetResolvedPolicyRequest) { *got = req }).
		Return(resp, err).Once()
	return got
}

func (f *fixture) commit(tenant string, raw []byte, revision uint64) {
	f.t.Helper()
	f.reply(modified(raw, revision), nil)
	res, err := f.fetcher.Fetch(context.Background(), request(tenant))
	require.NoError(f.t, err)
	require.Equal(f.t, FetchModified, res.Status)
}

func (f *fixture) state() *State {
	f.t.Helper()
	st, err := f.store.State()
	require.NoError(f.t, err)
	return st
}

func (f *fixture) policyFiles() []string {
	f.t.Helper()
	entries, err := os.ReadDir(f.dir)
	require.NoError(f.t, err)
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), policyPrefix) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names
}

func fileOf(t *testing.T, raw ...[]byte) []string {
	t.Helper()
	var names []string
	for _, r := range raw {
		name, err := policyFileName(Digest(r))
		require.NoError(t, err)
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func modified(raw []byte, revision uint64) *policyv1.GetResolvedPolicyResponse {
	return policyv1.GetResolvedPolicyResponse_builder{
		Status:         policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_MODIFIED,
		Digest:         Digest(raw),
		Revision:       revision,
		ResolvedPolicy: raw,
	}.Build()
}

func notModified(digest string) *policyv1.GetResolvedPolicyResponse {
	return policyv1.GetResolvedPolicyResponse_builder{
		Status: policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_NOT_MODIFIED,
		Digest: digest,
	}.Build()
}

func request(tenant string) FetchRequest {
	return FetchRequest{
		TenantDomain: tenant,
		Endpoint: controltowerv1.EndpointIdentity_builder{
			Identifier: "laptop-1",
			MachineId:  "machine-1",
		}.Build(),
		Labels: []*controltowerv1.Label{
			controltowerv1.Label_builder{Key: "reported/os", Value: "linux"}.Build(),
		},
		ToolName:    "pmg",
		ToolVersion: "1.2.3",
		Validate:    func([]byte) error { return nil },
	}
}

func TestFetch_FirstModifiedCommits(t *testing.T) {
	f := newFixture(t)
	got := f.reply(modified(policyA, 7), nil)

	res, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)
	assert.Equal(t, &FetchResult{Status: FetchModified, Digest: Digest(policyA), Revision: 7}, res)

	req := *got
	require.NotNil(t, req)
	assert.Empty(t, req.GetDigest(), "an empty cache sends an empty digest")
	assert.Equal(t, "machine-1", req.GetEndpoint().GetMachineId())
	assert.Equal(t, "pmg", req.GetToolName())
	assert.Equal(t, "1.2.3", req.GetToolVersion())
	require.Len(t, req.GetLabels(), 1)
	assert.Equal(t, "reported/os", req.GetLabels()[0].GetKey())

	now := f.clock
	assert.Equal(t, &State{
		FormatVersion:   FormatVersion,
		Tenant:          tenantA,
		Digest:          Digest(policyA),
		Revision:        7,
		HighestRevision: 7,
		StoredAt:        now,
		FetchedAt:       now,
		LastAttemptAt:   now,
	}, f.state())
	assert.Equal(t, fileOf(t, policyA), f.policyFiles())

	loaded, err := f.store.Load(nil)
	require.NoError(t, err)
	assert.Equal(t, policyA, loaded.Policy)
	assert.False(t, loaded.FromPrevious)
}

func TestFetch_SecondModifiedKeepsPrevious(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	f.tick()

	got := f.reply(modified(policyB, 2), nil)
	_, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)

	assert.Equal(t, Digest(policyA), (*got).GetDigest(), "the fetch sends the digest of the cache")
	st := f.state()
	assert.Equal(t, Digest(policyB), st.Digest)
	assert.Equal(t, Digest(policyA), st.PreviousDigest)
	assert.Equal(t, uint64(1), st.PreviousRevision)
	assert.Equal(t, uint64(2), st.HighestRevision)
	assert.Equal(t, fileOf(t, policyA, policyB), f.policyFiles())
}

func TestFetch_NotModifiedMovesOnlyFetchedAt(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 3)
	before := f.state()

	now := f.tick()
	got := f.reply(notModified(Digest(policyA)), nil)
	res, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)
	assert.Equal(t, &FetchResult{Status: FetchNotModified, Digest: Digest(policyA), Revision: 3}, res)
	assert.Equal(t, Digest(policyA), (*got).GetDigest())

	want := *before
	want.FetchedAt = now
	assert.Equal(t, &want, f.state())
	assert.Equal(t, fileOf(t, policyA), f.policyFiles())
}

// Each failed check keeps the cache: no policy bytes, no move of
// fetched_at, and a record in last_attempt_at and last_error.
func TestFetch_FailedChecksKeepTheCache(t *testing.T) {
	refuse := errors.New("header: tenant mismatch")

	cases := []struct {
		name         string
		resp         *policyv1.GetResolvedPolicyResponse
		rpcErr       error
		validate     func([]byte) error
		wantErr      error
		wantCode     codes.Code
		wantRejected bool
	}{
		{
			name: "wrong digest writes nothing",
			resp: policyv1.GetResolvedPolicyResponse_builder{
				Status:         policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_MODIFIED,
				Digest:         Digest([]byte("other bytes")),
				Revision:       6,
				ResolvedPolicy: policyB,
			}.Build(),
			wantErr: ErrDigestMismatch,
		},
		{
			name:    "lower revision is ignored",
			resp:    modified(policyB, 4),
			wantErr: ErrLowerRevision,
		},
		{
			name:         "validator failure keeps the cache and records the rejection",
			resp:         modified(policyB, 6),
			validate:     func([]byte) error { return refuse },
			wantErr:      ErrRejected,
			wantRejected: true,
		},
		{
			name:    "not modified with another digest is a failed fetch",
			resp:    notModified(Digest(policyB)),
			wantErr: ErrDigestMismatch,
		},
		{
			name: "unknown status",
			resp: policyv1.GetResolvedPolicyResponse_builder{
				Status: policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_UNSPECIFIED,
			}.Build(),
			wantErr: ErrUnknownStatus,
		},
		{
			name:     "grpc error",
			rpcErr:   status.Error(codes.Unavailable, "try later"),
			wantCode: codes.Unavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(tenantA, policyA, 5)
			before := f.state()

			now := f.tick()
			f.reply(tc.resp, tc.rpcErr)
			req := request(tenantA)
			if tc.validate != nil {
				req.Validate = tc.validate
			}

			res, err := f.fetcher.Fetch(context.Background(), req)
			require.Error(t, err)
			assert.Nil(t, res)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			if tc.wantCode != codes.OK {
				assert.Equal(t, tc.wantCode, status.Code(err))
			}

			assert.Equal(t, fileOf(t, policyA), f.policyFiles(), "a failed check writes no policy bytes")

			want := *before
			want.LastAttemptAt = now
			want.LastError = err.Error()
			if tc.wantRejected {
				want.Rejected = &Rejection{Digest: Digest(policyB), Revision: 6, Reason: refuse.Error(), At: now}
			}
			st := f.state()
			assert.Equal(t, &want, st)
			assert.True(t, st.LastFetchFailed())
			assert.Equal(t, tc.wantRejected, st.LastFetchRejected())

			loaded, err := f.store.Load(nil)
			require.NoError(t, err)
			assert.Equal(t, policyA, loaded.Policy)
		})
	}
}

func TestFetch_SameRevisionWithNewDigestIsAccepted(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 5)
	f.tick()
	f.commit(tenantA, policyB, 5)

	st := f.state()
	assert.Equal(t, Digest(policyB), st.Digest)
	assert.Equal(t, uint64(5), st.HighestRevision)
}

func TestFetch_CommitClearsTheLastFailure(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 5)
	f.tick()
	f.reply(modified(policyB, 6), nil)
	req := request(tenantA)
	req.Validate = func([]byte) error { return errors.New("bad") }
	_, err := f.fetcher.Fetch(context.Background(), req)
	require.ErrorIs(t, err, ErrRejected)

	now := f.tick()
	f.commit(tenantA, policyC, 7)

	st := f.state()
	assert.Nil(t, st.Rejected)
	assert.Empty(t, st.LastError)
	assert.Equal(t, now, st.LastAttemptAt)
	assert.False(t, st.LastFetchFailed())
}

func TestFetch_NewTenantResetsHighestRevision(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 40)
	f.tick()
	f.commit(tenantA, policyB, 50)
	f.tick()

	got := f.reply(modified(policyC, 3), nil)
	req := request(tenantB)
	req.CapabilitySources = func([]byte) map[string]string {
		return map[string]string{"package_admission": Digest(policyB)}
	}
	res, err := f.fetcher.Fetch(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, FetchModified, res.Status)

	assert.Empty(t, (*got).GetDigest(), "a cache of another tenant sends an empty digest")
	st := f.state()
	assert.Equal(t, tenantB, st.Tenant)
	assert.Equal(t, uint64(3), st.HighestRevision)
	assert.Empty(t, st.PreviousDigest, "a policy of another tenant is no fallback")
	assert.Nil(t, st.CapabilitySources)
	assert.Equal(t, fileOf(t, policyC), f.policyFiles())
}

func TestFetch_FailedFetchForNewTenantKeepsTheCache(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 40)
	f.tick()

	f.reply(nil, status.Error(codes.Unavailable, "down"))
	_, err := f.fetcher.Fetch(context.Background(), request(tenantB))
	require.Error(t, err)

	st := f.state()
	assert.Equal(t, tenantA, st.Tenant)
	assert.Equal(t, uint64(40), st.HighestRevision)
	assert.Equal(t, fileOf(t, policyA), f.policyFiles())
}

func TestFetch_GRPCErrorWithNoCache(t *testing.T) {
	f := newFixture(t)
	now := f.tick()
	f.reply(nil, status.Error(codes.Unavailable, "down"))

	_, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))

	st := f.state()
	assert.False(t, st.HasPolicy())
	assert.Equal(t, now, st.LastAttemptAt)
	assert.Contains(t, st.LastError, "down")

	_, err = f.store.Load(nil)
	assert.ErrorIs(t, err, ErrNoCache)
}

func TestFetch_NotModifiedWithNoCacheFails(t *testing.T) {
	f := newFixture(t)
	f.reply(notModified(""), nil)

	_, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	assert.ErrorIs(t, err, ErrDigestMismatch)
	assert.True(t, f.state().LastFetchFailed())
}

func TestFetch_DamagedCurrentFileSendsEmptyDigest(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	f.tick()
	f.commit(tenantA, policyB, 2)
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, fileOf(t, policyB)[0]), []byte("damaged"), fileMode))
	f.tick()

	got := f.reply(modified(policyB, 2), nil)
	_, err := f.fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)

	assert.Empty(t, (*got).GetDigest())
	st := f.state()
	assert.Equal(t, Digest(policyB), st.Digest)
	assert.Equal(t, Digest(policyA), st.PreviousDigest, "the previous file stays the fallback")
	loaded, err := f.store.Load(nil)
	require.NoError(t, err)
	assert.Equal(t, policyB, loaded.Policy)
}

// The test stops the commit between the rename of the policy file and the
// rename of state.json, as a crash does, and then reads the folder with a
// new Store, as the next process does.
func TestFetch_CrashBetweenRenamesLeavesThePreviousCommit(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	before := f.state()

	f.tick()
	f.store.afterPolicyRename = func() { panic("crash") }
	f.reply(modified(policyB, 2), nil)
	require.PanicsWithValue(t, "crash", func() {
		_, _ = f.fetcher.Fetch(context.Background(), request(tenantA))
	})

	assert.Equal(t, fileOf(t, policyA, policyB), f.policyFiles(), "the policy file of the crashed commit is on disk")

	store, err := NewStore(f.dir)
	require.NoError(t, err)
	st, err := store.State()
	require.NoError(t, err)
	assert.Equal(t, before, st)
	loaded, err := store.Load(nil)
	require.NoError(t, err)
	assert.Equal(t, policyA, loaded.Policy)
	assert.False(t, loaded.FromPrevious)

	// The next commit deletes the file that no state names.
	transport := NewMockTransport(t)
	fetcher, err := NewFetcher(store, transport)
	require.NoError(t, err)
	transport.EXPECT().GetResolvedPolicy(mock.Anything, mock.Anything).Return(modified(policyC, 3), nil).Once()
	_, err = fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)
	assert.Equal(t, fileOf(t, policyA, policyC), f.policyFiles())
}

func TestFetch_CapabilitySourcesKeepTheirFiles(t *testing.T) {
	f := newFixture(t)
	fetch := func(raw []byte, revision uint64, sources map[string]string) {
		t.Helper()
		f.tick()
		f.reply(modified(raw, revision), nil)
		req := request(tenantA)
		req.CapabilitySources = func(got []byte) map[string]string {
			assert.Equal(t, raw, got)
			return sources
		}
		_, err := f.fetcher.Fetch(context.Background(), req)
		require.NoError(t, err)
	}

	fetch(policyA, 1, nil)
	fetch(policyB, 2, map[string]string{"package_admission": Digest(policyA)})
	fetch(policyC, 3, map[string]string{
		"package_admission": Digest(policyA),
		"cooldown":          Digest(policyC), // the current file is no source
		"broken":            "../state.json",
	})

	st := f.state()
	assert.Equal(t, map[string]string{"package_admission": Digest(policyA)}, st.CapabilitySources)
	assert.Equal(t, Digest(policyB), st.PreviousDigest)
	assert.Equal(t, fileOf(t, policyA, policyB, policyC), f.policyFiles())

	raw, err := f.store.ReadPolicy(st.CapabilitySources["package_admission"])
	require.NoError(t, err)
	assert.Equal(t, policyA, raw)

	fetch(policyD, 4, nil)
	assert.Nil(t, f.state().CapabilitySources)
	assert.Equal(t, fileOf(t, policyC, policyD), f.policyFiles())
}

func TestFetch_InvalidRequest(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*FetchRequest)
	}{
		{"no tenant", func(r *FetchRequest) { r.TenantDomain = "" }},
		{"no endpoint", func(r *FetchRequest) { r.Endpoint = nil }},
		{"no tool name", func(r *FetchRequest) { r.ToolName = "" }},
		{"no validator", func(r *FetchRequest) { r.Validate = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			req := request(tenantA)
			tc.mutate(&req)
			_, err := f.fetcher.Fetch(context.Background(), req)
			assert.ErrorIs(t, err, ErrInvalidRequest)
			_, statErr := os.Stat(f.dir)
			assert.ErrorIs(t, statErr, os.ErrNotExist, "an invalid request writes nothing")
		})
	}
}

func TestNewFetcher_RequiresStoreAndTransport(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	_, err = NewFetcher(nil, NewMockTransport(t))
	assert.ErrorIs(t, err, ErrInvalidRequest)
	_, err = NewFetcher(store, nil)
	assert.ErrorIs(t, err, ErrInvalidRequest)
}
