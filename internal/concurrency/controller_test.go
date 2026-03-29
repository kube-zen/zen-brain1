package concurrency

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestNoBacklog(t *testing.T) {
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
	}, &NoopCPUReader{})

	desired, reason := ctrl.DesiredConcurrency(0, 0)
	if desired != 0 {
		t.Errorf("expected 0 with no backlog, got %d", desired)
	}
	if reason != "no_runnable_work" {
		t.Errorf("expected 'no_runnable_work', got %s", reason)
	}
}

func TestBacklogLowCPU(t *testing.T) {
	// CPU at 40%: should fill up to usable slots (10 - 2 = 8)
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
	}, &FixedCPUReader{Percent: 40})

	// 15 ready tickets, 0 running → should get 8 (usable slots)
	desired, reason := ctrl.DesiredConcurrency(15, 0)
	if desired != 8 {
		t.Errorf("expected 8 with 15 ready and low CPU, got %d", desired)
	}
	if reason != "" {
		t.Errorf("expected no throttle reason, got %s", reason)
	}
}

func TestBacklogHighCPU(t *testing.T) {
	// CPU at 85%: should throttle (above soft cap)
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
	}, &FixedCPUReader{Percent: 85})

	desired, reason := ctrl.DesiredConcurrency(15, 0)
	// Should reduce from 8, not zero (min_general=1)
	if desired > 8 {
		t.Errorf("expected <= 8 with high CPU, got %d", desired)
	}
	if desired < 1 {
		t.Errorf("expected >= 1 (min general), got %d", desired)
	}
	if reason == "" {
		t.Errorf("expected throttle reason with CPU at 85%%, got empty")
	}
}

func TestCPUHardCap(t *testing.T) {
	// CPU at 92%: hard stop → 0 additional dispatch
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
	}, &FixedCPUReader{Percent: 92})

	desired, reason := ctrl.DesiredConcurrency(15, 0)
	if desired != 0 {
		t.Errorf("expected 0 with CPU above hard cap, got %d", desired)
	}
	if reason != "cpu_hard_cap" {
		t.Errorf("expected 'cpu_hard_cap', got %s", reason)
	}
}

func TestScheduledReserve(t *testing.T) {
	// 10 total, 2 reserved → only 8 usable for general work
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
	}, &NoopCPUReader{})

	// 20 ready, 0 running → should get 8, NOT 10
	desired, _ := ctrl.DesiredConcurrency(20, 0)
	if desired != 8 {
		t.Errorf("expected 8 (total - reserved), got %d", desired)
	}

	m := ctrl.Metrics()
	if m.UsableSlots != 8 {
		t.Errorf("expected usable_slots=8, got %d", m.UsableSlots)
	}
	if m.ReservedSlots != 2 {
		t.Errorf("expected reserved_slots=2, got %d", m.ReservedSlots)
	}
}

func TestHysteresis(t *testing.T) {
	// CPU crosses soft cap, should stay throttled even if CPU drops slightly
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
		HysteresisDur: 5 * time.Second,
	}, nil) // we'll inject CPU reader manually

	// Set CPU reader to fluctuate
	cpuValue := 85.0
	ctrl.cpuReader = &FixedCPUReader{Percent: cpuValue}

	// First call: CPU at 85%, should throttle
	desired, reason := ctrl.DesiredConcurrency(15, 0)
	if reason != "cpu_soft_cap_reducing" && reason != "cpu_soft_cap_hysteresis" {
		t.Errorf("expected soft cap throttle at 85%%, got %s (desired=%d)", reason, desired)
	}

	// CPU drops to 78%: still within hysteresis window
	cpuValue = 78.0
	ctrl.cpuReader = &FixedCPUReader{Percent: cpuValue}
	desired, reason = ctrl.DesiredConcurrency(15, 0)
	if desired > 1 && reason != "cpu_hysteresis_waiting" {
		t.Errorf("expected hysteresis waiting with recent soft cap, got desired=%d reason=%s", desired, reason)
	}

	// Wait for hysteresis to expire
	ctrl.mu.Lock()
	ctrl.cpuHighSince = time.Now().Add(-10 * time.Second)
	ctrl.mu.Unlock()

	// CPU at 78%: should now be un-throttled
	ctrl.cpuReader = &FixedCPUReader{Percent: cpuValue}
	desired, reason = ctrl.DesiredConcurrency(15, 0)
	if reason != "" {
		t.Errorf("expected no throttle after hysteresis, got reason=%s", reason)
	}
	if desired != 8 {
		t.Errorf("expected 8 after hysteresis, got %d", desired)
	}
}

func TestTelemetryFailure(t *testing.T) {
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		MinGeneral:    2,
	}, &FixedCPUReader{Percent: 0, Err: fmt.Errorf("telemetry unavailable")})

	desired, reason := ctrl.DesiredConcurrency(15, 0)
	// Should fall back to min_general (2)
	if desired != 2 {
		t.Errorf("expected min_general=2 on telemetry failure, got %d", desired)
	}
	if reason != "cpu_telemetry_unavailable" {
		t.Errorf("expected 'cpu_telemetry_unavailable', got %s", reason)
	}

	m := ctrl.Metrics()
	if !m.IsConservative {
		t.Error("expected IsConservative=true on telemetry failure")
	}
}

func TestSmallRunnableWork(t *testing.T) {
	// Only 3 ready tickets, 0 running → should get 3, not 8
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
	}, &NoopCPUReader{})

	desired, _ := ctrl.DesiredConcurrency(3, 0)
	if desired != 3 {
		t.Errorf("expected 3 (capped by runnable work), got %d", desired)
	}
}

func TestAlreadyFull(t *testing.T) {
	// 8 running, 8 desired → no more dispatch
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
	}, &NoopCPUReader{})

	slots := ctrl.SlotsAvailable(15, 8)
	if slots != 0 {
		t.Errorf("expected 0 slots available when at capacity, got %d", slots)
	}
}

func TestMetricsObservable(t *testing.T) {
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
	}, &FixedCPUReader{Percent: 50})

	ctrl.DesiredConcurrency(5, 2)
	m := ctrl.Metrics()

	if m.TotalSlots != 10 {
		t.Errorf("expected total_slots=10, got %d", m.TotalSlots)
	}
	if m.ReservedSlots != 2 {
		t.Errorf("expected reserved_slots=2, got %d", m.ReservedSlots)
	}
	if m.UsableSlots != 8 {
		t.Errorf("expected usable_slots=8, got %d", m.UsableSlots)
	}
	if m.DesiredGeneral != 5 {
		t.Errorf("expected desired_general=5, got %d", m.DesiredGeneral)
	}
	if m.CPUPercent != 50.0 {
		t.Errorf("expected cpu_percent=50.0, got %.1f", m.CPUPercent)
	}
	if m.IsThrottled {
		t.Error("expected not throttled at 50%% CPU")
	}

	// Verify JSON serializable
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("metrics not JSON serializable: %v", err)
	}
	t.Logf("Metrics:\n%s", string(data))
}

func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		totalSlots     int
		reservedSlots  int
		runnable       int
		running        int
		cpuPct         float64
		expectMin      int // desired >= this
		expectMax      int // desired <= this
	}{
		{"all slots reserved", 5, 5, 10, 0, 30, 1, 1},
		{"more reserved than total", 3, 10, 10, 0, 30, 1, 1},
		{"zero runnable", 10, 2, 0, 0, 30, 0, 0},
		{"one runnable", 10, 2, 1, 0, 30, 1, 1},
		{"CPU exactly at soft cap", 10, 2, 15, 0, 80, 1, 8},
		{"CPU exactly at hard cap", 10, 2, 15, 0, 90, 0, 0},
		{"20 core host low load", 20, 2, 50, 0, 15, 18, 18},
		{"20 core host high load", 20, 2, 50, 0, 95, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewController(Config{
				TotalSlots:    tt.totalSlots,
				ReservedSlots: tt.reservedSlots,
				CPUSoftCapPct: 80,
				CPUHardCapPct: 90,
			}, &FixedCPUReader{Percent: tt.cpuPct})

			// Clear hysteresis state for clean test
			ctrl.mu.Lock()
			ctrl.throttled = false
			ctrl.cpuHighSince = time.Time{}
			ctrl.mu.Unlock()

			desired, _ := ctrl.DesiredConcurrency(tt.runnable, tt.running)
			if desired < tt.expectMin || desired > tt.expectMax {
				t.Errorf("expected [%d,%d], got %d", tt.expectMin, tt.expectMax, desired)
			}
		})
	}
}

func TestNoFlapping(t *testing.T) {
	// Simulate CPU oscillating around soft cap — hysteresis should prevent flapping
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80,
		CPUHardCapPct: 90,
		HysteresisDur: 30 * time.Second,
	}, nil)

	// Simulate rapid oscillation
	cpuReadings := []float64{82, 78, 85, 77, 83, 76, 84, 79, 81, 77}
	desiredValues := make([]int, len(cpuReadings))

	for i, cpu := range cpuReadings {
		ctrl.cpuReader = &FixedCPUReader{Percent: cpu}
		desired, _ := ctrl.DesiredConcurrency(15, 0)
		desiredValues[i] = desired
	}

	// Count unique desired values — should not flip between every reading
	unique := make(map[int]bool)
	for _, d := range desiredValues {
		unique[d] = true
	}

	// Without hysteresis, we'd see 8s and 1s alternating rapidly
	// With hysteresis, should stabilize after first throttle
	if len(unique) > 3 {
		t.Errorf("too much flapping: %d unique values in %v", len(unique), desiredValues)
	}

	t.Logf("CPU readings: %v", cpuReadings)
	t.Logf("Desired values: %v", desiredValues)
	t.Logf("Unique desired: %d", len(unique))
}

func TestString(t *testing.T) {
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
	}, &FixedCPUReader{Percent: 45})

	ctrl.DesiredConcurrency(5, 2)
	s := ctrl.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
	t.Logf("Controller: %s", s)
}

func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected int
	}{
		{"CONCURRENCY_TOTAL_SLOTS", "20", 20},
		{"CONCURRENCY_RESERVED_SLOTS", "5", 5},
		{"CONCURRENCY_MIN_GENERAL", "3", 3},
	}

	for _, tt := range tests {
		t.Setenv(tt.key, tt.value)
	}

	cfg := LoadConfigFromEnv()
	if cfg.TotalSlots != 20 {
		t.Errorf("expected TotalSlots=20, got %d", cfg.TotalSlots)
	}
	if cfg.ReservedSlots != 5 {
		t.Errorf("expected ReservedSlots=5, got %d", cfg.ReservedSlots)
	}
	if cfg.MinGeneral != 3 {
		t.Errorf("expected MinGeneral=3, got %d", cfg.MinGeneral)
	}
}

func TestFloatApprox(t *testing.T) {
	// Ensure CPU percentage calculations don't have floating point issues
	_ = math.Round(80.5) // just verify math is available
	ctrl := NewController(Config{
		TotalSlots:    10,
		ReservedSlots: 2,
		CPUSoftCapPct: 80.0,
		CPUHardCapPct: 90.0,
	}, &FixedCPUReader{Percent: 79.999999})

	desired, reason := ctrl.DesiredConcurrency(15, 0)
	// 79.999% is below 80% soft cap → should not throttle
	if reason != "" {
		t.Errorf("expected no throttle at 79.99%%, got reason=%s", reason)
	}
	if desired != 8 {
		t.Errorf("expected 8, got %d", desired)
	}
}
