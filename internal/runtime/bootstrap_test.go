package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
)

func TestBootstrap_WithDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := context.Background()
	rt, err := Bootstrap(ctx, cfg)
	if err != nil {
		// Bootstrap may fail if Redis not available; we still get a report
		t.Logf("Bootstrap returned error (expected in CI): %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil Runtime even on error")
	}
	if rt.Report == nil {
		t.Fatal("expected non-nil Report")
	}
	// ZenContext may be real or degraded depending on env
	if rt.Report.ZenContext.Name != "zen_context" {
		t.Errorf("report.ZenContext.Name = %s", rt.Report.ZenContext.Name)
	}
	if rt.Report.Ledger.Name != "ledger" {
		t.Errorf("report.Ledger.Name = %s", rt.Report.Ledger.Name)
	}
}

func TestStrictness_Env(t *testing.T) {
	os.Setenv("ZEN_BRAIN_REQUIRE_LEDGER", "1")
	defer os.Unsetenv("ZEN_BRAIN_REQUIRE_LEDGER")
	cfg := config.DefaultConfig()
	_, _, reqLedger, _ := strictness(cfg)
	if !reqLedger {
		t.Error("expected requireLedger true when ZEN_BRAIN_REQUIRE_LEDGER set")
	}
}

func TestInferTier2Tier3Journal(t *testing.T) {
	t2, t3, j := inferTier2Tier3Journal(nil)
	if t2.Mode != ModeDisabled || t3.Mode != ModeDisabled || j.Mode != ModeDisabled {
		t.Errorf("nil config should yield disabled: tier2=%s tier3=%s journal=%s", t2.Mode, t3.Mode, j.Mode)
	}
}
