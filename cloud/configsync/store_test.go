package configsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStoreLoad(t *testing.T) {
	refuseB := func(raw []byte) (Accepted, error) {
		if bytes.Equal(raw, policyB) {
			return Accepted{}, errors.New("header: machine id mismatch")
		}
		return Accepted{}, nil
	}

	cases := []struct {
		name string
		// prepare changes the cache after two commits: A, then B.
		prepare          func(t *testing.T, f *fixture)
		validate         Validator
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
			name: "oversized current file falls back to the previous file",
			prepare: func(t *testing.T, f *fixture) {
				f.writePolicyFile(t, policyB, make([]byte, maxPolicySize+1))
			},
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   ErrTooLarge,
		},
		{
			name: "current file that is a folder falls back to the previous file",
			prepare: func(t *testing.T, f *fixture) {
				path := filepath.Join(f.dir, fileOf(t, policyB)[0])
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Mkdir(path, defaultDirMode))
			},
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   ErrUnsafeFile,
		},
		{
			name:             "current file that the validator refuses falls back to the previous file",
			validate:         refuseB,
			wantPolicy:       policyA,
			wantFromPrevious: true,
			wantCurrentErr:   ErrRejected,
		},
		{
			name:     "both files fail the validator",
			validate: refuseWith(errors.New("header: tenant mismatch")),
			wantErr:  []error{ErrNoCache, ErrRejected},
		},
		{
			name: "previous file fails its digest and current file fails the validator",
			prepare: func(t *testing.T, f *fixture) {
				f.writePolicyFile(t, policyA, []byte("damaged"))
			},
			validate: refuseB,
			wantErr:  []error{ErrNoCache, ErrRejected, ErrDigestMismatch},
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
				require.NoError(t, os.WriteFile(filepath.Join(f.dir, stateFileName), []byte("{"), defaultFileMode))
			},
			wantErr: []error{ErrNoCache},
		},
		{
			name: "oversized state",
			prepare: func(t *testing.T, f *fixture) {
				big := append([]byte(`{"format_version": 1, "pad": "`), bytes.Repeat([]byte("x"), maxStateSize)...)
				big = append(big, []byte(`"}`)...)
				require.NoError(t, os.WriteFile(filepath.Join(f.dir, stateFileName), big, defaultFileMode))
			},
			wantErr: []error{ErrNoCache, ErrTooLarge},
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

func TestStore_RefusesSymlinks(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secret, policyB, defaultFileMode))

	cases := []struct {
		name    string
		link    func(t *testing.T, f *fixture) string
		readErr func(f *fixture) error
	}{
		{
			name: "policy file",
			link: func(t *testing.T, f *fixture) string { return filepath.Join(f.dir, fileOf(t, policyB)[0]) },
			readErr: func(f *fixture) error {
				_, err := f.store.ReadPolicy(Digest(policyB))
				return err
			},
		},
		{
			name: "state file",
			link: func(t *testing.T, f *fixture) string { return filepath.Join(f.dir, stateFileName) },
			readErr: func(f *fixture) error {
				_, err := f.store.State()
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(tenantA, policyA, 1)
			f.tick()
			f.commit(tenantA, policyB, 2)

			path := tc.link(t, f)
			require.NoError(t, os.Remove(path))
			if err := os.Symlink(secret, path); err != nil {
				t.Skipf("cannot create a symlink: %v", err)
			}

			assert.ErrorIs(t, tc.readErr(f), ErrUnsafeFile)
		})
	}
}

// The error of a digest mismatch must not give the digest of the bytes. A
// tool that runs as root reads the cache of the user under sudo.
func TestStore_DigestMismatchHidesTheDigestOfTheBytes(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	content := []byte("a file that only root reads")
	f.writePolicyFile(t, policyA, content)

	_, err := f.store.ReadPolicy(Digest(policyA))
	require.ErrorIs(t, err, ErrDigestMismatch)
	assert.NotContains(t, err.Error(), Digest(content))
	assert.NotContains(t, err.Error(), Digest(content)[len(digestPrefix):])

	_, err = f.store.Load(nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), Digest(content)[len(digestPrefix):])
}

func TestStore_FileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows relies on the ACL of the folder")
	}

	cases := []struct {
		name     string
		opts     []StoreOption
		wantDir  os.FileMode
		wantFile os.FileMode
	}{
		{name: "defaults for a cache of one user", wantDir: 0o700, wantFile: 0o600},
		{
			name:     "daemon cache that users read",
			opts:     []StoreOption{WithDirMode(0o755), WithFileMode(0o644)},
			wantDir:  0o755,
			wantFile: 0o644,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cloud-policy")
			store, err := NewStore(dir, tc.opts...)
			require.NoError(t, err)
			transport := NewMockTransport(t)
			fetcher, err := NewFetcher(store, transport)
			require.NoError(t, err)
			for i, raw := range [][]byte{policyA, policyB} {
				transport.EXPECT().GetResolvedPolicy(mock.Anything, mock.Anything).Return(modified(raw, uint64(i+1)), nil).Once()
				_, err := fetcher.Fetch(context.Background(), request(tenantA))
				require.NoError(t, err)
			}

			info, err := os.Stat(dir)
			require.NoError(t, err)
			assert.Equal(t, tc.wantDir, info.Mode().Perm())

			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 3, "state.json and two policy files")
			for _, e := range entries {
				info, err := e.Info()
				require.NoError(t, err)
				assert.Equal(t, tc.wantFile, info.Mode().Perm(), e.Name())
			}
		})
	}
}

func TestStore_CommitRemovesOldTempFiles(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)

	old := filepath.Join(f.dir, tempPrefix+"old")
	require.NoError(t, os.WriteFile(old, []byte("part"), defaultFileMode))
	past := time.Now().Add(-2 * tempFileMinAge)
	require.NoError(t, os.Chtimes(old, past, past))

	young := filepath.Join(f.dir, tempPrefix+"young")
	require.NoError(t, os.WriteFile(young, []byte("part"), defaultFileMode))

	other := filepath.Join(f.dir, "notes.txt")
	require.NoError(t, os.WriteFile(other, []byte("keep"), defaultFileMode))

	f.tick()
	f.commit(tenantA, policyB, 2)

	assert.NoFileExists(t, old)
	assert.FileExists(t, young, "another writer can still fill a young temp file")
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
	req.Validate = withSources(map[string]string{"package_admission": Digest(policyA)})
	_, err := f.fetcher.Fetch(t.Context(), req)
	require.NoError(t, err)

	f.tick()
	f.reply(modified(policyC, 3), nil)
	req.Validate = refuseWith(errors.New("bad"))
	_, err = f.fetcher.Fetch(t.Context(), req)
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
		"capability_sources",
	}, keys)
	assert.ElementsMatch(t, []string{"digest", "revision", "reason", "at"}, keysOf(doc["rejected"]))
	assert.EqualValues(t, FormatVersion, doc["format_version"])
	assert.Equal(t, Digest(policyA), doc["previous_digest"])
	assert.EqualValues(t, 1, doc["previous_revision"])
	assert.Equal(t, map[string]any{"package_admission": Digest(policyA)}, doc["capability_sources"])
	assert.Equal(t, "2026-10-30T10:01:00Z", doc["stored_at"])
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
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, fileOf(t, of)[0]), content, defaultFileMode))
}

func (f *fixture) editState(t *testing.T, edit func(*State)) {
	t.Helper()
	st := f.state()
	edit(st)
	data, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, stateFileName), data, defaultFileMode))
}

func keysOf(v any) []string {
	m, _ := v.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
