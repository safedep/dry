package stream

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type protoBinarySerializer[T proto.Message] struct{}

var _ StreamEntitySerializer[proto.Message] = &protoBinarySerializer[proto.Message]{}

// NewProtoBinarySerializer creates a StreamEntitySerializer that uses binary
// protobuf (proto wire format) to serialize and deserialize stream entities.
//
// This is the wire format for stable SafeDep events: identical bytes across
// transports, compact, and decodable into typed messages in every language.
// Prefer this over the ProtoJSON serializer for event streams; ProtoJSON is a
// debugging/inspection rendering, not the contract.
func NewProtoBinarySerializer[T proto.Message]() (StreamEntitySerializer[T], error) {
	return &protoBinarySerializer[T]{}, nil
}

func (s *protoBinarySerializer[T]) Serialize(record T) ([]byte, error) {
	serialized, err := proto.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize record: %w", err)
	}

	return serialized, nil
}

func (s *protoBinarySerializer[T]) Deserialize(data []byte, record T) error {
	if err := proto.Unmarshal(data, record); err != nil {
		return fmt.Errorf("failed to deserialize record: %w", err)
	}

	return nil
}
