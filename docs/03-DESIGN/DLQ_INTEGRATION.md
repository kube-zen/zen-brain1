# DLQ (Dead Letter Queue) Integration Plan

## Overview

zen-brain1 uses DLQ (Dead Letter Queue) via zen-sdk/pkg/dlq to capture and retry failed events:
- Failed Redis message bus operations
- Failed task dispatch operations
- Failed HTTP requests to external services
- Failed office connector operations

## Target Components

### 1. Message Bus (`internal/messagebus/redis/`)
- Capture failed message publishing
- Capture failed message delivery
- Retry failed messages

### 2. QMD Orchestrator (`internal/qmd/`)
- Capture failed task dispatch
- Capture failed worker assignment
- Retry failed tasks

### 3. Office Connectors (`internal/office/`)
- Capture failed Jira API calls
- Capture failed webhook deliveries
- Retry failed operations

### 4. API Server (`internal/apiserver/`)
- Capture failed HTTP requests
- Expose DLQ management API endpoints
- Provide DLQ statistics

---

## Implementation Pattern

### Step 1: Create DLQ Manager Helper

Create `internal/dlq/manager.go`:

```go
package dlq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/dlq"
	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"
)

var (
	// Global DLQ manager instance
	manager *dlq.Manager
	// Initialization state
	initOnce sync.Once
	// Logger
	logger *zenlog.Logger
)

// Init initializes the DLQ manager.
// Safe to call multiple times.
func Init(ctx context.Context) error {
	var initErr error
	initOnce.Do(func() {
		logger = zenlog.NewLogger("zen-brain.dlq")

		// Determine capacity from environment
		capacity := 10000
		if c := os.Getenv("DLQ_CAPACITY"); c != "" {
			if n, err := fmt.Sscanf(c, "%d", &capacity); err == nil && n == 1 {
				logger.Info("DLQ capacity set from environment",
					zenlog.Int("capacity", capacity),
				)
			}
		}

		// Create DLQ manager with default retry config
		manager = dlq.NewManager(logger, capacity, dlq.DefaultRetryConfig())

		logger.Info("DLQ manager initialized",
			zenlog.Int("capacity", capacity),
			zenlog.String("retry_config", "default"),
		)
	})
	return initErr
}

// GetManager returns the DLQ manager instance.
// Must call Init() first.
func GetManager() *dlq.Manager {
	if manager == nil {
		panic("DLQ manager not initialized. Call dlq.Init() first.")
	}
	return manager
}

// AddFailedEvent adds a failed event to the DLQ.
func AddFailedEvent(
	ctx context.Context,
	source string,
	event dlq.Event,
	destination string,
	err error,
	errorType string,
	category string,
) error {
	if manager == nil {
		// Log warning but don't fail
		zenlog.NewLogger("zen-brain.dlq").Warn("DLQ manager not initialized, event not added to DLQ",
			zenlog.String("source", source),
			zenlog.String("destination", destination),
			zenlog.Error(err),
		)
		return nil
	}

	addErr := manager.AddFailedEvent(ctx, source, event, destination, err, errorType, category)
	if addErr != nil {
		logger.Error(addErr, "Failed to add event to DLQ",
			zenlog.String("source", source),
			zenlog.String("destination", destination),
		)
		return addErr
	}

	logger.Warn("Event added to DLQ",
		zenlog.String("source", source),
		zenlog.String("destination", destination),
		zenlog.String("error_type", errorType),
		zenlog.String("category", category),
		zenlog.Error(err),
	)

	return nil
}

// ListFailedEvents lists failed events from the DLQ.
func ListFailedEvents(filter *dlq.Filter) []dlq.Event {
	if manager == nil {
		return nil
	}
	return manager.ListFailedEvents(filter)
}

// GetFailedEvent gets a single failed event by ID.
func GetFailedEvent(id string) (dlq.Event, bool) {
	if manager == nil {
		return dlq.Event{}, false
	}
	return manager.GetFailedEvent(id)
}

// ReplayFailedEvent replays a failed event (removes from DLQ).
func ReplayFailedEvent(id string) (dlq.Event, bool) {
	if manager == nil {
		return dlq.Event{}, false
	}
	event, exists := manager.ReplayFailedEvent(id)
	if exists {
		logger.Info("Event replayed from DLQ",
			zenlog.String("event_id", id),
		)
	}
	return event, exists
}

// RemoveFailedEvent removes a failed event from the DLQ.
func RemoveFailedEvent(id string) bool {
	if manager == nil {
		return false
	}
	removed := manager.RemoveFailedEvent(id)
	if removed {
		logger.Info("Event removed from DLQ",
			zenlog.String("event_id", id),
		)
	}
	return removed
}

// GetStats returns DLQ statistics.
func GetStats() dlq.Stats {
	if manager == nil {
		return dlq.Stats{}
	}
	return manager.GetStats()
}

// StartReplayWorker starts a background worker that periodically replays failed events.
func StartReplayWorker(ctx context.Context, interval time.Duration, filter *dlq.Filter) {
	if manager == nil {
		return
	}

	logger.Info("Starting DLQ replay worker",
		zenlog.String("interval", interval.String()),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("DLQ replay worker stopped")
			return
		case <-ticker.C:
			replayFailedEvents(ctx, filter)
		}
	}
}

// replayFailedEvents replays all failed events matching the filter.
func replayFailedEvents(ctx context.Context, filter *dlq.Filter) {
	events := manager.ListFailedEvents(filter)
	if len(events) == 0 {
		return
	}

	logger.Info("Replaying failed events",
		zenlog.Int("count", len(events)),
	)

	successCount := 0
	failureCount := 0

	for _, event := range events {
		// Check if event is retryable
		if event.ErrorType != "transient" {
			logger.Debug("Skipping non-retryable event",
				zenlog.String("event_id", event.ID),
				zenlog.String("error_type", event.ErrorType),
			)
			continue
		}

		// Check retry limit
		if event.RetryCount >= 10 {
			logger.Warn("Event exceeded retry limit, skipping",
				zenlog.String("event_id", event.ID),
				zenlog.Int("retry_count", event.RetryCount),
			)
			continue
		}

		// Wait before retry (exponential backoff)
		backoff := time.Duration(event.RetryCount) * time.Second * 10
		time.Sleep(backoff)

		// Replay event
		_, exists := manager.ReplayFailedEvent(event.ID)
		if exists {
			successCount++
		} else {
			failureCount++
		}
	}

	logger.Info("Replay worker completed",
		zenlog.Int("success", successCount),
		zenlog.Int("failure", failureCount),
	)
}
```

---

### Step 2: Integrate into Message Bus

Modify `internal/messagebus/redis/publisher.go`:

```go
func (p *Publisher) Publish(ctx context.Context, topic string, message []byte) error {
	err := p.client.Publish(ctx, topic, message)
	if err != nil {
		// Add to DLQ
		event := dlq.Event{
			Source:    "redis-publisher",
			Timestamp:  time.Now(),
			RawData:    map[string]interface{}{
				"topic":   topic,
				"message": base64.StdEncoding.EncodeToString(message),
			},
		}

		// Categorize error
		errorType := "transient"
		if isNetworkError(err) {
			errorType = "transient"
		} else if isAuthError(err) {
			errorType = "permanent"
		}

		_ = dlq.AddFailedEvent(ctx, "redis-publisher", event, topic, err, errorType, "network")
		return err
	}
	return nil
}
```

---

### Step 3: Integrate into QMD Orchestrator

Modify `internal/qmd/orchestrator.go`:

```go
func (o *Orchestrator) DispatchTask(ctx context.Context, task *Task) error {
	err := o.scheduler.Enqueue(ctx, task)
	if err != nil {
		// Add to DLQ
		event := dlq.Event{
			Source:   "qmd-orchestrator",
			Timestamp: time.Now(),
			RawData:   map[string]interface{}{
				"task_id": task.ID,
				"type":    task.Type,
				"payload": task.Payload,
			},
		}

		_ = dlq.AddFailedEvent(ctx, "qmd-orchestrator", event, "worker-pool", err, "transient", "scheduling")
		return err
	}
	return nil
}
```

---

### Step 4: Integrate into API Server

Modify `internal/apiserver/server.go` to add DLQ endpoints:

```go
func (s *Server) setupDLQHandlers() {
	s.mux.HandleFunc("/api/v1/dlq/events", s.handleListDLQEvents)
	s.mux.HandleFunc("/api/v1/dlq/events/", s.handleDLQEventDetail)
	s.mux.HandleFunc("/api/v1/dlq/stats", s.handleDLQStats)
}

func (s *Server) handleListDLQEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	filter := &dlq.Filter{
		Source:      r.URL.Query().Get("source"),
		Destination: r.URL.Query().Get("destination"),
		ErrorType:   r.URL.Query().Get("error_type"),
	}

	events := dlq.ListFailedEvents(filter)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

func (s *Server) handleDLQEventDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/dlq/events/")

	switch r.Method {
	case http.MethodGet:
		// Get single event
		event, exists := dlq.GetFailedEvent(id)
		if !exists {
			http.Error(w, "Event not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(event)

	case http.MethodDelete:
		// Delete event
		removed := dlq.RemoveFailedEvent(id)
		if !removed {
			http.Error(w, "Event not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPost:
		// Replay event
		if strings.HasSuffix(r.URL.Path, "/replay") {
			event, exists := dlq.ReplayFailedEvent(id)
			if !exists {
				http.Error(w, "Event not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(event)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDLQStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := dlq.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
```

---

### Step 5: Documentation

Create `docs/03-DESIGN/DLQ_INTEGRATION.md`:

```markdown
# DLQ (Dead Letter Queue) Integration

## Overview

zen-brain1 uses DLQ (Dead Letter Queue) via zen-sdk/pkg/dlq to capture and retry failed events:
- Failed Redis message bus operations
- Failed task dispatch operations
- Failed office connector operations
- Failed HTTP requests

## Features

- ✅ Automatic capture of failed events
- ✅ Configurable retry logic with exponential backoff
- ✅ Filtering by source, destination, error type
- ✅ HTTP API for DLQ management
- ✅ Background replay worker
- ✅ Statistics and monitoring

## HTTP API

### List Failed Events

```bash
curl http://localhost:8080/api/v1/dlq/events
```

Query parameters:
- `source` - Filter by event source
- `destination` - Filter by destination
- `error_type` - Filter by error type (transient/permanent)

### Get Single Event

```bash
curl http://localhost:8080/api/v1/dlq/events/{event-id}
```

### Replay Event

```bash
curl -X POST http://localhost:8080/api/v1/dlq/events/{event-id}/replay
```

### Delete Event

```bash
curl -X DELETE http://localhost:8080/api/v1/dlq/events/{event-id}
```

### Get DLQ Statistics

```bash
curl http://localhost:8080/api/v1/dlq/stats
```

Response:
```json
{
  "total_events": 42,
  "by_source": {
    "redis-publisher": 20,
    "qmd-orchestrator": 15,
    "jira-connector": 7
  },
  "by_error_type": {
    "transient": 38,
    "permanent": 4
  },
  "oldest_event": "2024-03-01T10:00:00Z",
  "newest_event": "2024-03-12T07:00:00Z"
}
```

## Environment Variables

- `DLQ_CAPACITY` - Maximum number of events in DLQ (default: 10000)
- `DLQ_REPLAY_INTERVAL` - Replay worker interval (default: 5m)
- `DLQ_MAX_RETRIES` - Maximum retry attempts (default: 10)

## Error Categorization

### Transient Errors (retryable)
- Network timeouts
- Connection failures
- Rate limit exceeded
- Service unavailable

### Permanent Errors (not retryable)
- Invalid credentials
- Bad request
- Resource not found
- Permission denied

## Usage Example

```go
import "github.com/kube-zen/zen-brain1/internal/dlq"

// Initialize DLQ
err := dlq.Init(ctx)

// Add failed event
event := dlq.Event{
    Source:    "my-component",
    Timestamp: time.Now(),
    RawData:   map[string]interface{}{"key": "value"},
}
err = dlq.AddFailedEvent(ctx, "my-component", event, "destination", err, "transient", "network")

// Start replay worker
go dlq.StartReplayWorker(ctx, 5*time.Minute, nil)
```

## Monitoring

### Key Metrics
- `dlq_total_events` - Total events in DLQ
- `dlq_events_by_source` - Events by source component
- `dlq_events_by_error_type` - Events by error type
- `dlq_replay_success` - Successful replays
- `dlq_replay_failure` - Failed replays

### Alerts
- DLQ size > 80% of capacity
- Permanent error rate > 10%
- Replay failure rate > 20%

## Best Practices

1. **Set appropriate capacity** - Balance memory usage vs. event retention
2. **Monitor DLQ size** - Alerts for capacity issues
3. **Categorize errors correctly** - Only retry transient errors
4. **Review permanent errors** - Manual intervention may be required
5. **Regular cleanup** - Remove old or resolved events

## Troubleshooting

### DLQ Full
- Increase `DLQ_CAPACITY`
- Check for permanent errors requiring manual intervention
- Review error logs for common patterns

### High Replay Failure Rate
- Check error categorization (may be incorrectly marked as transient)
- Verify replay logic is correct
- Check underlying system health

## References

- [zen-sdk/pkg/dlq](../../../zen-sdk/pkg/dlq/README.md)
- [DLQ Integration Plan](./DLQ_INTEGRATION.md)
```

---

## Migration Steps

### 1. Initialize DLQ in main()

```go
import "github.com/kube-zen/zen-brain1/internal/dlq"

func main() {
    ctx := context.Background()

    // Initialize DLQ
    if err := dlq.Init(ctx); err != nil {
        log.Printf("Failed to initialize DLQ: %v", err)
    }

    // Start replay worker
    go dlq.StartReplayWorker(ctx, 5*time.Minute, nil)

    // ... rest of application
}
```

### 2. Add DLQ to API Server

```go
// In api server setup
server := apiserver.New(":8080", checker)
server.SetupDLQHandlers()
```

### 3. Test DLQ

```bash
# Add test event (simulate failure)
curl -X POST http://localhost:8080/api/v1/dlq/test-event

# List events
curl http://localhost:8080/api/v1/dlq/events

# Replay event
curl -X POST http://localhost:8080/api/v1/dlq/events/{id}/replay

# Check stats
curl http://localhost:8080/api/v1/dlq/stats
```

---

## Success Criteria

- ✅ DLQ manager helper package created
- ✅ DLQ integrated into message bus
- ✅ DLQ integrated into QMD orchestrator
- ✅ DLQ HTTP API endpoints
- ✅ Background replay worker
- ✅ Documentation complete
- ✅ Tests passing
- ✅ No performance regression (DLQ overhead <5%)
