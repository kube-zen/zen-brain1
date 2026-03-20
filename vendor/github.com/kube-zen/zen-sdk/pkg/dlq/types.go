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
	"time"
)

// Event represents a generic event that can be stored in the DLQ
// Components should convert their specific event types (e.g., adapter.RawEvent) to this format
type Event struct {
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	RawData   map[string]interface{} `json:"raw_data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventConverter interface for components to convert their RawEvent types to DLQ Event
type EventConverter interface {
	ToDLQEvent() Event
}

// ConvertToDLQEvent converts a component-specific event to DLQ Event format
// Supports both EventConverter interface and raw data
func ConvertToDLQEvent(source string, timestamp time.Time, rawData map[string]interface{}, metadata map[string]interface{}) Event {
	return Event{
		Source:    source,
		Timestamp: timestamp,
		RawData:   rawData,
		Metadata:  metadata,
	}
}

// FailedEvent represents a failed event in the DLQ
type FailedEvent struct {
	ID            string                 `json:"id"`
	Source        string                 `json:"source"`
	Event         Event                  `json:"event"`
	Destination   string                 `json:"destination"`
	Error         string                 `json:"error"`
	ErrorType     string                 `json:"error_type"`     // transient, permanent, unknown
	ErrorCategory string                 `json:"error_category"` // network, auth, validation, etc.
	RetryCount    int                    `json:"retry_count"`
	FirstFailed   time.Time              `json:"first_failed"`
	LastFailed    time.Time              `json:"last_failed"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries        int           // Maximum retry attempts (default: 3)
	InitialBackoff    time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff        time.Duration // Maximum backoff duration (default: 60s)
	BackoffMultiplier float64       // Backoff multiplier (default: 2.0)
	RetryTransient    bool          // Retry transient errors (default: true)
	RetryPermanent    bool          // Retry permanent errors (default: false)
}

// Filter for listing failed events
type Filter struct {
	Source        string
	Destination   string
	ErrorType     string
	ErrorCategory string
	MinRetryCount int
	MaxRetryCount int
	Since         *time.Time
	Until         *time.Time
}

// Stats holds DLQ statistics
type Stats struct {
	TotalEvents int            `json:"total_events"`
	ByErrorType map[string]int `json:"by_error_type"`
	ByCategory  map[string]int `json:"by_category"`
	BySource    map[string]int `json:"by_source"`
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		RetryTransient:    true,
		RetryPermanent:    false,
	}
}

// Matches checks if an event matches the filter
func (f *Filter) Matches(event *FailedEvent) bool {
	if f.Source != "" && event.Source != f.Source {
		return false
	}
	if f.Destination != "" && event.Destination != f.Destination {
		return false
	}
	if f.ErrorType != "" && event.ErrorType != f.ErrorType {
		return false
	}
	if f.ErrorCategory != "" && event.ErrorCategory != f.ErrorCategory {
		return false
	}
	if event.RetryCount < f.MinRetryCount {
		return false
	}
	if f.MaxRetryCount > 0 && event.RetryCount > f.MaxRetryCount {
		return false
	}
	if f.Since != nil && event.FirstFailed.Before(*f.Since) {
		return false
	}
	if f.Until != nil && event.FirstFailed.After(*f.Until) {
		return false
	}
	return true
}
