package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// Doctor prints a readable summary of Block 3 runtime state (config source, capabilities, strict flags).
func Doctor(ctx context.Context, cfg *config.Config, report *RuntimeReport) {
	if report == nil {
		fmt.Println("runtime report: not available")
		return
	}
	configSource := "default"
	if path := os.Getenv("ZEN_CONFIG_PATH"); path != "" {
		configSource = path
	} else if cfg != nil {
		configSource = "config file / env"
	}
	fmt.Println("Block 3 Runtime Doctor")
	fmt.Println("----------------------")
	fmt.Printf("Config source:    %s\n", configSource)
	fmt.Printf("ZenContext:       %s  healthy=%v  required=%v  %s\n",
		report.ZenContext.Mode, report.ZenContext.Healthy, report.ZenContext.Required, report.ZenContext.Message)
	fmt.Printf("Tier1 (Hot):      %s  healthy=%v  required=%v  %s\n",
		report.Tier1Hot.Mode, report.Tier1Hot.Healthy, report.Tier1Hot.Required, report.Tier1Hot.Message)
	fmt.Printf("Tier2 (Warm):     %s  healthy=%v  %s\n",
		report.Tier2Warm.Mode, report.Tier2Warm.Healthy, report.Tier2Warm.Message)
	fmt.Printf("Tier3 (Cold):     %s  healthy=%v  %s\n",
		report.Tier3Cold.Mode, report.Tier3Cold.Healthy, report.Tier3Cold.Message)
	fmt.Printf("Journal:          %s  healthy=%v  %s\n",
		report.Journal.Mode, report.Journal.Healthy, report.Journal.Message)
	fmt.Printf("Ledger:           %s  healthy=%v  required=%v  %s\n",
		report.Ledger.Mode, report.Ledger.Healthy, report.Ledger.Required, report.Ledger.Message)
	fmt.Printf("MessageBus:       %s  healthy=%v  required=%v  %s\n",
		report.MessageBus.Mode, report.MessageBus.Healthy, report.MessageBus.Required, report.MessageBus.Message)
	fmt.Println("Strict flags:     ZEN_BRAIN_STRICT_RUNTIME, ZEN_BRAIN_REQUIRE_* (env)")
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
