package keychain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/safedep/dry/log"
)

const (
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
		configDir, err := localConfigDir()
		if err != nil {
			return nil, fmt.Errorf("keychain: failed to get config directory: %w", err)
		}
		filePath = filepath.Join(configDir, appName, "creds.json")
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
