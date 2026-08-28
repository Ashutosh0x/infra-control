package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ashutosh0x/infra-control/pkg/types"
)

// Publisher handles publishing events with standard formatting and correlation.
type Publisher struct {
	bus Bus
}

// NewPublisher creates a new Publisher wrapping an event bus.
func NewPublisher(bus Bus) *Publisher {
	return &Publisher{bus: bus}
}

// Publish publishes an event to the underlying bus, serializing it to JSON.
func (p *Publisher) Publish(ctx context.Context, subject string, event types.Event) error {
	// Inject correlation ID if missing (assume context carries it, or generate)
	// For simplicity, we assume event already has relevant IDs set up by the caller.

	return p.bus.Publish(ctx, subject, event)
}

// SerializeEvent utility for serializing events
func SerializeEvent(event types.Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event: %w", err)
	}
	return data, nil
}
