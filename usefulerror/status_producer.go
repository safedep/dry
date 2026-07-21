package usefulerror

import (
	errorv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/error/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DefaultErrorDomain is the google.rpc.ErrorInfo.domain used by SafeDep
// producers unless overridden with WithDomain.
const DefaultErrorDomain = "safedep.io"

// StatusBuilder assembles a google.rpc.Status carrying the SafeDep typed error
// model: a canonical gRPC code, a developer-facing message, and optional rich
// details. It is the server-side counterpart to the consumer helpers ReasonOf
// and DetailAs.
//
// Build one with NewStatus, chain the With* methods, and call Err (for the
// error to return from an RPC handler) or Status (for the *status.Status).
// Every method returns the builder so calls can be chained.
type StatusBuilder struct {
	code      codes.Code
	message   string
	reasonSet bool
	reason    ErrorReason
	domain    string
	metadata  map[string]string
	details   []proto.Message
}

// NewStatus starts a status for the given canonical gRPC code and a concise,
// developer-facing message. The message is for developers reading logs; clients
// must branch on the code and details, never on the message text.
func NewStatus(code codes.Code, message string) *StatusBuilder {
	return &StatusBuilder{
		code:    code,
		message: message,
		domain:  DefaultErrorDomain,
	}
}

// WithReason attaches the typed business reason. A single call is responsible
// for the whole reason contract: it emits both the typed
// safedep.messages.error.v1.ErrorDetail and a derived google.rpc.ErrorInfo
// (reason string plus domain) so business-aware and generic clients stay in
// sync. Callers supply only the enum; the ErrorInfo.reason string is derived
// via ReasonWire and must not be set by hand.
func (b *StatusBuilder) WithReason(reason ErrorReason) *StatusBuilder {
	b.reasonSet = true
	b.reason = reason
	return b
}

// WithDomain overrides the ErrorInfo.domain. It only has effect alongside
// WithReason, which is what causes an ErrorInfo to be emitted.
func (b *StatusBuilder) WithDomain(domain string) *StatusBuilder {
	b.domain = domain
	return b
}

// WithMetadata adds one incidental, scalar key/value to ErrorInfo.metadata.
// Use it for context a client may display or log but must not branch on; data a
// client acts on belongs in a typed detail added via WithDetail. Metadata is
// only emitted alongside WithReason.
func (b *StatusBuilder) WithMetadata(key, value string) *StatusBuilder {
	if b.metadata == nil {
		b.metadata = make(map[string]string)
	}
	b.metadata[key] = value
	return b
}

// WithDetail attaches a sibling detail such as a standard google.rpc.* message
// (RetryInfo, QuotaFailure, ...) or a SafeDep-specific typed detail. Details are
// peers in Status.details, never nested inside ErrorDetail.
//
// Each protobuf detail type may appear at most once; adding the same type twice
// keeps the later value. The ErrorInfo and ErrorDetail emitted by WithReason
// take precedence over details of those same types added here.
func (b *StatusBuilder) WithDetail(detail proto.Message) *StatusBuilder {
	if detail == nil {
		return b
	}

	name := detail.ProtoReflect().Descriptor().FullName()
	for i, existing := range b.details {
		if existing.ProtoReflect().Descriptor().FullName() == name {
			b.details[i] = detail
			return b
		}
	}

	b.details = append(b.details, detail)
	return b
}

// Status materializes the builder into a *status.Status. It returns an error
// only if the assembled details cannot be marshalled into the status.
func (b *StatusBuilder) Status() (*status.Status, error) {
	st := status.New(b.code, b.message)

	details := b.assembleDetails()
	if len(details) == 0 {
		return st, nil
	}

	adapted := make([]protoadapt.MessageV1, 0, len(details))
	for _, d := range details {
		adapted = append(adapted, protoadapt.MessageV1Of(d))
	}

	return st.WithDetails(adapted...)
}

// Err returns the status as an error, ready to return from an RPC handler. If
// detail marshalling fails, it falls back to a status carrying just the code
// and message so the canonical failure is never lost.
func (b *StatusBuilder) Err() error {
	st, err := b.Status()
	if err != nil {
		return status.New(b.code, b.message).Err()
	}
	return st.Err()
}

// assembleDetails orders the reason-owned details (ErrorDetail, then ErrorInfo)
// ahead of caller-supplied details and drops any caller detail that collides
// with a reason-owned type, keeping the reason-owned one authoritative.
func (b *StatusBuilder) assembleDetails() []proto.Message {
	var details []proto.Message
	if b.reasonSet {
		details = append(details,
			errorv1.ErrorDetail_builder{Reason: b.reason}.Build(),
			&errdetails.ErrorInfo{
				Reason:   ReasonWire(b.reason),
				Domain:   b.domain,
				Metadata: b.metadata,
			},
		)
	}

	seen := make(map[protoreflect.FullName]struct{}, len(details))
	for _, d := range details {
		seen[d.ProtoReflect().Descriptor().FullName()] = struct{}{}
	}

	for _, d := range b.details {
		if _, exists := seen[d.ProtoReflect().Descriptor().FullName()]; exists {
			continue
		}
		details = append(details, d)
	}

	return details
}
