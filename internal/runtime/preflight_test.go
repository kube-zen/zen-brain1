package runtime

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStrictPreflight_AllPassed(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Tier2Warm:  CapabilityStatus{Name: "tier2_warm", Mode: ModeReal, Healthy: true},
		Tier3Cold:  CapabilityStatus{Name: "tier3_cold", Mode: ModeReal, Healthy: true},
		Journal:    CapabilityStatus{Name: "journal", Mode: ModeReal, Healthy: true},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
		MessageBus: CapabilityStatus{Name: "message_bus", Mode: ModeReal, Healthy: true},
	}

	ctx := context.Background()
	preflight, err := StrictPreflight(ctx, nil, report)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if !preflight.AllPassed {
		t.Errorf("Expected all checks to pass, got failures")
	}

	t.Logf("Preflight summary: %s", preflight.Summary)
}

func TestStrictPreflight_CriticalFailure(t *testing.T) {
	// Set strict mode
	os.Setenv("ZEN_BRAIN_STRICT_RUNTIME", "1")
	defer os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")

	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: false, Required: true, Message: "connection refused"},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
	}

	ctx := context.Background()
	preflight, err := StrictPreflight(ctx, nil, report)
	if err == nil {
		t.Fatal("Expected error when critical service fails in strict mode")
	}

	if preflight.AllPassed {
		t.Error("Expected AllPassed=false when critical service fails")
	}

	t.Logf("Got expected error: %v", err)
}

func TestStrictPreflight_NonCriticalFailure(t *testing.T) {
	// Non-strict mode
	os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")

	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Tier2Warm:  CapabilityStatus{Name: "tier2_warm", Mode: ModeReal, Healthy: false, Message: "QMD unavailable"},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
	}

	ctx := context.Background()
	preflight, err := StrictPreflight(ctx, nil, report)
	if err != nil {
		t.Fatalf("Unexpected error in non-strict mode: %v", err)
	}

	if !preflight.AllPassed {
		t.Error("Expected AllPassed=true when only non-critical service fails in non-strict mode")
	}

	t.Logf("Preflight summary: %s", preflight.Summary)
}

func TestStrictPreflight_DegradedNotAllowed(t *testing.T) {
	os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
	defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeDegraded, Healthy: false, Required: true, Message: "degraded mode"},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
	}

	ctx := context.Background()
	preflight, err := StrictPreflight(ctx, nil, report)
	if err == nil {
		t.Fatal("Expected error when degraded mode in prod")
	}

	if preflight.AllPassed {
		t.Error("Expected AllPassed=false when degraded in prod")
	}

	t.Logf("Got expected error: %v", err)
}

func TestStrictPreflight_StubNotAllowedInProd(t *testing.T) {
	os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
	defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: true, Message: "using stub"},
	}

	ctx := context.Background()
	_, err := StrictPreflight(ctx, nil, report)
	if err == nil {
		t.Fatal("Expected error when stub in prod mode")
	}

	t.Logf("Got expected error: %v", err)
}

func TestReadinessCheck_CriticalServices(t *testing.T) {
	tests := []struct {
		name        string
		report      *RuntimeReport
		expectError bool
	}{
		{
			name: "all critical healthy",
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
			},
			expectError: false,
		},
		{
			name: "critical service unhealthy",
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: false, Required: true, Message: "redis down"},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
			},
			expectError: true,
		},
		{
			name: "optional service unhealthy",
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Tier2Warm:  CapabilityStatus{Name: "tier2_warm", Mode: ModeReal, Healthy: false, Message: "QMD down"},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := ReadinessCheck(ctx, tt.report)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err != nil {
				t.Logf("Got error: %v", err)
			}
		})
	}
}

func TestLivenessCheck(t *testing.T) {
	ctx := context.Background()
	err := LivenessCheck(ctx)
	if err != nil {
		t.Errorf("Unexpected liveness check failure: %v", err)
	}
}

func TestValidateRuntimeGuarantees(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		report      *RuntimeReport
		expectError bool
	}{
		{
			name: "strict_mode_all_healthy",
			env:  map[string]string{"ZEN_RUNTIME_PROFILE": "prod"},
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
			},
			expectError: false,
		},
		{
			name: "strict_mode_degraded_violation",
			env:  map[string]string{"ZEN_RUNTIME_PROFILE": "prod"},
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeDegraded, Healthy: false, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeReal, Healthy: true, Required: true},
			},
			expectError: true,
		},
		{
			name: "strict_mode_stub_violation",
			env:  map[string]string{"ZEN_BRAIN_STRICT_RUNTIME": "1"},
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: true},
			},
			expectError: true,
		},
		{
			name: "non_strict_mode_allows_stub",
			env:  map[string]string{},
			report: &RuntimeReport{
				ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
				Tier1Hot:   CapabilityStatus{Name: "tier1_hot", Mode: ModeReal, Healthy: true, Required: true},
				Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: true},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()

			err := ValidateRuntimeGuarantees(tt.report)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err != nil {
				t.Logf("Got error: %v", err)
			}
		})
	}
}

func TestPreflightConfig_Default(t *testing.T) {
	cfg := DefaultPreflightConfig()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", cfg.Timeout)
	}

	if len(cfg.CriticalServices) == 0 {
		t.Error("Expected at least one critical service")
	}

	t.Logf("Critical services: %v", cfg.CriticalServices)
}
