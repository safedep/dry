package endpointsync

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"google.golang.org/protobuf/proto"
)

// DedupKeyFunc returns the content-key parts for an event. Return ok=false
// when the rule does not apply to the event. Two events dedup together only
// when every part is equal. The framework owns the key encoding, so parts
// may contain any bytes.
type DedupKeyFunc func(*servicev1.ToolEvent) (parts []string, ok bool)

// DedupRule declares which events collapse before cloud sync. The first
// event of a window is persisted for delivery at once. Later events with an
// equal key inside the window are held back and counted. The last held-back
// event delivers the count when the window closes.
type DedupRule struct {
	// Name is the rule's stable identity, used by state rows and logs.
	Name string

	// Key extracts the content-key parts for an event.
	Key DedupKeyFunc

	// Window is the dedup window. The first event of a window opens it.
	Window time.Duration
}

// validateDedupRules keeps configuration errors at client construction,
// never as odd behavior in Emit.
func validateDedupRules(rules []DedupRule) error {
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.Name == "" {
			return fmt.Errorf("endpointsync: dedup rule name is required")
		}
		if _, dup := seen[r.Name]; dup {
			return fmt.Errorf("endpointsync: duplicate dedup rule name %q", r.Name)
		}
		seen[r.Name] = struct{}{}

		if r.Key == nil {
			return fmt.Errorf("endpointsync: dedup rule %q requires a key function", r.Name)
		}
		// The window is stored at millisecond granularity. A smaller
		// window would truncate to zero and expire at once.
		if r.Window < time.Millisecond {
			return fmt.Errorf("endpointsync: dedup rule %q requires a window of at least one millisecond", r.Name)
		}
	}
	return nil
}

// dedupKeyHash derives the storage key as a hash over the rule name and the
// length-prefixed parts. Length prefixes make distinct part lists collide
// never, and the fixed-size hash bounds the state key column.
func dedupKeyHash(rule string, parts []string) []byte {
	h := sha256.New()
	var lenBuf [8]byte

	writePart := func(p string) {
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(p)))
		h.Write(lenBuf[:])
		h.Write([]byte(p))
	}

	writePart(rule)
	for _, p := range parts {
		writePart(p)
	}
	return h.Sum(nil)
}

// carrierWithRepeatCount rewrites a held-back event with the count of raw
// events it stands for. The carrier keeps its own event_id and timestamp,
// so server-side idempotency on event_id holds across retries.
func carrierWithRepeatCount(carrier []byte, suppressed int64) (string, []byte, error) {
	var te servicev1.ToolEvent
	if err := proto.Unmarshal(carrier, &te); err != nil {
		return "", nil, fmt.Errorf("endpointsync: failed to unmarshal carrier event: %w", err)
	}

	te.DedupContext = &servicev1.DedupContext{
		RepeatCount: uint64(suppressed),
	}

	payload, err := proto.Marshal(&te)
	if err != nil {
		return "", nil, fmt.Errorf("endpointsync: failed to marshal carrier event: %w", err)
	}
	return te.GetEventId(), payload, nil
}
