package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultMetricsDir is the canonical location for runtime metrics.
	DefaultMetricsDir = "/var/lib/zen-brain1/metrics"

	// PerTaskFile is the JSONL file containing per-task telemetry records.
	PerTaskFile = "per-task.jsonl"

	// SummaryFile is the latest computed summary.
	SummaryFile = "latest-summary.json"

	// SummaryHistoryFile is the JSONL history of summaries.
	SummaryHistoryFile = "summary-history.jsonl"
)

// Collector collects per-task telemetry and persists to JSONL.
// It is safe for concurrent use from multiple goroutines.
type Collector struct {
	mu       sync.Mutex
	dir      string
	file     *os.File
	writer   *bufio.Writer
	records  []TaskTelemetryRecord // in-memory buffer for live queries
	maxInMem int                   // max records kept in memory
}

// NewCollector creates or opens a metrics collector at the given directory.
// If dir is empty, uses DefaultMetricsDir.
func NewCollector(dir string) (*Collector, error) {
	if dir == "" {
		dir = DefaultMetricsDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create metrics dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, PerTaskFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open metrics file %s: %w", path, err)
	}
	log.Printf("[Metrics] Collector opened: %s", path)
	return &Collector{
		dir:      dir,
		file:     f,
		writer:   bufio.NewWriter(f),
		maxInMem: 5000,
	}, nil
}

// Record writes a single telemetry record to the JSONL file and in-memory buffer.
func (c *Collector) Record(rec TaskTelemetryRecord) error {
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal telemetry record: %w", err)
	}
	data = append(data, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("write telemetry record: %w", err)
	}
	c.writer.Flush()

	c.records = append(c.records, rec)
	if len(c.records) > c.maxInMem {
		c.records = c.records[len(c.records)-c.maxInMem:]
	}
	return nil
}

// RecordBatch writes multiple telemetry records.
func (c *Collector) RecordBatch(recs []TaskTelemetryRecord) error {
	for _, rec := range recs {
		if err := c.Record(rec); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes and closes the metrics file.
func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writer != nil {
		c.writer.Flush()
	}
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// Dir returns the metrics directory path.
func (c *Collector) Dir() string {
	return c.dir
}

// RecentRecords returns the in-memory buffer of recent records (up to n).
func (c *Collector) RecentRecords(n int) []TaskTelemetryRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n <= 0 || n > len(c.records) {
		n = len(c.records)
	}
	result := make([]TaskTelemetryRecord, n)
	copy(result, c.records[len(c.records)-n:])
	return result
}

// LoadRecords reads all records from the JSONL file.
// Use with caution on large files — prefer ComputeMetrics with a time window.
func LoadRecords(path string) ([]TaskTelemetryRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readRecords(f)
}

// LoadRecordsFromDir reads records from the per-task JSONL in dir.
func LoadRecordsFromDir(dir string) ([]TaskTelemetryRecord, error) {
	return LoadRecords(filepath.Join(dir, PerTaskFile))
}

func readRecords(r io.Reader) ([]TaskTelemetryRecord, error) {
	var records []TaskTelemetryRecord
	decoder := json.NewDecoder(r)
	for {
		var rec TaskTelemetryRecord
		if err := decoder.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			// Skip malformed lines
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// LoadRecordsInRange loads records within a time window from the JSONL file.
func LoadRecordsInRange(path string, start, end time.Time) ([]TaskTelemetryRecord, error) {
	all, err := LoadRecords(path)
	if err != nil {
		return nil, err
	}
	var filtered []TaskTelemetryRecord
	for _, r := range all {
		if !r.Timestamp.Before(start) && r.Timestamp.Before(end) {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// ComputeMetrics computes aggregated metrics from a set of records.
func ComputeMetrics(records []TaskTelemetryRecord, windowName string) *ComputedMetrics {
	if len(records) == 0 {
		return &ComputedMetrics{
			ComputedAt: time.Now(),
			WindowName: windowName,
			ByModel:    make(map[string]ModelMetrics),
			ByLane:     make(map[string]LaneMetrics),
			ByClass:    make(map[string]int),
		}
	}

	cm := &ComputedMetrics{
		ComputedAt: time.Now(),
		WindowName: windowName,
		ByModel:    make(map[string]ModelMetrics),
		ByLane:     make(map[string]LaneMetrics),
		ByClass:    make(map[string]int),
	}

	// Find time range
	cm.WindowStart = records[0].Timestamp
	cm.WindowEnd = records[0].Timestamp
	for _, r := range records {
		if r.Timestamp.Before(cm.WindowStart) {
			cm.WindowStart = r.Timestamp
		}
		if r.Timestamp.After(cm.WindowEnd) {
			cm.WindowEnd = r.Timestamp
		}
	}

	// Per-model and per-lane accumulators
	modelBuckets := make(map[string]*modelAccum)
	laneBuckets := make(map[string]*laneAccum)
	var latencies []float64
	var totalWallMs float64
	var qualitySum float64
	var qualityCount int

	for _, r := range records {
		cm.TotalTasks++

		isSuccess := r.CompletionClass == ClassFastProductive ||
			r.CompletionClass == ClassSlowButProductive ||
			r.CompletionClass == ClassTruncatedRepaired
		if isSuccess {
			cm.SuccessTasks++
		} else {
			cm.FailedTasks++
		}

		// Completion class
		cm.ByClass[string(r.CompletionClass)]++

		// Quality
		if r.QualityScore > 0 {
			qualitySum += r.QualityScore
			qualityCount++
		}

		// Latency
		wallMs := float64(r.WallTimeMs)
		latencies = append(latencies, wallMs)
		totalWallMs += wallMs

		// Per-model
		ma := modelBuckets[r.Model]
		if ma == nil {
			ma = &modelAccum{}
			modelBuckets[r.Model] = ma
		}
		ma.count++
		ma.totalMs += wallMs
		ma.latencies = append(ma.latencies, wallMs)
		if isSuccess {
			ma.successCount++
		}
		if r.CompletionClass == ClassTimeout {
			ma.timeouts++
		}
		if r.CompletionClass == ClassTruncatedRepaired {
			ma.truncated++
		}
		if r.RepairUsed {
			ma.repairs++
		}
		if r.ProducedBy == ProducedByL1 || r.ProducedBy == ProducedByL1Partial {
			ma.l1Produced++
		}
		if r.OutputSizeChars > 0 && wallMs > 0 {
			ma.totalChars += float64(r.OutputSizeChars)
			ma.totalSec += wallMs / 1000.0
		}

		// Per-lane
		la := laneBuckets[r.Lane]
		if la == nil {
			la = &laneAccum{}
			laneBuckets[r.Lane] = la
		}
		la.count++
		la.totalMs += wallMs
		la.latencies = append(la.latencies, wallMs)
		if isSuccess {
			la.successCount++
		}
		if r.CompletionClass == ClassTimeout {
			la.timeouts++
		}
		if r.ProducedBy == ProducedByL1 || r.ProducedBy == ProducedByL1Partial {
			la.l1Produced++
		}
	}

	// Overall rates
	if cm.TotalTasks > 0 {
		l1Count := 0
		timeoutCount := cm.ByClass[string(ClassTimeout)]
		truncCount := cm.ByClass[string(ClassTruncatedRepaired)]
		parseFailCount := cm.ByClass[string(ClassParseFail)]
		validFailCount := cm.ByClass[string(ClassValidationFail)]

		for _, r := range records {
			if r.ProducedBy == ProducedByL1 || r.ProducedBy == ProducedByL1Partial {
				l1Count++
			}
		}

		cm.L1ProducedRate = float64(l1Count) / float64(cm.TotalTasks)
		cm.TimeoutRate = float64(timeoutCount) / float64(cm.TotalTasks)
		cm.TruncationRate = float64(truncCount) / float64(cm.TotalTasks)
		cm.ParseFailRate = float64(parseFailCount) / float64(cm.TotalTasks)
		cm.ValidationFailRate = float64(validFailCount) / float64(cm.TotalTasks)
		if truncCount > 0 {
			repairCount := 0
			for _, r := range records {
				if r.RepairUsed && r.RepairSucceeded {
					repairCount++
				}
			}
			cm.RepairRate = float64(repairCount) / float64(truncCount)
		}
	}

	// Latency percentiles
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		cm.AvgLatencyMs = totalWallMs / float64(len(latencies))
		cm.P50LatencyMs = percentile(latencies, 0.50)
		cm.P95LatencyMs = percentile(latencies, 0.95)
		cm.MaxLatencyMs = latencies[len(latencies)-1]
	}

	// Throughput
	windowHours := cm.WindowEnd.Sub(cm.WindowStart).Hours()
	if windowHours > 0 {
		cm.TasksPerHour = float64(cm.TotalTasks) / windowHours
		cm.DonePerHour = float64(cm.SuccessTasks) / windowHours
		cm.DonePerDay = cm.DonePerHour * 24
	}

	// Chars/sec
	totalChars := 0.0
	totalSec := 0.0
	for _, r := range records {
		if r.OutputSizeChars > 0 && r.WallTimeMs > 0 {
			totalChars += float64(r.OutputSizeChars)
			totalSec += float64(r.WallTimeMs) / 1000.0
		}
	}
	if totalSec > 0 {
		cm.CharsPerSec = totalChars / totalSec
	}

	// Quality
	if qualityCount > 0 {
		cm.AvgQualityScore = qualitySum / float64(qualityCount)
	}

	// Build per-model map
	for model, ma := range modelBuckets {
		mm := ModelMetrics{
			Count:        ma.count,
			SuccessCount: ma.successCount,
			AvgLatencyMs: ma.totalMs / float64(ma.count),
		}
		if ma.count > 0 {
			mm.TimeoutRate = float64(ma.timeouts) / float64(ma.count)
			mm.L1ProducedRate = float64(ma.l1Produced) / float64(ma.count)
			mm.TruncationRate = float64(ma.truncated) / float64(ma.count)
		}
		if len(ma.latencies) > 0 {
			sort.Float64s(ma.latencies)
			mm.P95LatencyMs = percentile(ma.latencies, 0.95)
		}
		if ma.totalSec > 0 {
			mm.AvgCharsPerSec = ma.totalChars / ma.totalSec
		}
		cm.ByModel[model] = mm
	}

	// Build per-lane map
	for lane, la := range laneBuckets {
		lm := LaneMetrics{
			Count:        la.count,
			SuccessCount: la.successCount,
			AvgLatencyMs: la.totalMs / float64(la.count),
		}
		if la.count > 0 {
			lm.TimeoutRate = float64(la.timeouts) / float64(la.count)
			lm.L1ProducedRate = float64(la.l1Produced) / float64(la.count)
		}
		if len(la.latencies) > 0 {
			sort.Float64s(la.latencies)
			lm.P95LatencyMs = percentile(la.latencies, 0.95)
		}
		cm.ByLane[lane] = lm
	}

	return cm
}

// ComputeAndSave computes metrics and writes summary files.
func ComputeAndSave(dir string, records []TaskTelemetryRecord, windowName string) error {
	cm := ComputeMetrics(records, windowName)

	// Write latest summary
	summaryPath := filepath.Join(dir, SummaryFile)
	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(summaryPath, data, 0644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	// Append to history
	histPath := filepath.Join(dir, SummaryHistoryFile)
	hf, err := os.OpenFile(histPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hf.Close()
	compact, _ := json.Marshal(cm)
	compact = append(compact, '\n')
	hf.Write(compact)

	log.Printf("[Metrics] Summary computed: window=%s tasks=%d l1_produced=%.1f%% avg_latency=%.0fms p95=%.0fms done/h=%.1f",
		windowName, cm.TotalTasks, cm.L1ProducedRate*100, cm.AvgLatencyMs, cm.P95LatencyMs, cm.DonePerHour)
	return nil
}

// FormatHumanReadable produces a terminal-friendly summary string.
func FormatHumanReadable(cm *ComputedMetrics) string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== zen-brain1 Throughput Report: %s ===\n", cm.WindowName)
	fmt.Fprintf(&b, "Window: %s → %s (%.1fh)\n\n",
		cm.WindowStart.Format("2006-01-02 15:04"), cm.WindowEnd.Format("2006-01-02 15:04"),
		cm.WindowEnd.Sub(cm.WindowStart).Hours())

	fmt.Fprintf(&b, "Tasks:     %d total, %d success, %d failed\n", cm.TotalTasks, cm.SuccessTasks, cm.FailedTasks)
	fmt.Fprintf(&b, "L1-prod:   %.1f%%\n", cm.L1ProducedRate*100)
	fmt.Fprintf(&b, "Timeout:   %.1f%%\n", cm.TimeoutRate*100)
	fmt.Fprintf(&b, "Truncated: %.1f%% (repair: %.1f%%)\n", cm.TruncationRate*100, cm.RepairRate*100)
	fmt.Fprintf(&b, "ParseFail: %.1f%%\n", cm.ParseFailRate*100)
	fmt.Fprintf(&b, "ValidFail: %.1f%%\n", cm.ValidationFailRate*100)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "Latency:\n")
	fmt.Fprintf(&b, "  Avg: %.0fms  P50: %.0fms  P95: %.0fms  Max: %.0fms\n",
		cm.AvgLatencyMs, cm.P50LatencyMs, cm.P95LatencyMs, cm.MaxLatencyMs)
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "Throughput:\n")
	fmt.Fprintf(&b, "  Tasks/h: %.1f  Done/h: %.1f  Done/day: %.1f\n",
		cm.TasksPerHour, cm.DonePerHour, cm.DonePerDay)
	if cm.CharsPerSec > 0 {
		fmt.Fprintf(&b, "  Chars/s: %.1f\n", cm.CharsPerSec)
	}
	if cm.AvgQualityScore > 0 {
		fmt.Fprintf(&b, "  Avg quality: %.1f/25\n", cm.AvgQualityScore)
	}
	fmt.Fprintf(&b, "\n")

	if cm.ActiveWorkers > 0 {
		fmt.Fprintf(&b, "Workers: %d active, utilization %.0f%%, queue depth %d\n",
			cm.ActiveWorkers, cm.WorkerUtilization*100, cm.QueueDepth)
		fmt.Fprintf(&b, "\n")
	}

	if len(cm.ByModel) > 0 {
		fmt.Fprintf(&b, "By Model:\n")
		for model, mm := range cm.ByModel {
			fmt.Fprintf(&b, "  %-25s count=%3d success=%3d l1=%.0f%% avg=%.0fms p95=%.0fms timeout=%.0f%%",
				model, mm.Count, mm.SuccessCount, mm.L1ProducedRate*100, mm.AvgLatencyMs, mm.P95LatencyMs, mm.TimeoutRate*100)
			if mm.AvgCharsPerSec > 0 {
				fmt.Fprintf(&b, " chars/s=%.1f", mm.AvgCharsPerSec)
			}
			fmt.Fprintf(&b, "\n")
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(cm.ByLane) > 0 {
		fmt.Fprintf(&b, "By Lane:\n")
		for lane, lm := range cm.ByLane {
			fmt.Fprintf(&b, "  %-15s count=%3d success=%3d l1=%.0f%% avg=%.0fms p95=%.0fms\n",
				lane, lm.Count, lm.SuccessCount, lm.L1ProducedRate*100, lm.AvgLatencyMs, lm.P95LatencyMs)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "By Completion Class:\n")
	for class, count := range cm.ByClass {
		fmt.Fprintf(&b, "  %-25s %d (%.1f%%)\n", class, count, float64(count)/float64(cm.TotalTasks)*100)
	}

	return b.String()
}

// Internal accumulators

type modelAccum struct {
	count        int
	successCount int
	timeouts     int
	truncated    int
	repairs      int
	l1Produced   int
	totalMs      float64
	latencies    []float64
	totalChars   float64
	totalSec     float64
}

type laneAccum struct {
	count        int
	successCount int
	timeouts     int
	l1Produced   int
	totalMs      float64
	latencies    []float64
}

// percentile returns the p-th percentile from a sorted slice of floats.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
