package events

import (
	"context"

	"github.com/ashutosh0x/infra-control/pkg/types"
)

// Bus defines the event bus interface for publishing and subscribing to events.
type Bus interface {
	// Publish publishes an event to the given subject.
	Publish(ctx context.Context, subject string, event types.Event) error

	// Subscribe subscribes to a subject and invokes the handler for each event.
	Subscribe(ctx context.Context, subject string, handler types.EventHandler) (Subscription, error)

	// SubscribeQueue subscribes to a subject with a queue group for load balancing.
	SubscribeQueue(ctx context.Context, subject string, queue string, handler types.EventHandler) (Subscription, error)

	// Close closes the event bus connection.
	Close() error
}

// Subscription represents an active subscription to an event bus.
type Subscription interface {
	// Unsubscribe removes the subscription.
	Unsubscribe() error
}
