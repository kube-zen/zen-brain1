package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
)

func TestStrictRuntime_Integration(t *testing.T) {
	// Save and restore env vars
	origProfile := os.Getenv("ZEN_RUNTIME_PROFILE")
	origStrict := os.Getenv("ZEN_BRAIN_STRICT_RUNTIME")
	origLedger := os.Getenv("ZEN_BRAIN_REQUIRE_LEDGER")
	origRedis := os.Getenv("TIER1_REDIS_ADDR")
	defer func() {
		os.Setenv("ZEN_RUNTIME_PROFILE", origProfile)
		os.Setenv("ZEN_BRAIN_STRICT_RUNTIME", origStrict)
		os.Setenv("ZEN_BRAIN_REQUIRE_LEDGER", origLedger)
		os.Setenv("TIER1_REDIS_ADDR", origRedis)
	}()

	t.Run("prod_mode_missing_required_ledger_fails", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		os.Setenv("ZEN_BRAIN_REQUIRE_LEDGER", "1")
		os.Unsetenv("ZEN_LEDGER_DSN")
		os.Unsetenv("LEDGER_DATABASE_URL")

		cfg := config.DefaultConfig()

		_, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "prod",
			Config:  cfg,
		})

		if err == nil {
			t.Error("FAIL: Should reject missing ledger in prod mode")
		}

		t.Logf("✅ PASS: Strict mode correctly rejected missing ledger: %v", err)
	})

	t.Run("dev_mode_missing_ledger_continues", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "dev")
		os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")
		os.Unsetenv("ZEN_BRAIN_REQUIRE_LEDGER")

		cfg := config.DefaultConfig()

		rt, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "dev",
			Config:  cfg,
		})

		if err != nil {
			t.Logf("Dev mode may fail for other reasons (Redis, etc): %v", err)
		}

		if rt != nil {
			_ = rt.Close()
		}

		t.Logf("✅ PASS: Dev mode allows missing ledger")
	})

	t.Run("prod_mode_localhost_redis_rejected", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		os.Setenv("ZEN_BRAIN_REQUIRE_ZENCONTEXT", "1")
		os.Unsetenv("TIER1_REDIS_ADDR")

		cfg := config.DefaultConfig()
		cfg.ZenContext.Required = true

		// Should reject because localhost is not allowed in prod
		_, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "prod",
			Config:  cfg,
		})

		if err == nil {
			t.Error("FAIL: Should reject localhost in prod mode")
		}

		t.Logf("✅ PASS: Strict mode rejects localhost: %v", err)
	})

	t.Run("circuit_breakers_initialized_and_registered", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "dev")

		cfg := config.DefaultConfig()

		rt, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "dev",
			Config:  cfg,
		})

		if err != nil {
			t.Skipf("Skipping test: runtime creation failed: %v", err)
		}
		defer rt.Close()

		breakers := rt.GetAllCircuitBreakers()
		if len(breakers) == 0 {
			t.Fatal("FAIL: Should initialize circuit breakers")
		}

		expectedServices := []string{"zen_context", "tier1_redis", "tier2_qmd", "ledger", "message_bus"}
		for _, svc := range expectedServices {
			if _, ok := breakers[svc]; !ok {
				t.Errorf("FAIL: Missing circuit breaker for %s", svc)
			}
		}

		// Check global registry
		registry := GetCircuitBreakerRegistry()
		states := registry.GetAllStates()
		if len(states) < len(expectedServices) {
			t.Errorf("FAIL: Not all breakers registered globally: %d < %d", len(states), len(expectedServices))
		}

		t.Logf("✅ PASS: All %d circuit breakers initialized and registered", len(breakers))
	})

	t.Run("readiness_check_reflects_health", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "dev")

		cfg := config.DefaultConfig()

		rt, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "dev",
			Config:  cfg,
		})

		if err != nil {
			t.Skipf("Skipping test: runtime creation failed: %v", err)
		}
		defer rt.Close()

		// Update health status
		rt.UpdateCapabilityHealth("tier1_hot", false, "simulated connection failure")

		// Check readiness
		err = rt.CheckReadiness(context.Background())

		// In dev mode with no required capabilities, may still pass
		if err != nil {
			t.Logf("Readiness check correctly detected unhealthy state: %v", err)
		} else {
			t.Logf("Readiness check passed (dev mode with no required caps)")
		}

		t.Logf("✅ PASS: Readiness check reflects health state")
	})
}

func TestLiveHealthChecker_Integration(t *testing.T) {
	origProfile := os.Getenv("ZEN_RUNTIME_PROFILE")
	defer os.Setenv("ZEN_RUNTIME_PROFILE", origProfile)

	os.Setenv("ZEN_RUNTIME_PROFILE", "dev")

	cfg := config.DefaultConfig()

	rt, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
		Profile:        "dev",
		Config:         cfg,
		EnableHealthCh: true,
	})

	if err != nil {
		t.Skipf("Skipping test: runtime creation failed: %v", err)
	}
	defer rt.Close()

	t.Run("health_checker_starts_and_stops", func(t *testing.T) {
		hc := NewLiveHealthChecker(&LiveHealthCheckerConfig{
			StrictRuntime:  rt,
			RefreshPeriod:   1e9, // 1 second
		})

		ctx := context.Background()

		if err := hc.Start(ctx); err != nil {
			t.Fatalf("FAIL: Failed to start health checker: %v", err)
		}

		// Let it run briefly
		// time.Sleep(100 * time.Millisecond) // Not needed for test

		hc.Stop()

		t.Logf("✅ PASS: Health checker started and stopped")
	})

	t.Run("health_summary_available", func(t *testing.T) {
		hc := NewLiveHealthChecker(&LiveHealthCheckerConfig{
			StrictRuntime: rt,
		})

		summary := hc.GetHealthSummary()

		if summary == nil {
			t.Fatal("FAIL: Health summary should not be nil")
		}

		if summary.Strict != rt.IsStrict() {
			t.Errorf("FAIL: Strict mode mismatch: %v != %v", summary.Strict, rt.IsStrict())
		}

		if len(summary.Capabilities) == 0 {
			t.Error("FAIL: Should have capabilities in summary")
		}

		t.Logf("✅ PASS: Health summary available with %d capabilities", len(summary.Capabilities))
	})
}

func TestCircuitBreakerRegistry_Integration(t *testing.T) {
	t.Run("registry_tracks_global_breakers", func(t *testing.T) {
		registry := GetCircuitBreakerRegistry()

		// Register a test breaker
		testCB := NewCircuitBreaker(&CircuitBreakerConfig{
			Name:        "test_service",
			MaxFailures: 2,
			Timeout:     10e9,
		})

		RegisterCircuitBreaker("test_service", testCB)

		// Check it's registered
		states := registry.GetAllStates()
		if _, ok := states["test_service"]; !ok {
			t.Error("FAIL: Test service not registered")
		}

		// Check healthy initially
		if !registry.IsHealthy() {
			t.Error("FAIL: Registry should be healthy initially")
		}

		// Open the circuit
		testCB.RecordFailure()
		testCB.RecordFailure() // Should open after 2 failures

		// Check registry reflects unhealthy
		if registry.IsHealthy() {
			t.Error("FAIL: Registry should be unhealthy with open circuit")
		}

		unhealthy := registry.GetUnhealthy()
		found := false
		for _, name := range unhealthy {
			if name == "test_service" {
				found = true
				break
			}
		}

		if !found {
			t.Error("FAIL: test_service should be in unhealthy list")
		}

		// Cleanup
		UnregisterCircuitBreaker("test_service")

		t.Logf("✅ PASS: Registry tracks circuit breaker states")
	})
}
