package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"   // Normal operation
	CircuitStateOpen     CircuitState = "open"     // Failing, reject all requests
	CircuitStateHalfOpen CircuitState = "half_open" // Testing if recovered
)

// CircuitBreaker implements a circuit breaker for a service.
type CircuitBreaker struct {
	name          string
	maxFailures   int
	timeout       time.Duration
	state         CircuitState
	failures      int
	lastFailTime  time.Time
	nextRetryTime time.Time
	mu            sync.RWMutex
}

// CircuitBreakerConfig holds configuration for circuit breaker.
type CircuitBreakerConfig struct {
	Name        string        `json:"name" yaml:"name"`
	MaxFailures int           `json:"max_failures" yaml:"max_failures"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultCircuitBreakerConfig returns default configuration.
func DefaultCircuitBreakerConfig(name string) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:        name,
		MaxFailures: 3,
		Timeout:     30 * time.Second,
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig("default")
	}
	return &CircuitBreaker{
		name:        config.Name,
		maxFailures: config.MaxFailures,
		timeout:     config.Timeout,
		state:       CircuitStateClosed,
	}
}

// Allow checks if the circuit allows the request.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = CircuitStateHalfOpen
			cb.failures = 0
			return true
		}
		return false
	case CircuitStateHalfOpen:
		// Allow one request through to test
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == CircuitStateHalfOpen {
		cb.state = CircuitStateClosed
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitStateOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":          cb.name,
		"state":         string(cb.state),
		"failures":      cb.failures,
		"max_failures":  cb.maxFailures,
		"last_fail":     cb.lastFailTime,
		"next_retry":    cb.nextRetryTime,
	}
}

// CircuitBreakerManager manages circuit breakers for multiple services.
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager.
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetBreaker gets or creates a circuit breaker for a service.
func (m *CircuitBreakerManager) GetBreaker(name string) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.breakers[name]; !exists {
		m.breakers[name] = NewCircuitBreaker(DefaultCircuitBreakerConfig(name))
	}

	return m.breakers[name]
}

// WrapHealthCheck wraps a health check with circuit breaker protection.
func (m *CircuitBreakerManager) WrapHealthCheck(ctx context.Context, name string, check func() error) error {
	breaker := m.GetBreaker(name)

	if !breaker.Allow() {
		return fmt.Errorf("circuit breaker open for %s (state: %s)", name, breaker.State())
	}

	err := check()
	if err != nil {
		breaker.RecordFailure()
		return err
	}

	breaker.RecordSuccess()
	return nil
}

// AllBreakers returns all circuit breaker states.
func (m *CircuitBreakerManager) AllBreakers() map[string]CircuitState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]CircuitState)
	for name, breaker := range m.breakers {
		states[name] = breaker.State()
	}

	return states
}

// Stats returns statistics for all circuit breakers.
func (m *CircuitBreakerManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, breaker := range m.breakers {
		stats[name] = breaker.Stats()
	}

	return stats
}

// HealthReport represents a unified health report for Block 3.
type HealthReport struct {
	Status      string                        `json:"status"`
	Timestamp   time.Time                     `json:"timestamp"`
	Capabilities map[string]CapabilityHealth  `json:"capabilities"`
	Circuits    map[string]CircuitState       `json:"circuits"`
	Summary     string                        `json:"summary"`
}

// CapabilityHealth represents health status of a capability.
type CapabilityHealth struct {
	Name     string         `json:"name"`
	Mode     DependencyMode `json:"mode"`
	Healthy  bool           `json:"healthy"`
	Required bool           `json:"required"`
	Message  string         `json:"message,omitempty"`
	Circuit  CircuitState   `json:"circuit"`
}

// HealthAggregator aggregates health from all Block 3 components.
type HealthAggregator struct {
	report          *RuntimeReport
	circuitManager  *CircuitBreakerManager
	healthCheckers  map[string]func(context.Context) error
}

// NewHealthAggregator creates a new health aggregator.
func NewHealthAggregator(report *RuntimeReport) *HealthAggregator {
	return &HealthAggregator{
		report:         report,
		circuitManager: NewCircuitBreakerManager(),
		healthCheckers: make(map[string]func(context.Context) error),
	}
}

// RegisterHealthCheck registers a health check for a capability.
func (h *HealthAggregator) RegisterHealthCheck(name string, check func(context.Context) error) {
	h.healthCheckers[name] = check
}

// CheckHealth runs all health checks with circuit breaker protection.
func (h *HealthAggregator) CheckHealth(ctx context.Context) *HealthReport {
	start := time.Now()
	health := &HealthReport{
		Timestamp:   start,
		Capabilities: make(map[string]CapabilityHealth),
		Circuits:    h.circuitManager.AllBreakers(),
	}

	// Run health checks for each capability
	capabilities := []struct {
		name string
		status CapabilityStatus
	}{
		{"zen_context", h.report.ZenContext},
		{"tier1_hot", h.report.Tier1Hot},
		{"tier2_warm", h.report.Tier2Warm},
		{"tier3_cold", h.report.Tier3Cold},
		{"journal", h.report.Journal},
		{"ledger", h.report.Ledger},
		{"message_bus", h.report.MessageBus},
	}

	healthyCount := 0
	criticalFailures := 0

	for _, cap := range capabilities {
		healthCheck := h.healthCheckers[cap.name]

		var circuitState CircuitState = CircuitStateClosed
		err := h.circuitManager.WrapHealthCheck(ctx, cap.name, func() error {
			if healthCheck == nil {
				return nil
			}
			return healthCheck(ctx)
		})

		if err != nil {
			circuitState = h.circuitManager.GetBreaker(cap.name).State()
		}

		capHealth := CapabilityHealth{
			Name:     cap.name,
			Mode:     cap.status.Mode,
			Healthy:  cap.status.Healthy && err == nil,
			Required: cap.status.Required,
			Message:  cap.status.Message,
			Circuit:  circuitState,
		}

		if err != nil {
			capHealth.Message = err.Error()
		}

		health.Capabilities[cap.name] = capHealth

		if capHealth.Healthy {
			healthyCount++
		}

		if capHealth.Required && !capHealth.Healthy {
			criticalFailures++
		}
	}

	// Determine overall status
	if criticalFailures > 0 {
		health.Status = "unhealthy"
	} else if healthyCount == len(capabilities) {
		health.Status = "healthy"
	} else {
		health.Status = "degraded"
	}

	health.Summary = fmt.Sprintf("%d/%d capabilities healthy (%d critical failures)",
		healthyCount, len(capabilities), criticalFailures)

	return health
}

// CircuitManager returns the circuit breaker manager.
func (h *HealthAggregator) CircuitManager() *CircuitBreakerManager {
	return h.circuitManager
}
