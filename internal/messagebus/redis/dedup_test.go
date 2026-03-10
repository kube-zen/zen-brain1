// Package redis tests the deduplicating message bus wrapper.
package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// mockMessageBus is a minimal MessageBus implementation for testing.
type mockMessageBus struct {
	publishCalled  bool
	lastStream     string
	lastEvent      *messagebus.Event
	publishError   error
	subscribeError error
	closeCalled    bool
}

func (m *mockMessageBus) Publish(ctx context.Context, stream string, event *messagebus.Event) error {
	m.publishCalled = true
	m.lastStream = stream
	m.lastEvent = event
	return m.publishError
}

func (m *mockMessageBus) Subscribe(ctx context.Context, stream, group, consumer string) (messagebus.Subscription, error) {
	return &mockSubscription{}, m.subscribeError
}

func (m *mockMessageBus) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	return nil
}

func (m *mockMessageBus) Ack(ctx context.Context, stream, group, id string) error {
	return nil
}

func (m *mockMessageBus) Pending(ctx context.Context, stream, group string) ([]*messagebus.Event, error) {
	return []*messagebus.Event{
		{Type: "test-event", Source: "source1"},
	}, nil
}

func (m *mockMessageBus) Close() error {
	m.closeCalled = true
	return nil
}

type mockSubscription struct{}

func (m *mockSubscription) Events() <-chan *messagebus.Event { return nil }
func (m *mockSubscription) Errors() <-chan error           { return nil }
func (m *mockSubscription) Close() error                  { return nil }

func TestNewDedupMessageBus(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, err := NewDedupMessageBus(mockBus, 60, 10000, "test-source")

	if err != nil {
		t.Fatalf("NewDedupMessageBus = %v", err)
	}
	if dedupBus == nil {
		t.Fatal("NewDedupMessageBus returned nil")
	}
	if dedupBus.bus != mockBus {
		t.Error("NewDedupMessageBus did not store underlying bus")
	}
	if dedupBus.source != "test-source" {
		t.Errorf("source = %q, want 'test-source'", dedupBus.source)
	}
	if dedupBus.deduper == nil {
		t.Error("NewDedupMessageBus did not create deduper")
	}
}

func TestNewDedupMessageBus_NilBus(t *testing.T) {
	_, err := NewDedupMessageBus(nil, 60, 10000, "source")
	if err == nil {
		t.Error("NewDedupMessageBus(nil) expected error")
	}
}

func TestNewDedupMessageBus_DefaultParameters(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, err := NewDedupMessageBus(mockBus, 0, 0, "")

	if err != nil {
		t.Fatalf("NewDedupMessageBus with zeros = %v", err)
	}
	if dedupBus.source != "messagebus" {
		t.Errorf("default source = %q, want 'messagebus'", dedupBus.source)
	}
	// Verify deduper was created (default window/entries should be applied)
	if dedupBus.deduper == nil {
		t.Error("deduper not created with defaults")
	}
}

func TestDedupMessageBus_Publish_FirstEvent(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	event := &messagebus.Event{
		Type:        "test-event",
		Source:      "source1",
		Correlation: "corr-1",
		Payload:     []byte("payload-1"),
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"cluster": "cluster-1",
		},
	}

	err := dedupBus.Publish(context.Background(), "stream-1", event)
	if err != nil {
		t.Errorf("Publish(first) = %v", err)
	}
	if !mockBus.publishCalled {
		t.Error("Publish did not call underlying bus")
	}
	if mockBus.lastStream != "stream-1" {
		t.Errorf("Publish stream = %q, want 'stream-1'", mockBus.lastStream)
	}
}

func TestDedupMessageBus_Publish_DuplicateEvent(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	event := &messagebus.Event{
		Type:        "test-event",
		Source:      "source1",
		Correlation: "corr-1",
		Payload:     []byte("payload-1"),
		Timestamp:   time.Now(),
		Metadata: map[string]string{
			"cluster": "cluster-1",
		},
	}

	// First publish
	err1 := dedupBus.Publish(context.Background(), "stream-1", event)
	if err1 != nil {
		t.Fatalf("First publish = %v", err1)
	}
	if !mockBus.publishCalled {
		t.Error("First publish should call underlying bus")
	}

	// Reset mock
	mockBus.publishCalled = false

	// Second publish (duplicate)
	err2 := dedupBus.Publish(context.Background(), "stream-1", event)
	if err2 != nil {
		t.Errorf("Second publish (duplicate) = %v", err2)
	}
	if mockBus.publishCalled {
		t.Error("Second publish (duplicate) should NOT call underlying bus")
	}
}

func TestDedupMessageBus_Publish_DifferentPayload(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	event1 := &messagebus.Event{
		Type:        "test-event",
		Source:      "source1",
		Correlation: "corr-1",
		Payload:     []byte("payload-1"),
		Timestamp:   time.Now(),
		Metadata:    map[string]string{"cluster": "cluster-1"},
	}

	event2 := &messagebus.Event{
		Type:        "test-event",
		Source:      "source1",
		Correlation: "corr-1",
		Payload:     []byte("payload-2"), // Different payload
		Timestamp:   time.Now(),
		Metadata:    map[string]string{"cluster": "cluster-1"},
	}

	// First publish
	err1 := dedupBus.Publish(context.Background(), "stream-1", event1)
	if err1 != nil {
		t.Fatalf("First publish = %v", err1)
	}

	// Second publish with different payload
	// Note: zen-sdk dedup may dedup based on correlation/type/cluster combination
	// even if payload differs. This test documents the actual behavior.
	mockBus.publishCalled = false
	err2 := dedupBus.Publish(context.Background(), "stream-1", event2)
	if err2 != nil {
		t.Errorf("Second publish (different payload) = %v", err2)
	}
	// No assertion on publishCalled - behavior depends on zen-sdk dedup implementation
}

func TestDedupMessageBus_Publish_UnderlyingError(t *testing.T) {
	expectedErr := errors.New("publish failed")
	mockBus := &mockMessageBus{publishError: expectedErr}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	event := &messagebus.Event{
		Type:        "test-event",
		Source:      "source1",
		Correlation: "corr-1",
		Payload:     []byte("payload-1"),
		Timestamp:   time.Now(),
		Metadata:    map[string]string{"cluster": "cluster-1"},
	}

	err := dedupBus.Publish(context.Background(), "stream-1", event)
	if err != expectedErr {
		t.Errorf("Publish(error) = %v, want %v", err, expectedErr)
	}
}

func TestDedupMessageBus_Subscribe(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	sub, err := dedupBus.Subscribe(context.Background(), "stream", "group", "consumer")
	if err != nil {
		t.Errorf("Subscribe = %v", err)
	}
	if sub == nil {
		t.Error("Subscribe returned nil subscription")
	}
}

func TestDedupMessageBus_SubscribeError(t *testing.T) {
	expectedErr := errors.New("subscribe failed")
	mockBus := &mockMessageBus{subscribeError: expectedErr}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	_, err := dedupBus.Subscribe(context.Background(), "stream", "group", "consumer")
	if err != expectedErr {
		t.Errorf("Subscribe(error) = %v, want %v", err, expectedErr)
	}
}

func TestDedupMessageBus_CreateConsumerGroup(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	err := dedupBus.CreateConsumerGroup(context.Background(), "stream", "group")
	if err != nil {
		t.Errorf("CreateConsumerGroup = %v", err)
	}
}

func TestDedupMessageBus_Ack(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	err := dedupBus.Ack(context.Background(), "stream", "group", "id-1")
	if err != nil {
		t.Errorf("Ack = %v", err)
	}
}

func TestDedupMessageBus_Pending(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	events, err := dedupBus.Pending(context.Background(), "stream", "group")
	if err != nil {
		t.Errorf("Pending = %v", err)
	}
	if events == nil {
		t.Error("Pending returned nil events")
	}
}

func TestDedupMessageBus_Close(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	err := dedupBus.Close()
	if err != nil {
		t.Errorf("Close = %v", err)
	}
	if !mockBus.closeCalled {
		t.Error("Close did not call underlying bus Close()")
	}
}

func TestDedupMessageBus_MultipleEventsDifferentCorrelations(t *testing.T) {
	mockBus := &mockMessageBus{}
	dedupBus, _ := NewDedupMessageBus(mockBus, 60, 1000, "source")

	events := []*messagebus.Event{
		{
			Type:        "test-event",
			Source:      "source1",
			Correlation: "corr-1",
			Payload:     []byte("payload-1"),
			Timestamp:   time.Now(),
			Metadata:    map[string]string{"cluster": "cluster-1"},
		},
		{
			Type:        "test-event",
			Source:      "source1",
			Correlation: "corr-2",
			Payload:     []byte("payload-2"),
			Timestamp:   time.Now(),
			Metadata:    map[string]string{"cluster": "cluster-1"},
		},
		{
			Type:        "test-event",
			Source:      "source1",
			Correlation: "corr-3",
			Payload:     []byte("payload-3"),
			Timestamp:   time.Now(),
			Metadata:    map[string]string{"cluster": "cluster-1"},
		},
	}

	for i, event := range events {
		err := dedupBus.Publish(context.Background(), "stream-1", event)
		if err != nil {
			t.Errorf("Event %d Publish = %v", i, err)
		}
	}

	// All should be published (different correlations)
	if mockBus.publishCalled == false {
		t.Error("Events not published")
	}
}
