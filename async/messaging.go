package async

import (
	"context"
	"time"
)

// MessageExtra contains additional metadata for a message
// depending on the messaging service
type MessageExtra struct {
	Subject string
	ReplyTo string
	Headers map[string][]string
}

// Application defined handler function for incoming messages
type MessageHandler func(context.Context, []byte, MessageExtra) error

type ClosableMessagingService interface {
	Close() error
}

// Low level messaging service interface
type MessagingService interface {
	ClosableMessagingService

	Publish(ctx context.Context, topic string, data []byte) error
	QueueSubscribe(ctx context.Context, topic string, queue string, callback MessageHandler) error
}

// Async request response service interface
type AsyncRequestResponseService interface {
	ClosableMessagingService

	Request(ctx context.Context, topic string, data []byte, timeout time.Duration) ([]byte, error)
}

// Async request response RPC client interface
// This is our opinionated way of calling a gRPC service
// over an async channel using conventional topic names
type AsyncRpcClient interface {
	AsyncRequestResponseService

	// Call a remote service method
	Call(ctx context.Context, topic string,
		data []byte, timeout time.Duration) ([]byte, error)
}
