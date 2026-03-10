// Package v1alpha1 defines the v1alpha1 API for zen-brain CRDs (Block 4.1).
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// WorkItemID is the source work item identifier.
	WorkItemID string `json:"workItemID"`
	// SessionID links this task to a work session.
	SessionID string `json:"sessionID"`
	// SourceKey is the external key (e.g. Jira PROJ-123).
	SourceKey string `json:"sourceKey"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	WorkType    string `json:"workType"`
	WorkDomain  string `json:"workDomain"`
	Priority    string `json:"priority,omitempty"`

	Objective          string   `json:"objective"`
	AcceptanceCriteria []string `json:"acceptanceCriteria,omitempty"`
	Constraints        []string `json:"constraints,omitempty"`

	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`
	MaxRetries     int   `json:"maxRetries,omitempty"`
	EstimatedCost  string `json:"estimatedCost,omitempty"`

	DependsOn []string `json:"dependsOn,omitempty"`
	KBScopes  []string `json:"kbScopes,omitempty"`

	// QueueName is the name of the BrainQueue to use for scheduling (optional). Foreman skips scheduling if that queue is Paused.
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
