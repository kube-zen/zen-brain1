// Package gate provides the ZenGate interface for admission control.
// ZenGate validates and authorizes requests before they enter the Factory.
// It implements input validation, authorization checks, and policy enforcement.
package gate

import (
	"context"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/policy"
)

// AdmissionRequest represents a request to enter the Factory.
type AdmissionRequest struct {
	// RequestID uniquely identifies this admission request
	RequestID string `json:"request_id"`

	// WorkItemID is the work item being processed
	WorkItemID string `json:"work_item_id"`

	// SessionID is the session making the request
	SessionID string `json:"session_id"`

	// TaskID is the task being requested
	TaskID string `json:"task_id,omitempty"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`

	// Action is the requested action
	Action policy.Action `json:"action"`

	// Resource is the resource being acted upon
	Resource policy.Resource `json:"resource"`

	// Subject is the entity making the request
	Subject policy.Subject `json:"subject"`

	// Payload contains request-specific data
	Payload map[string]interface{} `json:"payload,omitempty"`

	// Timestamp is when the request was made
	Timestamp time.Time `json:"timestamp"`
}

// AdmissionResponse represents the admission decision.
type AdmissionResponse struct {
	// RequestID matches the admission request
	RequestID string `json:"request_id"`

	// Allowed indicates whether the request is allowed
	Allowed bool `json:"allowed"`

	// Reason explains the decision
	Reason string `json:"reason,omitempty"`

	// Error contains any error during evaluation
	Error string `json:"error,omitempty"`

	// RequiresApproval indicates whether approval is required
	RequiresApproval bool `json:"requires_approval,omitempty"`

	// ApprovalLevel indicates the required approval level
	ApprovalLevel string `json:"approval_level,omitempty"`

	// Conditions are conditions that must be satisfied
	Conditions []policy.Condition `json:"conditions,omitempty"`

	// Obligations are obligations that must be performed
	Obligations []policy.Obligation `json:"obligations,omitempty"`

	// EvaluatedAt is when the decision was made
	EvaluatedAt time.Time `json:"evaluated_at"`

	// EvaluationDuration is how long evaluation took
	EvaluationDuration time.Duration `json:"evaluation_duration"`
}

// ValidationError represents a validation failure.
type ValidationError struct {
	// Field is the field that failed validation
	Field string `json:"field"`

	// Message describes the validation failure
	Message string `json:"message"`

	// Code is a machine-readable error code
	Code string `json:"code,omitempty"`
}

// ZenGate is the interface for admission control.
type ZenGate interface {
	// Admit evaluates an admission request and returns a decision.
	Admit(ctx context.Context, req AdmissionRequest) (*AdmissionResponse, error)

	// Validate validates an admission request without making a decision.
	// Returns validation errors if any.
	Validate(ctx context.Context, req AdmissionRequest) ([]ValidationError, error)

	// RegisterValidator registers a custom validator for a request type.
	RegisterValidator(ctx context.Context, validator Validator) error

	// RegisterPolicy registers a policy evaluator for a request type.
	RegisterPolicy(ctx context.Context, policy policy.ZenPolicy) error

	// Stats returns gate statistics.
	Stats(ctx context.Context) (map[string]interface{}, error)

	// Close closes the gate.
	Close() error
}

// Validator is the interface for custom validation logic.
type Validator interface {
	// Name returns the validator name
	Name() string

	// SupportedActions returns the actions this validator supports
	SupportedActions() []policy.Action

	// Validate validates the request
	Validate(ctx context.Context, req AdmissionRequest) ([]ValidationError, error)
}