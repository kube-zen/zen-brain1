# Dead Letter Queue (DLQ)

Provides dead letter queue functionality for failed events with retry support.

## Usage

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/dlq"
    "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Create DLQ manager
logger := logging.NewLogger("my-component")
manager := dlq.NewManager(logger, 10000, dlq.DefaultRetryConfig())

// Add failed event
event := dlq.Event{
    Source:    "my-source",
    Timestamp: time.Now(),
    RawData:   map[string]interface{}{"key": "value"},
}
err := manager.AddFailedEvent(
    ctx,
    "my-source",
    event,
    "destination",
    fmt.Errorf("dispatch failed"),
    "transient",
    "network",
)

// List failed events
events := manager.ListFailedEvents(nil)

// Filter events
filter := &dlq.Filter{
    Source:      "my-source",
    ErrorType:   "transient",
    MinRetryCount: 1,
}
filteredEvents := manager.ListFailedEvents(filter)

// Get single event
event, exists := manager.GetFailedEvent("event-id")

// Replay event (removes from DLQ)
replayedEvent, exists := manager.ReplayFailedEvent("event-id")

// Delete event
deleted := manager.RemoveFailedEvent("event-id")

// Get statistics
stats := manager.GetStats()
```

## HTTP API

The DLQ package includes HTTP API handlers for REST endpoints:

```go
import "github.com/kube-zen/zen-sdk/pkg/dlq"

// Create API
api := dlq.NewAPI(manager, logger)

// Register endpoints
mux.HandleFunc("/api/v1/dlq/events", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        api.HandleListFailedEvents(w, r)
    } else {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})

mux.HandleFunc("/api/v1/dlq/events/", func(w http.ResponseWriter, r *http.Request) {
    // Route based on method and path
    // GET -> HandleGetFailedEvent
    // DELETE -> HandleDeleteFailedEvent
    // POST /replay -> HandleReplayFailedEvent
})

mux.HandleFunc("/api/v1/dlq/stats", api.HandleGetStats)
```

## API Endpoints

- `GET /api/v1/dlq/events` - List all failed events (supports query filters)
- `GET /api/v1/dlq/events/:id` - Get single failed event
- `POST /api/v1/dlq/events/:id/replay` - Replay failed event (removes from DLQ)
- `DELETE /api/v1/dlq/events/:id` - Delete failed event
- `GET /api/v1/dlq/stats` - Get DLQ statistics

## Features

- **Thread-safe**: Safe for concurrent use
- **FIFO eviction**: Oldest events dropped when at capacity
- **Retry support**: Configurable retry logic with exponential backoff
- **Filtering**: Query events by source, destination, error type, category, retry count, time range
- **Statistics**: Get aggregated statistics about DLQ contents

## Implementation

Reused from zen-ingester and zen-egress patterns for consistent DLQ handling across components.

