package redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

func TestRedisBus_PublishSubscribe(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Skip test if Redis is not available
	cfg := &Config{
		RedisURL: redisURL,
	}
	bus, err := New(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	stream := "test:stream"
	group := "test-group"
	consumer := "test-consumer"

	// Ensure clean state
	// (Redis streams are append-only; we'll just use a unique stream name)
	stream = fmt.Sprintf("test:stream:%d", time.Now().UnixNano())

	// Create consumer group
	err = bus.CreateConsumerGroup(ctx, stream, group)
	if err != nil {
		t.Fatalf("CreateConsumerGroup failed: %v", err)
	}

	// Subscribe
	sub, err := bus.Subscribe(ctx, stream, group, consumer)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Publish an event
	event := &messagebus.Event{
		Type:        "test.event",
		Source:      "test",
		Correlation: "corr-123",
		Payload:     []byte(`{"key":"value"}`),
		Timestamp:   time.Now(),
		Metadata:    map[string]string{"cluster": "test-cluster"},
	}
	err = bus.Publish(ctx, stream, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for event
	select {
	case received := <-sub.Events():
		if received.Type != event.Type {
			t.Errorf("Expected type %q, got %q", event.Type, received.Type)
		}
		if received.Source != event.Source {
			t.Errorf("Expected source %q, got %q", event.Source, received.Source)
		}
		if string(received.Payload) != string(event.Payload) {
			t.Errorf("Payload mismatch")
		}
		// Ack
		err = bus.Ack(ctx, stream, group, received.ID)
		if err != nil {
			t.Errorf("Ack failed: %v", err)
		}
	case err := <-sub.Errors():
		t.Fatalf("Subscription error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestRedisBus_Dedup(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	cfg := &Config{
		RedisURL: redisURL,
	}
	bus, err := New(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer bus.Close()

	// Wrap with deduplication
	dedupBus, err := NewDedupMessageBus(bus, 60, 1000, "test-dedup")
	if err != nil {
		t.Fatalf("NewDedupMessageBus failed: %v", err)
	}
	defer dedupBus.Close()

	ctx := context.Background()
	stream := fmt.Sprintf("test:dedup:%d", time.Now().UnixNano())

	// Publish same event twice
	event := &messagebus.Event{
		Type:        "dedup.test",
		Source:      "test",
		Correlation: "same",
		Payload:     []byte(`{"id":1}`),
		Timestamp:   time.Now(),
	}

	err = dedupBus.Publish(ctx, stream, event)
	if err != nil {
		t.Fatalf("First publish failed: %v", err)
	}

	// Second publish should be deduplicated (no error, no message)
	err = dedupBus.Publish(ctx, stream, event)
	if err != nil {
		t.Fatalf("Second publish should not error, got: %v", err)
	}

	// Subscribe and verify only one event arrived
	group := "dedup-group"
	consumer := "dedup-consumer"
	err = dedupBus.CreateConsumerGroup(ctx, stream, group)
	if err != nil {
		t.Fatalf("CreateConsumerGroup failed: %v", err)
	}

	sub, err := dedupBus.Subscribe(ctx, stream, group, consumer)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	eventCount := 0
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-sub.Events():
			eventCount++
			if eventCount > 1 {
				t.Fatal("Expected only one event due to deduplication")
			}
		case <-sub.Errors():
			t.Fatal("Subscription error")
		case <-timeout:
			goto done
		}
	}
done:
	if eventCount != 1 {
		t.Errorf("Expected exactly 1 event, got %d", eventCount)
	}
}
