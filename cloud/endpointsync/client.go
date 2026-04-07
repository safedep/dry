package endpointsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/google/uuid"
	"github.com/safedep/dry/log"
	gobreaker "github.com/sony/gobreaker/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SyncClient handles reliable event sync to SafeDep Cloud.
type SyncClient struct {
	name      string
	transport EventTransport
	identity  *controltowerv1.EndpointIdentity
	config    *syncConfig
	store     *wal
	breaker   *gobreaker.CircuitBreaker[*servicev1.SyncEventsResponse]
}

// NewSyncClient creates a new sync client.
func NewSyncClient(name string, transport EventTransport, identity EndpointIdentityResolver, opts ...SyncOption) (*SyncClient, error) {
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

	cfg := defaultSyncConfig(name)
	for _, opt := range opts {
		opt(cfg)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.walPath), 0o755); err != nil {
		return nil, fmt.Errorf("%w: failed to create WAL directory: %v", ErrWALOpen, err)
	}

	store, err := openWAL(cfg.walPath)
	if err != nil {
		return nil, err
	}
	store.maxPending = cfg.maxPending

	breaker := gobreaker.NewCircuitBreaker[*servicev1.SyncEventsResponse](gobreaker.Settings{
		Name:        fmt.Sprintf("endpointsync-%s", name),
		MaxRequests: 1,
		Timeout:     5 * time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Infof("Circuit breaker %s: %s -> %s", name, from, to)
		},
	})

	return &SyncClient{
		name:      name,
		transport: transport,
		identity:  endpointIdentity,
		config:    cfg,
		store:     store,
		breaker:   breaker,
	}, nil
}

// NewEvent creates a ToolEvent with pre-filled fields.
func (c *SyncClient) NewEvent() (*servicev1.ToolEvent, error) {
	return &servicev1.ToolEvent{
		EventId:     uuid.New().String(),
		ToolName:    c.name,
		ToolVersion: c.config.toolVersion,
		Timestamp:   timestamppb.Now(),
	}, nil
}

// Emit persists a ToolEvent to the local WAL.
func (c *SyncClient) Emit(ctx context.Context, event *servicev1.ToolEvent) error {
	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("endpointsync: failed to marshal event: %w", err)
	}

	return c.store.insert(event.GetEventId(), payload)
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
		if len(corruptedIDs) > 0 {
			if err := c.store.markDelivered(corruptedIDs); err != nil {
				log.Errorf("endpointsync: failed to discard corrupted events: %v", err)
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
			if err := c.store.markDelivered(confirmedIDs); err != nil {
				return totalSynced, fmt.Errorf("endpointsync: failed to mark delivered: %w", err)
			}
			if err := c.store.purge(); err != nil {
				log.Errorf("endpointsync: failed to purge delivered events: %v", err)
			}
			totalSynced += len(confirmedIDs)
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
			if err := c.store.markDelivered(permanentFailureIDs); err != nil {
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

// Close releases resources.
func (c *SyncClient) Close() error {
	return c.store.close()
}
