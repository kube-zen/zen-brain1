package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/kube-zen/zen-brain1/internal/runtime"
	zenhealth "github.com/kube-zen/zen-sdk/pkg/health"
)

// RuntimeChecker implements zen-sdk/pkg/health.Checker using Block 3 RuntimeReport.
// Readiness fails when any required capability is unhealthy.
type RuntimeChecker struct {
	Report *runtime.RuntimeReport
	mu     sync.RWMutex
}

// NewRuntimeChecker returns a checker that uses the given report.
// Report may be updated by the bootstrap layer; checker reads under mutex.
func NewRuntimeChecker(report *runtime.RuntimeReport) *RuntimeChecker {
	return &RuntimeChecker{Report: report}
}

// LivenessCheck returns nil if the process is alive (no dependency check).
func (c *RuntimeChecker) LivenessCheck(*http.Request) error {
	return nil
}

// ReadinessCheck fails if any required capability is unhealthy.
func (c *RuntimeChecker) ReadinessCheck(*http.Request) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Report == nil {
		return nil
	}
	for _, cap := range []runtime.CapabilityStatus{
		c.Report.ZenContext, c.Report.Tier1Hot, c.Report.Tier2Warm, c.Report.Tier3Cold,
		c.Report.Journal, c.Report.Ledger, c.Report.MessageBus,
	} {
		if cap.Required && !cap.Healthy {
			return fmt.Errorf("required capability %s unhealthy: %s", cap.Name, cap.Message)
		}
	}
	return nil
}

// StartupCheck mirrors readiness (can be looser in future if needed).
func (c *RuntimeChecker) StartupCheck(r *http.Request) error {
	return c.ReadinessCheck(r)
}

// Ensure RuntimeChecker implements zenhealth.Checker.
var _ zenhealth.Checker = (*RuntimeChecker)(nil)

// ============================================================================
// LiveRuntimeChecker - Block 3 A002: Live readiness with dependency truth
// ============================================================================

// LiveRuntimeChecker implements zenhealth.Checker using StrictRuntime and LiveHealthChecker.
// Unlike static RuntimeChecker, this reflects CURRENT dependency health, not just bootstrap state.
type LiveRuntimeChecker struct {
	strictRuntime *runtime.StrictRuntime
	healthChecker *runtime.LiveHealthChecker
	mu            sync.RWMutex
}

// NewLiveRuntimeChecker creates a live readiness checker.
func NewLiveRuntimeChecker(strictRT *runtime.StrictRuntime, hc *runtime.LiveHealthChecker) *LiveRuntimeChecker {
	return &LiveRuntimeChecker{
		strictRuntime: strictRT,
		healthChecker: hc,
	}
}

// LivenessCheck returns nil if the process is alive (lightweight).
func (c *LiveRuntimeChecker) LivenessCheck(*http.Request) error {
	return nil
}

// ReadinessCheck performs LIVE dependency health check.
// This reflects post-startup dependency loss, not just bootstrap state.
func (c *LiveRuntimeChecker) ReadinessCheck(r *http.Request) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// No strict runtime = always ready (dev mode without bootstrap)
	if c.strictRuntime == nil {
		return nil
	}

	// Use StrictRuntime's CheckReadiness which checks current health
	ctx := context.Background()
	if err := c.strictRuntime.CheckReadiness(ctx); err != nil {
		return err
	}

	// Also check circuit breaker states
	breakers := c.strictRuntime.GetAllCircuitBreakers()
	for name, cb := range breakers {
		if cb.State() == runtime.CircuitStateOpen {
			return fmt.Errorf("circuit breaker open for %s", name)
		}
	}

	return nil
}

// StartupCheck uses StrictRuntime validation for startup.
func (c *LiveRuntimeChecker) StartupCheck(r *http.Request) error {
	return c.ReadinessCheck(r)
}

// Ensure LiveRuntimeChecker implements zenhealth.Checker.
var _ zenhealth.Checker = (*LiveRuntimeChecker)(nil)
