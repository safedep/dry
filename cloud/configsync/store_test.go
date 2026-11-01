package configsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreLoad(t *testing.T) {
	refuseB := func(raw []byte) error {
		if bytes.Equal(raw, policyB) {
			return errors.New("header: machine id mismatch")
		}
		return nil
	}
	refuseAll := func([]byte) error { return errors.New("header: tenant mismatch") }

	cases := []struct {
		name string
		// prepare changes the cache after two commits: A, then B.
		prepare          func(t *testing.T, f *fixture)
		validate         func([]byte) error
		wantPolicy       []byte
		wantFromPrevious bool
		wantCurrentErr   error
		wantErr          []error
	}{
		{
			name:       "current file passes",
			wantPolicy: policyB,
		},
		{
			name: "damaged current file falls back to the previous file",
			prepare: func(t *testing.T, f *fixture) {
				f.writePolicyFile(t, policyB, []byte("damaged"))
			},
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   ErrDigestMismatch,
		},
		{
			name: "missing current file falls back to the previous file",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, os.Remove(filepath.Join(f.dir, fileOf(t, policyB)[0])))
			},
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   os.ErrNotExist,
		},
		{
			name:             "current file that the validator refuses falls back to the previous file",
			validate:         refuseB,
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   ErrRejected,
		},
		{
			name:     "both files fail",
			validate: refuseAll,
			wantErr:  []error{ErrNoCache, ErrRejected},
		},
		{
			name: "no previous file and the current file fails",
			prepare: func(t *testing.T, f *fixture) {
				f.editState(t, func(st *State) { st.PreviousDigest = "" })
			},
			validate: refuseB,
			wantErr:  []error{ErrNoCache, ErrRejected},
		},
		{
			name: "no state",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, os.Remove(filepath.Join(f.dir, stateFileName)))
			},
			wantErr: []error{ErrNoCache, os.ErrNotExist},
		},
		{
			name: "state of a newer format",
			prepare: func(t *testing.T, f *fixture) {
				f.editState(t, func(st *State) { st.FormatVersion = FormatVersion + 1 })
			},
			wantErr: []error{ErrNoCache, ErrUnsupportedFormat},
		},
		{
			name: "state that does not parse",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, os.WriteFile(filepath.Join(f.dir, stateFileName), []byte("{"), fileMode))
			},
			wantErr: []error{ErrNoCache},
		},
		{
			name: "state with a digest outside the layout",
			prepare: func(t *testing.T, f *fixture) {
				f.editState(t, func(st *State) {
					st.Digest = "sha256:../../etc/passwd"
					st.PreviousDigest = ""
				})
			},
			wantErr: []error{ErrNoCache, ErrInvalidDigest},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(tenantA, policyA, 1)
			f.tick()
			f.commit(tenantA, policyB, 2)
			if tc.prepare != nil {
				tc.prepare(t, f)
			}

			loaded, err := f.store.Load(tc.validate)
			if len(tc.wantErr) > 0 {
				require.Error(t, err)
				for _, want := range tc.wantErr {
					assert.ErrorIs(t, err, want)
				}
				assert.Nil(t, loaded)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantPolicy, loaded.Policy)
			assert.Equal(t, Digest(tc.wantPolicy), loaded.Digest)
			assert.Equal(t, tc.wantFromPrevious, loaded.FromPrevious)
			if tc.wantFromPrevious {
				assert.Equal(t, uint64(1), loaded.Revision)
			} else {
				assert.Equal(t, uint64(2), loaded.Revision)
			}
			if tc.wantCurrentErr != nil {
				assert.ErrorIs(t, loaded.CurrentErr, tc.wantCurrentErr)
			} else {
				assert.NoError(t, loaded.CurrentErr)
			}
		})
	}
}

func TestStore_FileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows relies on the ACL of the per-user config folder")
	}
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)

	info, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(dirMode), info.Mode().Perm())

	for _, name := range []string{stateFileName, fileOf(t, policyA)[0]} {
		info, err := os.Stat(filepath.Join(f.dir, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(fileMode), info.Mode().Perm(), name)
	}
}

func TestStore_CommitRemovesTempFiles(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	stale := filepath.Join(f.dir, tempPrefix+"123")
	require.NoError(t, os.WriteFile(stale, []byte("part"), fileMode))
	other := filepath.Join(f.dir, "notes.txt")
	require.NoError(t, os.WriteFile(other, []byte("keep"), fileMode))

	f.tick()
	f.commit(tenantA, policyB, 2)

	assert.NoFileExists(t, stale)
	assert.FileExists(t, other, "the store deletes only files of its layout")
}

// The keys of state.json are a format that the M2 daemon and older tools
// read, so the test pins them.
func TestStore_StateJSONKeys(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	f.tick()
	f.reply(modified(policyB, 2), nil)
	req := request(tenantA)
	req.Validate = func([]byte) error { return errors.New("bad") }
	_, err := f.fetcher.Fetch(t.Context(), req)
	require.Error(t, err)

	data, err := os.ReadFile(filepath.Join(f.dir, stateFileName))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))

	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{
		"format_version", "tenant", "digest", "revision", "previous_digest", "previous_revision",
		"highest_revision", "stored_at", "fetched_at", "last_attempt_at", "last_error", "rejected",
	}, keys)
	assert.ElementsMatch(t, []string{"digest", "revision", "reason", "at"}, keysOf(doc["rejected"]))
	assert.EqualValues(t, FormatVersion, doc["format_version"])
	assert.Equal(t, "2026-10-30T10:00:00Z", doc["stored_at"])
}

func TestStore_ReadPolicyRefusesAnInvalidDigest(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	for _, d := range []string{"", "sha256:", "sha256:../state.json", "md5:abc", Digest(policyA) + "x"} {
		_, err := store.ReadPolicy(d)
		assert.ErrorIs(t, err, ErrInvalidDigest, d)
	}
}

func TestNewStore_RequiresAFolder(t *testing.T) {
	_, err := NewStore("")
	assert.Error(t, err)
}

func (f *fixture) writePolicyFile(t *testing.T, of, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, fileOf(t, of)[0]), content, fileMode))
}

func (f *fixture) editState(t *testing.T, edit func(*State)) {
	t.Helper()
	st := f.state()
	edit(st)
	data, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, stateFileName), data, fileMode))
}

func keysOf(v any) []string {
	m, _ := v.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
