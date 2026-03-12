package runtime

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// StrictRuntime enforces fail-closed behavior for Block 3 runtime.
// In strict mode, any required capability failure results in startup failure.
type StrictRuntime struct {
	runtime    *Runtime
	strict     bool
	mu         sync.RWMutex
	breakers   map[string]*CircuitBreaker
	healthChan chan HealthEvent
}

// HealthEvent represents a health state change.
type HealthEvent struct {
	Capability string    `json:"capability"`
	Healthy    bool      `json:"healthy"`
	Message    string    `json:"message"`
	Timestamp  int64     `json:"timestamp"`
	FromState  string    `json:"from_state,omitempty"`
	ToState    string    `json:"to_state,omitempty"`
}

// StrictRuntimeConfig holds configuration for strict runtime.
type StrictRuntimeConfig struct {
	Profile        string
	Config         *config.Config
	EnableHealthCh bool // Enable health event channel
}

// NewStrictRuntime creates a strict runtime that enforces fail-closed behavior.
func NewStrictRuntime(ctx context.Context, cfg *StrictRuntimeConfig) (*StrictRuntime, error) {
	strict := isStrictMode(cfg.Profile)
	rt, err := Bootstrap(ctx, cfg.Config)
	if err != nil {
		if strict {
			// In strict mode, bootstrap errors are fatal
			return nil, fmt.Errorf("strict runtime bootstrap failed: %w", err)
		}
		// In non-strict mode, continue with degraded runtime
		if rt == nil {
			rt = &Runtime{Report: &RuntimeReport{}}
		}
	}

	sr := &StrictRuntime{
		runtime:  rt,
		strict:   strict,
		breakers: make(map[string]*CircuitBreaker),
	}

	if cfg.EnableHealthCh {
		sr.healthChan = make(chan HealthEvent, 100)
	}

	// In strict mode, verify all required capabilities are healthy
	if strict {
		if err := sr.validateStrictRequirements(); err != nil {
			return nil, fmt.Errorf("strict runtime validation failed: %w", err)
		}
	}

	// Initialize circuit breakers for critical services
	sr.initCircuitBreakers()

	return sr, nil
}

// isStrictMode determines if runtime should be in strict mode.
func isStrictMode(profile string) bool {
	// Check explicit profile
	if profile == "prod" || profile == "staging" {
		return true
	}

	// Check environment variables
	if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" ||
		os.Getenv("ZEN_RUNTIME_PROFILE") == "staging" ||
		os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		return true
	}

	// Check for individual strict flags
	req := GetRequirements(nil)
	return req.ZenContext || req.QMD || req.Ledger || req.MessageBus
}

// validateStrictRequirements validates all required capabilities are healthy in strict mode.
func (sr *StrictRuntime) validateStrictRequirements() error {
	if sr.runtime == nil || sr.runtime.Report == nil {
		return fmt.Errorf("runtime report is nil")
	}

	report := sr.runtime.Report

	// Check all required capabilities
	var failures []string

	caps := []CapabilityStatus{
		report.ZenContext,
		report.Tier1Hot,
		report.Tier2Warm,
		report.Tier3Cold,
		report.Journal,
		report.Ledger,
		report.MessageBus,
	}

	for _, cap := range caps {
		if cap.Required && !cap.Healthy {
			failures = append(failures,
				fmt.Sprintf("%s: %s", cap.Name, cap.Message))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("required capabilities unhealthy: %v", failures)
	}

	return nil
}

// initCircuitBreakers initializes circuit breakers for critical services.
func (sr *StrictRuntime) initCircuitBreakers() {
	services := []string{"zen_context", "tier1_redis", "tier2_qmd", "ledger", "message_bus"}

	for _, svc := range services {
		cb := NewCircuitBreaker(&CircuitBreakerConfig{
			Name:        svc,
			MaxFailures: 3,
			Timeout:     30e9, // 30 seconds
		})
		sr.breakers[svc] = cb

		// Register with global registry for doctor/preflight access
		RegisterCircuitBreaker(svc, cb)
	}
}

// Runtime returns the underlying runtime.
func (sr *StrictRuntime) Runtime() *Runtime {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.runtime
}

// Report returns the runtime report.
func (sr *StrictRuntime) Report() *RuntimeReport {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	if sr.runtime == nil {
		return nil
	}
	return sr.runtime.Report
}

// IsStrict returns if runtime is in strict mode.
func (sr *StrictRuntime) IsStrict() bool {
	return sr.strict
}

// CheckReadiness performs live readiness check.
// Unlike static RuntimeReport, this checks current health state.
func (sr *StrictRuntime) CheckReadiness(ctx context.Context) error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if sr.runtime == nil || sr.runtime.Report == nil {
		if sr.strict {
			return fmt.Errorf("runtime not initialized")
		}
		return nil
	}

	report := sr.runtime.Report

	// Check required capabilities
	var failures []string

	caps := []CapabilityStatus{
		report.ZenContext,
		report.Tier1Hot,
		report.Tier2Warm,
		report.Tier3Cold,
		report.Journal,
		report.Ledger,
		report.MessageBus,
	}

	for _, cap := range caps {
		if cap.Required && !cap.Healthy {
			failures = append(failures,
				fmt.Sprintf("%s: %s", cap.Name, cap.Message))
		}
	}

	// Check circuit breaker states
	for name, breaker := range sr.breakers {
		state := breaker.State()
		if state == CircuitStateOpen {
			failures = append(failures,
				fmt.Sprintf("%s: circuit breaker open", name))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("readiness check failed: %v", failures)
	}

	return nil
}

// GetCircuitBreaker returns circuit breaker for a service.
func (sr *StrictRuntime) GetCircuitBreaker(name string) *CircuitBreaker {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.breakers[name]
}

// GetAllCircuitBreakers returns all circuit breakers.
func (sr *StrictRuntime) GetAllCircuitBreakers() map[string]*CircuitBreaker {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	// Return copy to avoid race conditions
	result := make(map[string]*CircuitBreaker, len(sr.breakers))
	for k, v := range sr.breakers {
		result[k] = v
	}
	return result
}

// UpdateCapabilityHealth updates health status for a capability.
func (sr *StrictRuntime) UpdateCapabilityHealth(name string, healthy bool, message string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.runtime == nil || sr.runtime.Report == nil {
		return
	}

	// Update the appropriate capability
	report := sr.runtime.Report

	switch name {
	case "zen_context":
		oldHealthy := report.ZenContext.Healthy
		report.ZenContext.Healthy = healthy
		report.ZenContext.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "tier1_hot":
		oldHealthy := report.Tier1Hot.Healthy
		report.Tier1Hot.Healthy = healthy
		report.Tier1Hot.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "tier2_warm":
		oldHealthy := report.Tier2Warm.Healthy
		report.Tier2Warm.Healthy = healthy
		report.Tier2Warm.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "tier3_cold":
		oldHealthy := report.Tier3Cold.Healthy
		report.Tier3Cold.Healthy = healthy
		report.Tier3Cold.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "journal":
		oldHealthy := report.Journal.Healthy
		report.Journal.Healthy = healthy
		report.Journal.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "ledger":
		oldHealthy := report.Ledger.Healthy
		report.Ledger.Healthy = healthy
		report.Ledger.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	case "message_bus":
		oldHealthy := report.MessageBus.Healthy
		report.MessageBus.Healthy = healthy
		report.MessageBus.Message = message
		sr.emitHealthEvent(name, healthy, message, oldHealthy)
	}
}

// emitHealthEvent emits a health event if channel is enabled.
func (sr *StrictRuntime) emitHealthEvent(name string, healthy bool, message string, oldHealthy bool) {
	if sr.healthChan == nil {
		return
	}

	event := HealthEvent{
		Capability: name,
		Healthy:    healthy,
		Message:    message,
		Timestamp:  currentTimestamp(),
	}

	if oldHealthy != healthy {
		if oldHealthy {
			event.FromState = "healthy"
			event.ToState = "unhealthy"
		} else {
			event.FromState = "unhealthy"
			event.ToState = "healthy"
		}
	}

	// Non-blocking send
	select {
	case sr.healthChan <- event:
	default:
		// Channel full, drop event
	}
}

// HealthEvents returns the health event channel (nil if not enabled).
func (sr *StrictRuntime) HealthEvents() <-chan HealthEvent {
	return sr.healthChan
}

// Close closes the strict runtime and releases resources.
func (sr *StrictRuntime) Close() error {
	// Unregister circuit breakers from global registry
	for name := range sr.breakers {
		UnregisterCircuitBreaker(name)
	}

	if sr.healthChan != nil {
		close(sr.healthChan)
	}

	if sr.runtime != nil {
		return sr.runtime.Close()
	}

	return nil
}

// currentTimestamp returns current Unix timestamp in nanoseconds.
func currentTimestamp() int64 {
	return time.Now().UnixNano()
}
