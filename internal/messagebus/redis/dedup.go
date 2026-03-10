// Package redis implements the MessageBus interface using Redis Streams.
// This file provides deduplication wrapper using zen-sdk/pkg/dedup.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/messagebus"
	"github.com/kube-zen/zen-sdk/pkg/dedup"
)

// DedupMessageBus wraps a MessageBus with deduplication.
// It prevents duplicate events from being published within a configurable window.
type DedupMessageBus struct {
	bus     messagebus.MessageBus
	deduper *dedup.Deduper
	source  string
}

// NewDedupMessageBus creates a new deduplicating message bus.
func NewDedupMessageBus(bus messagebus.MessageBus, windowSeconds, maxEntries int, source string) (*DedupMessageBus, error) {
	if bus == nil {
		return nil, fmt.Errorf("bus cannot be nil")
	}
	if windowSeconds <= 0 {
		windowSeconds = 60 // default 60-second deduplication window
	}
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	if source == "" {
		source = "messagebus"
	}

	deduper := dedup.NewDeduper(windowSeconds, maxEntries)

	return &DedupMessageBus{
		bus:     bus,
		deduper: deduper,
		source:  source,
	}, nil
}

// Publish publishes an event if it hasn't been seen within the deduplication window.
func (d *DedupMessageBus) Publish(ctx context.Context, stream string, event *messagebus.Event) error {
	key := dedup.DedupKey{
		Source:      d.source,
		Namespace:   event.Metadata["cluster"],
		Kind:        "messagebus",
		Name:        event.Type,
		Reason:      event.Correlation,
		MessageHash: dedup.HashMessage(string(event.Payload)),
	}

	// Convert event to content map for fingerprinting
	content := map[string]interface{}{
		"type":        event.Type,
		"source":      event.Source,
		"correlation": event.Correlation,
		"payload":     string(event.Payload),
		"timestamp":   event.Timestamp.Format(time.RFC3339Nano),
	}

	if !d.deduper.ShouldCreateWithContent(key, content) {
		// Duplicate detected within window, skip publishing
		return nil
	}

	return d.bus.Publish(ctx, stream, event)
}

// Subscribe delegates to the underlying bus (deduplication is at publish side).
func (d *DedupMessageBus) Subscribe(ctx context.Context, stream, group, consumer string) (messagebus.Subscription, error) {
	return d.bus.Subscribe(ctx, stream, group, consumer)
}

// CreateConsumerGroup delegates to the underlying bus.
func (d *DedupMessageBus) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	return d.bus.CreateConsumerGroup(ctx, stream, group)
}

// Ack delegates to the underlying bus.
func (d *DedupMessageBus) Ack(ctx context.Context, stream, group, id string) error {
	return d.bus.Ack(ctx, stream, group, id)
}

// Pending delegates to the underlying bus.
func (d *DedupMessageBus) Pending(ctx context.Context, stream, group string) ([]*messagebus.Event, error) {
	return d.bus.Pending(ctx, stream, group)
}

// Close closes the underlying bus and stops the deduper.
func (d *DedupMessageBus) Close() error {
	d.deduper.Stop()
	return d.bus.Close()
}
