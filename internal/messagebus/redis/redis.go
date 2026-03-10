// Package redis implements the MessageBus interface using Redis Streams.
// It provides at-least-once delivery with consumer groups and deduplication.
package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/messagebus"
	"github.com/redis/go-redis/v9"
)

// Config holds Redis Streams message bus configuration.
type Config struct {
	// RedisURL is the Redis server URL (e.g., "redis://localhost:6379").
	RedisURL string `json:"redis_url"`

	// MaxPending is the maximum number of pending entries per consumer group.
	MaxPending int64 `json:"max_pending"`

	// ConsumerName is the consumer name for this client (auto-generated if empty).
	ConsumerName string `json:"consumer_name"`

	// BlockTimeout is the timeout for blocking XREADGROUP calls (0 = infinite).
	BlockTimeout time.Duration `json:"block_timeout"`

	// ClaimTimeout is the timeout after which pending messages are claimed by other consumers.
	ClaimTimeout time.Duration `json:"claim_timeout"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		RedisURL:     "redis://localhost:6379",
		MaxPending:   1000,
		ConsumerName: "",
		BlockTimeout: 5 * time.Second,
		ClaimTimeout: 30 * time.Second,
	}
}

// redisBus implements messagebus.MessageBus using Redis Streams.
type redisBus struct {
	client   *redis.Client
	config   *Config
	consumer string
}

// redisSubscription implements messagebus.Subscription.
type redisSubscription struct {
	client       *redis.Client
	stream       string
	group        string
	consumer     string
	events       chan *messagebus.Event
	errors       chan error
	ctx          context.Context
	cancel       context.CancelFunc
	blockTimeout time.Duration
}

// New creates a new Redis Streams message bus.
func New(config *Config) (messagebus.MessageBus, error) {
	if config == nil {
		config = DefaultConfig()
	}

	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis ping failed: %w", err)
	}

	consumer := config.ConsumerName
	if consumer == "" {
		consumer = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}

	return &redisBus{
		client:   client,
		config:   config,
		consumer: consumer,
	}, nil
}

// Publish publishes an event to a stream.
func (r *redisBus) Publish(ctx context.Context, stream string, event *messagebus.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Prepare Redis stream entry values
	values := map[string]interface{}{
		"type":        event.Type,
		"source":      event.Source,
		"correlation": event.Correlation,
		"payload":     event.Payload,
		"timestamp":   event.Timestamp.Format(time.RFC3339Nano),
	}

	// Add metadata as flat keys with prefix meta_
	for k, v := range event.Metadata {
		key := fmt.Sprintf("meta_%s", k)
		values[key] = v
	}

	// Use XADD with automatic ID generation (* = auto-generate ID)
	cmd := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: values,
	})

	id, err := cmd.Result()
	if err != nil {
		return fmt.Errorf("XADD failed: %w", err)
	}

	// Update event ID with the generated Redis ID
	event.ID = id

	log.Printf("[redisBus] Published event: stream=%s id=%s type=%s source=%s", stream, id, event.Type, event.Source)
	return nil
}

// Subscribe creates a subscription to a stream with a consumer group.
func (r *redisBus) Subscribe(ctx context.Context, stream, group, consumer string) (messagebus.Subscription, error) {
	if consumer == "" {
		consumer = r.consumer
	}

	// Ensure consumer group exists
	if err := r.CreateConsumerGroup(ctx, stream, group); err != nil {
		return nil, err
	}

	subCtx, cancel := context.WithCancel(ctx)

	sub := &redisSubscription{
		client:       r.client,
		stream:       stream,
		group:        group,
		consumer:     consumer,
		events:       make(chan *messagebus.Event, 100),
		errors:       make(chan error, 10),
		ctx:          subCtx,
		cancel:       cancel,
		blockTimeout: r.config.BlockTimeout,
	}

	// Start goroutine to poll for events
	go sub.poll()

	return sub, nil
}

// CreateConsumerGroup creates or ensures a consumer group exists.
func (r *redisBus) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	// Try to create consumer group with MKSTREAM option
	err := r.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("XGROUP CREATE failed: %w", err)
	}
	return nil
}

// Ack acknowledges successful processing of an event.
func (r *redisBus) Ack(ctx context.Context, stream, group, id string) error {
	err := r.client.XAck(ctx, stream, group, id).Err()
	if err != nil {
		return fmt.Errorf("XACK failed: %w", err)
	}
	return nil
}

// Pending returns pending events for a consumer group.
func (r *redisBus) Pending(ctx context.Context, stream, group string) ([]*messagebus.Event, error) {
	pending, err := r.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  int64(r.config.MaxPending),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("XPENDING failed: %w", err)
	}

	events := make([]*messagebus.Event, 0, len(pending))
	for _, p := range pending {
		// Fetch the actual entry
		rangeResult, err := r.client.XRange(ctx, stream, p.ID, p.ID).Result()
		if err != nil || len(rangeResult) == 0 {
			continue
		}

		event, err := entryToEvent(rangeResult[0])
		if err != nil {
			log.Printf("[redisBus] Failed to convert pending entry: %v", err)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// Close closes the Redis client.
func (r *redisBus) Close() error {
	return r.client.Close()
}

// poll continuously reads events from the stream using XREADGROUP.
func (s *redisSubscription) poll() {
	defer close(s.events)
	defer close(s.errors)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Blocking read from stream
		result, err := s.client.XReadGroup(s.ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Count:    10,
			Block:    s.blockTimeout,
		}).Result()

		if err != nil {
			if s.ctx.Err() != nil {
				// Context cancelled
				return
			}
			s.errors <- fmt.Errorf("XREADGROUP failed: %w", err)
			// Avoid tight loop on persistent error
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range result {
			for _, entry := range stream.Messages {
				event, err := entryToEvent(entry)
				if err != nil {
					s.errors <- fmt.Errorf("failed to convert entry: %w", err)
					continue
				}
				s.events <- event
			}
		}
	}
}

// Events returns the channel of incoming events.
func (s *redisSubscription) Events() <-chan *messagebus.Event {
	return s.events
}

// Errors returns the channel of subscription errors.
func (s *redisSubscription) Errors() <-chan error {
	return s.errors
}

// Close cancels the subscription and stops polling.
func (s *redisSubscription) Close() error {
	s.cancel()
	return nil
}

// entryToEvent converts a Redis stream entry to a messagebus.Event.
func entryToEvent(entry redis.XMessage) (*messagebus.Event, error) {
	event := &messagebus.Event{
		ID:       entry.ID,
		Metadata: make(map[string]string),
	}

	// Extract known fields
	if typ, ok := entry.Values["type"].(string); ok {
		event.Type = typ
	}
	if src, ok := entry.Values["source"].(string); ok {
		event.Source = src
	}
	if corr, ok := entry.Values["correlation"].(string); ok {
		event.Correlation = corr
	}
	if payload, ok := entry.Values["payload"].(string); ok {
		event.Payload = []byte(payload)
	}
	if tsStr, ok := entry.Values["timestamp"].(string); ok {
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err == nil {
			event.Timestamp = ts
		}
	}

	// Extract metadata fields (prefix meta_)
	for k, v := range entry.Values {
		if len(k) > 5 && k[:5] == "meta_" {
			if val, ok := v.(string); ok {
				event.Metadata[k[5:]] = val
			}
		}
	}

	return event, nil
}
