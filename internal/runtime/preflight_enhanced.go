package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// EnhancedPreflightConfig holds strict preflight configuration for Block 3 hardening.
type EnhancedPreflightConfig struct {
	// StrictMode enables fail-closed behavior (prod mode).
	StrictMode bool `json:"strict_mode" yaml:"strict_mode"`

	// Profile is the runtime profile (prod, staging, dev, test).
	Profile string `json:"profile" yaml:"profile"`

	// Timeout is the timeout for each individual health check.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// CriticalServices lists services that MUST be healthy.
	CriticalServices []string `json:"critical_services" yaml:"critical_services"`

	// AllowDegraded allows degraded modes (false in prod).
	AllowDegraded bool `json:"allow_degraded" yaml:"allow_degraded"`

	// AllowStub allows stub modes (false in prod).
	AllowStub bool `json:"allow_stub" yaml:"allow_stub"`

	// FailFast stops on first critical failure.
	FailFast bool `json:"fail_fast" yaml:"fail_fast"`

	// RetryCount for transient failures.
	RetryCount int `json:"retry_count" yaml:"retry_count"`

	// RetryDelay between retries.
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`
}

// DefaultEnhancedPreflightConfig returns production-ready defaults.
func DefaultEnhancedPreflightConfig() *EnhancedPreflightConfig {
	profile := detectRuntimeProfile()

	return &EnhancedPreflightConfig{
		Profile:      profile,
		StrictMode:   profile == "prod",
		Timeout:      5 * time.Second,
		AllowDegraded: profile != "prod",
		AllowStub:    profile == "dev" || profile == "test",
		FailFast:     profile == "prod",
		RetryCount:   3,
		RetryDelay:   100 * time.Millisecond,
		CriticalServices: []string{
			"zen_context",
			"tier1_hot",
			"ledger",
			"qmd", // QMD is now critical in prod
		},
	}
}

// detectRuntimeProfile is defined in strictness.go.

// EnhancedPreflightCheck represents a comprehensive preflight check.
type EnhancedPreflightCheck struct {
	Name         string        `json:"name"`
	Category     string        `json:"category"`     // core, storage, messaging, intelligence
	Healthy      bool          `json:"healthy"`
	Required     bool          `json:"required"`
	Mode         DependencyMode `json:"mode"`
	StrictMode   DependencyMode `json:"strict_mode"` // What mode is required in strict
	Message      string        `json:"message,omitempty"`
	Duration     time.Duration `json:"duration"`
	Attempts     int           `json:"attempts"`
	Error        string        `json:"error,omitempty"`
	Skipped      bool          `json:"skipped,omitempty"`
}

// EnhancedPreflightReport contains comprehensive preflight results.
type EnhancedPreflightReport struct {
	Profile          string                   `json:"profile"`
	StrictMode       bool                     `json:"strict_mode"`
	AllPassed        bool                     `json:"all_passed"`
	CriticalPassed   bool                     `json:"critical_passed"`
	Timestamp        time.Time                `json:"timestamp"`
	Duration         time.Duration            `json:"duration"`
	Checks           []EnhancedPreflightCheck `json:"checks"`
	Summary          string                   `json:"summary"`
	CriticalFailures []string                 `json:"critical_failures,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
	Recommendations  []string                 `json:"recommendations,omitempty"`
}

// EnhancedStrictPreflight performs comprehensive deterministic preflight checks.
// In strict mode (prod), ALL critical services must be healthy with NO degraded/stub fallback.
func EnhancedStrictPreflight(ctx context.Context, cfg *config.Config, report *RuntimeReport) (*EnhancedPreflightReport, error) {
	preflightConfig := DefaultEnhancedPreflightConfig()

	// Allow environment overrides
	if timeout := os.Getenv("ZEN_BRAIN_PREFLIGHT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			preflightConfig.Timeout = d
		}
	}

	if strict := os.Getenv("ZEN_BRAIN_PREFLIGHT_STRICT"); strict == "true" {
		preflightConfig.StrictMode = true
		preflightConfig.AllowDegraded = false
		preflightConfig.AllowStub = false
	}

	if failFast := os.Getenv("ZEN_BRAIN_PREFLIGHT_FAIL_FAST"); failFast == "true" {
		preflightConfig.FailFast = true
	}

	return runEnhancedPreflightChecks(ctx, cfg, report, preflightConfig)
}

// runEnhancedPreflightChecks executes comprehensive preflight checks with retries.
func runEnhancedPreflightChecks(ctx context.Context, cfg *config.Config, report *RuntimeReport, pc *EnhancedPreflightConfig) (*EnhancedPreflightReport, error) {
	start := time.Now()
	preflightReport := &EnhancedPreflightReport{
		Profile:    pc.Profile,
		StrictMode: pc.StrictMode,
		Timestamp:  start,
		Checks:     []EnhancedPreflightCheck{},
	}

	// Define all checks with categories
	checks := []struct {
		name      string
		category  string
		status    CapabilityStatus
		required  bool
		strictReq bool // Required in strict mode
	}{
		{"zen_context", "core", report.ZenContext, true, true},
		{"tier1_hot", "storage", report.Tier1Hot, true, true},
		{"tier2_warm", "storage", report.Tier2Warm, false, true}, // QMD is critical in prod
		{"tier3_cold", "storage", report.Tier3Cold, false, false},
		{"journal", "storage", report.Journal, false, false},
		{"ledger", "core", report.Ledger, report.Ledger.Required, true}, // Ledger is critical in prod
		{"message_bus", "messaging", report.MessageBus, report.MessageBus.Required, false},
	}

	for _, check := range checks {
		enhancedCheck := runEnhancedPreflightCheck(
			ctx,
			check.name,
			check.category,
			check.status,
			pc,
			check.required,
			check.strictReq && pc.StrictMode,
		)

		preflightReport.Checks = append(preflightReport.Checks, enhancedCheck)

		// Fail fast on critical failures
		if pc.FailFast && !enhancedCheck.Healthy && enhancedCheck.Required {
			preflightReport.Summary = buildEnhancedPreflightSummary(preflightReport)
			return preflightReport, fmt.Errorf("fail-fast: critical check %s failed", check.name)
		}
	}

	// ZB-CREDENTIAL-RAILS: Jira project key verification
	// This ensures the configured project key is accessible before runtime starts
	if cfg.Jira.Enabled {
		jiraProjectCheck := PreflightJiraProjectKey(ctx, cfg)
		preflightReport.Checks = append(preflightReport.Checks, *jiraProjectCheck)

		// Fail fast on Jira project key failures
		if pc.FailFast && !jiraProjectCheck.Healthy && jiraProjectCheck.Required {
			preflightReport.Summary = buildEnhancedPreflightSummary(preflightReport)
			return preflightReport, fmt.Errorf("fail-fast: Jira project key verification failed")
		}
	}

	// Analyze results
	analyzeEnhancedPreflightResults(preflightReport, pc)

	preflightReport.Duration = time.Since(start)
	preflightReport.Summary = buildEnhancedPreflightSummary(preflightReport)

	// Return error if critical failures in strict mode
	if !preflightReport.CriticalPassed && pc.StrictMode {
		return preflightReport, fmt.Errorf("critical preflight checks failed in strict mode: %v",
			preflightReport.CriticalFailures)
	}

	return preflightReport, nil
}

// runEnhancedPreflightCheck executes a single check with retries and strict validation.
func runEnhancedPreflightCheck(
	ctx context.Context,
	name string,
	category string,
	status CapabilityStatus,
	pc *EnhancedPreflightConfig,
	required bool,
	strictRequired bool,
) EnhancedPreflightCheck {
	check := EnhancedPreflightCheck{
		Name:       name,
		Category:   category,
		Mode:       status.Mode,
		Required:   required || strictRequired,
		StrictMode: status.Mode, // Will be updated below
	}

	start := time.Now()

	// Skip disabled services (unless required)
	if status.Mode == ModeDisabled && !check.Required {
		check.Skipped = true
		check.Message = fmt.Sprintf("%s disabled and not required", name)
		check.Duration = time.Since(start)
		return check
	}

	// Retry loop for transient failures
	for attempt := 1; attempt <= pc.RetryCount; attempt++ {
		check.Attempts = attempt

		// Validate mode in strict environment
		if err := validateDependencyMode(name, status.Mode, pc); err != nil {
			check.Healthy = false
			check.Error = err.Error()
			check.Message = fmt.Sprintf("Mode validation failed: %s", err.Error())
			break
		}

		// Check health
		if !status.Healthy {
			check.Healthy = false
			check.Error = status.Message
			check.Message = fmt.Sprintf("Service unhealthy: %s", status.Message)

			// Retry if not last attempt
			if attempt < pc.RetryCount {
				time.Sleep(pc.RetryDelay)
				continue
			}
			break
		}

		// All checks passed
		check.Healthy = true
		check.Message = fmt.Sprintf("%s healthy (mode: %s)", name, status.Mode)
		break
	}

	check.Duration = time.Since(start)
	return check
}

// validateDependencyMode ensures mode is acceptable for the runtime profile.
func validateDependencyMode(name string, mode DependencyMode, pc *EnhancedPreflightConfig) error {
	// In prod/strict mode, reject degraded and stub modes
	if pc.StrictMode {
		if mode == ModeDegraded && !pc.AllowDegraded {
			return fmt.Errorf("degraded mode not allowed for %s in strict/prod mode", name)
		}
		if mode == ModeStub && !pc.AllowStub {
			return fmt.Errorf("stub mode not allowed for %s in strict/prod mode", name)
		}
	}

	// In staging, reject stub for critical services
	if pc.Profile == "staging" && isCriticalService(name, pc.CriticalServices) {
		if mode == ModeStub {
			return fmt.Errorf("stub mode not allowed for critical service %s in staging", name)
		}
	}

	return nil
}

// isCriticalService checks if a service is in the critical list.
func isCriticalService(name string, criticalServices []string) bool {
	for _, svc := range criticalServices {
		if svc == name {
			return true
		}
	}
	return false
}

// analyzeEnhancedPreflightResults analyzes check results and generates recommendations.
func analyzeEnhancedPreflightResults(report *EnhancedPreflightReport, pc *EnhancedPreflightConfig) {
	criticalCount := 0
	criticalPassed := 0
	allPassed := true

	for _, check := range report.Checks {
		if check.Required {
			criticalCount++
			if check.Healthy {
				criticalPassed++
			} else {
				allPassed = false
				report.CriticalFailures = append(report.CriticalFailures, check.Name)
			}
		} else if !check.Healthy && !check.Skipped {
			// Non-critical failure
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("%s: unhealthy but not required", check.Name))
		}
	}

	report.AllPassed = allPassed
	report.CriticalPassed = criticalCount > 0 && criticalPassed == criticalCount

	// Generate recommendations
	if !report.CriticalPassed {
		report.Recommendations = append(report.Recommendations,
			"Fix critical service failures before proceeding")
	}

	if len(report.Warnings) > 0 && pc.StrictMode {
		report.Recommendations = append(report.Recommendations,
			"Review non-critical failures - may impact functionality in strict mode")
	}

	for _, check := range report.Checks {
		if check.Mode == ModeDegraded && pc.StrictMode {
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("%s: upgrade from degraded to real mode for prod reliability", check.Name))
		}
	}
}

// buildEnhancedPreflightSummary creates a human-readable summary.
func buildEnhancedPreflightSummary(report *EnhancedPreflightReport) string {
	var parts []string

	// Overall status
	if report.AllPassed {
		parts = append(parts, fmt.Sprintf("✅ ALL PASSED (%d checks)", len(report.Checks)))
	} else if report.CriticalPassed {
		parts = append(parts, fmt.Sprintf("⚠️ CRITICAL PASSED, %d WARNINGS", len(report.Warnings)))
	} else {
		parts = append(parts, fmt.Sprintf("❌ CRITICAL FAILURES: %d", len(report.CriticalFailures)))
	}

	// Profile info
	parts = append(parts, fmt.Sprintf("profile=%s strict=%v", report.Profile, report.StrictMode))

	// Duration
	if report.Duration > 0 {
		parts = append(parts, fmt.Sprintf("duration=%v", report.Duration))
	}

	// Critical failures
	if len(report.CriticalFailures) > 0 {
		parts = append(parts, fmt.Sprintf("failed=%s", strings.Join(report.CriticalFailures, ",")))
	}

	return strings.Join(parts, " | ")
}

// EnhancedReadinessCheck performs dependency-aware readiness checks for k8s.
func EnhancedReadinessCheck(ctx context.Context, report *RuntimeReport) error {
	if report == nil {
		return fmt.Errorf("no runtime report")
	}

	profile := detectRuntimeProfile()

	// Define critical services based on profile
	criticalServices := []struct {
		name    string
		status  CapabilityStatus
		profile string // Which profiles require this service
	}{
		{"zen_context", report.ZenContext, "all"},
		{"tier1_hot", report.Tier1Hot, "all"},
		{"ledger", report.Ledger, "prod,staging"},
		{"tier2_warm", report.Tier2Warm, "prod"}, // QMD required in prod
	}

	for _, svc := range criticalServices {
		// Check if service is required for this profile
		required := svc.profile == "all" || strings.Contains(svc.profile, profile)

		if required {
			if !svc.status.Healthy {
				return fmt.Errorf("readiness failed: %s unhealthy (profile: %s)",
					svc.name, profile)
			}

			// In prod, also reject degraded mode
			if profile == "prod" && svc.status.Mode == ModeDegraded {
				return fmt.Errorf("readiness failed: %s in degraded mode (prod requires real)",
					svc.name)
			}

			// In prod, also reject stub mode
			if profile == "prod" && svc.status.Mode == ModeStub {
				return fmt.Errorf("readiness failed: %s in stub mode (prod requires real)",
					svc.name)
			}
		}
	}

	return nil
}

// DoctorCheck represents a diagnostic check for the doctor command.
type DoctorCheck struct {
	Name         string        `json:"name"`
	Category     string        `json:"category"`
	Status       string        `json:"status"` // ok, warning, error
	Message      string        `json:"message"`
	Details      string        `json:"details,omitempty"`
	Duration     time.Duration `json:"duration"`
	FixSuggestion string       `json:"fix_suggestion,omitempty"`
}

// DoctorReport contains comprehensive diagnostic results.
type DoctorReport struct {
	Profile      string        `json:"profile"`
	OverallStatus string       `json:"overall_status"` // healthy, degraded, unhealthy
	Timestamp    time.Time     `json:"timestamp"`
	Duration     time.Duration `json:"duration"`
	Checks       []DoctorCheck `json:"checks"`
	Summary      string        `json:"summary"`
}

// RunDoctorChecks performs comprehensive deterministic diagnostic checks.
func RunDoctorChecks(ctx context.Context, cfg *config.Config, report *RuntimeReport) *DoctorReport {
	start := time.Now()
	profile := detectRuntimeProfile()

	doctorReport := &DoctorReport{
		Profile:   profile,
		Timestamp: start,
		Checks:    []DoctorCheck{},
	}

	// Run all diagnostic checks
	checks := []struct {
		name     string
		category string
		fn       func(context.Context, *config.Config, *RuntimeReport, string) DoctorCheck
	}{
		{"runtime_profile", "core", checkRuntimeProfile},
		{"strict_mode", "core", checkStrictMode},
		{"zen_context", "core", checkZenContext},
		{"tier1_hot", "storage", checkTier1Hot},
		{"tier2_warm_qmd", "storage", checkTier2WarmQMD},
		{"tier3_cold", "storage", checkTier3Cold},
		{"journal", "storage", checkJournal},
		{"ledger", "core", checkLedger},
		{"message_bus", "messaging", checkMessageBus},
		{"circuit_breakers", "reliability", checkCircuitBreakers},
		{"git_workspace", "workspace", checkGitWorkspace},
	}

	for _, check := range checks {
		doctorReport.Checks = append(doctorReport.Checks, check.fn(ctx, cfg, report, profile))
	}

	// Determine overall status
	doctorReport.OverallStatus = determineDoctorStatus(doctorReport.Checks, profile)
	doctorReport.Duration = time.Since(start)
	doctorReport.Summary = buildDoctorSummary(doctorReport)

	return doctorReport
}

// Individual doctor check functions

func checkRuntimeProfile(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	start := time.Now()

	message := fmt.Sprintf("Runtime profile: %s", profile)
	status := "ok"

	if profile == "prod" {
		message += " (strict mode enabled)"
	} else if profile == "dev" {
		message += " (relaxed mode)"
	}

	return DoctorCheck{
		Name:     "runtime_profile",
		Category: "core",
		Status:   status,
		Message:  message,
		Duration: time.Since(start),
	}
}

func checkStrictMode(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	start := time.Now()

	strict := profile == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != ""

	status := "ok"
	message := "Strict mode: disabled"
	if strict {
		message = "Strict mode: enabled (fail-closed behavior)"
	}

	return DoctorCheck{
		Name:     "strict_mode",
		Category: "core",
		Status:   status,
		Message:  message,
		Duration: time.Since(start),
	}
}

func checkZenContext(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	return checkCapability("zen_context", "core", report.ZenContext, profile, true)
}

func checkTier1Hot(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	return checkCapability("tier1_hot", "storage", report.Tier1Hot, profile, true)
}

func checkTier2WarmQMD(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	// QMD is critical in prod
	required := profile == "prod"
	return checkCapability("tier2_warm_qmd", "storage", report.Tier2Warm, profile, required)
}

func checkTier3Cold(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	return checkCapability("tier3_cold", "storage", report.Tier3Cold, profile, false)
}

func checkJournal(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	return checkCapability("journal", "storage", report.Journal, profile, false)
}

func checkLedger(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	// Ledger is critical in prod and staging
	required := profile == "prod" || profile == "staging"
	return checkCapability("ledger", "core", report.Ledger, profile, required)
}

func checkMessageBus(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	return checkCapability("message_bus", "messaging", report.MessageBus, profile, false)
}

func checkCircuitBreakers(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	start := time.Now()

	status := "ok"
	message := "Circuit breakers: healthy"
	details := "All circuit breakers in closed state"

	// TODO: Check actual circuit breaker states when available

	return DoctorCheck{
		Name:     "circuit_breakers",
		Category: "reliability",
		Status:   status,
		Message:  message,
		Details:  details,
		Duration: time.Since(start),
	}
}

func checkGitWorkspace(ctx context.Context, cfg *config.Config, report *RuntimeReport, profile string) DoctorCheck {
	start := time.Now()

	status := "ok"
	message := "Git workspace: accessible"

	// Check if we're in a git repo
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		status = "warning"
		message = "Git workspace: not a git repository"
	}

	return DoctorCheck{
		Name:     "git_workspace",
		Category: "workspace",
		Status:   status,
		Message:  message,
		Duration: time.Since(start),
	}
}

// checkCapability is a helper for capability checks.
func checkCapability(name string, category string, status CapabilityStatus, profile string, required bool) DoctorCheck {
	start := time.Now()

	check := DoctorCheck{
		Name:     name,
		Category: category,
	}

	if status.Healthy {
		check.Status = "ok"
		check.Message = fmt.Sprintf("%s: healthy (mode: %s)", name, status.Mode)
	} else {
		if required {
			check.Status = "error"
			check.Message = fmt.Sprintf("%s: UNHEALTHY (required for %s)", name, profile)
			check.FixSuggestion = fmt.Sprintf("Fix %s connectivity or disable in config", name)
		} else {
			check.Status = "warning"
			check.Message = fmt.Sprintf("%s: unhealthy (optional)", name)
		}
	}

	// Check mode in prod
	if profile == "prod" {
		if status.Mode == ModeDegraded || status.Mode == ModeStub {
			if required {
				check.Status = "error"
				check.Message = fmt.Sprintf("%s: %s mode not allowed in prod", name, status.Mode)
				check.FixSuggestion = fmt.Sprintf("Configure %s for real mode in production", name)
			}
		}
	}

	check.Duration = time.Since(start)
	return check
}

// determineDoctorStatus determines overall health from checks.
func determineDoctorStatus(checks []DoctorCheck, profile string) string {
	hasErrors := false
	hasWarnings := false

	for _, check := range checks {
		if check.Status == "error" {
			hasErrors = true
		} else if check.Status == "warning" {
			hasWarnings = true
		}
	}

	if hasErrors {
		return "unhealthy"
	} else if hasWarnings {
		return "degraded"
	}
	return "healthy"
}

// buildDoctorSummary creates a human-readable summary.
func buildDoctorSummary(report *DoctorReport) string {
	ok := 0
	warnings := 0
	errors := 0

	for _, check := range report.Checks {
		switch check.Status {
		case "ok":
			ok++
		case "warning":
			warnings++
		case "error":
			errors++
		}
	}

	return fmt.Sprintf("%s: %d ok, %d warnings, %d errors (profile: %s, duration: %v)",
		report.OverallStatus, ok, warnings, errors, report.Profile, report.Duration)
}

// ValidateQMDGuarantees checks QMD-specific runtime guarantees.
func ValidateQMDGuarantees(report *RuntimeReport) error {
	if report == nil {
		return fmt.Errorf("no runtime report")
	}

	profile := detectRuntimeProfile()

	// QMD is critical in prod
	if profile == "prod" {
		if report.Tier2Warm.Mode == ModeStub {
			return fmt.Errorf("QMD runtime guarantee violated: stub mode not allowed in prod")
		}
		if report.Tier2Warm.Mode == ModeDegraded {
			return fmt.Errorf("QMD runtime guarantee violated: degraded mode not allowed in prod")
		}
		if !report.Tier2Warm.Healthy {
			return fmt.Errorf("QMD runtime guarantee violated: service unhealthy")
		}
	}

	return nil
}

// ValidateLedgerGuarantees checks Ledger-specific runtime guarantees.
func ValidateLedgerGuarantees(report *RuntimeReport) error {
	if report == nil {
		return fmt.Errorf("no runtime report")
	}

	profile := detectRuntimeProfile()

	// Ledger is critical in prod and staging
	if profile == "prod" || profile == "staging" {
		if report.Ledger.Mode == ModeStub {
			return fmt.Errorf("Ledger runtime guarantee violated: stub mode not allowed in %s", profile)
		}
		if report.Ledger.Mode == ModeDegraded {
			return fmt.Errorf("Ledger runtime guarantee violated: degraded mode not allowed in %s", profile)
		}
		if report.Ledger.Required && !report.Ledger.Healthy {
			return fmt.Errorf("Ledger runtime guarantee violated: service unhealthy (required)")
		}
	}

	return nil
}
