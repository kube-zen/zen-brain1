// Package gatekeeper provides the Human Gatekeeper for zen-brain.
// The Human Gatekeeper provides human-facing interfaces for approval workflows,
// notifications, and audit logging.
package gatekeeper

import (
	"context"
	"time"

	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// Gatekeeper provides human interfaces for approval workflows.
type Gatekeeper interface {
	// GetPendingApprovals returns sessions waiting for human approval.
	GetPendingApprovals(ctx context.Context, filter ApprovalFilter) ([]*ApprovalRequest, error)
	
	// GetApproval returns a specific approval request.
	GetApproval(ctx context.Context, approvalID string) (*ApprovalRequest, error)
	
	// Approve approves a session.
	Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
	
	// Reject rejects a session.
	Reject(ctx context.Context, approvalID string, decision ApprovalDecision) error
	
	// DelegateApproval delegates an approval to another user.
	DelegateApproval(ctx context.Context, approvalID string, delegateTo string, reason string) error
	
	// EscalateApproval escalates an approval to a higher authority.
	EscalateApproval(ctx context.Context, approvalID string, reason string) error
	
	// GetApprovalHistory returns approval history for a session.
	GetApprovalHistory(ctx context.Context, sessionID string) ([]*ApprovalEvent, error)
	
	// RegisterNotifier registers a notification channel.
	RegisterNotifier(ctx context.Context, notifier Notifier) error
	
	// Close closes the gatekeeper.
	Close() error
}

// ApprovalRequest represents a request for human approval.
type ApprovalRequest struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	WorkItem        *contracts.WorkItem    `json:"work_item"`
	Analysis        *contracts.AnalysisResult `json:"analysis,omitempty"`
	EstimatedCostUSD float64               `json:"estimated_cost_usd"`
	Requester       string                 `json:"requester"` // Agent/system that requested approval
	RequestedAt     time.Time              `json:"requested_at"`
	Deadline        *time.Time             `json:"deadline,omitempty"`
	Priority        string                 `json:"priority"` // "low", "medium", "high", "critical"
	ApprovalLevel   string                 `json:"approval_level"` // "team_lead", "manager", "director", "executive"
	Notes           string                 `json:"notes,omitempty"`
	AssignedTo      []string               `json:"assigned_to"` // Users/roles who can approve
	Status          ApprovalStatus         `json:"status"`
}

// ApprovalDecision represents a human decision on an approval request.
type ApprovalDecision struct {
	Decision    string    `json:"decision"` // "approved", "rejected"
	DecidedBy   string    `json:"decided_by"` // User ID
	DecidedAt   time.Time `json:"decided_at"`
	Reason      string    `json:"reason,omitempty"`
	Attachments []string  `json:"attachments,omitempty"` // Evidence/justification attachments
}

// ApprovalEvent records an approval-related event.
type ApprovalEvent struct {
	ID          string            `json:"id"`
	ApprovalID  string            `json:"approval_id"`
	SessionID   string            `json:"session_id"`
	EventType   string            `json:"event_type"` // "requested", "approved", "rejected", "delegated", "escalated", "reminded"
	Actor       string            `json:"actor"`      // Who performed the action
	Timestamp   time.Time         `json:"timestamp"`
	Details     map[string]string `json:"details,omitempty"`
}

// ApprovalStatus represents the status of an approval request.
type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusDelegated ApprovalStatus = "delegated"
	ApprovalStatusEscalated ApprovalStatus = "escalated"
	ApprovalStatusExpired   ApprovalStatus = "expired"
)

// ApprovalFilter filters approval requests.
type ApprovalFilter struct {
	Status        *ApprovalStatus `json:"status,omitempty"`
	AssignedTo    *string         `json:"assigned_to,omitempty"`
	Priority      *string         `json:"priority,omitempty"`
	ApprovalLevel *string         `json:"approval_level,omitempty"`
	SessionID     *string         `json:"session_id,omitempty"`
	WorkItemID    *string         `json:"work_item_id,omitempty"`
	RequestedAfter *time.Time     `json:"requested_after,omitempty"`
	RequestedBefore *time.Time    `json:"requested_before,omitempty"`
	Limit         int             `json:"limit,omitempty"`
	Offset        int             `json:"offset,omitempty"`
}

// Notifier sends notifications to humans.
type Notifier interface {
	// Name returns the notifier name.
	Name() string
	
	// SendApprovalRequest sends an approval request notification.
	SendApprovalRequest(ctx context.Context, req *ApprovalRequest) error
	
	// SendApprovalDecision sends an approval decision notification.
	SendApprovalDecision(ctx context.Context, req *ApprovalRequest, decision ApprovalDecision) error
	
	// SendReminder sends a reminder for a pending approval.
	SendReminder(ctx context.Context, req *ApprovalRequest) error
	
	// SupportsChannel returns true if the notifier supports the given channel.
	SupportsChannel(channel string) bool
}

// NotificationChannel represents a notification delivery channel.
type NotificationChannel string

const (
	ChannelSlack   NotificationChannel = "slack"
	ChannelEmail   NotificationChannel = "email"
	ChannelTeams   NotificationChannel = "teams"
	ChannelWebhook NotificationChannel = "webhook"
	ChannelConsole NotificationChannel = "console"
)

// Config holds configuration for the Human Gatekeeper.
type Config struct {
	// Planner integration
	Planner planner.Planner `yaml:"-" json:"-"`
	
	// Notification channels
	DefaultChannels []NotificationChannel `yaml:"default_channels" json:"default_channels"`
	
	// Approval workflow
	DefaultApprovalLevel string        `yaml:"default_approval_level" json:"default_approval_level"`
	DefaultDeadlineHours int           `yaml:"default_deadline_hours" json:"default_deadline_hours"`
	ReminderInterval     time.Duration `yaml:"reminder_interval" json:"reminder_interval"`
	EscalationInterval   time.Duration `yaml:"escalation_interval" json:"escalation_interval"`
	
	// Audit logging
	AuditLogEnabled     bool   `yaml:"audit_log_enabled" json:"audit_log_enabled"`
	AuditLogDirectory   string `yaml:"audit_log_directory" json:"audit_log_directory"`
	
	// Web interface
	HTTPEnabled         bool   `yaml:"http_enabled" json:"http_enabled"`
	HTTPPort           int    `yaml:"http_port" json:"http_port"`
	HTTPAuthEnabled    bool   `yaml:"http_auth_enabled" json:"http_auth_enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultChannels: []NotificationChannel{ChannelConsole},
		DefaultApprovalLevel: "team_lead",
		DefaultDeadlineHours: 24,
		ReminderInterval:     4 * time.Hour,
		EscalationInterval:   8 * time.Hour,
		AuditLogEnabled:      true,
		AuditLogDirectory:    "./data/audit",
		HTTPEnabled:         true,
		HTTPPort:           8080,
		HTTPAuthEnabled:    false,
	}
}