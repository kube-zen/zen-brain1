/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package health provides generic health check interfaces and utilities for Kubernetes controllers.
// This package enables consistent health check patterns across zen-gc, zen-lock, zen-watcher, and other components.
package health

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Static errors for health checks.
var (
	ErrNotInitialized = errors.New("component not initialized")
	ErrNotReady       = errors.New("component not ready")
)

// Checker defines the interface for health checks.
// Implementations should provide ReadinessCheck, LivenessCheck, and optionally StartupCheck.
type Checker interface {
	// ReadinessCheck verifies that the component is ready to serve requests.
	ReadinessCheck(req *http.Request) error

	// LivenessCheck verifies that the component is actively processing.
	LivenessCheck(req *http.Request) error

	// StartupCheck is an optional check for startup probe.
	// Returns nil if component is initialized, error otherwise.
	StartupCheck(req *http.Request) error
}

// InformerSyncChecker provides a health check that verifies informer sync status.
// This is useful for Kubernetes controllers that use informers.
type InformerSyncChecker struct {
	// GetInformers returns a map of informer names to HasSynced functions.
	// The function should be safe to call concurrently.
	GetInformers func() map[string]func() bool
}

// NewInformerSyncChecker creates a new informer sync checker.
func NewInformerSyncChecker(getInformers func() map[string]func() bool) *InformerSyncChecker {
	return &InformerSyncChecker{
		GetInformers: getInformers,
	}
}

// ReadinessCheck verifies that all informers are synced.
func (c *InformerSyncChecker) ReadinessCheck(req *http.Request) error {
	if c.GetInformers == nil {
		return fmt.Errorf("%w: informer getter not set", ErrNotInitialized)
	}

	informers := c.GetInformers()
	if len(informers) == 0 {
		// No informers means nothing to sync - consider ready
		return nil
	}

	unsyncedCount := 0
	for name, hasSynced := range informers {
		if hasSynced == nil {
			continue
		}
		if !hasSynced() {
			unsyncedCount++
		}
		_ = name // Suppress unused variable warning
	}

	if unsyncedCount > 0 {
		return fmt.Errorf("%w: %d informers still syncing", ErrNotReady, unsyncedCount)
	}

	return nil
}

// LivenessCheck verifies that the component is alive.
// For informer-based controllers, this checks if informers exist and are synced.
func (c *InformerSyncChecker) LivenessCheck(req *http.Request) error {
	if c.GetInformers == nil {
		return fmt.Errorf("%w: informer getter not set", ErrNotInitialized)
	}

	informers := c.GetInformers()
	if len(informers) == 0 {
		// No informers means nothing to check - consider alive
		return nil
	}

	// Check if all informers are synced
	allSynced := true
	for _, hasSynced := range informers {
		if hasSynced != nil && !hasSynced() {
			allSynced = false
			break
		}
	}

	if !allSynced {
		// Informers not synced yet - this is normal during startup
		// The readiness check will catch this
		return nil
	}

	return nil
}

// StartupCheck is a simple check for startup probe.
func (c *InformerSyncChecker) StartupCheck(req *http.Request) error {
	if c.GetInformers == nil {
		return fmt.Errorf("%w: informer getter not set", ErrNotInitialized)
	}
	return nil
}

// ActivityChecker provides a health check that verifies component activity.
// It tracks the last activity time and considers the component unhealthy if no activity
// has occurred within a specified time window.
type ActivityChecker struct {
	// GetLastActivityTime returns the last time the component was active.
	// The function should be safe to call concurrently.
	GetLastActivityTime func() time.Time

	// MaxTimeSinceActivity is the maximum time since last activity before considering unhealthy.
	MaxTimeSinceActivity time.Duration
}

// NewActivityChecker creates a new activity checker.
func NewActivityChecker(getLastActivityTime func() time.Time, maxTimeSinceActivity time.Duration) *ActivityChecker {
	return &ActivityChecker{
		GetLastActivityTime:  getLastActivityTime,
		MaxTimeSinceActivity: maxTimeSinceActivity,
	}
}

// ReadinessCheck verifies that the component is ready.
// For activity-based checks, readiness means the component has been active recently.
func (c *ActivityChecker) ReadinessCheck(req *http.Request) error {
	if c.GetLastActivityTime == nil {
		return fmt.Errorf("%w: activity getter not set", ErrNotInitialized)
	}

	lastActivity := c.GetLastActivityTime()
	if lastActivity.IsZero() {
		// No activity yet - this might be normal during startup
		// Consider ready if max time hasn't passed
		return nil
	}

	timeSinceActivity := time.Since(lastActivity)
	if timeSinceActivity > c.MaxTimeSinceActivity {
		return fmt.Errorf("%w: no activity for %v (max: %v)", ErrNotReady, timeSinceActivity, c.MaxTimeSinceActivity)
	}

	return nil
}

// LivenessCheck verifies that the component is actively processing.
func (c *ActivityChecker) LivenessCheck(req *http.Request) error {
	if c.GetLastActivityTime == nil {
		return fmt.Errorf("%w: activity getter not set", ErrNotInitialized)
	}

	lastActivity := c.GetLastActivityTime()
	if lastActivity.IsZero() {
		// No activity yet - might be normal if component just started
		return nil
	}

	timeSinceActivity := time.Since(lastActivity)
	if timeSinceActivity > c.MaxTimeSinceActivity {
		return fmt.Errorf("%w: no activity for %v (max: %v)", ErrNotReady, timeSinceActivity, c.MaxTimeSinceActivity)
	}

	return nil
}

// StartupCheck is a simple check for startup probe.
func (c *ActivityChecker) StartupCheck(req *http.Request) error {
	if c.GetLastActivityTime == nil {
		return fmt.Errorf("%w: activity getter not set", ErrNotInitialized)
	}
	return nil
}

// CompositeChecker combines multiple health checkers.
// All checkers must pass for the composite to be healthy.
type CompositeChecker struct {
	checkers []Checker
	mu       sync.RWMutex
}

// NewCompositeChecker creates a new composite checker.
func NewCompositeChecker(checkers ...Checker) *CompositeChecker {
	return &CompositeChecker{
		checkers: checkers,
	}
}

// AddChecker adds a checker to the composite.
func (c *CompositeChecker) AddChecker(checker Checker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkers = append(c.checkers, checker)
}

// ReadinessCheck verifies that all checkers are ready.
func (c *CompositeChecker) ReadinessCheck(req *http.Request) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, checker := range c.checkers {
		if err := checker.ReadinessCheck(req); err != nil {
			return err
		}
	}
	return nil
}

// LivenessCheck verifies that all checkers are alive.
func (c *CompositeChecker) LivenessCheck(req *http.Request) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, checker := range c.checkers {
		if err := checker.LivenessCheck(req); err != nil {
			return err
		}
	}
	return nil
}

// StartupCheck verifies that all checkers are initialized.
func (c *CompositeChecker) StartupCheck(req *http.Request) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, checker := range c.checkers {
		if err := checker.StartupCheck(req); err != nil {
			return err
		}
	}
	return nil
}
