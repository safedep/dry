package async

import (
	"context"
	"time"
)

// Application defined handler function for incoming messages
type MessageHandler func(context.Context, []byte) error

type ClosableMessagingService interface {
	Close() error
}

// Interface for a subscribed queue
type MessagingQueueSubscription interface {
	Unsubscribe() error
}

// Low level messaging service interface
type MessagingService interface {
	ClosableMessagingService

	Publish(ctx context.Context, topic string, data []byte) error
	QueueSubscribe(topic string, queue string, callback MessageHandler) (MessagingQueueSubscription, error)
}

// Async request response service interface
type AsyncRequestResponseService interface {
	ClosableMessagingService

	Request(ctx context.Context, topic string, data []byte, timeout time.Duration) ([]byte, error)
}
