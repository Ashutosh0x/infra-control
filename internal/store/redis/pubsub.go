package redis

import (
	"context"
	"encoding/json"
	"fmt"
)

// Publish sends a message to the specified Redis channel.
func (c *Cache) Publish(ctx context.Context, channel string, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message for channel %s: %w", channel, err)
	}
	if err := c.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}
	return nil
}

// Subscribe listens for messages on the given Redis channel and invokes the handler.
func (c *Cache) Subscribe(ctx context.Context, channel string, handler func(string)) error {
	pubsub := c.client.Subscribe(ctx, channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			handler(msg.Payload)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
