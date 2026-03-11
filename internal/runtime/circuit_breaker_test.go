package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 2,
		Timeout:     1 * time.Second,
	})

	// Should be closed initially
	if cb.State() != CircuitStateClosed {
		t.Errorf("Expected closed state, got %s", cb.State())
	}

	// Record failures
	cb.RecordFailure()
	if cb.State() != CircuitStateClosed {
		t.Errorf("Expected closed state after 1 failure, got %s", cb.State())
	}

	cb.RecordFailure()
	if cb.State() != CircuitStateOpen {
		t.Errorf("Expected open state after 2 failures, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
	})

	// Open the circuit
	cb.RecordFailure()
	if cb.State() != CircuitStateOpen {
		t.Errorf("Expected open state, got %s", cb.State())
	}

	// Should not allow immediately
	if cb.Allow() {
		t.Error("Expected circuit to reject request when open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open and allow
	if !cb.Allow() {
		t.Error("Expected circuit to allow request after timeout (half-open)")
	}

	if cb.State() != CircuitStateHalfOpen {
		t.Errorf("Expected half-open state, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
	})

	// Open the circuit
	cb.RecordFailure()
	time.Sleep(150 * time.Millisecond)
	_ = cb.Allow() // Transition to half-open

	// Record success
	cb.RecordSuccess()
	if cb.State() != CircuitStateClosed {
		t.Errorf("Expected closed state after success, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
	})

	// Open the circuit
	cb.RecordFailure()
	time.Sleep(150 * time.Millisecond)
	_ = cb.Allow() // Transition to half-open

	// Record failure in half-open state
	cb.RecordFailure()
	if cb.State() != CircuitStateOpen {
		t.Errorf("Expected open state after failure in half-open, got %s", cb.State())
	}
}

func TestCircuitBreakerManager_GetBreaker(t *testing.T) {
	mgr := NewCircuitBreakerManager()

	// Get breaker for first time (should create)
	cb1 := mgr.GetBreaker("service1")
	if cb1 == nil {
		t.Fatal("Expected non-nil breaker")
	}

	// Get same breaker again (should return same instance)
	cb2 := mgr.GetBreaker("service1")
	if cb1 != cb2 {
		t.Error("Expected same breaker instance")
	}

	// Get different breaker
	cb3 := mgr.GetBreaker("service2")
	if cb1 == cb3 {
		t.Error("Expected different breaker instances")
	}

	// Check all breakers
	states := mgr.AllBreakers()
	if len(states) != 2 {
		t.Errorf("Expected 2 breakers, got %d", len(states))
	}
}

func TestCircuitBreakerManager_WrapHealthCheck(t *testing.T) {
	mgr := NewCircuitBreakerManager()
	ctx := context.Background()

	// Successful check
	callCount := 0
	err := mgr.WrapHealthCheck(ctx, "test", func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Failing check (should trigger circuit breaker after 3 failures)
	failureCount := 0
	for i := 0; i < 5; i++ {
		err := mgr.WrapHealthCheck(ctx, "failing", func() error {
			failureCount++
			return errors.New("service unavailable")
		})
		if err == nil {
			t.Error("Expected error from failing health check")
		}
	}

	// Circuit should be open after 3 failures
	states := mgr.AllBreakers()
	if states["failing"] != CircuitStateOpen {
		t.Errorf("Expected circuit open for failing service, got %s", states["failing"])
	}

	// Additional calls should be rejected by circuit breaker
	rejectedCount := 0
	for i := 0; i < 3; i++ {
		err := mgr.WrapHealthCheck(ctx, "failing", func() error {
			return nil
		})
		if err != nil && err.Error() == "circuit breaker open for failing (state: open)" {
			rejectedCount++
		}
	}

	if rejectedCount == 0 {
		t.Error("Expected some requests to be rejected by circuit breaker")
	}

	t.Logf("Circuit breaker rejected %d requests", rejectedCount)
}

func TestHealthAggregator_CheckHealth(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Tier2Warm:  CapabilityStatus{Name: "tier2_warm", Mode: ModeReal, Healthy: true},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
	}

	agg := NewHealthAggregator(report)
	ctx := context.Background()

	// Register health checks
	agg.RegisterHealthCheck("zen_context", func(ctx context.Context) error {
		return nil // Healthy
	})

	agg.RegisterHealthCheck("tier1_hot", func(ctx context.Context) error {
		return nil // Healthy
	})

	agg.RegisterHealthCheck("tier2_warm", func(ctx context.Context) error {
		return errors.New("QMD unavailable") // Unhealthy but optional
	})

	agg.RegisterHealthCheck("ledger", func(ctx context.Context) error {
		return nil // Healthy
	})

	// Check health
	health := agg.CheckHealth(ctx)

	if health.Status != "degraded" {
		t.Errorf("Expected status degraded, got %s", health.Status)
	}

	// Check summary
	t.Logf("Health summary: %s", health.Summary)

	// Check capabilities
	if health.Capabilities["zen_context"].Healthy != true {
		t.Error("Expected zen_context to be healthy")
	}

	if health.Capabilities["tier2_warm"].Healthy != false {
		t.Error("Expected tier2_warm to be unhealthy")
	}

	if health.Capabilities["tier2_warm"].Required != false {
		t.Error("Expected tier2_warm to be optional")
	}

	// Check that critical services are healthy
	criticalHealthy := true
	for name, cap := range health.Capabilities {
		if cap.Required && !cap.Healthy {
			criticalHealthy = false
			t.Errorf("Critical service %s is unhealthy", name)
		}
	}

	if !criticalHealthy {
		t.Error("Expected all critical services to be healthy")
	}
}

func TestHealthAggregator_CriticalFailure(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: false, Required: true, Message: "redis down"},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
	}

	agg := NewHealthAggregator(report)
	ctx := context.Background()

	// Check health
	health := agg.CheckHealth(ctx)

	if health.Status != "unhealthy" {
		t.Errorf("Expected status unhealthy, got %s", health.Status)
	}

	// Check that critical failure is detected
	if health.Capabilities["tier1_hot"].Healthy != false {
		t.Error("Expected tier1_hot to be unhealthy")
	}

	t.Logf("Health summary: %s", health.Summary)
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 2,
		Timeout:     30 * time.Second,
	})

	stats := cb.Stats()

	if stats["name"] != "test" {
		t.Errorf("Expected name=test, got %v", stats["name"])
	}

	if stats["state"] != "closed" {
		t.Errorf("Expected state=closed, got %v", stats["state"])
	}

	if stats["max_failures"] != 2 {
		t.Errorf("Expected max_failures=2, got %v", stats["max_failures"])
	}
}

func TestCircuitBreakerManager_Stats(t *testing.T) {
	mgr := NewCircuitBreakerManager()

	// Create some breakers
	_ = mgr.GetBreaker("service1")
	_ = mgr.GetBreaker("service2")

	stats := mgr.Stats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 breakers in stats, got %d", len(stats))
	}

	t.Logf("Manager stats: %v", stats)
}
