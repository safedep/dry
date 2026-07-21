# API Errors

The `usefulerror` package speaks the SafeDep RPC error model defined in the
`safedep/api` repository. It gives servers a typed way to produce gRPC errors
and gives clients a typed way to read them.

An error travels as a `google.rpc.Status` with three parts:

1. A canonical gRPC code that every client understands.
2. A typed business reason (`safedep.messages.error.v1.ErrorReason`) that tells
   a SafeDep aware client which recovery path to take.
3. Optional detail messages such as `RetryInfo`, `QuotaFailure`, or a custom
   SafeDep message that carry structured data for the recovery.

A client that does not understand the reason or the details falls back to the
canonical code. This keeps old clients and generic clients working.

## Producing an error

Use `NewStatus` to build the status. `WithReason` attaches the typed reason and
also derives the legacy `google.rpc.ErrorInfo` so both new and old clients stay
in sync. You never write the `ErrorInfo.reason` string yourself.

```go
import (
    errorv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/error/v1"
    "github.com/safedep/dry/usefulerror"
    "google.golang.org/grpc/codes"
)

func (s *Server) StartScan(ctx context.Context, req *pb.StartScanRequest) (*pb.StartScanResponse, error) {
    if !s.hasQuota(ctx) {
        return nil, usefulerror.NewStatus(codes.ResourceExhausted, "scan quota exhausted").
            WithReason(errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED).
            WithMetadata("tenantId", tenantID).
            Err()
    }
    // ...
}
```

Rules to keep in mind.

- `WithReason` owns the `ErrorDetail` and the derived `ErrorInfo`. Do not add
  those two detail types yourself.
- `WithMetadata` is for incidental values a client may display or log. Do not
  put values a client must act on in metadata. Use a detail message instead.
- `WithDetail` attaches a sibling detail. Each detail type appears at most once.
- The message is for developers reading logs. Clients must not branch on it.

## Consuming an error

Three helpers read an error and look through wrapping.

```go
reason, ok := usefulerror.ReasonOf(err) // typed business reason
code, ok := usefulerror.GRPCCode(err)   // canonical gRPC code, the fallback
detail, ok := usefulerror.DetailAs[*errdetails.RetryInfo](err) // a typed detail
```

Branch on the reason and always keep a default that falls back to the canonical
code. The enum is open. New reasons can arrive at any time, so an unknown value
must not break the client.

```go
switch reason, _ := usefulerror.ReasonOf(err); reason {
case errorv1.ErrorReason_ERROR_REASON_GITHUB_REAUTHORIZATION_REQUIRED:
    return startReauthFlow()
case errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED:
    return showQuotaMessage()
default:
    code, _ := usefulerror.GRPCCode(err)
    return handleByCode(code)
}
```

## Rendering an error for the CLI

`AsUsefulError` turns an error into a `UsefulError` with a code, a human message,
and help text. When the error carries a typed reason, the presentation comes
from a registered `ReasonPresentation`. When it does not, the generic per code
converters handle it.

```go
if useful, ok := usefulerror.AsUsefulError(err); ok {
    fmt.Println(useful.HumanError())
    fmt.Println(useful.Help())
}
```

Applications can register their own presentation or override a default.

```go
usefulerror.RegisterReason(errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE, usefulerror.ReasonPresentation{
    Code:       usefulerror.ErrBadRequest,
    HumanError: "Project not scannable",
    Help:       "Refresh the project source and retry.",
})
```

## Adding a custom detail message later

The reason enum classifies a failure. When a client needs extra structured data
to recover, add a custom detail message in the `safedep/api` repository and
attach it as a sibling. The `usefulerror` package does not change.

Producer side attaches the new message.

```go
return usefulerror.NewStatus(codes.ResourceExhausted, "scan quota exhausted").
    WithReason(errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED).
    WithDetail(scanv1.OnDemandScanQuotaDetail_builder{
        UsedCredits: 120,
        CreditLimit: 120,
    }.Build()).
    Err()
```

Consumer side reads it with `DetailAs`.

```go
if q, ok := usefulerror.DetailAs[*scanv1.OnDemandScanQuotaDetail](err); ok {
    offerBilling(q.GetUsedCredits(), q.GetCreditLimit())
}
```

If the detail is absent, for example when the server is older, the client falls
back to the reason and then to the canonical code.

## Backward compatibility

- All the typed helpers are additive. Existing callers of `AsUsefulError` and
  `RegisterErrorConverter` keep working.
- `ReasonOf` also recovers the reason from a legacy `ErrorInfo`, so it reads
  errors from older servers.
- The typed reason path only activates when a typed `ErrorDetail` is present.
  Statuses that carry only a code or only an `ErrorInfo` follow the existing
  conversion path unchanged.
