package endpointsync

import (
	"os"
	"path/filepath"
)

// maxBatchSize is the maximum number of events per sync batch, matching the
// proto field constraint of max_items: 100 on SyncEventsRequest.events.
// This limit is enforced by the backend API; do not increase beyond 100.
const maxBatchSize = 100

// SyncOption configures optional sync client behavior.
type SyncOption func(*syncConfig)

type syncConfig struct {
	batchSize  int
	maxPending int
	walPath    string
}

func defaultSyncConfig(name string) *syncConfig {
	return &syncConfig{
		batchSize:  maxBatchSize,
		maxPending: defaultMaxPending,
		walPath:    defaultWALPath(name),
	}
}

// WithBatchSize sets events per batch in Sync(). Default: 100. Maximum: 100
// (the proto enforces max_items: 100 on SyncEventsRequest.events).
func WithBatchSize(n int) SyncOption {
	return func(c *syncConfig) {
		if n > 0 {
			if n > maxBatchSize {
				n = maxBatchSize
			}
			c.batchSize = n
		}
	}
}

// WithMaxPending sets the WAL pending event limit. Default: 100000.
// Emit() returns ErrWALFull when reached.
func WithMaxPending(n int) SyncOption {
	return func(c *syncConfig) {
		if n > 0 {
			c.maxPending = n
		}
	}
}

// WithWALPath overrides the default WAL path.
// Default: os.UserConfigDir()/safedep/<name>/sync.db
func WithWALPath(path string) SyncOption {
	return func(c *syncConfig) {
		if path != "" {
			c.walPath = path
		}
	}
}

func defaultWALPath(name string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "safedep", name, "sync.db")
}
