// Package v1alpha1 defines the v1alpha1 API for zen-brain CRDs (Block 4.1).
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bt
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.sessionID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BrainTask is the Schema for the braintasks API (Block 4.1).
// It represents a single AI task produced by the Office and consumed by the Factory.
type BrainTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BrainTaskSpec   `json:"spec,omitempty"`
	Status BrainTaskStatus `json:"status,omitempty"`
}

// BrainTaskSpec defines the desired state of BrainTask.
// Mirrors pkg/contracts.BrainTaskSpec for Office-to-Factory contract.
type BrainTaskSpec struct {
	// ID is the task identifier (canonical with contracts.BrainTaskSpec).
	// +optional
	ID string `json:"id,omitempty"`
	// WorkItemID is the source work item identifier.
	// +kubebuilder:validation:MinLength=1
	WorkItemID string `json:"workItemID"`
	// SessionID links this task to a work session.
	// +kubebuilder:validation:MinLength=1
	SessionID string `json:"sessionID"`
	// SourceKey is the external key (e.g. Jira PROJ-123).
	// +optional
	SourceKey string `json:"sourceKey"`

	// +kubebuilder:validation:MinLength=1
	Title       string `json:"title"`
	// +optional
	Description string `json:"description,omitempty"`
	// +kubebuilder:validation:Enum=research;design;implementation;debug;refactor;documentation;analysis;operations;security;testing
	WorkType contracts.WorkType `json:"workType"`
	// +kubebuilder:validation:Enum=office;factory;sdk;policy;memory;observability;infrastructure;integration;core
	WorkDomain contracts.WorkDomain `json:"workDomain"`
	// +optional
	// +kubebuilder:validation:Enum=critical;high;medium;low;background
	Priority contracts.Priority `json:"priority,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Objective          string   `json:"objective"`
	AcceptanceCriteria []string `json:"acceptanceCriteria,omitempty"`
	Constraints        []string `json:"constraints,omitempty"`

	// EvidenceRequirement and SR&ED fields (canonical with contracts.BrainTaskSpec).
	// +optional
	// +kubebuilder:validation:Enum=none;summary;logs;diff;test_results;full_artifact
	EvidenceRequirement contracts.EvidenceRequirement `json:"evidenceRequirement,omitempty"`
	// +optional
	// +kubebuilder:validation:UniqueItems=true
	SREDTags []contracts.SREDTag `json:"sredTags,omitempty"`
	// +optional
	Hypothesis string `json:"hypothesis,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=0
	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxRetries int `json:"maxRetries,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	EstimatedCostUSD float64 `json:"estimatedCostUSD,omitempty"`

	// +optional
	DependsOn []string `json:"dependsOn,omitempty"`
	// +optional
	KBScopes []string `json:"kbScopes,omitempty"`

	// QueueName is the name of the BrainQueue to use for scheduling (optional). Foreman skips scheduling if that queue is Paused.
	// +optional
	QueueName string `json:"queueName,omitempty"`
}

// BrainTaskPhase is the phase of a BrainTask.
type BrainTaskPhase string

const (
	BrainTaskPhasePending   BrainTaskPhase = "Pending"
	BrainTaskPhaseScheduled BrainTaskPhase = "Scheduled"
	BrainTaskPhaseRunning   BrainTaskPhase = "Running"
	BrainTaskPhaseCompleted BrainTaskPhase = "Completed"
	BrainTaskPhaseFailed    BrainTaskPhase = "Failed"
	BrainTaskPhaseCanceled  BrainTaskPhase = "Canceled"
)

// BrainTaskStatus defines the observed state of BrainTask.
type BrainTaskStatus struct {
	// Phase is the current phase of the task.
	Phase BrainTaskPhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// AssignedAgent is the worker/agent handling this task (if any).
	AssignedAgent string `json:"assignedAgent,omitempty"`
	// Message is a human-readable message for the current state.
	Message string `json:"message,omitempty"`
	// ObservedGeneration is the .metadata.generation last observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true

// BrainTaskList contains a list of BrainTask.
type BrainTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BrainTask `json:"items"`
}
