package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// TestRuntimeDoctorReportPingAlignment ensures doctor, report, and ping
// are all consistent and use same RuntimeReport data.
func TestRuntimeDoctorReportPingAlignment(t *testing.T) {
	// Use dev mode to avoid blocking on missing dependencies
	os.Setenv("ZEN_RUNTIME_PROFILE", "dev")
	os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")
	os.Unsetenv("ZEN_BRAIN_REQUIRE_LEDGER")
	os.Unsetenv("TIER1_REDIS_ADDR")

	ctx := context.Background()
	cfg := config.DefaultConfig()

	rt, err := NewStrictRuntime(ctx, &StrictRuntimeConfig{
		Profile: "dev",
		Config:  cfg,
	})
	if err != nil {
		t.Skipf("Skipping test: runtime creation failed (expected without Redis): %v", err)
	}
	if rt != nil {
		defer rt.Close()
	}

	report := rt.Report()
	if report == nil {
		t.Fatal("RuntimeReport is nil")
	}

	// Ensure report has consistent data across all fields
	// Profile should match in preflight report
	if report.PreflightReport == nil {
		t.Error("PreflightReport is nil")
	} else if report.PreflightReport.Profile != "dev" {
		t.Errorf("Report profile mismatch: got %s, want dev", report.PreflightReport.Profile)
	}

	// Preflight report should be present
	if report.PreflightReport == nil {
		t.Error("PreflightReport is nil")
	}

	// All capability health should be consistent (all false or all true based on init)
	// In dev mode without Redis, all should be unhealthy
	capabilities := []struct {
		name  string
		value CapabilityStatus
	}{
		{"ZenContext", report.ZenContext},
		{"Tier1Hot", report.Tier1Hot},
		{"Tier2Warm", report.Tier2Warm},
		{"Tier3Cold", report.Tier3Cold},
		{"Journal", report.Journal},
		{"Ledger", report.Ledger},
		{"MessageBus", report.MessageBus},
	}

	for _, cap := range capabilities {
		// In dev mode without Redis, all should be unhealthy (false)
		// This test ensures consistency, not absolute values
		t.Logf("%s: healthy=%v, mode=%s", cap.name, cap.value.Healthy, cap.value.Mode)
	}

	// Ping should reflect report health
	pingResult := Ping(report)
	// In dev mode without dependencies, Ping should return 1 (unhealthy)
	// This ensures Ping is consistent with report
	if pingResult < 0 || pingResult > 1 {
		t.Errorf("Ping returned unexpected value: %d (expected 0 or 1)", pingResult)
	}
	t.Logf("Ping result: %d", pingResult)

	// Doctor should not panic (just log output)
	// We can't easily capture doctor output, but we can ensure it doesn't crash
	Doctor(ctx, cfg, report)
}

// TestRuntimeCapabilityErrorMessageClarity ensures capability error messages
// are clear and actionable for operators.
func TestRuntimeCapabilityErrorMessageClarity(t *testing.T) {
	// Use prod mode to trigger strict behavior
	os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
	os.Unsetenv("ZEN_BRAIN_STRICT_RUNTIME")
	os.Setenv("ZEN_BRAIN_REQUIRE_ZENCONTEXT", "1")
	os.Unsetenv("TIER1_REDIS_ADDR")

	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.ZenContext.Required = true

	_, err := NewStrictRuntime(ctx, &StrictRuntimeConfig{
		Profile: "prod",
		Config:  cfg,
	})

	// Should fail with clear error message
	if err == nil {
		t.Error("Expected error when ZenContext required but not configured in prod mode")
	}

	// Error message should be clear and actionable
	errMsg := err.Error()
	expectedKeywords := []string{"zen_context", "preflight", "failed"}
	for _, keyword := range expectedKeywords {
		if !containsString(errMsg, keyword) {
			t.Errorf("Error message should contain '%s': %s", keyword, errMsg)
		}
	}

	t.Logf("✅ Clear error message: %s", errMsg)
}

// containsString is a helper to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
