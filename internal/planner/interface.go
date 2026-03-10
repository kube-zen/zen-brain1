// Package planner provides the Planner Agent for zen-brain.
// The Planner Agent coordinates the complete pipeline:
// Office → Analyzer → Factory → Session → Evidence
package planner

import (
	"context"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/session"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// Planner coordinates work execution across the zen-brain pipeline.
type Planner interface {
	// ProcessWorkItem processes a new work item from an Office connector.
	// This is the main entry point for the Planner.
	ProcessWorkItem(ctx context.Context, workItem *contracts.WorkItem) error

	// ProcessBatch processes multiple work items in batch.
	ProcessBatch(ctx context.Context, workItems []*contracts.WorkItem) error

	// GetSessionStatus returns the current status of a session.
	GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error)

	// ApproveSession approves a session that's pending approval.
	ApproveSession(ctx context.Context, sessionID string, approver string, notes string) error

	// RejectSession rejects a session that's pending approval.
	RejectSession(ctx context.Context, sessionID string, rejector string, reason string) error

	// CancelSession cancels an active session.
	CancelSession(ctx context.Context, sessionID string, canceller string, reason string) error

	// GetPendingApprovals returns sessions waiting for approval.
	GetPendingApprovals(ctx context.Context) ([]*contracts.Session, error)

	// Close closes the planner and releases resources.
	Close() error
}

// SessionStatus represents the detailed status of a session.
type SessionStatus struct {
	Session        *contracts.Session        `json:"session"`
	WorkItem       *contracts.WorkItem       `json:"work_item,omitempty"`
	Analysis       *contracts.AnalysisResult `json:"analysis,omitempty"`
	BrainTaskSpecs []contracts.BrainTaskSpec `json:"brain_task_specs,omitempty"`
	Evidence       []contracts.EvidenceItem  `json:"evidence,omitempty"`

	// Metrics
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	ActualCostUSD    float64 `json:"actual_cost_usd,omitempty"`
	TimeElapsed      string  `json:"time_elapsed,omitempty"`
	ProgressPercent  float64 `json:"progress_percent,omitempty"`
}

// Config holds configuration for the Planner Agent.
type Config struct {
	// Component references
	OfficeManager  *office.Manager         `yaml:"-" json:"-"`
	Analyzer       analyzer.IntentAnalyzer `yaml:"-" json:"-"`
	SessionManager session.Manager         `yaml:"-" json:"-"`
	LedgerClient   ledger.ZenLedgerClient  `yaml:"-" json:"-"`
	ZenContext     zenctx.ZenContext       `yaml:"-" json:"-"`
	Factory        factory.Factory         `yaml:"-" json:"-"`

	// Model selection
	DefaultModel  string  `yaml:"default_model" json:"default_model"`
	FallbackModel string  `yaml:"fallback_model" json:"fallback_model"`
	MaxCostUSD    float64 `yaml:"max_cost_usd" json:"max_cost_usd"`

	// Approval workflow
	RequireApproval bool    `yaml:"require_approval" json:"require_approval"`
	AutoApproveCost float64 `yaml:"auto_approve_cost" json:"auto_approve_cost"` // Auto-approve if cost < threshold

	// Timeouts
	AnalysisTimeout  int `yaml:"analysis_timeout" json:"analysis_timeout"`   // seconds
	ExecutionTimeout int `yaml:"execution_timeout" json:"execution_timeout"` // seconds

	// Monitoring
	MetricsEnabled bool `yaml:"metrics_enabled" json:"metrics_enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultModel:     "glm-4.7",
		FallbackModel:    "glm-4.7",
		MaxCostUSD:       10.0,
		RequireApproval:  true,
		AutoApproveCost:  2.0,
		AnalysisTimeout:  300,  // 5 minutes
		ExecutionTimeout: 3600, // 1 hour
		MetricsEnabled:   true,
	}
}

// ModelSelection represents a model selected for a task.
type ModelSelection struct {
	ModelID          string   `json:"model_id"`
	Reason           string   `json:"reason"`
	EstimatedCostUSD float64  `json:"estimated_cost_usd"`
	Confidence       float64  `json:"confidence"`
	Alternatives     []string `json:"alternatives,omitempty"`
}

// ApprovalRequest represents a request for human approval.
type ApprovalRequest struct {
	SessionID      string  `json:"session_id"`
	WorkItemTitle  string  `json:"work_item_title"`
	EstimatedCost  float64 `json:"estimated_cost"`
	AnalysisNotes  string  `json:"analysis_notes"`
	BrainTaskCount int     `json:"brain_task_count"`
	RequestedAt    string  `json:"requested_at"`
	RequestedBy    string  `json:"requested_by"`
	Urgency        string  `json:"urgency"` // "low", "medium", "high"
}
