package stream

import (
	"context"
	"fmt"
	"os"

	"github.com/s2-streamstore/s2-sdk-go/s2"
	"google.golang.org/protobuf/proto"
)

const (
	// Default basin for SafeDep on S2
	s2defaultBasin = "safedep-001"

	// https://s2.dev/docs/limits
	s2MaxAppendBatchSize = 1000
)

// S2BasinResolver defines a contract for resolving S2 basin information.
// This is S2 specific and our way of keeping option open for sharding
// across multiple S2 basins.
type S2BasinResolver interface {
	GetBasin(ctx context.Context, serviceId, tenantId string) (string, error)
}

type defaultS2BasinResolver struct{}

var _ S2BasinResolver = &defaultS2BasinResolver{}

// NewDefaultS2BasinResolver creates a new instance of the default S2 basin resolver.
func NewDefaultS2BasinResolver() S2BasinResolver {
	return &defaultS2BasinResolver{}
}

// GetBasin returns the default basin information for the given service and tenant IDs.
// For now, we do not care about the tenantId and serviceId. We will use a single basin.
// But this abstraction allows us to extend functionality in the future if needed such
// as service based sharding.
func (r *defaultS2BasinResolver) GetBasin(_ context.Context, _, _ string) (string, error) {
	basin := os.Getenv("STREAM_PROVIDER_S2_BASIN")
	if basin == "" {
		basin = s2defaultBasin
	}

	return basin, nil
}

type S2StreamProviderConfig struct {
	// ApiKey is the API key used to authenticate with the S2 service.
	ApiKey string

	// Batch size to use for appending records.
	AppendBatchSize uint
}

func DefaultS2StreamProviderConfig() S2StreamProviderConfig {
	return S2StreamProviderConfig{
		ApiKey:          os.Getenv("STREAM_PROVIDER_S2_API_KEY"),
		AppendBatchSize: 100,
	}
}

type s2StreamWriter[T proto.Message] struct {
	streamClient *s2.StreamClient
	serializer   StreamEntitySerializer[T]
	config       S2StreamProviderConfig
}

var _ StreamWriter[proto.Message] = &s2StreamWriter[proto.Message]{}

// NewS2StreamWriter creates a new S2 stream writer. It always appends to the stream.
// Stream and basin must exist in the S2 service.
func NewS2StreamWriter[T proto.Message](config S2StreamProviderConfig,
	basinResolver S2BasinResolver,
	stream Stream, serializer StreamEntitySerializer[T]) (StreamWriter[T], error) {

	if config.ApiKey == "" {
		return nil, fmt.Errorf("S2 API key is not set")
	}

	if config.AppendBatchSize == 0 || config.AppendBatchSize > s2MaxAppendBatchSize {
		return nil, fmt.Errorf("batch size must be between 1 and %d", s2MaxAppendBatchSize)
	}

	basin, err := basinResolver.GetBasin(context.Background(), stream.Namespace, stream.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get basin: %w", err)
	}

	streamId, err := stream.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream ID: %w", err)
	}

	client := s2.New(config.ApiKey, nil)
	streamClient := client.Basin(basin).Stream(s2.StreamName(streamId))

	return &s2StreamWriter[T]{
		streamClient: streamClient,
		serializer:   serializer,
		config:       config,
	}, nil
}

func (s *s2StreamWriter[T]) AppendOne(ctx context.Context, record *StreamEntity[T]) error {
	return s.AppendMany(ctx, []*StreamEntity[T]{record})
}

func (s *s2StreamWriter[T]) AppendMany(ctx context.Context, records []*StreamEntity[T]) error {
	if len(records) == 0 {
		return nil
	}

	if uint(len(records)) > s.config.AppendBatchSize {
		return fmt.Errorf("too many records: %d exceeds batch size %d", len(records), s.config.AppendBatchSize)
	}

	appendRecords := make([]s2.AppendRecord, 0, len(records))
	for _, record := range records {
		recordBytes, err := s.serializer.Serialize(record.Record)
		if err != nil {
			return fmt.Errorf("failed to serialize record: %w", err)
		}

		headers := make([]s2.Header, 0, len(record.Headers))
		for k, v := range record.Headers {
			headers = append(headers, s2.Header{Name: []byte(k), Value: []byte(v)})
		}

		appendRecords = append(appendRecords, s2.AppendRecord{
			Headers: headers,
			Body:    recordBytes,
		})
	}

	if _, err := s.streamClient.Append(ctx, &s2.AppendInput{
		Records: appendRecords,
	}); err != nil {
		return fmt.Errorf("failed to send records: %w", err)
	}

	return nil
}
