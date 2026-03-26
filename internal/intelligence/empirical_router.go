package intelligence

import (
	"fmt"
	"log"
	"sync"
)

// Model identifiers for MLQ lanes.
const (
	ModelL1 = "qwen3.5:0.8b-q4"
	ModelL2 = "qwen3.5:2b-q4"
	ModelL3 = "glm-4.7"
)

// EmpiricalTaskTelemetry records per-task execution metrics for routing policy learning.
type EmpiricalTaskTelemetry struct {
	TaskID       string   `json:"task_id"`
	TaskName     string    `json:"task_name"`
	TaskClass    string    `json:"task_class"`
	Lane         string    `json:"lane"`
	LaneFirst    bool      `json:"lane_first"`
	LaneSuccess  bool      `json:"lane_success"`
	OutputQuality string   `json:"output_quality"`
	BuildSuccess bool      `json:"build_success"`
	ScannerUsed  []string  `json:"scanner_used,omitempty"`
}

// EmpiricalRecommendation is a routing recommendation from the empirical router.
// Separate from ModelRouter's ModelRecommendation to avoid type conflicts.
type EmpiricalRecommendation struct {
	ModelID           string   `json:"model_id"`
	Source            string   `json:"source"`   // "default", "telemetry", "escalation", "fallback"
	Reason            string   `json:"reason"`
	Confidence        float64  `json:"confidence"`
	PromotedFrom      string   `json:"promoted_from,omitempty"`
	Lane              string   `json:"lane"`
	EscalationHistory []string `json:"escalation_history,omitempty"`
}

// EmpiricalRouter learns from L1/L2 success/failure patterns to inform routing.
// PHASE 20/22/23: L1-first empirical routing with telemetry-aware escalation.
//
// Rules:
//  1. L1 is DEFAULT for all regular tasks
//  2. L2 is FALLBACK after repeated L1 failure
//  3. Escalation requires EMPIRICAL evidence (3+ failures on same task class)
//  4. "might be hard" is NOT a reason to skip L1
type EmpiricalRouter struct {
	defaultLane    string
	failureHistory map[string]int // taskClass → consecutive L1 failure count
	mu             sync.RWMutex
}

// NewEmpiricalRouter creates an empirical router with L1-first policy.
func NewEmpiricalRouter(defaultLane string) *EmpiricalRouter {
	if defaultLane != ModelL1 && defaultLane != ModelL2 {
		defaultLane = ModelL1
		log.Printf("[EmpiricalRouter] Invalid default lane %q, defaulting to L1", defaultLane)
	}
	return &EmpiricalRouter{
		defaultLane:    defaultLane,
		failureHistory: make(map[string]int),
	}
}

// RecordFailure records an L1 failure for a task class.
func (r *EmpiricalRouter) RecordFailure(taskClass string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failureHistory[taskClass]++
	log.Printf("[EmpiricalRouter] L1 failure recorded for %q (count=%d)", taskClass, r.failureHistory[taskClass])
}

// RecordSuccess resets the failure counter for a task class.
func (r *EmpiricalRouter) RecordSuccess(taskClass string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failureHistory, taskClass)
}

// ShouldEscalateToL2 returns true if task class has 3+ consecutive L1 failures.
func (r *EmpiricalRouter) ShouldEscalateToL2(taskClass string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := r.failureHistory[taskClass]
	if count >= 3 {
		log.Printf("[EmpiricalRouter] Escalating to L2: %q has %d L1 failures", taskClass, count)
		return true
	}
	return false
}

// GetFailureCount returns the current L1 failure count for a task class.
func (r *EmpiricalRouter) GetFailureCount(taskClass string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failureHistory[taskClass]
}

// Recommend returns a routing recommendation for the given task class.
func (r *EmpiricalRouter) Recommend(taskClass string) *EmpiricalRecommendation {
	if r.ShouldEscalateToL2(taskClass) {
		return &EmpiricalRecommendation{
			ModelID:      ModelL2,
			Source:       "telemetry",
			Reason:       fmt.Sprintf("L1 has %d consecutive failures on %q", r.GetFailureCount(taskClass), taskClass),
			Confidence:   0.9,
			PromotedFrom: ModelL1,
			Lane:         ModelL2,
		}
	}
	return &EmpiricalRecommendation{
		ModelID:    r.defaultLane,
		Source:     "default",
		Reason:     fmt.Sprintf("Default lane %q (no escalation evidence)", r.defaultLane),
		Confidence: 0.7,
		Lane:       r.defaultLane,
	}
}

// IsReportingTask checks if a task class is a regular reporting task (should go to L1 first).
func IsReportingTask(taskClass string) bool {
	switch taskClass {
	case "dead_code", "defects", "tech_debt", "roadmap",
		"bug_hunting", "stub_hunting", "executive_summary",
		"package_hotspot", "test_gap", "config_drift":
		return true
	}
	return false
}
