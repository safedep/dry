package configsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rebind gives the fixture a store and a fetcher on the same folder with
// opts, as a tool does when it starts with new credentials.
func (f *fixture) rebind(opts ...StoreOption) {
	f.t.Helper()
	store, err := NewStore(f.dir, opts...)
	require.NoError(f.t, err)
	fetcher, err := NewFetcher(store, f.transport)
	require.NoError(f.t, err)
	fetcher.now = func() time.Time { return f.clock }
	f.store, f.fetcher = store, fetcher
}

func TestStoreLoad_Tenant(t *testing.T) {
	cases := []struct {
		name       string
		opts       []StoreOption
		wantPolicy []byte
	}{
		{name: "no tenant loads the cache of any tenant", wantPolicy: policyA},
		{name: "same tenant loads the cache", opts: []StoreOption{WithTenant(tenantA)}, wantPolicy: policyA},
		{name: "another tenant refuses the cache", opts: []StoreOption{WithTenant(tenantB)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(tenantA, policyA, 1)
			f.rebind(tc.opts...)

			validated := false
			loaded, err := f.store.Load(func([]byte) (Accepted, error) {
				validated = true
				return Accepted{}, nil
			})

			if tc.wantPolicy == nil {
				require.ErrorIs(t, err, ErrNoCache)
				assert.ErrorIs(t, err, ErrTenantMismatch)
				assert.Nil(t, loaded)
				assert.False(t, validated, "Load must refuse the cache before it reads a policy file")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPolicy, loaded.Policy)
		})
	}
}

func TestFetch_RefusesARequestForAnotherTenant(t *testing.T) {
	f := newFixture(t, WithTenant(tenantA))
	f.commit(tenantA, policyA, 1)
	before := f.state()

	// The mock transport fails the test on a call that it does not expect.
	_, err := f.fetcher.Fetch(context.Background(), request(tenantB))

	require.ErrorIs(t, err, ErrInvalidRequest)
	assert.ErrorIs(t, err, ErrTenantMismatch)
	assert.Equal(t, before, f.state(), "a refused request must not write the state")
}

// The credentials of the tool move from tenant A to tenant B. The cache of A
// must not apply while the fetch for B fails, and the first fetch for B
// replaces it.
func TestStore_TenantMigration(t *testing.T) {
	f := newFixture(t, WithTenant(tenantA))
	f.commit(tenantA, policyA, 7)

	f.rebind(WithTenant(tenantB))
	_, err := f.store.Load(acceptAll)
	require.ErrorIs(t, err, ErrTenantMismatch)

	f.tick()
	f.reply(modified(policyB, 7), nil)
	refuse := refuseWith(assert.AnError)
	req := request(tenantB)
	req.Validate = refuse
	_, err = f.fetcher.Fetch(context.Background(), req)
	require.ErrorIs(t, err, ErrRejected)
	_, err = f.store.Load(acceptAll)
	require.ErrorIs(t, err, ErrTenantMismatch, "a rejected policy of B must not bring back the cache of A")

	f.tick()
	f.commit(tenantB, policyB, 1)
	loaded, err := f.store.Load(acceptAll)
	require.NoError(t, err)
	assert.Equal(t, policyB, loaded.Policy)
	assert.Equal(t, fileOf(t, policyB), f.policyFiles(), "the commit for B deletes the files of A")
}
