package configsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/safedep/dry/log"
)

const (
	stateFileName = "state.json"
	tempPrefix    = ".tmp-"
	policyPrefix  = "policy-"
	policySuffix  = ".pb"

	defaultDirMode  os.FileMode = 0o700
	defaultFileMode os.FileMode = 0o600

	// tempFileMinAge keeps the cleanup away from a temp file that another
	// writer still fills, when two writers run without the lock of the tool.
	tempFileMinAge = time.Minute
)

// Accepted is the result of a Validator that accepts a policy.
type Accepted struct {
	// CapabilitySources maps a capability name to the digest of the policy
	// file that the capability comes from, for each capability that failed
	// in the checked policy. It can be empty. Store.Load ignores it.
	CapabilitySources map[string]string
}

// Validator checks the bytes of a policy. A tool passes its header check and
// its projection, because only the tool decodes the policy. An error rejects
// the policy. The fetcher and Store.Load take the same type, so a tool runs
// its projection once for each check and passes one function to both.
type Validator func(raw []byte) (Accepted, error)

// StoreOption changes a default of a Store.
type StoreOption func(*Store)

// WithDirMode sets the mode of the cache folder when the store creates it.
// The default is 0700, for a cache that one user owns. The SafeDep daemon
// writes as root and the tools read as the user, so it passes 0755.
func WithDirMode(mode os.FileMode) StoreOption {
	return func(s *Store) { s.dirMode = mode.Perm() }
}

// WithFileMode sets the mode of each file that the store writes. The
// default is 0600. The SafeDep daemon passes 0644. Windows ignores it and
// relies on the ACL of the folder.
func WithFileMode(mode os.FileMode) StoreOption {
	return func(s *Store) { s.fileMode = mode.Perm() }
}

// Store owns the layout of one policy cache folder.
type Store struct {
	dir      string
	dirMode  os.FileMode
	fileMode os.FileMode

	// mu serializes the writers of one process. Writers in other processes
	// hold a lock of the tool.
	mu sync.Mutex

	// afterPolicyRename runs between the rename of the policy file and the
	// rename of state.json. A test sets it to stop the commit there, as a
	// crash does.
	afterPolicyRename func()
}

// NewStore returns the store of the cache folder dir. It does not create the
// folder. The first commit creates it, with mode 0700 unless WithDirMode
// sets another mode.
func NewStore(dir string, opts ...StoreOption) (*Store, error) {
	if dir == "" {
		return nil, errors.New("configsync: cache folder is required")
	}
	s := &Store{dir: dir, dirMode: defaultDirMode, fileMode: defaultFileMode}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Dir returns the cache folder.
func (s *Store) Dir() string {
	return s.dir
}

// State reads state.json. A missing, unreadable or newer state returns an
// error that wraps ErrNoCache.
func (s *Store) State() (*State, error) {
	data, err := readCacheFile(s.dir, stateFileName, maxStateSize)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoCache, err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("%w: cannot parse %s: %w", ErrNoCache, stateFileName, err)
	}
	if st.FormatVersion > FormatVersion {
		return nil, fmt.Errorf("%w: %w: format_version %d, this reader knows %d",
			ErrNoCache, ErrUnsupportedFormat, st.FormatVersion, FormatVersion)
	}
	if st.FormatVersion < 1 {
		return nil, fmt.Errorf("%w: %w: format_version %d", ErrNoCache, ErrUnsupportedFormat, st.FormatVersion)
	}
	return &st, nil
}

// ReadPolicy returns the bytes of the policy file of digest. It fails when
// the SHA-256 of the bytes does not equal the digest. A tool reads the file
// that State.CapabilitySources names for a capability with it.
func (s *Store) ReadPolicy(digest string) ([]byte, error) {
	name, err := policyFileName(digest)
	if err != nil {
		return nil, err
	}
	raw, err := readCacheFile(s.dir, name, maxPolicySize)
	if err != nil {
		return nil, fmt.Errorf("configsync: cannot read policy %s: %w", digest, err)
	}
	// The error does not give the digest of the bytes. A tool that runs as
	// root reads the cache of the user, and the digest of a file that the
	// user links in would tell the user about a file that only root reads.
	if Digest(raw) != digest {
		return nil, fmt.Errorf("%w: policy file %s does not match its name", ErrDigestMismatch, name)
	}
	return raw, nil
}

// Loaded is the policy that Store.Load found.
type Loaded struct {
	// State is the state.json that Load read. When Load falls back,
	// State.CapabilitySources describes the current file, not the loaded
	// one.
	State *State

	// Policy holds the bytes of the loaded policy file.
	Policy []byte

	// Digest and Revision name the loaded policy file.
	Digest   string
	Revision uint64

	// FromPrevious is true when the current file failed a check and Load
	// loaded the previous file.
	FromPrevious bool

	// CurrentErr tells why the current file failed. It is nil when Load
	// loaded the current file.
	CurrentErr error
}

// Load returns the current policy of the cache. It checks that the SHA-256
// of the file equals the digest, then calls validate when it is not nil. A
// tool passes the Validator of its fetch, or only its header check. Load
// ignores the Accepted result. When the current file fails a check, Load
// returns the previous file with the same checks and reports the failure in
// Loaded.CurrentErr.
//
// Load returns an error that wraps ErrNoCache when the state is missing,
// unreadable or of a newer format, or when both files fail.
func (s *Store) Load(validate Validator) (*Loaded, error) {
	st, err := s.State()
	if err != nil {
		return nil, err
	}
	if !st.HasPolicy() {
		return nil, fmt.Errorf("%w: the state names no policy", ErrNoCache)
	}

	check := func(digest string) ([]byte, error) {
		raw, err := s.ReadPolicy(digest)
		if err != nil {
			return nil, err
		}
		if validate != nil {
			if _, err := validate(raw); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrRejected, err)
			}
		}
		return raw, nil
	}

	raw, currentErr := check(st.Digest)
	if currentErr == nil {
		return &Loaded{State: st, Policy: raw, Digest: st.Digest, Revision: st.Revision}, nil
	}

	if st.PreviousDigest == "" {
		return nil, fmt.Errorf("%w: current policy: %w", ErrNoCache, currentErr)
	}

	raw, previousErr := check(st.PreviousDigest)
	if previousErr != nil {
		return nil, fmt.Errorf("%w: current policy: %w, previous policy: %w", ErrNoCache, currentErr, previousErr)
	}

	return &Loaded{
		State:        st,
		Policy:       raw,
		Digest:       st.PreviousDigest,
		Revision:     st.PreviousRevision,
		FromPrevious: true,
		CurrentErr:   currentErr,
	}, nil
}

// commit writes raw as the policy file of st.Digest, then st as state.json.
// The rename of state.json is the commit point. After it, commit deletes
// each policy file that st does not name.
func (s *Store) commit(st *State, raw []byte) error {
	name, err := policyFileName(st.Digest)
	if err != nil {
		return err
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	if err := s.writeFileAtomic(name, raw); err != nil {
		return fmt.Errorf("configsync: cannot write policy file: %w", err)
	}

	if s.afterPolicyRename != nil {
		s.afterPolicyRename()
	}

	if err := s.writeState(st); err != nil {
		return err
	}

	s.removeUnnamed(st)
	return nil
}

// writeState replaces state.json in one rename.
func (s *Store) writeState(st *State) error {
	st.FormatVersion = FormatVersion
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("configsync: cannot encode state: %w", err)
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	if err := s.writeFileAtomic(stateFileName, append(data, '\n')); err != nil {
		return fmt.Errorf("configsync: cannot write state: %w", err)
	}
	return nil
}

// ensureDir creates the cache folder. Chmod sets the mode after the create,
// because the umask of the process can remove bits, and a folder of the
// daemon that readers cannot open hides the policy from each tool. Chmod
// never changes a folder that exists.
func (s *Store) ensureDir() error {
	if _, err := os.Stat(s.dir); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("configsync: cannot read cache folder: %w", err)
	}
	if err := os.MkdirAll(s.dir, s.dirMode); err != nil {
		return fmt.Errorf("configsync: cannot create cache folder: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(s.dir, s.dirMode); err != nil {
			return fmt.Errorf("configsync: cannot set mode of cache folder: %w", err)
		}
	}
	return nil
}

// removeUnnamed deletes the policy files that st does not name and the temp
// files of an earlier writer that stopped. A failure here leaves only an
// extra file, so it does not fail the commit. The next commit tries again.
func (s *Store) removeUnnamed(st *State) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		log.Warnf("configsync: cannot list cache folder for cleanup: %v", err)
		return
	}

	keep := map[string]bool{}
	for d := range st.keepDigests() {
		if name, err := policyFileName(d); err == nil {
			keep[name] = true
		}
	}

	now := time.Now()
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || keep[name] {
			continue
		}
		isPolicy := strings.HasPrefix(name, policyPrefix) && strings.HasSuffix(name, policySuffix)
		isTemp := strings.HasPrefix(name, tempPrefix)
		if !isPolicy && !isTemp {
			continue
		}
		if isTemp {
			info, err := e.Info()
			if err != nil || now.Sub(info.ModTime()) < tempFileMinAge {
				continue
			}
		}
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warnf("configsync: cannot remove %s: %v", name, err)
		}
	}
}

// writeFileAtomic writes data to name in the folder through a synced temp
// file and a rename, so a reader sees the old file or the new file and never
// a part.
func (s *Store) writeFileAtomic(name string, data []byte) (err error) {
	tmp, err := os.CreateTemp(s.dir, tempPrefix+"*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	// CreateTemp uses mode 0600. Chmod sets the mode of the store, so the
	// readers of a daemon cache can read it.
	if runtime.GOOS != "windows" {
		if err = tmp.Chmod(s.fileMode); err != nil {
			return err
		}
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = renameReplace(tmpPath, filepath.Join(s.dir, name)); err != nil {
		return err
	}
	syncDir(s.dir)
	return nil
}

// renameReplace renames from to to and replaces to. On Windows a reader that
// holds to open makes the rename fail for a short time, so it tries again.
func renameReplace(from, to string) error {
	err := os.Rename(from, to)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	for i := 1; i <= 5 && err != nil; i++ {
		time.Sleep(time.Duration(i) * 20 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}

// syncDir makes a rename in dir durable. Windows cannot sync a folder. A
// failure does not undo the rename, and Load falls back to the previous file
// when the current file is lost, so a failure is only logged.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		log.Warnf("configsync: cannot open cache folder to sync it: %v", err)
		return
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		log.Warnf("configsync: cannot sync cache folder: %v", err)
	}
}
