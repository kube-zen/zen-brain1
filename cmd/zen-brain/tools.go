// Package main: tools subcommands (metrics, etc.)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/metrics"
)

func runToolsCommand() {
	if len(os.Args) < 3 {
		printToolsUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "metrics":
		runToolsMetrics()
	default:
		fmt.Printf("Unknown tools subcommand: %s\n", sub)
		printToolsUsage()
		os.Exit(1)
	}
}

func printToolsUsage() {
	fmt.Println("Usage: zen-brain tools <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  metrics    Compute and display zen-brain throughput metrics")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  zen-brain tools metrics --dir /var/lib/zen-brain1/metrics --window last_24h")
	fmt.Println("  zen-brain tools metrics --window all --json")
}

func runToolsMetrics() {
	// Parse flags
	dir := metrics.DefaultMetricsDir
	window := "last_24h"
	outputJSON := false

	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--dir":
			if i+1 < len(os.Args) {
				dir = os.Args[i+1]
				i++
			}
		case "--window":
			if i+1 < len(os.Args) {
				window = os.Args[i+1]
				i++
			}
		case "--json":
			outputJSON = true
		case "-h", "--help":
			fmt.Println("Usage: zen-brain tools metrics [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --dir <path>     Metrics directory (default: /var/lib/zen-brain1/metrics)")
			fmt.Println("  --window <w>     Time window: last_hour, last_6h, last_24h, all (default: last_24h)")
			fmt.Println("  --json           Output as JSON (default: human-readable)")
			os.Exit(0)
		}
	}

	records, err := metrics.LoadRecordsFromDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No per-task telemetry records found at", dir)
			fmt.Println("Run the remediation worker first to generate telemetry data.")
			os.Exit(0)
		}
		log.Fatalf("Failed to load records: %v", err)
	}

	if len(records) == 0 {
		fmt.Println("No telemetry records found. Run the remediation worker first.")
		os.Exit(0)
	}

	// Filter by window
	filtered := filterByWindow(records, window)

	cm := metrics.ComputeMetrics(filtered, window)

	if outputJSON {
		data, _ := json.MarshalIndent(cm, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Print(metrics.FormatHumanReadable(cm))
	}

	// Also save the summary
	if err := metrics.ComputeAndSave(dir, filtered, window); err != nil {
		log.Printf("Warning: failed to save summary: %v", err)
	}
}

func filterByWindow(records []metrics.TaskTelemetryRecord, window string) []metrics.TaskTelemetryRecord {
	now := time.Now()
	var start time.Time

	switch window {
	case "last_hour":
		start = now.Add(-1 * time.Hour)
	case "last_6h":
		start = now.Add(-6 * time.Hour)
	case "last_24h":
		start = now.Add(-24 * time.Hour)
	case "all":
		return records
	default:
		start = now.Add(-24 * time.Hour)
	}

	var filtered []metrics.TaskTelemetryRecord
	for _, r := range records {
		if !r.Timestamp.Before(start) {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return records // fall back to all if window is empty
	}
	return filtered
}
