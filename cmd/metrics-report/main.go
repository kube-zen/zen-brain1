// metrics-report computes and displays zen-brain1 throughput metrics.
//
// Usage:
//   go run ./cmd/metrics-report [--dir /var/lib/zen-brain1/metrics] [--window last_hour|last_6h|last_24h|all]
//   go run ./cmd/metrics-report --dir /var/lib/zen-brain1/metrics --window last_24h --json
//   go run ./cmd/metrics-report --dir /var/lib/zen-brain1/metrics --window all --human
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/metrics"
)

func main() {
	dir := flag.String("dir", metrics.DefaultMetricsDir, "metrics directory")
	window := flag.String("window", "last_24h", "time window: last_hour, last_6h, last_24h, all")
	outputJSON := flag.Bool("json", false, "output as JSON (default: human-readable)")
	flag.Parse()

	records, err := metrics.LoadRecordsFromDir(*dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No per-task telemetry records found at", *dir)
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
	filtered := filterByWindow(records, *window)

	cm := metrics.ComputeMetrics(filtered, *window)

	if *outputJSON {
		data, _ := json.MarshalIndent(cm, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Print(metrics.FormatHumanReadable(cm))
	}

	// Also save the summary
	if err := metrics.ComputeAndSave(*dir, filtered, *window); err != nil {
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
