package configsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			f.commit(policyA, 1)
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

// The credentials of the tool move from tenant A to tenant B. The cache of A
// must not apply while the fetch for B fails, and the first fetch for B
// replaces it.
func TestStore_TenantMigration(t *testing.T) {
	f := newFixture(t)
	f.commit(policyA, 7)

	f.rebind(WithTenant(tenantB))
	_, err := f.store.Load(acceptAll)
	require.ErrorIs(t, err, ErrTenantMismatch)

	f.tick()
	f.reply(modified(policyB, 7), nil)
	refuse := refuseWith(assert.AnError)
	req := request()
	req.Validate = refuse
	_, err = f.fetcher.Fetch(context.Background(), req)
	require.ErrorIs(t, err, ErrRejected)
	_, err = f.store.Load(acceptAll)
	require.ErrorIs(t, err, ErrTenantMismatch, "a rejected policy of B must not bring back the cache of A")

	f.tick()
	f.commit(policyB, 1)
	loaded, err := f.store.Load(acceptAll)
	require.NoError(t, err)
	assert.Equal(t, policyB, loaded.Policy)
	assert.Equal(t, fileOf(t, policyB), f.policyFiles(), "the commit for B deletes the files of A")
}
