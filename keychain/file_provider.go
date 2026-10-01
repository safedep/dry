package keychain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/safedep/dry/log"
)

const (
	credsFileName       = "creds.json"
	fileProviderVersion = 1
	dirPermissions      = 0o700
	filePermissions     = 0o600
)

type fileStore struct {
	Version int                `json:"version"`
	Secrets map[string]*Secret `json:"secrets"`
}

type fileProvider struct {
	mu       sync.RWMutex
	filePath string
}

func newFileProvider(appName, filePath string) (*fileProvider, error) {
	if filePath == "" {
		var err error
		filePath, err = defaultFilePath(appName)
		if err != nil {
			return nil, err
		}
	}

	log.Warnf("Using insecure plaintext credential storage at %s", filePath)

	return &fileProvider{
		filePath: filePath,
	}, nil
}

func (f *fileProvider) get(_ context.Context, key string) (*Secret, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	store, err := f.readStore()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	secret, ok := store.Secrets[key]
	if !ok {
		return nil, ErrNotFound
	}

	return &Secret{Value: secret.Value}, nil
}

func (f *fileProvider) set(_ context.Context, key string, secret *Secret) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	store, err := f.readStore()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		store = &fileStore{
			Version: fileProviderVersion,
			Secrets: make(map[string]*Secret),
		}
	}

	store.Secrets[key] = &Secret{Value: secret.Value}
	return f.writeStore(store)
}

func (f *fileProvider) delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	store, err := f.readStore()
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}

	if _, ok := store.Secrets[key]; !ok {
		return ErrNotFound
	}

	delete(store.Secrets, key)
	return f.writeStore(store)
}

func (f *fileProvider) close() error {
	return nil
}

func (f *fileProvider) readStore() (*fileStore, error) {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return nil, err
	}

	var store fileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("keychain: failed to parse credential file: %w", err)
	}

	if store.Secrets == nil {
		store.Secrets = make(map[string]*Secret)
	}

	return &store, nil
}

func (f *fileProvider) writeStore(store *fileStore) error {
	dir := filepath.Dir(f.filePath)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return fmt.Errorf("keychain: failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("keychain: failed to marshal credentials: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".creds-*.tmp")
	if err != nil {
		return fmt.Errorf("keychain: failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("keychain: failed to write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("keychain: failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, filePermissions); err != nil {
		return fmt.Errorf("keychain: failed to set file permissions: %w", err)
	}

	if err := os.Rename(tmpPath, f.filePath); err != nil {
		return fmt.Errorf("keychain: failed to rename credential file: %w", err)
	}

	committed = true
	return nil
}

// linkFile is a variable so that tests can simulate a concurrent move.
var linkFile = os.Link

// localStateDir returns the per-user directory for machine-local state. An
// absolute XDG_STATE_HOME wins on every platform. A relative value is
// ignored, as the XDG specification requires.
func localStateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}
	return platformStateDir()
}

// defaultFilePath returns <state dir>/<appName>/creds.json. A file at the
// path of earlier releases, <config dir>/<appName>/creds.json, moves to it on
// first use. If the move fails, the old path stays in use.
func defaultFilePath(appName string) (string, error) {
	legacyDir, legacyErr := legacyFallbackDir()

	stateDir, err := localStateDir()
	if err != nil {
		// Earlier releases need only the config directory. For example, an
		// absolute XDG_CONFIG_HOME with no HOME resolves it but not the
		// state directory. Such a setup keeps the old path.
		if legacyErr != nil {
			return "", fmt.Errorf("keychain: failed to get state directory: %w", err)
		}
		log.Warnf("keychain: failed to get the state directory, using %s: %v", legacyDir, err)
		return filepath.Join(legacyDir, appName, credsFileName), nil
	}
	path := filepath.Join(stateDir, appName, credsFileName)

	if legacyErr != nil {
		log.Warnf("keychain: failed to resolve the legacy credential directory: %v", legacyErr)
		return path, nil
	}
	legacy := filepath.Join(legacyDir, appName, credsFileName)
	if legacy == path {
		return path, nil
	}

	return moveLegacyFile(legacy, path), nil
}

// moveLegacyFile moves legacy to path when only legacy exists, and returns
// the path to use.
func moveLegacyFile(legacy, path string) string {
	before, err := os.Stat(legacy)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("keychain: failed to check %s: %v", legacy, err)
		}
		return path
	}

	// A run as another user, such as root under sudo with HOME kept, must
	// not move the file. The move would leave a file and directories that
	// the owner cannot read, and lock the owner out.
	if !ownedByCurrentUser(before) {
		return legacy
	}

	if _, err := os.Stat(path); err == nil {
		warnLegacyLeft(legacy)
		return path
	}

	if err := copyNoReplace(legacy, path); err != nil {
		// Another process can create path between the checks and the copy.
		// Its file wins, and this process uses it.
		if _, statErr := os.Stat(path); statErr == nil {
			warnLegacyLeft(legacy)
			return path
		}
		log.Warnf("keychain: failed to move %s to %s, using the old path: %v", legacy, path, err)
		return legacy
	}

	// A writer that still uses the old path, such as an older release, can
	// replace the file during the copy. Its file is then newer than the copy,
	// so it must not be deleted. A window remains between this check and the
	// delete. Closing it needs a lock that older releases do not take.
	if after, err := os.Stat(legacy); err != nil || !sameFileState(before, after) {
		log.Warnf("keychain: %s changed during the move to %s. Both files exist. Keep the one with your credentials and delete the other.", legacy, path)
		return path
	}

	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		log.Warnf("keychain: copied %s to %s, but failed to delete the old file: %v", legacy, path, err)
		return path
	}

	log.Infof("keychain: moved the plaintext credential file from %s to %s", legacy, path)
	return path
}

func warnLegacyLeft(legacy string) {
	log.Warnf("keychain: %s is not in use and holds plaintext secrets. Delete it.", legacy)
}

func sameFileState(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// copyNoReplace copies src to dst and fails if dst exists. os.Rename
// replaces an existing file on Unix, so a move by rename could overwrite a
// file that another process wrote. The copy goes to a temp file next to dst
// first, so that dst never holds a partial file. A hard link then puts it at
// dst, and the link fails if dst exists.
func copyNoReplace(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".creds-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			log.Warnf("keychain: failed to remove %s: %v", tmpPath, err)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, filePermissions); err != nil {
		return err
	}

	return linkFile(tmpPath, dst)
}
