package runtime

import (
	"context"
	"log"
	"sync"
	"time"

	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	internalLedger "github.com/kube-zen/zen-brain1/internal/ledger"
)

// LiveHealthChecker performs live health checks on runtime dependencies.
// Unlike static RuntimeReport, this actively checks dependency health.
type LiveHealthChecker struct {
	strictRuntime *StrictRuntime
	refreshPeriod time.Duration
	mu            sync.RWMutex
	running       bool
	cancel        context.CancelFunc
}

// LiveHealthCheckerConfig holds configuration for live health checker.
type LiveHealthCheckerConfig struct {
	StrictRuntime  *StrictRuntime
	RefreshPeriod  time.Duration
}

// NewLiveHealthChecker creates a live health checker.
func NewLiveHealthChecker(cfg *LiveHealthCheckerConfig) *LiveHealthChecker {
	refreshPeriod := cfg.RefreshPeriod
	if refreshPeriod == 0 {
		refreshPeriod = 30 * time.Second // Default 30 seconds
	}

	return &LiveHealthChecker{
		strictRuntime: cfg.StrictRuntime,
		refreshPeriod: refreshPeriod,
	}
}

// Start starts the periodic health check goroutine.
func (hc *LiveHealthChecker) Start(ctx context.Context) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	hc.cancel = cancel
	hc.running = true

	go hc.healthCheckLoop(ctx)

	log.Printf("[LiveHealthChecker] Started with refresh period: %v", hc.refreshPeriod)
	return nil
}

// Stop stops the health check goroutine.
func (hc *LiveHealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	if hc.cancel != nil {
		hc.cancel()
	}

	hc.running = false
	log.Printf("[LiveHealthChecker] Stopped")
}

// healthCheckLoop performs periodic health checks.
func (hc *LiveHealthChecker) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(hc.refreshPeriod)
	defer ticker.Stop()

	// Initial check
	hc.performHealthChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks checks health of all runtime dependencies.
func (hc *LiveHealthChecker) performHealthChecks(ctx context.Context) {
	if hc.strictRuntime == nil {
		return
	}

	rt := hc.strictRuntime.Runtime()
	if rt == nil {
		return
	}

	// Check Tier1 (Redis) if ZenContext is real
	if rt.ZenContext != nil {
		if err := internalcontext.CheckHot(ctx, rt.ZenContext); err != nil {
			hc.strictRuntime.UpdateCapabilityHealth("tier1_hot", false, err.Error())
			hc.recordFailure("tier1_redis")
		} else {
			hc.strictRuntime.UpdateCapabilityHealth("tier1_hot", true, "")
			hc.recordSuccess("tier1_redis")
		}

		// Check Tier2 (QMD) if enabled
		if rt.Report != nil && rt.Report.Tier2Warm.Mode == ModeReal {
			if err := internalcontext.CheckWarm(ctx, rt.ZenContext); err != nil {
				hc.strictRuntime.UpdateCapabilityHealth("tier2_warm", false, err.Error())
				hc.recordFailure("tier2_qmd")
			} else {
				hc.strictRuntime.UpdateCapabilityHealth("tier2_warm", true, "")
				hc.recordSuccess("tier2_qmd")
			}
		}

		// Check Tier3 (S3) if enabled
		if rt.Report != nil && rt.Report.Tier3Cold.Mode == ModeReal {
			if err := internalcontext.CheckCold(ctx, rt.ZenContext); err != nil {
				hc.strictRuntime.UpdateCapabilityHealth("tier3_cold", false, err.Error())
			} else {
				hc.strictRuntime.UpdateCapabilityHealth("tier3_cold", true, "")
			}
		}
	}

	// Check Ledger if real
	if rt.Ledger != nil {
		if err := internalLedger.Ping(ctx, rt.Ledger); err != nil {
			hc.strictRuntime.UpdateCapabilityHealth("ledger", false, err.Error())
			hc.recordFailure("ledger")
		} else {
			hc.strictRuntime.UpdateCapabilityHealth("ledger", true, "")
			hc.recordSuccess("ledger")
		}
	}

	// Check MessageBus if real
	if rt.MessageBus != nil {
		// MessageBus doesn't have Ping, check through report
		if rt.Report.MessageBus.Mode == ModeReal && !rt.Report.MessageBus.Healthy {
			hc.strictRuntime.UpdateCapabilityHealth("message_bus", false, rt.Report.MessageBus.Message)
		} else if rt.Report.MessageBus.Mode == ModeReal {
			hc.strictRuntime.UpdateCapabilityHealth("message_bus", true, "")
		}
	}
}

// recordSuccess records a successful health check to the circuit breaker.
func (hc *LiveHealthChecker) recordSuccess(service string) {
	if cb := hc.strictRuntime.GetCircuitBreaker(service); cb != nil {
		cb.RecordSuccess()
	}
}

// recordFailure records a failed health check to the circuit breaker.
func (hc *LiveHealthChecker) recordFailure(service string) {
	if cb := hc.strictRuntime.GetCircuitBreaker(service); cb != nil {
		cb.RecordFailure()
	}
}

// CheckNow performs an immediate health check (synchronous).
func (hc *LiveHealthChecker) CheckNow(ctx context.Context) error {
	hc.performHealthChecks(ctx)

	// Return error if any required capability is unhealthy
	return hc.strictRuntime.CheckReadiness(ctx)
}

// GetHealthSummary returns a summary of current health state.
func (hc *LiveHealthChecker) GetHealthSummary() *HealthSummary {
	if hc.strictRuntime == nil {
		return nil
	}

	rt := hc.strictRuntime.Runtime()
	if rt == nil || rt.Report == nil {
		return nil
	}

	report := rt.Report
	breakers := hc.strictRuntime.GetAllCircuitBreakers()

	summary := &HealthSummary{
		Timestamp: time.Now().Unix(),
		Strict:    hc.strictRuntime.IsStrict(),
		Overall:   "healthy",
		Capabilities: make(map[string]LiveCapabilityHealth),
		CircuitBreakers: make(map[string]CircuitBreakerHealth),
	}

	// Check capabilities
	caps := []struct {
		name string
		cap  CapabilityStatus
	}{
		{"zen_context", report.ZenContext},
		{"tier1_hot", report.Tier1Hot},
		{"tier2_warm", report.Tier2Warm},
		{"tier3_cold", report.Tier3Cold},
		{"journal", report.Journal},
		{"ledger", report.Ledger},
		{"message_bus", report.MessageBus},
	}

	for _, item := range caps {
		summary.Capabilities[item.name] = LiveCapabilityHealth{
			Healthy:  item.cap.Healthy,
			Required: item.cap.Required,
			Mode:     item.cap.Mode,
			Message:  item.cap.Message,
		}

		if item.cap.Required && !item.cap.Healthy {
			summary.Overall = "unhealthy"
		}
	}

	// Check circuit breakers
	for name, cb := range breakers {
		state := cb.State()
		summary.CircuitBreakers[name] = CircuitBreakerHealth{
			State:     string(state),
			Failures:  cb.Failures(),
			Healthy:   state == CircuitStateClosed || state == CircuitStateHalfOpen,
		}

		if state == CircuitStateOpen {
			summary.Overall = "degraded"
		}
	}

	return summary
}

// HealthSummary represents the overall health state.
type HealthSummary struct {
	Timestamp       int64                             `json:"timestamp"`
	Strict          bool                              `json:"strict"`
	Overall         string                            `json:"overall"` // healthy, degraded, unhealthy
	Capabilities    map[string]LiveCapabilityHealth   `json:"capabilities"`
	CircuitBreakers map[string]CircuitBreakerHealth   `json:"circuit_breakers"`
}

// LiveCapabilityHealth represents health of a single capability (for live health checker).
type LiveCapabilityHealth struct {
	Healthy  bool          `json:"healthy"`
	Required bool          `json:"required"`
	Mode     DependencyMode `json:"mode"`
	Message  string        `json:"message,omitempty"`
}

// CircuitBreakerHealth represents health of a circuit breaker.
type CircuitBreakerHealth struct {
	State    string `json:"state"`
	Failures int    `json:"failures"`
	Healthy  bool   `json:"healthy"`
}

// Ensure LiveHealthChecker implements health checking interface.
var _ interface {
	Start(context.Context) error
	Stop()
	CheckNow(context.Context) error
} = (*LiveHealthChecker)(nil)
