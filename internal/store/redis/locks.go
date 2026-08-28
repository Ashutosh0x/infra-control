package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Lock represents a distributed lock.
type Lock struct {
	client *redis.Client
	key    string
	value  string
}

// AcquireLock attempts to acquire a distributed lock.
func (c *Cache) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	val := uuid.NewString()
	success, err := c.client.SetNX(ctx, key, val, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for key %s: %w", key, err)
	}
	if !success {
		return nil, fmt.Errorf("lock for key %s is already held", key)
	}
	return &Lock{
		client: c.client,
		key:    key,
		value:  val,
	}, nil
}

// Release releases the distributed lock.
func (l *Lock) Release(ctx context.Context) error {
	script := `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock for key %s: %w", l.key, err)
	}
	if res.(int64) == 0 {
		return fmt.Errorf("lock for key %s was not held by this instance", l.key)
	}
	return nil
}

// Extend extends the TTL of the distributed lock.
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	script := `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("pexpire", KEYS[1], ARGV[2])
else
    return 0
end`
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.value, ttl.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock for key %s: %w", l.key, err)
	}
	if res.(int64) == 0 {
		return fmt.Errorf("lock for key %s was not held by this instance", l.key)
	}
	return nil
}
