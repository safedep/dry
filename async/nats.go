package async

import (
	"context"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/safedep/dry/log"
)

type NatsMessagingConfig struct {
	NatsURL string
}

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

func (n *natsMessaging) Publish(_ context.Context, topic string, data []byte) error {
	return n.conn.Publish(topic, data)
}

func (n *natsMessaging) QueueSubscribe(topic string, queue string, callback MessageHandler) (MessagingQueueSubscription, error) {
	return n.conn.QueueSubscribe(topic, queue, func(m *nats.Msg) {
		err := callback(context.Background(), m.Data)
		if err != nil {
			log.Errorf("Error processing message: %v", err)
		}
	})
}

func (n *natsMessaging) Request(_ context.Context,
	topic string, data []byte, timeout time.Duration) ([]byte, error) {
	res, err := n.conn.Request(topic, data, timeout)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}
