// Package dlqmgr provides Dead Letter Queue (DLQ) utilities for zen-brain1.
// Uses zen-sdk/pkg/dlq under the hood.
package dlqmgr

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
	// Replay worker context
	replayCancel context.CancelFunc
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
			var n int
			if _, err := fmt.Sscanf(c, "%d", &n); err == nil && n > 0 {
				capacity = n
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
func ListFailedEvents(filter *dlq.Filter) []*dlq.FailedEvent {
	if manager == nil {
		return nil
	}
	return manager.ListFailedEvents(filter)
}

// GetFailedEvent gets a single failed event by ID.
func GetFailedEvent(id string) (*dlq.FailedEvent, bool) {
	if manager == nil {
		return nil, false
	}
	return manager.GetFailedEvent(id)
}

// ReplayFailedEvent replays a failed event (removes from DLQ).
func ReplayFailedEvent(id string) (*dlq.FailedEvent, bool) {
	if manager == nil {
		return nil, false
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
// Returns a cancel function to stop the worker.
func StartReplayWorker(ctx context.Context, interval time.Duration, filter *dlq.Filter) context.CancelFunc {
	if manager == nil {
		logger.Warn("DLQ manager not initialized, replay worker not started")
		return func() {}
	}

	// Create cancelable context
	replayCtx, cancel := context.WithCancel(ctx)

	logger.Info("Starting DLQ replay worker",
		zenlog.String("interval", interval.String()),
	)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-replayCtx.Done():
				logger.Info("DLQ replay worker stopped")
				return
			case <-ticker.C:
				replayFailedEvents(replayCtx, filter)
			}
		}
	}()

	return cancel
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
		maxRetries := 10
		if c := os.Getenv("DLQ_MAX_RETRIES"); c != "" {
			var n int
			if _, err := fmt.Sscanf(c, "%d", &n); err == nil && n > 0 {
				maxRetries = n
			}
		}

		if event.RetryCount >= maxRetries {
			logger.Warn("Event exceeded retry limit, skipping",
				zenlog.String("event_id", event.ID),
				zenlog.Int("retry_count", event.RetryCount),
				zenlog.Int("max_retries", maxRetries),
			)
			continue
		}

		// Wait before retry (exponential backoff)
		backoff := time.Duration(event.RetryCount) * time.Second * 10
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

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

// StopReplayWorker stops the replay worker if running.
func StopReplayWorker() {
	if replayCancel != nil {
		replayCancel()
		replayCancel = nil
	}
}

// IsInitialized returns whether the DLQ manager is initialized.
func IsInitialized() bool {
	return manager != nil
}
