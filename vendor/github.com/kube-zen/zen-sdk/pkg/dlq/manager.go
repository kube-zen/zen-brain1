// Copyright 2025 The Zen SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/logging"
)

// Manager manages the Dead Letter Queue
// Reused from zen-ingester pattern for consistent DLQ handling across components
type Manager struct {
	events      map[string]*FailedEvent // event_id -> FailedEvent
	mu          sync.RWMutex
	logger      *logging.Logger
	maxSize     int
	retryConfig *RetryConfig
}

// NewManager creates a new DLQ manager
func NewManager(logger *logging.Logger, maxSize int, retryConfig *RetryConfig) *Manager {
	if maxSize <= 0 {
		maxSize = 10000 // Default: 10k events
	}
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}

	return &Manager{
		events:      make(map[string]*FailedEvent),
		logger:      logger,
		maxSize:     maxSize,
		retryConfig: retryConfig,
	}
}

// AddFailedEvent adds a failed event to the DLQ
func (m *Manager) AddFailedEvent(
	ctx context.Context,
	source string,
	event Event,
	destination string,
	err error,
	errorType string,
	errorCategory string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we're at capacity
	if len(m.events) >= m.maxSize {
		// Remove oldest event (FIFO)
		m.removeOldestEvent()
		m.logger.WithContext(ctx).Warn("DLQ at capacity, dropping oldest event",
			logging.Operation("dlq_add"),
			logging.Int("max_size", m.maxSize),
		)
	}

	// Generate event ID if not present
	eventID := m.generateEventID(source, event)

	// Check if event already exists
	failedEvent, exists := m.events[eventID]
	if exists {
		// Update existing failed event
		failedEvent.RetryCount++
		failedEvent.LastFailed = time.Now()
		failedEvent.Error = err.Error()
		failedEvent.ErrorType = errorType
		failedEvent.ErrorCategory = errorCategory
	} else {
		// Create new failed event
		now := time.Now()
		failedEvent = &FailedEvent{
			ID:            eventID,
			Source:        source,
			Event:         event,
			Destination:   destination,
			Error:         err.Error(),
			ErrorType:     errorType,
			ErrorCategory: errorCategory,
			RetryCount:    0,
			FirstFailed:   now,
			LastFailed:    now,
			Metadata:      make(map[string]interface{}),
		}
		m.events[eventID] = failedEvent
	}

	m.logger.WithContext(ctx).Info("Event added to DLQ",
		logging.Operation("dlq_add"),
		logging.String("event_id", eventID),
		logging.String("source", source),
		logging.String("destination", destination),
		logging.String("error_type", errorType),
		logging.String("error_category", errorCategory),
		logging.Int("retry_count", failedEvent.RetryCount),
	)

	return nil
}

// GetFailedEvent retrieves a failed event by ID
func (m *Manager) GetFailedEvent(eventID string) (*FailedEvent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, exists := m.events[eventID]
	return event, exists
}

// ListFailedEvents lists all failed events, optionally filtered
func (m *Manager) ListFailedEvents(filter *Filter) []*FailedEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FailedEvent, 0, len(m.events))
	for _, event := range m.events {
		if filter == nil || filter.Matches(event) {
			result = append(result, event)
		}
	}

	return result
}

// RemoveFailedEvent removes a failed event from the DLQ
func (m *Manager) RemoveFailedEvent(eventID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.events[eventID]; exists {
		delete(m.events, eventID)
		return true
	}

	return false
}

// ReplayFailedEvent marks an event for replay (removes from DLQ)
func (m *Manager) ReplayFailedEvent(eventID string) (*FailedEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	event, exists := m.events[eventID]
	if !exists {
		return nil, false
	}

	// Remove from DLQ (will be re-added if it fails again)
	delete(m.events, eventID)

	return event, true
}

// ShouldRetry determines if an event should be retried based on error type and retry count
func (m *Manager) ShouldRetry(failedEvent *FailedEvent) bool {
	if failedEvent.RetryCount >= m.retryConfig.MaxRetries {
		return false
	}

	// Check error type
	if failedEvent.ErrorType == "permanent" && !m.retryConfig.RetryPermanent {
		return false
	}

	if failedEvent.ErrorType == "transient" && !m.retryConfig.RetryTransient {
		return false
	}

	return true
}

// GetRetryBackoff calculates the backoff duration for retry
func (m *Manager) GetRetryBackoff(retryCount int) time.Duration {
	backoff := float64(m.retryConfig.InitialBackoff) *
		math.Pow(m.retryConfig.BackoffMultiplier, float64(retryCount))

	if backoff > float64(m.retryConfig.MaxBackoff) {
		backoff = float64(m.retryConfig.MaxBackoff)
	}

	return time.Duration(backoff)
}

// GetStats returns DLQ statistics
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		TotalEvents: len(m.events),
		ByErrorType: make(map[string]int),
		ByCategory:  make(map[string]int),
		BySource:    make(map[string]int),
	}

	for _, event := range m.events {
		stats.ByErrorType[event.ErrorType]++
		stats.ByCategory[event.ErrorCategory]++
		stats.BySource[event.Source]++
	}

	return stats
}

// removeOldestEvent removes the oldest event (FIFO)
func (m *Manager) removeOldestEvent() {
	var oldestID string
	var oldestTime time.Time

	for id, event := range m.events {
		if oldestID == "" || event.FirstFailed.Before(oldestTime) {
			oldestID = id
			oldestTime = event.FirstFailed
		}
	}

	if oldestID != "" {
		delete(m.events, oldestID)
	}
}

// generateEventID generates a unique event ID
func (m *Manager) generateEventID(source string, event Event) string {
	// Use event timestamp and source to generate ID
	return fmt.Sprintf("%s-%d", source, event.Timestamp.UnixNano())
}

// ToJSON converts stats to JSON
func (s *Stats) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}
