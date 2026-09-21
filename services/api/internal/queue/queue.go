package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/doc-intel/api/pkg/models"
)

const defaultQueueKey = "doc_intel:jobs"

// Producer pushes jobs onto the Redis queue for workers to consume.
type Producer interface {
	Enqueue(ctx context.Context, payload models.QueuePayload) error
}

// RedisProducer implements Producer using Redis LPUSH.
// Workers consume from the other end with BRPOP — this gives us FIFO ordering.
//
// Why LPUSH/BRPOP instead of a Redis Stream?
// For Phase 1 with a single worker, LPUSH/BRPOP is simpler and sufficient.
// Redis Streams (with consumer groups) become relevant when we need multiple
// workers, at-least-once delivery semantics, or per-message acknowledgement.
// We can migrate without changing the Producer interface.
type RedisProducer struct {
	client   *redis.Client
	queueKey string
}

// NewRedisProducer constructs a RedisProducer.
// It does not verify connectivity — the caller should ping Redis at startup.
func NewRedisProducer(client *redis.Client) *RedisProducer {
	return &RedisProducer{
		client:   client,
		queueKey: defaultQueueKey,
	}
}

// Enqueue JSON-encodes payload and pushes it to the left end of the queue list.
// Workers pop from the right (BRPOP), giving first-in-first-out processing order.
func (p *RedisProducer) Enqueue(ctx context.Context, payload models.QueuePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: marshal payload: %w", err)
	}

	if err := p.client.LPush(ctx, p.queueKey, data).Err(); err != nil {
		return fmt.Errorf("queue: lpush %q: %w", p.queueKey, err)
	}

	return nil
}

// NewRedisClient creates a connected Redis client and pings it.
// Returns an error if Redis is unreachable — call this at startup.
func NewRedisClient(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("queue: parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("queue: ping redis: %w", err)
	}

	return client, nil
}
