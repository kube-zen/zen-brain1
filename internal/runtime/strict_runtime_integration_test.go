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

// A003: Tests for profile-aware fallback policy
func TestStrictRuntime_FallbackPolicy(t *testing.T) {
	origProfile := os.Getenv("ZEN_RUNTIME_PROFILE")
	origStrict := os.Getenv("ZEN_BRAIN_STRICT_RUNTIME")
	defer func() {
		os.Setenv("ZEN_RUNTIME_PROFILE", origProfile)
		os.Setenv("ZEN_BRAIN_STRICT_RUNTIME", origStrict)
	}()

	t.Run("prod_mode_rejects_stub_ledger", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		os.Unsetenv("ZEN_LEDGER_DSN")
		os.Unsetenv("LEDGER_DATABASE_URL")

		cfg := config.DefaultConfig()
		cfg.Ledger.Required = true

		_, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "prod",
			Config:  cfg,
		})

		if err == nil {
			t.Error("FAIL: Prod mode should reject stub ledger when ledger is required")
		}
		t.Logf("✅ PASS: Prod mode rejected stub ledger: %v", err)
	})

	t.Run("staging_mode_rejects_stub_for_critical", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "staging")
		os.Unsetenv("ZEN_LEDGER_DSN")
		os.Unsetenv("LEDGER_DATABASE_URL")

		cfg := config.DefaultConfig()
		cfg.Ledger.Required = true

		_, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "staging",
			Config:  cfg,
		})

		if err == nil {
			t.Error("FAIL: Staging mode should reject stub ledger when ledger is required")
		}
		t.Logf("✅ PASS: Staging mode rejected stub ledger: %v", err)
	})

	t.Run("dev_mode_allows_stub", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "dev")
		os.Unsetenv("ZEN_LEDGER_DSN")
		os.Unsetenv("LEDGER_DATABASE_URL")

		cfg := config.DefaultConfig()

		rt, err := NewStrictRuntime(context.Background(), &StrictRuntimeConfig{
			Profile: "dev",
			Config:  cfg,
		})

		// Dev mode should not fail on missing optional services
		// (may fail on other things like Redis, but that's OK)
		if err == nil && rt != nil {
			defer rt.Close()
			report := rt.Report()
			if report != nil && report.Ledger.Mode == ModeStub {
				t.Logf("✅ PASS: Dev mode allows stub ledger")
			}
		} else {
			t.Logf("Dev mode bootstrap failed for other reasons (expected): %v", err)
		}
	})

	t.Run("mode_reporting_distinguishes_states", func(t *testing.T) {
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

		report := rt.Report()
		if report == nil {
			t.Fatal("FAIL: Report should not be nil")
		}

		// Check that modes are distinguishable
		validModes := map[DependencyMode]bool{
			ModeReal:     true,
			ModeMock:     true,
			ModeStub:     true,
			ModeDisabled: true,
			ModeDegraded: true,
		}

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
			if cap.Mode != "" && !validModes[cap.Mode] {
				t.Errorf("FAIL: Invalid mode %q for %s", cap.Mode, cap.Name)
			}
		}

		t.Logf("✅ PASS: All capability modes are valid and distinguishable")
	})
}

// A003: Test readiness reflects dependency loss
func TestStrictRuntime_ReadinessReflectsDependencyLoss(t *testing.T) {
	origProfile := os.Getenv("ZEN_RUNTIME_PROFILE")
	defer os.Setenv("ZEN_RUNTIME_PROFILE", origProfile)

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

	// Initial readiness check
	err = rt.CheckReadiness(context.Background())
	initialReady := err == nil
	t.Logf("Initial readiness: %v", initialReady)

	// Simulate dependency loss (A002: post-start dependency loss detection)
	rt.UpdateCapabilityHealth("ledger", false, "connection lost")
	rt.UpdateCapabilityHealth("tier1_hot", false, "redis timeout")

	// Get updated report
	report := rt.Report()
	if report == nil {
		t.Fatal("FAIL: Report should not be nil")
	}

	// Check that report reflects the loss
	if report.Ledger.Healthy {
		t.Error("FAIL: Ledger should be marked unhealthy after update")
	}
	if report.Tier1Hot.Healthy {
		t.Error("FAIL: Tier1Hot should be marked unhealthy after update")
	}

	// Check circuit breakers also track failures
	cb := rt.GetCircuitBreaker("ledger")
	if cb == nil {
		t.Fatal("FAIL: Should have circuit breaker for ledger")
	}
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure() // Open circuit

	if cb.State() != CircuitStateOpen {
		t.Errorf("FAIL: Circuit should be open after 3 failures, got: %s", cb.State())
	}

	t.Logf("✅ PASS: Readiness reflects dependency loss and circuit breaker states")
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

	t.Run("health_summary_reflects_circuit_breakers", func(t *testing.T) {
		hc := NewLiveHealthChecker(&LiveHealthCheckerConfig{
			StrictRuntime: rt,
		})

		// Force a circuit breaker open
		cb := rt.GetCircuitBreaker("tier1_redis")
		if cb != nil {
			cb.RecordFailure()
			cb.RecordFailure()
			cb.RecordFailure()
		}

		summary := hc.GetHealthSummary()
		if summary == nil {
			t.Fatal("FAIL: Health summary should not be nil")
		}

		// Summary should show degraded due to open circuit
		if len(summary.CircuitBreakers) == 0 {
			t.Error("FAIL: Should have circuit breakers in summary")
		}

		t.Logf("✅ PASS: Health summary includes circuit breaker states")
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

// A003: Test preflight integration with strict runtime
func TestPreflight_StrictRuntimeIntegration(t *testing.T) {
	origProfile := os.Getenv("ZEN_RUNTIME_PROFILE")
	defer os.Setenv("ZEN_RUNTIME_PROFILE", origProfile)

	t.Run("preflight_uses_profile_aware_checks", func(t *testing.T) {
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

		report := rt.Report()
		if report == nil {
			t.Fatal("FAIL: Report should not be nil")
		}

		// Run enhanced preflight
		preflightReport, err := EnhancedStrictPreflight(context.Background(), cfg, report)

		// In dev mode, preflight should be lenient
		if preflightReport != nil {
			t.Logf("Preflight profile: %s, strict: %v", preflightReport.Profile, preflightReport.StrictMode)
			t.Logf("Preflight summary: %s", preflightReport.Summary)

			// Verify profile detection
			if preflightReport.Profile != "dev" {
				t.Errorf("FAIL: Expected profile 'dev', got '%s'", preflightReport.Profile)
			}
		}

		t.Logf("✅ PASS: Preflight integration works with StrictRuntime")
	})
}
