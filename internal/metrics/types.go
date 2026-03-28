// Package metrics provides per-task observability for zen-brain1 L1/L2 execution.
//
// Every L1 remediation call records structured telemetry including model, lane,
// timing, completion class, attribution, and quality scores. Metrics are persisted
// as JSONL for replay/analysis and exposed as computed summaries.
//
// Core principle: healthy-but-slow is acceptable. Observability is required for
// evidence-based scaling decisions.
package metrics

import "time"

// CompletionClass describes how a task's L1 call ended.
type CompletionClass string

const (
	ClassFastProductive    CompletionClass = "fast-productive"    // <30s, produced usable output
	ClassSlowButProductive CompletionClass = "slow-but-productive" // >30s, produced usable output
	ClassTruncatedRepaired CompletionClass = "truncated-repaired"  // output was truncated, bracket-repair recovered it
	ClassTimeout           CompletionClass = "timeout"            // request timed out, no usable output
	ClassParseFail         CompletionClass = "parse-fail"         // output could not be parsed as JSON
	ClassValidationFail    CompletionClass = "validation-fail"    // output parsed but failed quality gate
)

// ProducedBy describes who/what produced the final artifact.
type ProducedBy string

const (
	ProducedByL1         ProducedBy = "l1"
	ProducedByL1Partial  ProducedBy = "l1-partial"
	ProducedByL1Failed   ProducedBy = "l1-failed"
	ProducedBySupervisor ProducedBy = "supervisor"
	ProducedByScript     ProducedBy = "script"
	ProducedByNone       ProducedBy = "none"
)

// TaskTelemetryRecord is the per-task telemetry entry.
// One record is written for each L1/L2 call attempt.
type TaskTelemetryRecord struct {
	// Identification
	Timestamp   time.Time `json:"timestamp"`
	RunID       string    `json:"run_id"`
	TaskID      string    `json:"task_id"`
	JiraKey     string    `json:"jira_key,omitempty"`
	ScheduleName string   `json:"schedule_name,omitempty"`

	// Model/Lane
	Model    string `json:"model"`
	Lane     string `json:"lane"`    // "l1-local", "l1-api", "l2-api", "l2-local"
	Provider string `json:"provider"` // "ollama", "llama-cpp", "openai-compat"

	// Sizing
	PromptSizeChars int `json:"prompt_size_chars"`
	OutputSizeChars int `json:"output_size_chars"`
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`

	// Timing
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	WallTimeMs       int64         `json:"wall_time_ms"`
	FirstTokenMs     int64         `json:"first_token_ms,omitempty"` // if available
	LoadDurationMs   int64         `json:"load_duration_ms,omitempty"`

	// Classification
	CompletionClass CompletionClass `json:"completion_class"`
	ProducedBy      ProducedBy      `json:"produced_by"`
	AttemptNumber   int             `json:"attempt_number"` // 1 = first attempt, 2+ = retry

	// Quality
	QualityScore    float64 `json:"quality_score,omitempty"`    // 0-25
	RepairUsed      bool    `json:"repair_used"`                // truncation repair
	RepairSucceeded bool    `json:"repair_succeeded,omitempty"`

	// Task metadata
	TaskClass    string `json:"task_class,omitempty"`    // "remediation", "discovery", "ticketize", "dedup"
	RemediationType string `json:"remediation_type,omitempty"` // "code_edit", "config_change", "doc_update"
	FinalStatus  string `json:"final_status,omitempty"`  // "success", "needs_review", "blocked", "to_escalate"

	// Jira outcome
	JiraTransition string `json:"jira_transition,omitempty"` // target status after processing
	JiraUpdated    bool   `json:"jira_updated"`

	// Evidence
	EvidencePackPath string `json:"evidence_pack_path,omitempty"`
}

// ComputedMetrics holds the aggregated metrics computed from telemetry records.
type ComputedMetrics struct {
	ComputedAt      time.Time `json:"computed_at"`
	WindowStart     time.Time `json:"window_start"`
	WindowEnd       time.Time `json:"window_end"`
	WindowName      string    `json:"window_name"` // "last_hour", "last_6h", "last_24h", "all"

	// Counts
	TotalTasks      int `json:"total_tasks"`
	SuccessTasks    int `json:"success_tasks"`
	FailedTasks     int `json:"failed_tasks"`

	// Rates
	L1ProducedRate  float64 `json:"l1_produced_rate"`   // fraction of tasks with produced_by = l1
	TimeoutRate     float64 `json:"timeout_rate"`
	TruncationRate  float64 `json:"truncation_rate"`
	RepairRate      float64 `json:"repair_rate"`        // fraction of truncated that were repaired
	ParseFailRate   float64 `json:"parse_fail_rate"`
	ValidationFailRate float64 `json:"validation_fail_rate"`

	// Latency (ms)
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	MaxLatencyMs  float64 `json:"max_latency_ms"`

	// Throughput
	CharsPerSec   float64 `json:"chars_per_sec,omitempty"`
	TasksPerHour  float64 `json:"tasks_per_hour"`
	DonePerHour   float64 `json:"done_per_hour"`
	DonePerDay    float64 `json:"done_per_day"`

	// By model
	ByModel map[string]ModelMetrics `json:"by_model,omitempty"`
	// By lane
	ByLane map[string]LaneMetrics `json:"by_lane,omitempty"`
	// By completion class
	ByClass map[string]int `json:"by_class"`

	// Worker info (set by caller)
	ActiveWorkers  int     `json:"active_workers,omitempty"`
	WorkerUtilization float64 `json:"worker_utilization,omitempty"`
	QueueDepth      int     `json:"queue_depth,omitempty"`

	// Quality
	AvgQualityScore float64 `json:"avg_quality_score,omitempty"`
}

// ModelMetrics holds per-model metrics.
type ModelMetrics struct {
	Count          int     `json:"count"`
	SuccessCount   int     `json:"success_count"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	P95LatencyMs   float64 `json:"p95_latency_ms"`
	TimeoutRate    float64 `json:"timeout_rate"`
	L1ProducedRate float64 `json:"l1_produced_rate"`
	TruncationRate float64 `json:"truncation_rate"`
	AvgCharsPerSec float64 `json:"avg_chars_per_sec,omitempty"`
}

// LaneMetrics holds per-lane metrics.
type LaneMetrics struct {
	Count          int     `json:"count"`
	SuccessCount   int     `json:"success_count"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	P95LatencyMs   float64 `json:"p95_latency_ms"`
	TimeoutRate    float64 `json:"timeout_rate"`
	L1ProducedRate float64 `json:"l1_produced_rate"`
}
