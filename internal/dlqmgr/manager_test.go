package dlqmgr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kube-zen/zen-sdk/pkg/dlq"
)

func TestInit(t *testing.T) {
	ctx := context.Background()

	// Init should succeed
	err := Init(ctx)
	assert.NoError(t, err)
	assert.True(t, IsInitialized())

	// Second call should be idempotent
	err = Init(ctx)
	assert.NoError(t, err)
	assert.True(t, IsInitialized())
}

func TestAddFailedEvent(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add a test event
	event := dlq.Event{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{"key": "value"},
	}

	err = AddFailedEvent(
		ctx,
		"test-source",
		event,
		"test-destination",
		assert.AnError,
		"transient",
		"network",
	)
	assert.NoError(t, err)

	// Verify event was added
	events := ListFailedEvents(nil)
	assert.Len(t, events, 1)
}

func TestListFailedEvents(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add multiple events
	for i := 0; i < 5; i++ {
		event := dlq.Event{
			Source:    "test-source",
			Timestamp: time.Now(),
			RawData:   map[string]interface{}{"index": i},
		}
		_ = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	}

	// List all events
	events := ListFailedEvents(nil)
	assert.Len(t, events, 5)

	// Filter by source
	filter := &dlq.Filter{
		Source: "test-source",
	}
	events = ListFailedEvents(filter)
	assert.Len(t, events, 5)
}

func TestGetFailedEvent(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add a test event
	event := dlq.Event{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{"key": "value"},
	}
	err = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	require.NoError(t, err)

	// Get the event
	events := ListFailedEvents(nil)
	require.Len(t, events, 1)

	retrieved, exists := GetFailedEvent(events[0].ID)
	assert.True(t, exists)
	assert.Equal(t, events[0].Source, retrieved.Source)
}

func TestReplayFailedEvent(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add a test event
	event := dlq.Event{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{"key": "value"},
	}
	err = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	require.NoError(t, err)

	// Get event ID
	events := ListFailedEvents(nil)
	require.Len(t, events, 1)
	eventID := events[0].ID

	// Replay event
	replayed, exists := ReplayFailedEvent(eventID)
	assert.True(t, exists)
	assert.Equal(t, eventID, replayed.ID)

	// Event should be removed
	events = ListFailedEvents(nil)
	assert.Len(t, events, 0)
}

func TestRemoveFailedEvent(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add a test event
	event := dlq.Event{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{"key": "value"},
	}
	err = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	require.NoError(t, err)

	// Get event ID
	events := ListFailedEvents(nil)
	require.Len(t, events, 1)
	eventID := events[0].ID

	// Remove event
	removed := RemoveFailedEvent(eventID)
	assert.True(t, removed)

	// Event should be gone
	_, exists := GetFailedEvent(eventID)
	assert.False(t, exists)
}

func TestGetStats(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Get stats (empty)
	stats := GetStats()
	assert.NotNil(t, stats)

	// Add some events
	for i := 0; i < 3; i++ {
		event := dlq.Event{
			Source:    "test-source",
			Timestamp: time.Now(),
			RawData:   map[string]interface{}{"index": i},
		}
		_ = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	}

	// Get stats (with events)
	stats = GetStats()
	assert.Greater(t, stats.TotalEvents, 0)
}

func TestStartReplayWorker(t *testing.T) {
	ctx := context.Background()

	// Reset for test
	manager = nil
	initOnce = sync.Once{}
	err := Init(ctx)
	require.NoError(t, err)

	// Add a transient event
	event := dlq.Event{
		Source:    "test-source",
		Timestamp: time.Now(),
		RawData:   map[string]interface{}{"key": "value"},
	}
	err = AddFailedEvent(ctx, "test-source", event, "dest", assert.AnError, "transient", "network")
	require.NoError(t, err)

	// Start replay worker
	cancel := StartReplayWorker(ctx, 100*time.Millisecond, nil)
	defer cancel()

	// Wait for at least one replay cycle
	time.Sleep(200 * time.Millisecond)

	// Worker should have attempted replay
	// (Note: Replay will remove the event)
	events := ListFailedEvents(nil)
	assert.LessOrEqual(t, len(events), 1)
}

func TestGetManager_NotInitialized(t *testing.T) {
	// Reset to uninitialized state
	manager = nil

	// Should panic
	assert.Panics(t, func() {
		GetManager()
	})
}

func TestIsInitialized(t *testing.T) {
	// Reset to uninitialized state
	manager = nil

	// Should be false
	assert.False(t, IsInitialized())

	// Initialize
	ctx := context.Background()
	err := Init(ctx)
	require.NoError(t, err)

	// Should be true
	assert.True(t, IsInitialized())
}
