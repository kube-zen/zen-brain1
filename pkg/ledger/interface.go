// Package ledger provides the ZenLedger interface for token and cost accounting.
// ZenLedger tracks the yield (value produced) per token spent.
//
// The core metric is not cost—it's value-per-token.
// A task that costs $0.40 in API tokens but produces a merged PR
// is cheaper than a task that costs $0.02 but produces a comment
// that needs three human corrections.
package ledger

import (
	"context"
	"time"
)

// Outcome represents the result of a task.
type Outcome string

const (
	OutcomeCompleted      Outcome = "completed"
	OutcomeFailed         Outcome = "failed"
	OutcomeHumanCorrected Outcome = "human_corrected"
	OutcomeAbandoned      Outcome = "abandoned"
)

// EvidenceClass represents the type of evidence produced.
type EvidenceClass string

const (
	EvidencePRMerged     EvidenceClass = "pr_merged"
	EvidenceTestPassed   EvidenceClass = "test_passed"
	EvidenceDocUpdated   EvidenceClass = "doc_updated"
	EvidencePlanApproved EvidenceClass = "plan_approved"
	EvidenceSummary      EvidenceClass = "summary"
)

// InferenceType represents the type of inference.
type InferenceType string

const (
	InferenceChat   InferenceType = "chat"
	InferenceEmbed  InferenceType = "embedding"
	InferenceRerank InferenceType = "rerank"
)

// Source indicates whether inference was local or API.
type Source string

const (
	SourceLocal Source = "local"
	SourceAPI   Source = "api"
)

// TokenRecord represents a single LLM usage record.
type TokenRecord struct {
	SessionID     string        `json:"session_id"`
	TaskID        string        `json:"task_id"`
	AgentRole     string        `json:"agent_role"`
	ModelID       string        `json:"model_id"`
	InferenceType InferenceType `json:"inference_type"`
	Source        Source        `json:"source"`

	// Cost side
	TokensInput  int64   `json:"tokens_input"`
	TokensOutput int64   `json:"tokens_output"`
	TokensCached int64   `json:"tokens_cached"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMs    int64   `json:"latency_ms"`

	// Yield side
	Outcome          Outcome       `json:"outcome"`
	EvidenceClass    EvidenceClass `json:"evidence_class"`
	HumanCorrections int           `json:"human_corrections"`
	SREDEligible     bool          `json:"sred_eligible"`

	Timestamp time.Time `json:"timestamp"`
	ClusterID string    `json:"cluster_id"`
	ProjectID string    `json:"project_id"`
}

// ModelEfficiency represents historical efficiency data for a model.
type ModelEfficiency struct {
	ModelID          string  `json:"model_id"`
	AvgCostPerTask   float64 `json:"avg_cost_per_task"`
	AvgTokensPerTask int64   `json:"avg_tokens_per_task"`
	SuccessRate      float64 `json:"success_rate"`
	AvgCorrections   float64 `json:"avg_corrections"`
	AvgLatencyMs     int64   `json:"avg_latency_ms"`
	SampleSize       int     `json:"sample_size"`
}

// BudgetStatus represents current spending against budget.
type BudgetStatus struct {
	ProjectID      string    `json:"project_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	SpentUSD       float64   `json:"spent_usd"`
	BudgetLimitUSD float64   `json:"budget_limit_usd"`
	RemainingUSD   float64   `json:"remaining_usd"`
	PercentUsed    float64   `json:"percent_used"`
}

// ZenLedgerClient is the interface used by the Planner Agent
// for cost-aware model selection and policy-aware routing.
//
// This interface is defined in Block 1.7.1 (before Planner in Block 2.5)
// to preserve clean build order without circular dependencies.
type ZenLedgerClient interface {
	// GetModelEfficiency returns historical efficiency data for models
	// on a specific task type. Used by Planner to select optimal model.
	GetModelEfficiency(ctx context.Context, projectID string, taskType string) ([]ModelEfficiency, error)

	// GetCostBudgetStatus returns current spending against budget limits.
	// Used by Planner to enforce budget constraints.
	GetCostBudgetStatus(ctx context.Context, projectID string) (*BudgetStatus, error)

	// RecordPlannedModelSelection logs the Planner's model choice
	// for later analysis of model selection quality.
	RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error
}

// TokenRecorder is the interface used by Worker Agents
// to record token usage after each LLM call.
type TokenRecorder interface {
	// Record records a token usage event.
	Record(ctx context.Context, record TokenRecord) error

	// RecordBatch records multiple token usage events.
	RecordBatch(ctx context.Context, records []TokenRecord) error
}
