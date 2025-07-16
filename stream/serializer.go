package stream

import (
	"bytes"
	"fmt"

	"github.com/safedep/dry/api/pb"
	"google.golang.org/protobuf/proto"
)

type protoJsonSerializer[T proto.Message] struct{}

var _ StreamEntitySerializer[proto.Message] = &protoJsonSerializer[proto.Message]{}

// NewProtoJsonSerializer creates a new StreamEntitySerializer that serializes
// that use ProtoJSON to serialize and deserialize protocol buffers messages.
func NewProtoJsonSerializer[T proto.Message]() (StreamEntitySerializer[T], error) {
	return &protoJsonSerializer[T]{}, nil
}

func (s *protoJsonSerializer[T]) Serialize(record T) ([]byte, error) {
	serialized, err := pb.ToJson(record, "")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize record: %w", err)
	}

	return serialized, nil
}

func (s *protoJsonSerializer[T]) Deserialize(data []byte, record T) error {
	err := pb.FromJson(bytes.NewReader(data), record)
	if err != nil {
		return fmt.Errorf("failed to deserialize record: %w", err)
	}

	return nil
}
