// Copyright 2025 The Zen Watcher Authors
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

package dedup

import (
	"time"
)

// DedupStrategy defines a deduplication strategy
// Strategies wrap the existing Deduper logic with strategy-specific behavior
type DedupStrategy interface {
	// ShouldCreate determines if a new Observation should be created
	// Returns true if the event should create an Observation, false if it should be dropped
	// The deduper parameter provides access to the underlying deduplication engine
	ShouldCreate(deduper *Deduper, key DedupKey, content map[string]interface{}) bool
	// Name returns the strategy name
	Name() string
	// GetWindow returns the effective deduplication window for this strategy
	GetWindow(defaultWindow time.Duration) time.Duration
}

// StrategyConfig holds configuration for a dedup strategy
type StrategyConfig struct {
	Strategy           string   // "fingerprint", "event-stream", "key"
	Window             string   // Duration string (e.g., "1h")
	Fields             []string // Fields for key-based strategy
	MaxEventsPerWindow int      // Max events per window for event-stream strategy
}

// GetStrategy returns a DedupStrategy by name
func GetStrategy(config StrategyConfig) DedupStrategy {
	switch config.Strategy {
	case "event-stream":
		return &EventStreamStrategy{
			maxEventsPerWindow: config.MaxEventsPerWindow,
		}
	case "key":
		return &KeyBasedStrategy{
			fields: config.Fields,
		}
	case "fingerprint", "":
		fallthrough
	default:
		return &FingerprintStrategy{}
	}
}

// FingerprintStrategy implements fingerprint-based deduplication (default)
// This wraps the existing Deduper behavior
type FingerprintStrategy struct{}

func (s *FingerprintStrategy) Name() string {
	return "fingerprint"
}

func (s *FingerprintStrategy) GetWindow(defaultWindow time.Duration) time.Duration {
	return defaultWindow
}

//nolint:gocritic // hugeParam: key is intentionally passed by value for immutability
func (s *FingerprintStrategy) ShouldCreate(deduper *Deduper, key DedupKey, content map[string]interface{}) bool {
	// Use existing Deduper logic (fingerprint-based)
	return deduper.ShouldCreateWithContent(key, content)
}

// EventStreamStrategy implements strict window-based deduplication for noisy sources
// Designed for high-volume, repetitive events (e.g., k8s events)
type EventStreamStrategy struct {
	maxEventsPerWindow int
}

func (s *EventStreamStrategy) Name() string {
	return "event-stream"
}

func (s *EventStreamStrategy) GetWindow(defaultWindow time.Duration) time.Duration {
	// Use shorter window for event streams (5 minutes default, or 1/12 of default if default is longer)
	shortWindow := 5 * time.Minute
	if defaultWindow < shortWindow {
		return defaultWindow
	}
	return shortWindow
}

//nolint:gocritic // hugeParam: key is intentionally passed by value for immutability
func (s *EventStreamStrategy) ShouldCreate(deduper *Deduper, key DedupKey, content map[string]interface{}) bool {
	// Use existing Deduper logic but with shorter effective window
	// The window adjustment is handled via GetWindow
	return deduper.ShouldCreateWithContent(key, content)
}

// KeyBasedStrategy implements field-based deduplication
// Uses explicit fields to build dedup key
type KeyBasedStrategy struct {
	fields []string
}

func (s *KeyBasedStrategy) Name() string {
	return "key"
}

func (s *KeyBasedStrategy) GetWindow(defaultWindow time.Duration) time.Duration {
	return defaultWindow
}

//nolint:gocritic // hugeParam: key is intentionally passed by value for immutability
func (s *KeyBasedStrategy) ShouldCreate(deduper *Deduper, key DedupKey, content map[string]interface{}) bool {
	// Key-based dedup uses explicit fields
	// If fields are specified, we could build a custom key, but for now
	// we use the existing Deduper logic with the provided key
	// Future enhancement: build key from specified fields
	return deduper.ShouldCreateWithContent(key, content)
}
