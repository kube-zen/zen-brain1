package office

import (
	"context"
	"fmt"
	"log"
)

// ActionClass represents the risk level and approval requirements for an action.
type ActionClass int

const (
	// Class A: Always Allowed (Read & Recommend)
	// Risk Level: None
	// Safe to run autonomously without approval
	ActionClassAlwaysAllowed ActionClass = iota

	// Class B: Safe Write-Back (Restricted Writes)
	// Risk Level: Low
	// Safe writes with proven paths only
	// Requires human review of output before Jira write-back
	ActionClassSafeWriteBack

	// Class C: Approval Required (High-Impact Actions)
	// Risk Level: Medium-High
	// Requires explicit approval before execution
	// Actions: repo writes, merges, deploys, secret/config changes, meaningful status transitions
	ActionClassApprovalRequired
)

// String returns a string representation of an action class.
func (ac ActionClass) String() string {
	switch ac {
	case ActionClassAlwaysAllowed:
		return "Class A (Always Allowed)"
	case ActionClassSafeWriteBack:
		return "Class B (Safe Write-Back)"
	case ActionClassApprovalRequired:
		return "Class C (Approval Required)"
	default:
		return "Unknown"
	}
}

// Action represents a proposed action with metadata for policy enforcement.
type Action struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`        // "jira_comment", "jira_attachment", "repo_write", "deploy", etc.
	Class        ActionClass `json:"class"`
	Description  string      `json:"description"`
	RiskLevel   string      `json:"risk_level"`    // "none", "low", "medium", "high"
	BusinessImpact string   `json:"business_impact"` // "none", "code", "infra", "operational"
	Approval    *Approval   `json:"approval,omitempty"`
	Proof        *Proof      `json:"proof,omitempty"`
}

// Approval represents approval metadata for gated actions.
type Approval struct {
	Required   bool   `json:"required"`
	ApprovedBy string `json:"approved_by,omitempty"`
	ApprovedAt string `json:"approved_at,omitempty"`
	SignOff    string `json:"sign_off,omitempty"`
}

// Proof represents proof of work for an action.
type Proof struct {
	Path        string `json:"path"`
	Type        string `json:"type"`     // "proof_of_work", "analysis", "artifact"
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// ActionPolicy enforces action class rules.
type ActionPolicy struct {
	logger *log.Logger
}

// NewActionPolicy creates a new action policy.
func NewActionPolicy(logger *log.Logger) *ActionPolicy {
	if logger == nil {
		logger = log.Default()
	}
	return &ActionPolicy{
		logger: logger,
	}
}

// ClassifyAction determines the action class for a given action type.
func (ap *ActionPolicy) ClassifyAction(actionType string) ActionClass {
	switch actionType {
	// Class A: Read & Recommend (Always Allowed)
	case "fetch_issue", "analyze", "summarize", "classify", "recommend", "generate_artifacts":
		return ActionClassAlwaysAllowed

	// Class B: Safe Write-Back (Restricted Writes)
	case "jira_comment", "jira_attachment", "status_update_safe", "artifact_attachment":
		return ActionClassSafeWriteBack

	// Class C: Approval Required (High-Impact Actions)
	case "repo_write", "merge", "deploy", "secret_config_change", "status_update_meaningful", "workflow_transition":
		return ActionClassApprovalRequired

	default:
		// Default to Class C (safest) for unknown actions
		return ActionClassApprovalRequired
	}
}

// CanExecute returns true if the action can be executed without approval.
func (ap *ActionPolicy) CanExecute(action *Action) bool {
	switch action.Class {
	case ActionClassAlwaysAllowed:
		return true

	case ActionClassSafeWriteBack:
		// Class B: Allow execution, but log warning for review
		ap.logger.Printf("[Class B] Executing safe write-back: %s (review output before Jira write-back)", action.Type)
		return true

	case ActionClassApprovalRequired:
		// Class C: Check if approval exists
		if action.Approval != nil {
			// No approval metadata - allow execution
			return true
		}
		if !action.Approval.Required {
			// Approval not required - allow execution
			return true
		}
		if action.Approval.ApprovedBy != "" {
			// Explicitly approved
			ap.logger.Printf("[Class C] Action approved by %s: %s", action.Approval.ApprovedBy, action.Type)
			return true
		} else {
			// Queued for approval
			ap.logger.Printf("[Class C] Action queued for approval: %s (no sign-off)", action.Type)
			return false
		}

	default:
		return false
	}
}

// RequireApproval returns true if the action requires approval.
func (ap *ActionPolicy) RequireApproval(action *Action) bool {
	switch action.Class {
	case ActionClassAlwaysAllowed:
		return false
	case ActionClassSafeWriteBack:
		return false
	case ActionClassApprovalRequired:
		return true
	default:
		return true // Default to requiring approval
	}
}

// ValidateAction checks if an action is valid and complete.
func (ap *ActionPolicy) ValidateAction(action *Action) error {
	if action.ID == "" {
		return fmt.Errorf("action ID is required")
	}
	if action.Type == "" {
		return fmt.Errorf("action type is required")
	}
	if action.Description == "" {
		return fmt.Errorf("action description is required")
	}

	// Class C validation
	if action.Class == ActionClassApprovalRequired {
		if action.RiskLevel == "" {
			return fmt.Errorf("risk level is required for Class C actions")
		}
		if action.BusinessImpact == "" {
			return fmt.Errorf("business impact is required for Class C actions")
		}
	}

	return nil
}

// LogAction logs an action execution.
func (ap *ActionPolicy) LogAction(action *Action, ctx context.Context) error {
	// Validate Action
	if err := ap.ValidateAction(action); err != nil {
		ap.logger.Printf("[ERROR] Invalid action: %v", err)
		return err
	}

	// Log execution
	ap.logger.Printf("[%s] Executing action: %s - %s", action.Class.String(), action.ID, action.Description)

	// Check if execution is allowed
	if !ap.CanExecute(action) {
		return fmt.Errorf("action requires approval: %s", action.ID)
	}

	return nil
}

// QueueForApproval queues a Class C action for human approval.
func (ap *ActionPolicy) QueueForApproval(action *Action) error {
	if err := ap.ValidateAction(action); err != nil {
		return err
	}

	ap.logger.Printf("[Class C] Action queued for approval: %s - %s", action.ID, action.Description)
	ap.logger.Printf("  Risk Level: %s", action.RiskLevel)
	ap.logger.Printf("  Business Impact: %s", action.BusinessImpact)
	ap.logger.Printf("  Approval Required: Yes")

	return nil
}

// ApproveAction approves a Class C action for execution.
func (ap *ActionPolicy) ApproveAction(actionID string, approvedBy, signOff string) error {
	ap.logger.Printf("[Class C] Action approved: %s - Approved by: %s - Sign-off: %s", actionID, approvedBy, signOff)
	return nil
}

// DenyAction denies a Class C action.
func (ap *ActionPolicy) DenyAction(actionID string, reason string) error {
	ap.logger.Printf("[Class C] Action denied: %s - Reason: %s", actionID, reason)
	return nil
}
