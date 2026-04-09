# Cloud Endpoint Sync

Reliable event sync from SafeDep endpoint tools (PMG, Gryph, etc.) to SafeDep Cloud. Events are
persisted locally in a SQLite WAL, then delivered in batches via gRPC when the tool explicitly
triggers sync.

## Quick Start

```go
import (
    "context"

    "github.com/safedep/dry/cloud"
    "github.com/safedep/dry/cloud/endpointsync"
)

// 1. Resolve credentials (reads SAFEDEP_API_KEY, SAFEDEP_TENANT_ID)
resolver, err := cloud.NewEnvCredentialResolver()
if err != nil {
    log.Fatal(err)
}
creds, err := resolver.Resolve()
if err != nil {
    // No credentials configured, skip sync
    return
}

// 2. Create cloud connection
cloudClient, err := cloud.NewDataPlaneClient("pmg", creds)
if err != nil {
    log.Fatal(err)
}
defer cloudClient.Close()

// 3. Create sync client
transport := endpointsync.NewGrpcTransport(cloudClient.Connection())
identity := endpointsync.NewEndpointIdentityResolver(
    endpointsync.WithEndpointID("my-machine"),  // optional, falls back to hostname
)

syncClient, err := endpointsync.NewSyncClient("pmg", "1.2.3", transport, identity)
if err != nil {
    log.Fatal(err)
}
defer syncClient.Close()
```

## Emitting Events

`Emit()` writes an event to the local SQLite WAL and returns immediately. No network I/O. The tool
is never blocked by sync.

```go
import (
    controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
    packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// Create an event (pre-fills event_id, tool_name, tool_version, timestamp)
event, err := syncClient.NewEvent()
if err != nil {
    log.Fatal(err)
}

// Set invocation context (same for all events in one command execution)
event.InvocationId = "inv-abc-123"
event.InvocationContext = &controltowerv1.EndpointInvocationContext{
    WorkingDirectory: "/home/dev/my-project",
    Command:          "npm install lodash",
}

// Set the tool-specific payload
event.PmgEvent = &controltowerv1.PmgEvent{
    EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
    SessionSummary: &controltowerv1.PmgSessionSummary{
        PackageManager: controltowerv1.PmgPackageManager_PMG_PACKAGE_MANAGER_NPM,
        FlowType:       controltowerv1.PmgFlowType_PMG_FLOW_TYPE_GUARD,
        TotalAnalyzed:  150,
        AllowedCount:   149,
        BlockedCount:   1,
        Outcome:        controltowerv1.PmgSessionOutcome_PMG_SESSION_OUTCOME_BLOCKED,
    },
}

// Persist to WAL
if err := syncClient.Emit(ctx, event); err != nil {
    if errors.Is(err, endpointsync.ErrWALFull) {
        // WAL is full, sync needs to run. Tool continues normally.
        log.Warn("Sync WAL is full, events will be dropped until sync runs")
    } else {
        log.Errorf("Failed to emit event: %v", err)
    }
}
```

### Emitting Multiple Events

All events from the same tool invocation share an `InvocationId`:

```go
invocationID := uuid.New().String()

// Session summary
summary, _ := syncClient.NewEvent()
summary.InvocationId = invocationID
summary.PmgEvent = &controltowerv1.PmgEvent{
    EventType:      controltowerv1.PmgEventType_PMG_EVENT_TYPE_SESSION_SUMMARY,
    SessionSummary: &controltowerv1.PmgSessionSummary{ /* ... */ },
}
syncClient.Emit(ctx, summary)

// Blocked package decision
decision, _ := syncClient.NewEvent()
decision.InvocationId = invocationID
decision.PmgEvent = &controltowerv1.PmgEvent{
    EventType: controltowerv1.PmgEventType_PMG_EVENT_TYPE_PACKAGE_DECISION,
    PackageDecision: &controltowerv1.PmgPackageDecision{
        PackageVersion: &packagev1.PackageVersion{
            Package: &packagev1.Package{
                Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
                Name:      "evil-package",
            },
            Version: "1.0.0",
        },
        Action:     controltowerv1.PmgPackageAction_PMG_PACKAGE_ACTION_BLOCKED,
        AnalysisId: "analysis-abc-123",
        IsMalware:  true,
        IsVerified: true,
    },
}
syncClient.Emit(ctx, decision)
```

## Syncing Events

`Sync()` delivers pending events from the WAL to SafeDep Cloud. Call it explicitly when the tool is
ready to sync. Events are sent in batches (default: 100 per batch).

```go
// Sync with a timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

synced, err := syncClient.Sync(ctx)
if err != nil {
    log.Warnf("Sync incomplete: %v (synced %d events)", err, synced)
} else {
    log.Infof("Synced %d events", synced)
}
```

## Configuration Options

```go
syncClient, err := endpointsync.NewSyncClient("pmg", "1.2.3", transport, identity,
    // Events per batch (default: 100, max: 100)
    endpointsync.WithBatchSize(50),

    // Max pending events in WAL before Emit() returns ErrWALFull
    // (default: 100,000)
    endpointsync.WithMaxPending(50000),

    // Override WAL path (default: os.UserConfigDir()/safedep/<name>/sync.db)
    endpointsync.WithWALPath("/custom/path/sync.db"),
)
```

## Credential Resolution

The `cloud` package provides credential resolvers:

```go
// From environment variables (SAFEDEP_API_KEY, SAFEDEP_TENANT_ID)
resolver, err := cloud.NewEnvCredentialResolver()

// Chain multiple resolvers (first success wins)
chain := cloud.NewChainCredentialResolver(keychainResolver, envResolver)
creds, err := chain.Resolve()
```

## Endpoint Identity

The identity resolver determines how this endpoint is identified in SafeDep Cloud:

```go
// Operator-provided identifier
identity := endpointsync.NewEndpointIdentityResolver(
    endpointsync.WithEndpointID("bilbo-macbook"),
)

// No identifier -- falls back to hostname
identity := endpointsync.NewEndpointIdentityResolver()
```

The resolver automatically collects:

- OS and architecture (from `runtime.GOOS`, `runtime.GOARCH`)
- Hostname
- Machine ID (HMAC-SHA256 hash of OS-provided hardware UUID, unique per machine)

## Error Handling

| Error | Meaning | Action |
|-------|---------|--------|
| `ErrWALFull` | WAL has reached MaxPending limit | Run sync, then continue emitting |
| `ErrMissingTransport` | No transport provided to NewSyncClient | Programming error, fix caller |
| `ErrMissingIdentity` | No identity resolver provided | Programming error, fix caller |
| `ErrWALOpen` | SQLite database cannot be opened | Check file permissions, disk space |
