package async

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/log"
)

type NatsMessagingConfig struct {
	NatsURL string

	// Enable NATS JetStream support
	// for consumers
	StreamListener bool

	// The name of the stream to create at NATs
	StreamName string

	// The name of the consumer to create at NATs
	// for consuming from the stream
	StreamListenerName string

	// Maximum number of messages to keep in the stream
	StreamMaxMessages int64

	// Stream listener callback timeout
	StreamListenerCallbackTimeout time.Duration

	// Stream listener acknowledgement wait time
	StreamListenerAckWait time.Duration
}

const (
	natsJetStreamMaxMessages          = 100000
	natsJetStreamMaxMessagesHardLimit = 1000000
	natsJetStreamAckWait              = 30 * time.Second
)

type natsMessaging struct {
	config NatsMessagingConfig
	conn   *nats.Conn
}

func NewNatsMessagingService(config NatsMessagingConfig) (MessagingService, error) {
	if config.NatsURL == "" {
		config.NatsURL = os.Getenv("NATS_URL")

		if config.NatsURL == "" {
			config.NatsURL = nats.DefaultURL
		}
	}

	log.Infof("Connecting to NATS server at %s", config.NatsURL)

	conn, err := nats.Connect(config.NatsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(1*time.Second))

	if err != nil {
		return nil, err
	}

	err = conn.Flush()
	if err != nil {
		return nil, err
	}

	rtt, err := conn.RTT()
	if err != nil {
		return nil, err
	}

	log.Infof("Connected to NATS server at %s. RTT: %v", config.NatsURL, rtt)

	return &natsMessaging{
		config: config,
		conn:   conn,
	}, nil
}

func NewNatsRequestResponseService(config NatsMessagingConfig) (AsyncRequestResponseService, error) {
	messagingService, err := NewNatsMessagingService(config)
	if err != nil {
		return nil, err
	}

	natsMessagingService := messagingService.(*natsMessaging)
	return natsMessagingService, nil
}

func NewNatsRpcClient(config NatsMessagingConfig) (AsyncRpcClient, error) {
	messagingService, err := NewNatsMessagingService(config)
	if err != nil {
		return nil, err
	}

	natsMessagingService := messagingService.(*natsMessaging)
	return natsMessagingService, nil
}

func (n *natsMessaging) Call(ctx context.Context, topic string,
	data []byte, timeout time.Duration) ([]byte, error) {
	return n.Request(ctx, topic, data, timeout)
}

func (n *natsMessaging) Close() error {
	n.conn.Close()
	return nil
}

func (n *natsMessaging) Publish(_ context.Context, topic string, data []byte) error {
	return n.conn.Publish(topic, data)
}

func (n *natsMessaging) QueueSubscribe(topic string, queue string, callback MessageHandler) (MessagingQueueSubscription, error) {
	if n.config.StreamListener {
		return n.queueSubscribeJetStream(topic, queue, callback)
	} else {
		return n.queueSubscribeNats(topic, queue, callback)
	}
}

func (n *natsMessaging) Request(_ context.Context,
	topic string, data []byte, timeout time.Duration) ([]byte, error) {
	res, err := n.conn.Request(topic, data, timeout)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (n *natsMessaging) queueSubscribeNats(topic string, queue string, callback MessageHandler) (MessagingQueueSubscription, error) {
	return n.conn.QueueSubscribe(topic, queue, func(m *nats.Msg) {
		err := callback(context.Background(), m.Data, MessageExtra{
			Subject: m.Subject,
			ReplyTo: m.Reply,
			Headers: m.Header,
		})

		if err != nil {
			log.Errorf("Error processing message by callback handler: %v", err)
		}
	})
}

func (n *natsMessaging) queueSubscribeJetStream(topic string, queue string, callback MessageHandler) (MessagingQueueSubscription, error) {
	if n.config.StreamName == "" {
		return nil, fmt.Errorf("StreamName is required when StreamListener is enabled")
	}

	if n.config.StreamMaxMessages > natsJetStreamMaxMessagesHardLimit {
		return nil, fmt.Errorf("StreamMaxMessages cannot exceed %d", natsJetStreamMaxMessagesHardLimit)
	}

	js, err := jetstream.New(n.conn)
	if err != nil {
		return nil, fmt.Errorf("error creating JetStream context: %v", err)
	}

	ctx := context.Background()

	// We will set limits to ensure we do not end up using unbounded
	// storage for the stream
	config := jetstream.StreamConfig{
		Name:           n.config.StreamName,
		Subjects:       []string{topic},
		Storage:        jetstream.FileStorage,
		Retention:      jetstream.LimitsPolicy,
		ConsumerLimits: jetstream.StreamConsumerLimits{},
		MaxMsgs:        natsJetStreamMaxMessages,
	}

	if n.config.StreamMaxMessages > 0 {
		config.MaxMsgs = n.config.StreamMaxMessages
	}

	stream, err := js.CreateStream(ctx, config)
	if err != nil {
		if err == jetstream.ErrStreamNameAlreadyInUse {
			stream, err = js.Stream(ctx, n.config.StreamName)
			if err != nil {
				return nil, fmt.Errorf("error loading JetStream stream: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error creating JetStream stream: %v", err)
		}
	}

	ackWait := n.config.StreamListenerAckWait
	if ackWait == 0 {
		ackWait = natsJetStreamAckWait
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:    n.config.StreamListenerName,
		Durable: fmt.Sprintf("durable-%s", queue),
		AckWait: ackWait,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating JetStream consumer: %v", err)
	}

	for {
		mc, err := consumer.Fetch(1)
		if err != nil {
			return nil, fmt.Errorf("error reading JetStream message: %v", err)
		}

		for msg := range mc.Messages() {
			var wg sync.WaitGroup

			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()

				var cancel context.CancelFunc
				if n.config.StreamListenerCallbackTimeout > 0 && n.config.StreamListenerCallbackTimeout < ackWait {
					ctx, cancel = context.WithTimeout(ctx, n.config.StreamListenerCallbackTimeout)
					defer cancel()
				} else {
					ctx, cancel = context.WithTimeout(ctx, ackWait)
					defer cancel()
				}

				err = callback(ctx, msg.Data(), MessageExtra{
					Subject: msg.Headers().Get("subject"),
					ReplyTo: msg.Headers().Get("reply"),
					Headers: msg.Headers(),
				})
				if err != nil {
					log.Errorf("Error processing message by callback handler: %v", err)
				}
			}(ctx)

			wg.Wait()

			err := msg.Ack()
			if err != nil {
				log.Errorf("Error acknowledging message: %v", err)
			}
		}
	}
}
