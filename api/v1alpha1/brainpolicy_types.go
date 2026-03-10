// Package v1alpha1 defines the BrainPolicy CRD (Block 4).
// BrainPolicy is a cluster-scoped policy for actions, budgets, and approvals.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=bp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BrainPolicy is a cluster-scoped policy for the Factory (Block 4).
type BrainPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BrainPolicySpec   `json:"spec,omitempty"`
	Status BrainPolicyStatus `json:"status,omitempty"`
}

// BrainPolicySpec defines the desired state of the policy.
type BrainPolicySpec struct {
	// Rules are policy rules (e.g. require approval for execute_task above cost threshold).
	Rules []PolicyRuleSpec `json:"rules,omitempty"`
	// DefaultApprovalLevel is the default approval level when RequiresApproval is true.
	DefaultApprovalLevel string `json:"defaultApprovalLevel,omitempty"`
	// BudgetLimitUSD is the default per-project budget cap in USD (0 = no cap).
	BudgetLimitUSD float64 `json:"budgetLimitUSD,omitempty"`
}

// PolicyRuleSpec defines a single policy rule.
type PolicyRuleSpec struct {
	// Name is a short name for the rule.
	Name string `json:"name"`
	// Action is the action this rule applies to (e.g. execute_task, call_llm).
	Action string `json:"action"`
	// RequiresApproval when true requires human approval before the action.
	RequiresApproval bool `json:"requiresApproval,omitempty"`
	// MaxCostUSD is the max cost in USD for a single call (0 = no limit).
	MaxCostUSD float64 `json:"maxCostUSD,omitempty"`
	// AllowedModels restricts to these model IDs when set (empty = all).
	AllowedModels []string `json:"allowedModels,omitempty"`
}

// BrainPolicyPhase is the phase of a BrainPolicy.
type BrainPolicyPhase string

const (
	BrainPolicyPhaseActive   BrainPolicyPhase = "Active"
	BrainPolicyPhaseInvalid  BrainPolicyPhase = "Invalid"
)

// BrainPolicyStatus defines the observed state of BrainPolicy.
type BrainPolicyStatus struct {
	Phase     BrainPolicyPhase `json:"phase,omitempty"`
	Message   string           `json:"message,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// BrainPolicyList contains a list of BrainPolicy.
type BrainPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BrainPolicy `json:"items"`
}
