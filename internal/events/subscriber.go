package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ashutosh0x/infra-control/pkg/types"
)

// Subscriber provides utility methods to handle incoming messages.
type Subscriber struct {
	bus Bus
}

// NewSubscriber creates a new Subscriber.
func NewSubscriber(bus Bus) *Subscriber {
	return &Subscriber{bus: bus}
}

// DeserializeEvent utility for deserializing events
func DeserializeEvent(data []byte) (types.Event, error) {
	var event types.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return types.Event{}, fmt.Errorf("failed to deserialize event: %w", err)
	}
	return event, nil
}

// ProcessWithDLQ is a helper that processes an event and sends failures to a dead letter queue.
func (s *Subscriber) ProcessWithDLQ(ctx context.Context, data []byte, handler func(context.Context, types.Event) error, dlqSubject string) {
	event, err := DeserializeEvent(data)
	if err != nil {
		log.Printf("Failed to deserialize event for DLQ processing: %v", err)
		return
	}

	if err := handler(ctx, event); err != nil {
		log.Printf("Handler failed, sending to DLQ %s: %v", dlqSubject, err)
		// Optionally attach error info to event before sending to DLQ
		_ = s.bus.Publish(ctx, dlqSubject, event)
	}
}
