package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyCompletion(t *testing.T) {
	tests := []struct {
		name           string
		wallMs         int64
		outputChars    int
		parseError     bool
		quality        float64
		threshold      float64
		want           CompletionClass
	}{
		{"fast productive", 10000, 500, false, 20, 15, ClassFastProductive},
		{"slow productive", 60000, 500, false, 20, 15, ClassSlowButProductive},
		{"timeout", 120000, 0, false, 0, 15, ClassTimeout},
		{"parse fail", 30000, 200, true, 0, 15, ClassParseFail},
		{"validation fail", 30000, 500, false, 10, 15, ClassValidationFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyCompletion(tt.wallMs, tt.outputChars, tt.parseError, tt.quality, tt.threshold)
			if got != tt.want {
				t.Errorf("ClassifyCompletion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyProducedBy(t *testing.T) {
	tests := []struct {
		name        string
		outputSize  int
		parseOK     bool
		quality     float64
		threshold   float64
		intervention string
		want        ProducedBy
	}{
		{"l1 produced", 500, true, 20, 15, "", ProducedByL1},
		{"l1 partial", 500, true, 12, 15, "", ProducedByL1Partial},
		{"l1 failed parse", 200, false, 0, 15, "", ProducedByL1Failed},
		{"l1 failed empty", 0, true, 0, 15, "", ProducedByL1Failed},
		{"supervisor override", 500, true, 20, 15, "manual_rewrite", ProducedBySupervisor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyProducedBy(tt.outputSize, tt.parseOK, tt.quality, tt.threshold, tt.intervention)
			if got != tt.want {
				t.Errorf("ClassifyProducedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectorRecordAndRead(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCollector(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	rec := TaskTelemetryRecord{
		Timestamp:       now,
		RunID:           "test-run",
		TaskID:          "task-1",
		JiraKey:         "ZB-100",
		Model:           "qwen3.5:0.8b",
		Lane:            "l1-local",
		Provider:        "llama-cpp",
		PromptSizeChars: 1000,
		OutputSizeChars: 800,
		WallTimeMs:      45000,
		CompletionClass: ClassSlowButProductive,
		ProducedBy:      ProducedByL1,
		QualityScore:    22.5,
	}

	if err := c.Record(rec); err != nil {
		t.Fatal(err)
	}
	c.Close()

	// Read back
	records, err := LoadRecordsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].JiraKey != "ZB-100" {
		t.Errorf("jira_key = %s, want ZB-100", records[0].JiraKey)
	}
	if records[0].CompletionClass != ClassSlowButProductive {
		t.Errorf("class = %s, want slow-but-productive", records[0].CompletionClass)
	}
}

func TestComputeMetrics(t *testing.T) {
	records := []TaskTelemetryRecord{
		{
			Timestamp:       time.Now().Add(-2 * time.Hour),
			Model:           "qwen3.5:0.8b",
			Lane:            "l1-local",
			WallTimeMs:      15000,
			OutputSizeChars: 500,
			CompletionClass: ClassFastProductive,
			ProducedBy:      ProducedByL1,
			QualityScore:    22,
		},
		{
			Timestamp:       time.Now().Add(-1 * time.Hour),
			Model:           "qwen3.5:0.8b",
			Lane:            "l1-local",
			WallTimeMs:      90000,
			OutputSizeChars: 600,
			CompletionClass: ClassSlowButProductive,
			ProducedBy:      ProducedByL1,
			QualityScore:    20,
		},
		{
			Timestamp:       time.Now().Add(-30 * time.Minute),
			Model:           "qwen3.5:0.8b",
			Lane:            "l1-local",
			WallTimeMs:      180000,
			OutputSizeChars: 0,
			CompletionClass: ClassTimeout,
			ProducedBy:      ProducedByL1Failed,
		},
	}

	cm := ComputeMetrics(records, "test")

	if cm.TotalTasks != 3 {
		t.Errorf("total = %d, want 3", cm.TotalTasks)
	}
	if cm.SuccessTasks != 2 {
		t.Errorf("success = %d, want 2", cm.SuccessTasks)
	}
	if cm.L1ProducedRate < 0.66 || cm.L1ProducedRate > 0.67 {
		t.Errorf("l1_produced = %.2f, want ~0.667", cm.L1ProducedRate)
	}
	if cm.TimeoutRate < 0.33 || cm.TimeoutRate > 0.34 {
		t.Errorf("timeout_rate = %.2f, want ~0.333", cm.TimeoutRate)
	}

	mm, ok := cm.ByModel["qwen3.5:0.8b"]
	if !ok {
		t.Fatal("missing model metrics for qwen3.5:0.8b")
	}
	if mm.Count != 3 {
		t.Errorf("model count = %d, want 3", mm.Count)
	}
}

func TestComputeAndSave(t *testing.T) {
	dir := t.TempDir()

	records := []TaskTelemetryRecord{
		{
			Timestamp:       time.Now().Add(-1 * time.Hour),
			Model:           "qwen3.5:0.8b",
			Lane:            "l1-local",
			WallTimeMs:      50000,
			OutputSizeChars: 400,
			CompletionClass: ClassSlowButProductive,
			ProducedBy:      ProducedByL1,
			QualityScore:    21,
		},
	}

	if err := ComputeAndSave(dir, records, "test-window"); err != nil {
		t.Fatal(err)
	}

	// Check summary file exists and parses
	data, err := os.ReadFile(filepath.Join(dir, SummaryFile))
	if err != nil {
		t.Fatal(err)
	}
	var cm ComputedMetrics
	if err := json.Unmarshal(data, &cm); err != nil {
		t.Fatal(err)
	}
	if cm.TotalTasks != 1 {
		t.Errorf("total = %d, want 1", cm.TotalTasks)
	}
	if cm.WindowName != "test-window" {
		t.Errorf("window = %s, want test-window", cm.WindowName)
	}
}

func TestFormatHumanReadable(t *testing.T) {
	cm := &ComputedMetrics{
		ComputedAt:      time.Now(),
		WindowStart:     time.Now().Add(-1 * time.Hour),
		WindowEnd:       time.Now(),
		WindowName:      "last_hour",
		TotalTasks:      10,
		SuccessTasks:    8,
		FailedTasks:     2,
		L1ProducedRate:  0.80,
		TimeoutRate:     0.10,
		TruncationRate:  0.20,
		RepairRate:      0.50,
		AvgLatencyMs:    45000,
		P50LatencyMs:    40000,
		P95LatencyMs:    120000,
		MaxLatencyMs:    180000,
		TasksPerHour:    10,
		DonePerHour:     8,
		DonePerDay:      192,
		CharsPerSec:     12.5,
		ByModel: map[string]ModelMetrics{
			"qwen3.5:0.8b": {Count: 10, SuccessCount: 8, AvgLatencyMs: 45000, P95LatencyMs: 120000},
		},
		ByLane: map[string]LaneMetrics{
			"l1-local": {Count: 10, SuccessCount: 8, AvgLatencyMs: 45000},
		},
		ByClass: map[string]int{
			"fast-productive":    3,
			"slow-but-productive": 5,
			"timeout":            1,
			"parse-fail":         1,
		},
		ActiveWorkers:    3,
		WorkerUtilization: 0.75,
		QueueDepth:       2,
	}

	s := FormatHumanReadable(cm)
	if len(s) == 0 {
		t.Error("FormatHumanReadable returned empty string")
	}
	// Quick sanity checks
	if !contains(s, "10 total") {
		t.Error("missing total count")
	}
	if !contains(s, "qwen3.5:0.8b") {
		t.Error("missing model name")
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	if p := percentile(vals, 0.50); p < 49 || p > 51 {
		t.Errorf("p50 = %.1f, want ~50", p)
	}
	if p := percentile(vals, 0.95); p < 94 || p > 96 {
		t.Errorf("p95 = %.1f, want ~95", p)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
