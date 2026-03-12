package runtime

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// PreflightConfig holds configuration for preflight checks.
type PreflightConfig struct {
	// Strict enables fail-closed behavior (prod mode).
	Strict bool `json:"strict" yaml:"strict"`

	// Timeout is the timeout for each individual health check.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// CriticalServices lists services that MUST be healthy in strict mode.
	CriticalServices []string `json:"critical_services" yaml:"critical_services"`

	// AllowDegraded allows degraded modes in non-strict mode.
	AllowDegraded bool `json:"allow_degraded" yaml:"allow_degraded"`
}

// DefaultPreflightConfig returns the default preflight configuration.
func DefaultPreflightConfig() *PreflightConfig {
	return &PreflightConfig{
		Strict:    false,
		Timeout:   5 * time.Second,
		AllowDegraded: true,
		CriticalServices: []string{
			"zen_context",
			"tier1_hot",
			"ledger",
		},
	}
}

// PreflightCheck represents the result of a single preflight check.
type PreflightCheck struct {
	Name     string        `json:"name"`
	Healthy  bool          `json:"healthy"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	Required bool          `json:"required"`
	Skipped  bool          `json:"skipped,omitempty"`
}

// PreflightReport contains all preflight check results.
type PreflightReport struct {
	Strict     bool            `json:"strict"`
	AllPassed  bool            `json:"all_passed"`
	Timestamp  time.Time       `json:"timestamp"`
	Checks     []PreflightCheck `json:"checks"`
	Summary    string          `json:"summary"`
}

// StrictPreflight performs deterministic preflight checks with fail-closed behavior in strict mode.
// In strict mode, ALL critical services must be healthy. In non-strict mode, only required services must be healthy.
func StrictPreflight(ctx context.Context, cfg *config.Config, report *RuntimeReport) (*PreflightReport, error) {
	preflightConfig := DefaultPreflightConfig()

	// Determine strictness from env and config
	if IsStrictProfile() || os.Getenv("ZEN_BRAIN_PREFLIGHT_STRICT") == "true" {
		preflightConfig.Strict = true
		preflightConfig.AllowDegraded = false
	}

	if os.Getenv("ZEN_BRAIN_PREFLIGHT_TIMEOUT") != "" {
		if timeout, err := time.ParseDuration(os.Getenv("ZEN_BRAIN_PREFLIGHT_TIMEOUT")); err == nil {
			preflightConfig.Timeout = timeout
		}
	}

	return runPreflightChecks(ctx, cfg, report, preflightConfig)
}

// convertEnhancedToLegacy converts an EnhancedPreflightReport to the legacy PreflightReport.
func convertEnhancedToLegacy(enhanced *EnhancedPreflightReport) *PreflightReport {
	checks := make([]PreflightCheck, len(enhanced.Checks))
	for i, ec := range enhanced.Checks {
		checks[i] = PreflightCheck{
			Name:     ec.Name,
			Healthy:  ec.Healthy,
			Duration: ec.Duration,
			Error:    "", // Enhanced check does not expose error separately
			Required: ec.Required,
			Skipped:  false,
		}
	}
	return &PreflightReport{
		Strict:    enhanced.StrictMode,
		AllPassed: enhanced.AllPassed,
		Timestamp: enhanced.Timestamp,
		Checks:    checks,
		Summary:   enhanced.Summary,
	}
}

// runPreflightChecks executes all preflight checks deterministically.
func runPreflightChecks(ctx context.Context, cfg *config.Config, report *RuntimeReport, pc *PreflightConfig) (*PreflightReport, error) {
	start := time.Now()
	preflightReport := &PreflightReport{
		Strict:    pc.Strict,
		Timestamp: start,
		Checks:    []PreflightCheck{},
	}

	// Determine required capabilities from config and environment
	req := GetRequirements(cfg)

	// 1. ZenContext (critical)
	zenContextRequired := req.ZenContext
	check := runPreflightCheck(ctx, "zen_context", report.ZenContext, pc.Timeout, zenContextRequired)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 2. Tier1 Hot (critical - Redis) - required if ZenContext required
	tier1Required := req.ZenContext
	check = runPreflightCheck(ctx, "tier1_hot", report.Tier1Hot, pc.Timeout, tier1Required)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 3. Tier2 Warm (optional - QMD)
	// QMD required if req.QMD, or if strict mode and QMD is configured (real)
	qmdRequired := req.QMD || (pc.Strict && report.Tier2Warm.Mode == ModeReal)
	check = runPreflightCheck(ctx, "tier2_warm", report.Tier2Warm, pc.Timeout, qmdRequired)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 4. Tier3 Cold (optional - S3)
	s3Required := pc.Strict && report.Tier3Cold.Mode == ModeReal
	check = runPreflightCheck(ctx, "tier3_cold", report.Tier3Cold, pc.Timeout, s3Required)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 5. Journal (optional)
	journalRequired := pc.Strict && report.Journal.Mode == ModeReal
	check = runPreflightCheck(ctx, "journal", report.Journal, pc.Timeout, journalRequired)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 6. Ledger (critical in prod)
	ledgerRequired := req.Ledger || (pc.Strict && report.Ledger.Mode == ModeReal)
	check = runPreflightCheck(ctx, "ledger", report.Ledger, pc.Timeout, ledgerRequired)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// 7. MessageBus (required if configured)
	busRequired := req.MessageBus || (pc.Strict && report.MessageBus.Mode == ModeReal)
	check = runPreflightCheck(ctx, "message_bus", report.MessageBus, pc.Timeout, busRequired)
	preflightReport.Checks = append(preflightReport.Checks, check)

	// Determine overall status
	allPassed := true
	criticalFailures := []string{}
	for _, check := range preflightReport.Checks {
		if !check.Healthy && check.Required {
			allPassed = false
			criticalFailures = append(criticalFailures, check.Name)
		}
	}

	preflightReport.AllPassed = allPassed
	preflightReport.Summary = buildPreflightSummary(preflightReport, criticalFailures)

	if !allPassed && pc.Strict {
		return preflightReport, fmt.Errorf("preflight checks failed in strict mode: %v", criticalFailures)
	}

	return preflightReport, nil
}

// runPreflightCheck executes a single health check with timeout.
func runPreflightCheck(ctx context.Context, name string, status CapabilityStatus, timeout time.Duration, required bool) PreflightCheck {
	start := time.Now()
	check := PreflightCheck{
		Name:     name,
		Required: required,
		Healthy:  status.Healthy,
	}

	// If capability is disabled and not required, skip
	if status.Mode == ModeDisabled && !required {
		check.Skipped = true
		check.Duration = time.Since(start)
		return check
	}

	// If capability is degraded and degraded is not allowed, fail
	if status.Mode == ModeDegraded && !required {
		check.Healthy = false
		check.Error = fmt.Sprintf("degraded mode not allowed for %s", name)
		check.Duration = time.Since(start)
		return check
	}

	// If capability is stub and required in strict mode, fail
	if status.Mode == ModeStub && required {
		check.Healthy = false
		check.Error = fmt.Sprintf("stub not allowed for %s in strict mode", name)
		check.Duration = time.Since(start)
		return check
	}

	// Use status health from bootstrap
	if !status.Healthy && status.Message != "" {
		check.Error = status.Message
	}

	check.Duration = time.Since(start)
	return check
}

// buildPreflightSummary creates a human-readable summary.
func buildPreflightSummary(report *PreflightReport, criticalFailures []string) string {
	if report.AllPassed {
		return fmt.Sprintf("All %d checks passed (strict=%v)", len(report.Checks), report.Strict)
	}

	passed := 0
	failed := 0
	skipped := 0
	for _, check := range report.Checks {
		if check.Skipped {
			skipped++
		} else if check.Healthy {
			passed++
		} else {
			failed++
		}
	}

	if len(criticalFailures) > 0 {
		return fmt.Sprintf("FAILED: %d passed, %d failed, %d skipped (critical failures: %v)",
			passed, failed, skipped, criticalFailures)
	}

	return fmt.Sprintf("WARNING: %d passed, %d failed, %d skipped (non-critical)",
		passed, failed, skipped)
}

// ReadinessCheck performs lightweight readiness checks for k8s readiness probes.
// This is a subset of preflight checks that are safe to run frequently.
func ReadinessCheck(ctx context.Context, report *RuntimeReport) error {
	if report == nil {
		return fmt.Errorf("no runtime report")
	}

	// Only check critical services for readiness
	criticalChecks := []struct {
		name string
		status CapabilityStatus
	}{
		{"zen_context", report.ZenContext},
		{"tier1_hot", report.Tier1Hot},
		{"ledger", report.Ledger},
	}

	for _, check := range criticalChecks {
		if check.status.Required && !check.status.Healthy {
			return fmt.Errorf("critical service %s not healthy: %s", check.name, check.status.Message)
		}
	}

	return nil
}

// LivenessCheck performs a minimal check to verify the runtime is not deadlocked.
func LivenessCheck(ctx context.Context) error {
	// Basic liveness: can we complete a context operation?
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ValidateRuntimeGuarantees checks that runtime guarantees are met based on mode.
func ValidateRuntimeGuarantees(report *RuntimeReport) error {
	if report == nil {
		return fmt.Errorf("no runtime report")
	}

	// In strict mode, validate that no services are in degraded/stub mode
	if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		for _, cap := range []CapabilityStatus{
			report.ZenContext,
			report.Tier1Hot,
			report.Ledger,
		} {
			if cap.Required {
				if cap.Mode == ModeDegraded {
					return fmt.Errorf("runtime guarantee violated: %s in degraded mode", cap.Name)
				}
				if cap.Mode == ModeStub {
					return fmt.Errorf("runtime guarantee violated: %s in stub mode", cap.Name)
				}
				if !cap.Healthy {
					return fmt.Errorf("runtime guarantee violated: %s unhealthy", cap.Name)
				}
			}
		}
	}

	return nil
}
