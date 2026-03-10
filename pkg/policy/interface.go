// Package policy provides the ZenPolicy interface for declarative rule evaluation.
// ZenPolicy defines what actions are allowed, required, or forbidden.
// Policies are defined once and enforced everywhere (Office and Factory).
package policy

import (
	"context"
	"time"
)

// Action represents an action that can be evaluated.
type Action string

const (
	ActionCreateSession   Action = "create_session"
	ActionAnalyzeIntent   Action = "analyze_intent"
	ActionGeneratePlan    Action = "generate_plan"
	ActionExecuteTask     Action = "execute_task"
	ActionUpdateStatus    Action = "update_status"
	ActionRequestApproval Action = "request_approval"
	ActionAccessKB        Action = "access_kb"
	ActionCallLLM         Action = "call_llm"
)

// Resource represents a resource being acted upon.
type Resource struct {
	// Type is the resource type (session, task, work_item, kb, etc.)
	Type string `json:"type"`

	// ID is the resource identifier
	ID string `json:"id,omitempty"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`

	// Attributes are additional resource attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Subject represents the entity performing the action.
type Subject struct {
	// Type is the subject type (human, agent, system)
	Type string `json:"type"`

	// ID is the subject identifier
	ID string `json:"id,omitempty"`

	// Roles are the roles assigned to the subject
	Roles []string `json:"roles,omitempty"`

	// Attributes are additional subject attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Context contains additional evaluation context.
type Context struct {
	// Timestamp is when the evaluation occurs
	Timestamp time.Time `json:"timestamp"`

	// SessionID is the current session
	SessionID string `json:"session_id,omitempty"`

	// TaskID is the current task
	TaskID string `json:"task_id,omitempty"`

	// WorkItemID is the current work item
	WorkItemID string `json:"work_item_id,omitempty"`

	// Environment is the execution environment (dev, staging, prod)
	Environment string `json:"environment,omitempty"`

	// Additional context data
	Data map[string]interface{} `json:"data,omitempty"`
}

// PolicyRule defines a single rule.
type PolicyRule struct {
	// Name is the rule name
	Name string `json:"name"`

	// Description explains the rule
	Description string `json:"description,omitempty"`

	// Version is the rule version
	Version string `json:"version"`

	// Priority determines rule evaluation order (higher = earlier)
	Priority int `json:"priority,omitempty"`

	// Conditions are the conditions that must be satisfied
	Conditions []Condition `json:"conditions,omitempty"`

	// Effect is the rule effect (allow, deny, require_approval, require_evidence)
	Effect string `json:"effect"`

	// Obligations are actions that must be performed if rule matches
	Obligations []Obligation `json:"obligations,omitempty"`

	// Metadata contains additional rule metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Condition defines a condition for rule evaluation.
type Condition struct {
	// Field is the field to evaluate (subject.id, resource.type, etc.)
	Field string `json:"field"`

	// Operator is the comparison operator (equals, not_equals, in, not_in, contains, matches, lt, gt, etc.)
	Operator string `json:"operator"`

	// Value is the expected value
	Value interface{} `json:"value,omitempty"`
}

// Obligation defines an action that must be performed.
type Obligation struct {
	// Type is the obligation type (log, notify, require_approval, collect_evidence, enforce_quota)
	Type string `json:"type"`

	// Parameters are obligation-specific parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// EvaluationRequest is a request to evaluate a policy.
type EvaluationRequest struct {
	// Action being performed
	Action Action `json:"action"`

	// Resource being acted upon
	Resource Resource `json:"resource"`

	// Subject performing the action
	Subject Subject `json:"subject"`

	// Context for evaluation
	Context Context `json:"context"`
}

// EvaluationResult is the result of policy evaluation.
type EvaluationResult struct {
	// Allowed indicates whether the action is allowed
	Allowed bool `json:"allowed"`

	// Denied indicates whether the action is denied
	Denied bool `json:"denied"`

	// RequiresApproval indicates whether approval is required
	RequiresApproval bool `json:"requires_approval"`

	// ApprovalLevel indicates the approval level (team_lead, manager, director, etc.)
	ApprovalLevel string `json:"approval_level,omitempty"`

	// Obligations that must be performed
	Obligations []Obligation `json:"obligations,omitempty"`

	// MatchedRules are the rules that matched
	MatchedRules []PolicyRule `json:"matched_rules,omitempty"`

	// EvaluationTime is when evaluation occurred
	EvaluationTime time.Time `json:"evaluation_time"`

	// Error if evaluation failed
	Error string `json:"error,omitempty"`
}

// ZenPolicy is the interface for policy evaluation.
type ZenPolicy interface {
	// Evaluate evaluates a policy for the given request.
	Evaluate(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error)

	// LoadRule loads a rule into the policy engine.
	LoadRule(ctx context.Context, rule PolicyRule) error

	// LoadRules loads multiple rules.
	LoadRules(ctx context.Context, rules []PolicyRule) error

	// RemoveRule removes a rule.
	RemoveRule(ctx context.Context, ruleName string) error

	// ListRules lists all loaded rules.
	ListRules(ctx context.Context) ([]PolicyRule, error)

	// ValidateRule validates a rule without loading it.
	ValidateRule(ctx context.Context, rule PolicyRule) error

	// Stats returns policy engine statistics.
	Stats(ctx context.Context) (map[string]interface{}, error)

	// Close closes the policy engine.
	Close() error
}
