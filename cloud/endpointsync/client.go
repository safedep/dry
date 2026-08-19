package endpointsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/google/uuid"
	"github.com/safedep/dry/log"
	gobreaker "github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// validToolName matches lowercase alphanumeric strings with hyphens (e.g., "pmg", "my-tool").
// Prevents path traversal when the tool name is used in WAL path.
var validToolName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func isValidToolName(name string) bool {
	return name != "" && validToolName.MatchString(name)
}

// SyncClient handles reliable event sync to SafeDep Cloud.
type SyncClient struct {
	*eventWriter
	transport      EventTransport
	identity       *controltowerv1.EndpointIdentity
	config         *syncConfig
	breaker        *gobreaker.CircuitBreaker[*servicev1.SyncEventsResponse]
	checkInBreaker *gobreaker.CircuitBreaker[*servicev1.CheckInResponse]
}

// EventEmitterClient creates ToolEvents and persists them to the local WAL.
type EventEmitterClient struct {
	*eventWriter
}

type eventWriter struct {
	toolName    string
	toolVersion string
	store       *wal
}

// NewSyncClient creates a new sync client.
//
//   - toolName: tool identifier (e.g., "pmg", "gryph"). Used for WAL path
//     (os.UserConfigDir()/safedep/<toolName>/sync.db) and internal logging/telemetry.
//     Must be lowercase alphanumeric with hyphens only.
//   - toolVersion: tool version (e.g., "1.2.3"). Included in every event for debugging
//     and telemetry. Must not be empty.
//   - transport: pre-configured delivery mechanism for sending events to SafeDep Cloud.
//   - identity: resolves endpoint identity (identifier + metadata) for sync requests.
//   - opts: optional overrides (WithBatchSize, WithMaxPending, WithWALPath).
func NewSyncClient(toolName string, toolVersion string, transport EventTransport, identity EndpointIdentityResolver, opts ...SyncOption) (*SyncClient, error) {
	if transport == nil {
		return nil, ErrMissingTransport
	}
	if identity == nil {
		return nil, ErrMissingIdentity
	}

	endpointIdentity, err := identity.Resolve()
	if err != nil {
		return nil, fmt.Errorf("endpointsync: failed to resolve endpoint identity: %w", err)
	}

	writer, cfg, err := newEventWriter(toolName, toolVersion, opts...)
	if err != nil {
		return nil, err
	}

	breaker := gobreaker.NewCircuitBreaker[*servicev1.SyncEventsResponse](gobreaker.Settings{
		Name:        fmt.Sprintf("endpointsync-%s", toolName),
		MaxRequests: 1,
		Timeout:     5 * time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Infof("Circuit breaker %s: %s -> %s", name, from, to)
		},
	})

	// CheckIn gets its own breaker so a failing check-in path can never
	// block event sync. Unimplemented is not a failure: it means an older
	// server that does not know the RPC yet.
	checkInBreaker := gobreaker.NewCircuitBreaker[*servicev1.CheckInResponse](gobreaker.Settings{
		Name:        fmt.Sprintf("endpointsync-checkin-%s", toolName),
		MaxRequests: 1,
		Timeout:     5 * time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			return err == nil || status.Code(err) == codes.Unimplemented
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Infof("Circuit breaker %s: %s -> %s", name, from, to)
		},
	})

	return &SyncClient{
		eventWriter:    writer,
		transport:      transport,
		identity:       endpointIdentity,
		config:         cfg,
		breaker:        breaker,
		checkInBreaker: checkInBreaker,
	}, nil
}

// NewEventEmitterClient creates a new emit-only client.
//
// The client validates tool metadata, opens the local WAL, and requires no
// transport or endpoint identity. WithBatchSize is accepted because SyncOption
// is shared with SyncClient, but it has no effect on emit-only behavior.
func NewEventEmitterClient(toolName string, toolVersion string, opts ...SyncOption) (*EventEmitterClient, error) {
	writer, _, err := newEventWriter(toolName, toolVersion, opts...)
	if err != nil {
		return nil, err
	}

	return &EventEmitterClient{eventWriter: writer}, nil
}

func newEventWriter(toolName string, toolVersion string, opts ...SyncOption) (*eventWriter, *syncConfig, error) {
	if !isValidToolName(toolName) {
		return nil, nil, fmt.Errorf("endpointsync: invalid tool name %q: must be non-empty, alphanumeric with hyphens only", toolName)
	}
	if toolVersion == "" {
		return nil, nil, fmt.Errorf("endpointsync: tool version is required")
	}

	cfg := defaultSyncConfig(toolName)
	for _, opt := range opts {
		opt(cfg)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.walPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("%w: failed to create WAL directory: %w", ErrWALOpen, err)
	}

	store, err := openWAL(cfg.walPath)
	if err != nil {
		return nil, nil, err
	}
	store.maxPending = cfg.maxPending

	return &eventWriter{
		toolName:    toolName,
		toolVersion: toolVersion,
		store:       store,
	}, cfg, nil
}

// NewEvent creates a ToolEvent with pre-filled fields.
func (w *eventWriter) NewEvent() (*servicev1.ToolEvent, error) {
	return &servicev1.ToolEvent{
		EventId:     uuid.New().String(),
		ToolName:    w.toolName,
		ToolVersion: w.toolVersion,
		Timestamp:   timestamppb.Now(),
	}, nil
}

// Emit persists a ToolEvent to the local WAL.
func (w *eventWriter) Emit(ctx context.Context, event *servicev1.ToolEvent) error {
	if event == nil {
		return fmt.Errorf("endpointsync: event must not be nil")
	}
	if event.GetEventId() == "" {
		return fmt.Errorf("endpointsync: event must have a non-empty event_id")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("endpointsync: emit cancelled: %w", err)
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("endpointsync: failed to marshal event: %w", err)
	}

	return w.store.insert(event.GetEventId(), payload)
}

// Close releases the WAL.
func (w *eventWriter) Close() error {
	return w.store.close()
}

// Sync delivers pending events from the WAL to the server.
func (c *SyncClient) Sync(ctx context.Context) (int, error) {
	totalSynced := 0

	for {
		if err := ctx.Err(); err != nil {
			return totalSynced, fmt.Errorf("endpointsync: sync cancelled: %w", err)
		}

		events, err := c.store.readPending(c.config.batchSize)
		if err != nil {
			return totalSynced, fmt.Errorf("endpointsync: failed to read pending events: %w", err)
		}

		if len(events) == 0 {
			return totalSynced, nil
		}

		toolEvents := make([]*servicev1.ToolEvent, 0, len(events))
		var corruptedIDs []string
		for _, e := range events {
			var te servicev1.ToolEvent
			if err := proto.Unmarshal(e.payload, &te); err != nil {
				log.Warnf("endpointsync: corrupted event %s will be dropped: %v", e.eventID, err)
				corruptedIDs = append(corruptedIDs, e.eventID)
				continue
			}
			toolEvents = append(toolEvents, &te)
		}

		// Mark corrupted events as delivered so they don't block the sync loop.
		// If this fails, return immediately to avoid an infinite loop re-reading
		// the same corrupted events.
		if len(corruptedIDs) > 0 {
			if _, err := c.store.markDelivered(corruptedIDs); err != nil {
				return totalSynced, fmt.Errorf("endpointsync: failed to discard corrupted events (aborting to prevent infinite loop): %w", err)
			}
			if err := c.store.purge(); err != nil {
				log.Errorf("endpointsync: failed to purge corrupted events: %v", err)
			}
		}

		if len(toolEvents) == 0 {
			// All events in this batch were corrupted; continue to drain the WAL.
			continue
		}

		req := &servicev1.SyncEventsRequest{
			Endpoint: c.identity,
			Events:   toolEvents,
		}

		resp, err := c.breaker.Execute(func() (*servicev1.SyncEventsResponse, error) {
			return c.transport.Send(ctx, req)
		})
		if err != nil {
			return totalSynced, fmt.Errorf("endpointsync: sync failed: %w", err)
		}

		confirmedIDs := resp.GetConfirmedEventIds()
		if len(confirmedIDs) > 0 {
			delivered, err := c.store.markDelivered(confirmedIDs)
			if err != nil {
				return totalSynced, fmt.Errorf("endpointsync: failed to mark delivered: %w", err)
			}
			if err := c.store.purge(); err != nil {
				log.Errorf("endpointsync: failed to purge delivered events: %v", err)
			}
			totalSynced += delivered
		}

		// Process failed events. Permanent failures (DUPLICATE, INVALID_PAYLOAD)
		// are marked delivered so they don't loop forever. QUOTA_EXCEEDED events
		// are left pending for the next Sync() call.
		var permanentFailureIDs []string
		failedEvents := resp.GetFailedEvents()
		for _, fe := range failedEvents {
			switch fe.GetErrorCode() {
			case servicev1.EventErrorCode_EVENT_ERROR_CODE_DUPLICATE,
				servicev1.EventErrorCode_EVENT_ERROR_CODE_INVALID_PAYLOAD:
				log.Warnf("endpointsync: event %s permanently failed (%s), dropping: %s",
					fe.GetEventId(), fe.GetErrorCode(), fe.GetMessage())
				permanentFailureIDs = append(permanentFailureIDs, fe.GetEventId())
			case servicev1.EventErrorCode_EVENT_ERROR_CODE_QUOTA_EXCEEDED:
				log.Warnf("endpointsync: event %s quota exceeded, will retry on next Sync()", fe.GetEventId())
			default:
				log.Warnf("endpointsync: event %s failed with unspecified error: %s", fe.GetEventId(), fe.GetMessage())
			}
		}
		if len(permanentFailureIDs) > 0 {
			if _, err := c.store.markDelivered(permanentFailureIDs); err != nil {
				return totalSynced, fmt.Errorf("endpointsync: failed to discard permanently failed events: %w", err)
			}
			if err := c.store.purge(); err != nil {
				log.Errorf("endpointsync: failed to purge permanently failed events: %v", err)
			}
		}

		// If no events were resolved this iteration (all quota-exceeded or
		// unrecognised failures), stop looping to avoid a tight retry spin.
		// QUOTA_EXCEEDED events remain pending for the next Sync() call.
		if len(confirmedIDs) == 0 && len(permanentFailureIDs) == 0 {
			return totalSynced, nil
		}
	}
}

// CheckIn reports endpoint presence to the server without events. The
// caller decides when to call it; this client holds no cooldown state.
// An Unimplemented error means an older server: the caller should treat
// it as a soft failure.
func (c *SyncClient) CheckIn(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("endpointsync: check-in cancelled: %w", err)
	}

	req := &servicev1.CheckInRequest{
		Endpoint:    c.identity,
		ToolName:    c.toolName,
		ToolVersion: c.toolVersion,
	}

	_, err := c.checkInBreaker.Execute(func() (*servicev1.CheckInResponse, error) {
		return c.transport.CheckIn(ctx, req)
	})
	if err != nil {
		return fmt.Errorf("endpointsync: check-in failed: %w", err)
	}

	return nil
}

// Close releases resources.
func (c *SyncClient) Close() error {
	var errs []error
	if err := c.eventWriter.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close WAL: %w", err))
	}
	if c.transport != nil {
		if err := c.transport.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close transport: %w", err))
		}
	}
	return errors.Join(errs...)
}
