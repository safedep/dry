// Package stream provides streaming data processing primitives.
// The design goal is to standardize the contracts and have different implementations
// leveraging different core infra technologies.
package stream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingTenantID = errors.New("missing tenant ID for multi-tenant stream")
)

const (
	// StreamAccessMinExpiry is the minimum allowed expiry for a scoped access token.
	StreamAccessMinExpiry = 1 * time.Minute

	// StreamAccessMaxExpiry is the maximum allowed expiry for a scoped access token.
	// Caps worst-case blast radius of a leaked token.
	StreamAccessMaxExpiry = 24 * time.Hour

	// StreamAccessDefaultExpiry is applied when StreamAccessRequest.Expiry is zero.
	StreamAccessDefaultExpiry = 1 * time.Hour
)

var (
	ErrExpiryTooShort    = errors.New("stream access expiry below minimum")
	ErrExpiryTooLong     = errors.New("stream access expiry above maximum")
	ErrInvalidAccessRole = errors.New("invalid or missing stream access role")
	ErrInvalidScope      = errors.New("invalid stream scope")
	ErrInvalidAccessID   = errors.New("invalid or missing access ID")
)

// normalizeExpiry enforces library-wide expiry bounds. Zero-valued input
// receives StreamAccessDefaultExpiry; out-of-range input is rejected (not
// silently clamped) so callers notice when they ask for something they
// cannot have.
func normalizeExpiry(d time.Duration) (time.Duration, error) {
	if d == 0 {
		return StreamAccessDefaultExpiry, nil
	}
	if d < StreamAccessMinExpiry {
		return 0, ErrExpiryTooShort
	}
	if d > StreamAccessMaxExpiry {
		return 0, ErrExpiryTooLong
	}
	return d, nil
}

// StreamScope describes a set of streams by prefix. When non-nil on a
// StreamAccessRequest, the issued token covers all streams matched by
// this scope instead of a single Stream. At least one field must be
// non-empty; an all-empty scope matches every stream in the basin and
// is rejected by Validate.
type StreamScope struct {
	TenantID   string // optional
	Namespace  string // optional
	NamePrefix string // optional; partial stream-name match. Requires Namespace to be set, otherwise the resulting prefix would match the namespace segment of stream IDs rather than stream names.
}

// Validate checks that at least one scoping field is populated and that
// NamePrefix is only used together with Namespace. Stream IDs are
// colon-delimited (e.g. "tenant:namespace:name"), so a bare NamePrefix
// without a Namespace anchor would match the wrong segment and could
// silently grant access to unintended streams.
func (s StreamScope) Validate() error {
	if s.TenantID == "" && s.Namespace == "" && s.NamePrefix == "" {
		return ErrInvalidScope
	}
	if s.NamePrefix != "" && s.Namespace == "" {
		return fmt.Errorf("%w: NamePrefix requires Namespace", ErrInvalidScope)
	}
	return nil
}

// Stream is a minimal description of a stream as a first class citizen.
// Implementations can use this to create a stream, read from it, write to it,
type Stream struct {
	// TenantID is the identifier for the tenant that owns the stream.
	// Used only for multi-tenant streams. Keep empty for global streams.
	TenantID string

	// A service specific namespace.
	Namespace string

	// Name of the stream.
	Name string

	// Special flags for declarative configuration of the stream specific
	// to our platform requirements.
	IsMultiTenant bool
}

func (s Stream) WithTenant(tenantId string) Stream {
	return Stream{
		TenantID:      tenantId,
		Namespace:     s.Namespace,
		Name:          s.Name,
		IsMultiTenant: true,
	}
}

// ID returns a unique identifier for the stream based on its properties.
// This is used to connect to the stream and perform operations on it.
// While individual provider may have its naming conventions, we want to
// guarantee isolation for multi-tenant streams
func (s Stream) ID() (string, error) {
	if s.Namespace == "" {
		return "", errors.New("namespace is required for stream ID")
	}

	if s.Name == "" {
		return "", errors.New("name is required for stream ID")
	}

	var parts []string
	if s.IsMultiTenant {
		if s.TenantID == "" {
			return "", ErrMissingTenantID
		}

		parts = append(parts, s.TenantID)
	}

	parts = append(parts, s.Namespace, s.Name)
	return strings.Join(parts, ":"), nil
}

type StreamAccessRole int

const (
	StreamAccessNone StreamAccessRole = iota
	StreamAccessRead
	StreamAccessWrite
	StreamAccessReadWrite
)

func (r StreamAccessRole) String() string {
	switch r {
	case StreamAccessNone:
		return "none"
	case StreamAccessRead:
		return "read"
	case StreamAccessWrite:
		return "write"
	case StreamAccessReadWrite:
		return "readwrite"
	default:
		return "unknown"
	}
}

// StreamAccessRequest describes a scoped, time-limited access token to mint.
// Exactly one of Stream or Scope must be the target of the token. Setting
// both is rejected by Validate to avoid silent precedence behavior.
type StreamAccessRequest struct {
	// Stream is the single-stream target. Mutually exclusive with Scope.
	// Used when Scope is nil and the token should bind to one stream's
	// fully-qualified ID.
	Stream Stream

	// Scope is the multi-stream target. Mutually exclusive with Stream.
	// When non-nil, the token is bound to every stream matched by the
	// scope's prefix.
	Scope *StreamScope

	// Access is the coarse role granted. Providers map this to their native
	// per-operation permissions.
	Access StreamAccessRole

	// Expiry is the lifetime of the minted token. Zero applies
	// StreamAccessDefaultExpiry. Out-of-range values are rejected.
	Expiry time.Duration

	// Other metadata such as TenantID, userID etc. goes here
}

// Validate runs provider-agnostic checks. Providers should call this at the
// top of CreateStreamAccess before any remote call.
func (r StreamAccessRequest) Validate() error {
	switch r.Access {
	case StreamAccessRead, StreamAccessWrite, StreamAccessReadWrite:
		// ok
	default:
		return ErrInvalidAccessRole
	}
	if _, err := normalizeExpiry(r.Expiry); err != nil {
		return err
	}
	streamSet := r.Stream != (Stream{})
	if r.Scope != nil && streamSet {
		return fmt.Errorf("%w: set exactly one of Stream or Scope, not both", ErrInvalidScope)
	}
	if r.Scope != nil {
		return r.Scope.Validate()
	}
	if _, err := r.Stream.ID(); err != nil {
		return fmt.Errorf("invalid stream in access request: %w", err)
	}
	return nil
}

// StreamAccess is the result of minting a scoped access token.
type StreamAccess struct {
	// AccessID is the opaque identifier used to later revoke this token.
	// Depending on the provider it may be client-generated (e.g. S2 accepts
	// a caller-supplied ID) or provider-assigned. Treat it as opaque.
	AccessID string

	// Token is the bearer credential to pass to the data-plane client.
	Token string

	// ExpiresAt is the absolute expiry time of the token. When the provider
	// echoes the expiry in its mint response, that value is used; otherwise
	// it is the value the library asked the provider to set (after expiry
	// normalization).
	ExpiresAt time.Time

	// Other information for stream access goes here.
}

// StreamLifecycle manages stream existence in the provider.
// Implementations should be idempotent.
type StreamLifecycle interface {
	CreateStream(ctx context.Context, stream Stream) error
	DeleteStream(ctx context.Context, stream Stream) error
}

// StreamAccessIssuer mints and revokes scoped, time-limited access tokens.
// The credential used to construct an issuer must have provider-level
// token-issuance privilege and is intentionally distinct from data-plane
// credentials used by writers/readers.
type StreamAccessIssuer interface {
	CreateStreamAccess(ctx context.Context, request StreamAccessRequest) (*StreamAccess, error)
	DeleteStreamAccess(ctx context.Context, accessID string) error
}

// StreamControlPlane is the composition of lifecycle and access-issuer
// for providers that support both. Callers that only need one half
// should depend on the narrower interface.
type StreamControlPlane interface {
	StreamLifecycle
	StreamAccessIssuer
}

// StreamSerializer is the contract for serializing and deserializing records
// This is application specific. The only assumption is, all stream providers
// allow reading and writing byte arrays.
type StreamEntitySerializer[T any] interface {
	// Serialize converts a record of type T into a byte slice.
	Serialize(record T) ([]byte, error)
	// Deserialize converts a byte slice into a record of type T.
	Deserialize(data []byte, record T) error
}

type StreamEntity[T any] struct {
	Record T

	// Various metadata about the record.
	Headers map[string]string
}

type StreamReader[T any] interface {
	Read(ctx context.Context, offset int64, limit int) ([]*StreamEntity[T], error)
}

type StreamListener[T any] interface {
	// Listen for new records in the stream. Limit -1 means no limit.
	Listen(ctx context.Context, offset int64, limit int) (<-chan *StreamEntity[T], error)
}

// StreamWriter is the contract for writing records to a stream.
// Implementations must perform provider specific validation
type StreamWriter[T any] interface {
	AppendOne(ctx context.Context, record *StreamEntity[T]) error
	AppendMany(ctx context.Context, records []*StreamEntity[T]) error
}
