package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// Model identifiers
const (
	ModelL1   = "qwen3.5:0.8b-q4"
	ModelL2   = "qwen3.5:2b-q4"
	ModelL3   = "glm-4.7"
)

// Task telemetry
type TaskTelemetry struct {
	Timestamp      time.Time     `json:"timestamp"`
	TaskID        string       `json:"task_id"`
	TaskName      string       `json:"task_name"`
	TaskClass     string       `json:"task_class"`
	Lane           string       `json:"lane"`
	LaneFirst      bool         `json:"lane_first"`
	LaneSuccess    bool         `json:"lane_success"`
	OutputQuality  string       `json:"output_quality"`
	BuildSuccess   bool         `json:"build_success"`
	ScannerUsed    []string     `json:"scanner_used"`
	Temperature    float64     `json:"temperature"`
	MaxTokens     int          `json:"max_tokens"`
	EvidencePath   string       `json:"evidence_path"`
}

// New policy: L1-first routing with empirical escalation
// PHASE 20 P001-P009: Replace restrictive front-door with L1-first optimistic routing
//
// Routing rules:
// 1. L1 (qwen3.5:0.8b) is DEFAULT lane for all regular tasks
// 2. L2 (qwen3.5:2b) is FALLBACK lane, used only after L1 fails or on bounded cross-file synthesis
// 3. Escalation requires EMPIRICAL evidence, not assumptions
// 4. L2 earned through L1-fail/L2-success pattern, not pre-assigned
// 5. "might be hard" is NOT a reason to skip L1
// 6. L3+ only after L1 also fails
//
// Telemetry: Capture per-task metrics to build empirical routing table
type EmpiricalRouter struct {
	ledger          ledger.ZenLedgerClient
	defaultLane     string // L1 or L2
	failureHistory  map[string][]string // Map lane → list of recent task classes that failed
	mu              sync.RWMutex
	minSamples       int // Minimum samples to trust ledger data
	initialized      bool
}

// NewEmpiricalRouter returns a router with L1-first empirical policy.
func NewEmpiricalRouter(lc ledger.ZenLedgerClient, defaultLane string, minSamples int) *EmpiricalRouter {
	if defaultLane != ModelL1 && defaultLane != ModelL2 {
		defaultLane = ModelL1
		log.Printf("[EmpiricalRouter] Invalid default lane %q, defaulting to L1", defaultLane)
	}

	return &EmpiricalRouter{
		ledger:         lc,
		defaultLane:     defaultLane,
		failureHistory:  make(map[string][]string),
		minSamples:       minSamples,
		mu:              sync.NewRWMutex(),
	}
}

// ModelRecommendation with promotion path and escalation history.
type ModelRecommendation struct {
	ModelID      string   `json:"model_id"`
	Source       string   `json:"source"`                 // "default", "telemetry", "escalation"
	Reason       string   `json:"reason"`
	Confidence   float64 `json:"confidence"`
	PromotedFrom string `json:"promoted_from,omitempty"` // Lane that caused escalation to this model
	Lane         string   `json:"lane"`                  // Recommended lane for this task
	EscalationHistory []string `json:"escalation_history,omitempty"` // Recent failures that led to this recommendation
}

// RecommendModelWithEmpiricism uses L1-first routing with telemetry-aware escalation.
// PHASE 20 P003: Send all current reporting tasks to L1 first
func (r *EmpiricalRouter) RecommendModelWithTelemetry(ctx context.Context, telemetry TaskTelemetry) (*ModelRecommendation, error) {
	// Use L1 as default for all regular tasks unless evidence proves otherwise
	lane := r.defaultLane
	source := "default"
	reason := fmt.Sprintf("Default lane %q", lane)
	promotedFrom := ""
	escalationHistory := []string{}
	confidence := 0.7 // High confidence in L1

	// Check for empirical escalation patterns
	// PHASE 20 P003: Escalate only when there's evidence of L1 failure
	if r.shouldEscalateToL2(ctx, telemetry.TaskClass) {
		lane = ModelL2
		source = "telemetry"
		reason = fmt.Sprintf("L1 has failed on %q tasks; escalating to L2", telemetry.TaskClass)
		promotedFrom = ModelL1
		confidence = 0.9
		escalationHistory = r.getRecentL1Failures(ctx, telemetry.TaskClass)
	}

	return &ModelRecommendation{
		ModelID:         lane,
		Source:           source,
		Reason:           reason,
		Confidence:       confidence,
		PromotedFrom:    promotedFrom,
		Lane:             lane,
		EscalationHistory: escalationHistory,
	}, nil
}

// RecordTaskTelemetry captures task execution metrics.
// PHASE 20 P004: Add per-task telemetry for empirical routing.
func (r *EmpiricalRouter) RecordTaskTelemetry(ctx context.Context, telemetry TaskTelemetry) error {
	r.recordTelemetryInternal(ctx, telemetry)
	return nil
}

// RecordTelemetryInternal stores telemetry for pattern learning.
func (r *EmpiricalRouter) recordTelemetryInternal(ctx context.Context, telemetry TaskTelemetry) {
	// In production: store to persistent storage (Redis, database)
	// For now: log to structured JSON
	log.Printf("[EmpiricalRouter] task_telemetry=%v", telemetry)

	// Update in-memory failure history for escalation decisions
	r.updateFailureHistory(ctx, telemetry.TaskClass, telemetry.LaneFirst, telemetry.LaneSuccess)
}

// updateFailureHistory tracks L1 failures to inform escalation decisions.
// PHASE 20 P003: Escalation requires EMPIRICAL evidence of L1 failure.
func (r *EmpiricalRouter) updateFailureHistory(ctx context.Context, taskClass, laneFirst, laneSuccess bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Only track L1 failures for escalation decisions
	if laneFirst && !laneSuccess {
		// Record L1 failure
		r.failureHistory[taskClass] = append(r.failureHistory[taskClass], taskClass)
		log.Printf("[EmpiricalRouter] L1 failure recorded for class %q", taskClass)
	}

	// Limit history size to prevent unbounded growth
	maxHistory := 50
	for class, failures := range r.failureHistory {
		if len(failures) > maxHistory {
			r.failureHistory[class] = failures[len(failures)-maxHistory:]
		}
	}
}

// getRecentL1Failures checks if L1 has empirically failed on this task class.
// PHASE 20 P003: Escalation requires EMPIRICAL evidence, not assumptions.
func (r *EmpiricalRouter) shouldEscalateToL2(ctx context.Context, taskClass string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Need empirical evidence: multiple recent L1 failures on this task class
	failures, exists := r.failureHistory[taskClass]
	// Require at least 3 failures to establish a pattern
	shouldEscalate := exists && len(failures) >= 3

	if shouldEscalate {
		log.Printf("[EmpiricalRouter] Escalating to L2: class %q has %d L1 failures", taskClass, len(failures))
	} else {
		log.Printf("[EmpiricalRouter] Staying on L1: class %q has %d failures (< threshold)", taskClass, len(failures))
	}

	return shouldEscalate
}

// RecommendModelWithFallback provides L2 as fallback when L1 is unavailable or clearly unsuitable.
// PHASE 20 P001: L2 is fallback lane, not default.
func (r *EmpiricalRouter) RecommendModelWithFallback(ctx context.Context, projectID, taskType string) (*ModelRecommendation, error) {
	reason := fmt.Sprintf("L1 unavailable or clearly unsuitable for %q/%q; using L2 fallback", projectID, taskType)
	return &ModelRecommendation{
		ModelID:      ModelL2,
		Source:       "fallback",
		Reason:       reason,
		Confidence:   0.6,
		Lane:         ModelL2,
	}, nil
}

// HasSufficientEfficiencyData checks if ledger has enough samples for reliable routing.
func (r *EmpiricalRouter) HasSufficientEfficiencyData(ctx context.Context) bool {
	// Check if we have enough samples for any model (minSamples)
	efficiencies, err := r.ledger.GetModelEfficiency(ctx, ModelL1, "reporting")
	if err == nil && len(efficiencies) >= r.minSamples {
		return true
	}
	// Check for L2
	efficiencies, err = r.ledger.GetModelEfficiency(ctx, ModelL2, "reporting")
	if err == nil && len(efficiencies) >= r.minSamples {
		return true
	}
	log.Printf("[EmpiricalRouter] Insufficient efficiency data: L1=%v L2=%v", err, err)
	return false
}

// IsReportingTask checks if a task class is for regular reporting (should go to L1 first).
func IsReportingTask(taskClass string) bool {
	reportingTasks := []string{
		"dead_code",
		"defects",
		"tech_debt",
		"roadmap",
		"bug_hunting",
		"stub_hunting",
		"executive_summary",
	}
	for _, t := range reportingTasks {
		if taskClass == t {
			return true
		}
	}
	return false
}
