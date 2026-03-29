package concurrency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HealthSignal captures the "idle workers while runnable work exists" condition.
// This is a bug per R041 invariant 1.
type HealthSignal struct {
	Timestamp      time.Time `json:"timestamp"`
	DesiredWorkers int       `json:"desired_workers"`
	ActualRunning  int       `json:"actual_running"`
	RunnableWork   int       `json:"runnable_work"`
	CPUPercent     float64   `json:"cpu_percent"`
	IsBuggy        bool      `json:"is_buggy"` // true when idle workers exist with runnable work
	ThrottleReason string    `json:"throttle_reason,omitempty"`
}

// Dashboard persists concurrency metrics for observability.
type Dashboard struct {
	mu         sync.Mutex
	metricsDir string
}

// NewDashboard creates a new concurrency dashboard.
func NewDashboard(metricsDir string) *Dashboard {
	os.MkdirAll(metricsDir, 0755)
	return &Dashboard{metricsDir: metricsDir}
}

// Record writes a metrics snapshot to disk.
func (d *Dashboard) Record(m ConcurrencyMetrics) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.metricsDir == "" {
		return
	}

	// JSON current state
	data, _ := json.MarshalIndent(m, "", "  ")
	path := filepath.Join(d.metricsDir, "concurrency-dashboard.json")
	_ = os.WriteFile(path, data, 0644)

	// JSONL history
	line, _ := json.Marshal(m)
	histPath := filepath.Join(d.metricsDir, "concurrency-history.jsonl")
	f, err := os.OpenFile(histPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		f.Write(line)
		f.Write([]byte("\n"))
		f.Close()
	}
}

// RecordHealth writes a health signal (idle-while-busy condition) to disk.
func (d *Dashboard) RecordHealth(s HealthSignal) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.metricsDir == "" {
		return
	}

	line, _ := json.Marshal(s)
	path := filepath.Join(d.metricsDir, "concurrency-health.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		f.Write(line)
		f.Write([]byte("\n"))
		f.Close()
	}
}
