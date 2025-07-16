// Package stream provides streaming data processing primitives.
// The design goal is to standardize the contracts and have different implementations
// leveraging different core infra technologies.
package stream

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrMissingTenantID = errors.New("missing tenant ID for multi-tenant stream")
)

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

type StreamAccessRequest struct {
	Stream Stream
	Access StreamAccessRole
	Expiry time.Duration

	// Other metadata such as TenantID, userID etc. goes here
}

type StreamAccess struct {
	AccessID string
	Token    string

	// Other information for stream access goes here.
}

// StreamControlPlane is the contract for providing administrative control
// over the underlying stream processing infrastructure.
type StreamControlPlane interface {
	// Stream management. Should be idempotent.
	CreateStream(ctx context.Context, stream Stream) error
	DeleteStream(ctx context.Context, stream Stream) error

	// Access control management operations.
	CreateStreamAccess(ctx context.Context, request StreamAccessRequest) (*StreamAccess, error)
	DeleteStreamAccess(ctx context.Context, accessID string) error
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
