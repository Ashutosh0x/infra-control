package events

import (
	"context"
	"fmt"
	"log"

	"github.com/ashutosh0x/infra-control/internal/config"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"github.com/nats-io/nats.go"
)

type natsBus struct {
	conn *nats.Conn
}

type natsSubscription struct {
	sub *nats.Subscription
}

// NewNATSBus creates a new NATS-based implementation of the Bus interface.
func NewNATSBus(cfg config.EventsConfig) (Bus, error) {
	opts := []nats.Option{
		nats.Name(cfg.ClientID),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("Disconnected from NATS: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("Reconnected to NATS: %s", nc.ConnectedUrl())
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &natsBus{conn: conn}, nil
}

func (b *natsBus) Publish(ctx context.Context, subject string, event types.Event) error {
	data, err := SerializeEvent(event)
	if err != nil {
		return err
	}
	return b.conn.Publish(subject, data)
}

func (b *natsBus) Subscribe(ctx context.Context, subject string, handler types.EventHandler) (Subscription, error) {
	sub, err := b.conn.Subscribe(subject, func(msg *nats.Msg) {
		event, err := DeserializeEvent(msg.Data)
		if err != nil {
			log.Printf("Error deserializing nats message on %s: %v", subject, err)
			return
		}

		// Typically you'd derive context from msg.Data tracing headers, but using background for simplicity
		if err := handler(context.Background(), event); err != nil {
			log.Printf("Error handling nats message on %s: %v", subject, err)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to NATS subject %s: %w", subject, err)
	}

	return &natsSubscription{sub: sub}, nil
}

func (b *natsBus) SubscribeQueue(ctx context.Context, subject string, queue string, handler types.EventHandler) (Subscription, error) {
	sub, err := b.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		event, err := DeserializeEvent(msg.Data)
		if err != nil {
			log.Printf("Error deserializing nats queue message on %s: %v", subject, err)
			return
		}

		if err := handler(context.Background(), event); err != nil {
			log.Printf("Error handling nats queue message on %s: %v", subject, err)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to queue subscribe to NATS subject %s: %w", subject, err)
	}

	return &natsSubscription{sub: sub}, nil
}

func (b *natsBus) Close() error {
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}

func (s *natsSubscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}
