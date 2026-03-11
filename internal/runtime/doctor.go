package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// Doctor prints a readable summary of Block 3 runtime state (config source, capabilities, strict flags).
// Now includes circuit breaker states for operational visibility (A002).
func Doctor(ctx context.Context, cfg *config.Config, report *RuntimeReport) {
	if report == nil {
		fmt.Println("runtime report: not available")
		return
	}
	profile := detectRuntimeProfile()
	configSource := "default"
	if path := os.Getenv("ZEN_CONFIG_PATH"); path != "" {
		configSource = path
	} else if cfg != nil {
		configSource = "config file / env"
	}
	fmt.Println("Block 3 Runtime Doctor")
	fmt.Println("----------------------")
	fmt.Printf("Profile:          %s\n", profile)
	fmt.Printf("Strict mode:      %v\n", profile == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "")
	fmt.Printf("Config source:    %s\n", configSource)
	fmt.Println()
	fmt.Println("CAPABILITIES:")
	printCapability("ZenContext", report.ZenContext, profile)
	printCapability("Tier1 (Hot)", report.Tier1Hot, profile)
	printCapability("Tier2 (Warm)", report.Tier2Warm, profile)
	printCapability("Tier3 (Cold)", report.Tier3Cold, profile)
	printCapability("Journal", report.Journal, profile)
	printCapability("Ledger", report.Ledger, profile)
	printCapability("MessageBus", report.MessageBus, profile)
	fmt.Println()

	// Show circuit breaker states (A002)
	fmt.Println("CIRCUIT BREAKERS:")
	registry := GetCircuitBreakerRegistry()
	states := registry.GetAllStates()
	if len(states) == 0 {
		fmt.Println("  (none registered)")
	} else {
		for name, state := range states {
			status := "✓ closed"
			if state.State == CircuitStateOpen {
				status = "✗ OPEN"
			} else if state.State == CircuitStateHalfOpen {
				status = "◐ half-open"
			}
			fmt.Printf("  %-15s %s (failures: %d)\n", name+":", status, state.Failures)
		}
	}
	fmt.Println()

	// Summary
	overall := "healthy"
	criticalFailures := []string{}
	for _, cap := range []CapabilityStatus{
		report.ZenContext, report.Tier1Hot, report.Tier2Warm, report.Tier3Cold,
		report.Journal, report.Ledger, report.MessageBus,
	} {
		if cap.Required && !cap.Healthy {
			overall = "unhealthy"
			criticalFailures = append(criticalFailures, cap.Name)
		}
	}
	if len(criticalFailures) > 0 {
		fmt.Printf("STATUS: %s (critical failures: %s)\n", strings.ToUpper(overall), strings.Join(criticalFailures, ", "))
	} else {
		fmt.Printf("STATUS: %s\n", strings.ToUpper(overall))
	}
	fmt.Println()
	fmt.Println("Strict flags:     ZEN_RUNTIME_PROFILE=prod | ZEN_BRAIN_STRICT_RUNTIME | ZEN_BRAIN_REQUIRE_* (env)")
}

// printCapability prints a capability with mode and health status
func printCapability(name string, cap CapabilityStatus, profile string) {
	status := "✓"
	if !cap.Healthy {
		status = "✗"
	}
	required := ""
	if cap.Required {
		required = " [required]"
	}
	modeDetail := string(cap.Mode)
	if modeDetail == "" {
		modeDetail = "unknown"
	}
	// In prod, highlight if using non-real modes
	if profile == "prod" && (cap.Mode == ModeStub || cap.Mode == ModeDegraded) {
		modeDetail = "⚠️ " + modeDetail + " (not allowed in prod)"
	}
	fmt.Printf("  %-15s %s mode=%-10s healthy=%v%s\n", name+":", status, modeDetail, cap.Healthy, required)
	if cap.Message != "" {
		fmt.Printf("                    └─ %s\n", cap.Message)
	}
}

// ReportJSON prints the RuntimeReport as JSON.
func ReportJSON(report *RuntimeReport) error {
	if report == nil {
		fmt.Println("null")
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// Ping runs lightweight checks and exits nonzero if any required capability is unhealthy.
func Ping(report *RuntimeReport) int {
	if report == nil {
		fmt.Fprintln(os.Stderr, "no runtime report")
		return 1
	}
	for _, cap := range []CapabilityStatus{
		report.ZenContext, report.Tier1Hot, report.Tier2Warm, report.Tier3Cold,
		report.Journal, report.Ledger, report.MessageBus,
	} {
		if cap.Required && !cap.Healthy {
			fmt.Fprintf(os.Stderr, "required capability %s unhealthy: %s\n", cap.Name, cap.Message)
			return 1
		}
	}
	fmt.Println("ok")
	return 0
}
