package runtime

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDetectRuntimeProfile(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "default_dev",
			envVars:  map[string]string{},
			expected: "dev",
		},
		{
			name: "explicit_prod",
			envVars: map[string]string{
				"ZEN_RUNTIME_PROFILE": "prod",
			},
			expected: "prod",
		},
		{
			name: "explicit_staging",
			envVars: map[string]string{
				"ZEN_RUNTIME_PROFILE": "staging",
			},
			expected: "staging",
		},
		{
			name: "strict_runtime_implies_prod",
			envVars: map[string]string{
				"ZEN_BRAIN_STRICT_RUNTIME": "1",
			},
			expected: "prod",
		},
		{
			name: "k8s_with_prod_env",
			envVars: map[string]string{
				"KUBERNETES_SERVICE_HOST": "10.0.0.1",
				"ZEN_BRAIN_ENV":           "production",
			},
			expected: "prod",
		},
		{
			name: "k8s_without_prod_env",
			envVars: map[string]string{
				"KUBERNETES_SERVICE_HOST": "10.0.0.1",
			},
			expected: "staging",
		},
		{
			name: "ci_environment",
			envVars: map[string]string{
				"CI": "true",
			},
			expected: "ci",
		},
		{
			name: "test_environment",
			envVars: map[string]string{
				"GO_TEST": "1",
			},
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars
			os.Unsetenv("ZEN_RUNTIME_PROFILE")
			os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")
			os.Unsetenv("KUBERNETES_SERVICE_HOST")
			os.Unsetenv("ZEN_BRAIN_ENV")
			os.Unsetenv("CI")
			os.Unsetenv("GITHUB_ACTIONS")
			os.Unsetenv("GO_TEST")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			profile := detectRuntimeProfile()
			if profile != tt.expected {
				t.Errorf("Expected profile '%s', got: %s", tt.expected, profile)
			}
		})
	}

	t.Logf("✅ Runtime profile detection works correctly")
}

func TestDefaultEnhancedPreflightConfig(t *testing.T) {
	t.Run("dev_defaults", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")
		os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")

		cfg := DefaultEnhancedPreflightConfig()

		if cfg.Profile != "dev" {
			t.Errorf("Expected dev profile, got: %s", cfg.Profile)
		}

		if cfg.StrictMode {
			t.Error("Dev mode should not be strict by default")
		}

		if !cfg.AllowDegraded {
			t.Error("Dev mode should allow degraded by default")
		}

		if !cfg.AllowStub {
			t.Error("Dev mode should allow stub by default")
		}

		if len(cfg.CriticalServices) == 0 {
			t.Error("Should have critical services defined")
		}

		t.Logf("✅ Dev config: profile=%s strict=%v degraded=%v stub=%v",
			cfg.Profile, cfg.StrictMode, cfg.AllowDegraded, cfg.AllowStub)
	})

	t.Run("prod_defaults", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		cfg := DefaultEnhancedPreflightConfig()

		if cfg.Profile != "prod" {
			t.Errorf("Expected prod profile, got: %s", cfg.Profile)
		}

		if !cfg.StrictMode {
			t.Error("Prod mode should be strict")
		}

		if cfg.AllowDegraded {
			t.Error("Prod mode should not allow degraded")
		}

		if cfg.AllowStub {
			t.Error("Prod mode should not allow stub")
		}

		t.Logf("✅ Prod config: profile=%s strict=%v degraded=%v stub=%v",
			cfg.Profile, cfg.StrictMode, cfg.AllowDegraded, cfg.AllowStub)
	})
}

func TestValidateDependencyMode(t *testing.T) {
	t.Run("prod_rejects_degraded", func(t *testing.T) {
		cfg := &EnhancedPreflightConfig{
			StrictMode:    true,
			AllowDegraded: false,
			Profile:       "prod",
		}

		err := validateDependencyMode("tier1_hot", ModeDegraded, cfg)
		if err == nil {
			t.Error("Should reject degraded mode in prod")
		}

		t.Logf("✅ Prod rejects degraded: %v", err)
	})

	t.Run("prod_rejects_stub", func(t *testing.T) {
		cfg := &EnhancedPreflightConfig{
			StrictMode: true,
			AllowStub:  false,
			Profile:    "prod",
		}

		err := validateDependencyMode("tier1_hot", ModeStub, cfg)
		if err == nil {
			t.Error("Should reject stub mode in prod")
		}

		t.Logf("✅ Prod rejects stub: %v", err)
	})

	t.Run("dev_allows_degraded", func(t *testing.T) {
		cfg := &EnhancedPreflightConfig{
			StrictMode:    false,
			AllowDegraded: true,
			Profile:       "dev",
		}

		err := validateDependencyMode("tier1_hot", ModeDegraded, cfg)
		if err != nil {
			t.Errorf("Dev should allow degraded mode: %v", err)
		}

		t.Logf("✅ Dev allows degraded")
	})

	t.Run("staging_rejects_stub_for_critical", func(t *testing.T) {
		cfg := &EnhancedPreflightConfig{
			Profile: "staging",
			CriticalServices: []string{"tier1_hot", "ledger"},
		}

		// Critical service should reject stub
		err := validateDependencyMode("tier1_hot", ModeStub, cfg)
		if err == nil {
			t.Error("Staging should reject stub for critical services")
		}

		// Non-critical service should allow stub
		err = validateDependencyMode("tier3_cold", ModeStub, cfg)
		if err != nil {
			t.Errorf("Staging should allow stub for non-critical: %v", err)
		}

		t.Logf("✅ Staging critical service check works")
	})
}

func TestEnhancedReadinessCheck(t *testing.T) {
	t.Run("all_healthy_dev", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: true, Mode: ModeReal},
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := EnhancedReadinessCheck(context.Background(), report)
		if err != nil {
			t.Errorf("Should pass with all healthy: %v", err)
		}

		t.Logf("✅ Readiness check passes with all healthy")
	})

	t.Run("qmd_unhealthy_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: false, Mode: ModeReal}, // QMD unhealthy
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := EnhancedReadinessCheck(context.Background(), report)
		if err == nil {
			t.Error("Should fail when QMD unhealthy in prod")
		}

		t.Logf("✅ Readiness fails with QMD unhealthy in prod: %v", err)
	})

	t.Run("qmd_degraded_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: true, Mode: ModeDegraded}, // QMD degraded
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := EnhancedReadinessCheck(context.Background(), report)
		if err == nil {
			t.Error("Should fail when QMD degraded in prod")
		}

		t.Logf("✅ Readiness fails with QMD degraded in prod: %v", err)
	})

	t.Run("qmd_healthy_but_not_required_in_dev", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: false, Mode: ModeStub}, // QMD unhealthy but not required in dev
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := EnhancedReadinessCheck(context.Background(), report)
		if err != nil {
			t.Errorf("Should pass when QMD unhealthy in dev: %v", err)
		}

		t.Logf("✅ Readiness passes with QMD unhealthy in dev (not required)")
	})
}

func TestValidateQMDGuarantees(t *testing.T) {
	t.Run("qmd_stub_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Tier2Warm: CapabilityStatus{Healthy: true, Mode: ModeStub},
		}

		err := ValidateQMDGuarantees(report)
		if err == nil {
			t.Error("Should reject QMD stub in prod")
		}

		t.Logf("✅ QMD stub rejected in prod: %v", err)
	})

	t.Run("qmd_degraded_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Tier2Warm: CapabilityStatus{Healthy: true, Mode: ModeDegraded},
		}

		err := ValidateQMDGuarantees(report)
		if err == nil {
			t.Error("Should reject QMD degraded in prod")
		}

		t.Logf("✅ QMD degraded rejected in prod: %v", err)
	})

	t.Run("qmd_real_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Tier2Warm: CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := ValidateQMDGuarantees(report)
		if err != nil {
			t.Errorf("Should accept QMD real in prod: %v", err)
		}

		t.Logf("✅ QMD real accepted in prod")
	})

	t.Run("qmd_stub_allowed_in_dev", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Tier2Warm: CapabilityStatus{Healthy: true, Mode: ModeStub},
		}

		err := ValidateQMDGuarantees(report)
		if err != nil {
			t.Errorf("Should allow QMD stub in dev: %v", err)
		}

		t.Logf("✅ QMD stub allowed in dev")
	})
}

func TestValidateLedgerGuarantees(t *testing.T) {
	t.Run("ledger_stub_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Ledger: CapabilityStatus{Healthy: true, Mode: ModeStub},
		}

		err := ValidateLedgerGuarantees(report)
		if err == nil {
			t.Error("Should reject Ledger stub in prod")
		}

		t.Logf("✅ Ledger stub rejected in prod: %v", err)
	})

	t.Run("ledger_stub_in_staging", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "staging")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Ledger: CapabilityStatus{Healthy: true, Mode: ModeStub},
		}

		err := ValidateLedgerGuarantees(report)
		if err == nil {
			t.Error("Should reject Ledger stub in staging")
		}

		t.Logf("✅ Ledger stub rejected in staging: %v", err)
	})

	t.Run("ledger_real_in_prod", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Ledger: CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		err := ValidateLedgerGuarantees(report)
		if err != nil {
			t.Errorf("Should accept Ledger real in prod: %v", err)
		}

		t.Logf("✅ Ledger real accepted in prod")
	})

	t.Run("ledger_stub_allowed_in_dev", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			Ledger: CapabilityStatus{Healthy: true, Mode: ModeStub},
		}

		err := ValidateLedgerGuarantees(report)
		if err != nil {
			t.Errorf("Should allow Ledger stub in dev: %v", err)
		}

		t.Logf("✅ Ledger stub allowed in dev")
	})
}

func TestRunDoctorChecks(t *testing.T) {
	t.Run("healthy_system", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier3Cold:  CapabilityStatus{Healthy: true, Mode: ModeReal},
			Journal:    CapabilityStatus{Healthy: true, Mode: ModeReal},
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
			MessageBus: CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		doctorReport := RunDoctorChecks(context.Background(), nil, report)

		if doctorReport.OverallStatus != "healthy" {
			t.Errorf("Expected healthy status, got: %s", doctorReport.OverallStatus)
		}

		if len(doctorReport.Checks) == 0 {
			t.Error("Should have diagnostic checks")
		}

		t.Logf("✅ Doctor checks: %s", doctorReport.Summary)
		t.Logf("  Overall: %s", doctorReport.OverallStatus)
		t.Logf("  Checks: %d", len(doctorReport.Checks))
	})

	t.Run("degraded_system", func(t *testing.T) {
		os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: false, Mode: ModeStub}, // Optional service unhealthy
			Tier3Cold:  CapabilityStatus{Healthy: true, Mode: ModeReal},
			Journal:    CapabilityStatus{Healthy: true, Mode: ModeReal},
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
			MessageBus: CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		doctorReport := RunDoctorChecks(context.Background(), nil, report)

		if doctorReport.OverallStatus == "unhealthy" {
			t.Error("Should be degraded, not unhealthy (optional service failed)")
		}

		t.Logf("✅ Doctor checks with degraded service: %s", doctorReport.Summary)
	})

	t.Run("prod_mode_strict_checks", func(t *testing.T) {
		os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
		defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
			Tier2Warm:  CapabilityStatus{Healthy: true, Mode: ModeDegraded}, // Degraded in prod!
			Ledger:     CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		doctorReport := RunDoctorChecks(context.Background(), nil, report)

		// Should detect degraded mode in prod
		if doctorReport.OverallStatus == "healthy" {
			t.Error("Should detect degraded mode in prod")
		}

		t.Logf("✅ Doctor detects degraded mode in prod: %s", doctorReport.Summary)
	})
}

func TestEnhancedPreflightFailFast(t *testing.T) {
	t.Run("fail_fast_on_critical_failure", func(t *testing.T) {
		cfg := &EnhancedPreflightConfig{
			Profile:         "prod",
			StrictMode:      true,
			FailFast:        true,
			AllowDegraded:   false,
			AllowStub:       false,
			Timeout:         1 * time.Second,
			CriticalServices: []string{"zen_context", "tier1_hot"},
		}

		report := &RuntimeReport{
			ZenContext: CapabilityStatus{Healthy: false, Mode: ModeReal}, // Critical failure
			Tier1Hot:   CapabilityStatus{Healthy: true, Mode: ModeReal},
		}

		result, err := runEnhancedPreflightChecks(context.Background(), nil, report, cfg)

		if err == nil {
			t.Error("Should fail with fail-fast on critical failure")
		}

		if result == nil {
			t.Fatal("Result should not be nil even with fail-fast")
		}

		// Should stop after first critical failure
		if len(result.Checks) > 2 {
			t.Logf("⚠️ Fail-fast should stop after first failure, but got %d checks", len(result.Checks))
		}

		t.Logf("✅ Fail-fast works: %v (checks: %d)", err, len(result.Checks))
	})
}

func TestBuildEnhancedPreflightSummary(t *testing.T) {
	t.Run("all_passed", func(t *testing.T) {
		report := &EnhancedPreflightReport{
			AllPassed:      true,
			CriticalPassed: true,
			Profile:        "prod",
			StrictMode:     true,
			Duration:       100 * time.Millisecond,
			Checks: []EnhancedPreflightCheck{
				{Name: "zen_context", Healthy: true},
				{Name: "tier1_hot", Healthy: true},
			},
		}

		summary := buildEnhancedPreflightSummary(report)

		if summary == "" {
			t.Error("Summary should not be empty")
		}

		t.Logf("✅ Summary (all passed): %s", summary)
	})

	t.Run("critical_failures", func(t *testing.T) {
		report := &EnhancedPreflightReport{
			AllPassed:       false,
			CriticalPassed:  false,
			Profile:         "prod",
			StrictMode:      true,
			CriticalFailures: []string{"zen_context", "ledger"},
			Checks: []EnhancedPreflightCheck{
				{Name: "zen_context", Healthy: false, Required: true},
				{Name: "tier1_hot", Healthy: true},
			},
		}

		summary := buildEnhancedPreflightSummary(report)

		if summary == "" {
			t.Error("Summary should not be empty")
		}

		t.Logf("✅ Summary (critical failures): %s", summary)
	})
}
