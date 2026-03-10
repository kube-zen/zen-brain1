// Package v1alpha1 defines the BrainQueue CRD (Block 4).
// BrainQueue represents a named queue for work; Foreman can route tasks by queue.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bq
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Depth",type=integer,JSONPath=`.status.depth`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BrainQueue is a named queue for BrainTasks (Block 4).
type BrainQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BrainQueueSpec   `json:"spec,omitempty"`
	Status BrainQueueStatus `json:"status,omitempty"`
}

// BrainQueueSpec defines the desired state of a queue.
type BrainQueueSpec struct {
	// Priority is relative priority (higher = more preferred). Default 0.
	// +optional
	Priority int32 `json:"priority,omitempty"`
	// MaxConcurrency is the max tasks that can be in-flight from this queue (0 = unbounded).
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxConcurrency int32 `json:"maxConcurrency,omitempty"`
	// SessionAffinity when true prefers dispatching to agents already serving this session.
	// +optional
	SessionAffinity bool `json:"sessionAffinity,omitempty"`
}

// BrainQueuePhase is the phase of a BrainQueue.
type BrainQueuePhase string

const (
	BrainQueuePhaseReady   BrainQueuePhase = "Ready"
	BrainQueuePhasePaused  BrainQueuePhase = "Paused"
	BrainQueuePhaseDraining BrainQueuePhase = "Draining"
)

// BrainQueueStatus defines the observed state of BrainQueue.
type BrainQueueStatus struct {
	Phase   BrainQueuePhase `json:"phase,omitempty"`
	// Depth is the number of tasks pending in this queue.
	Depth int32 `json:"depth,omitempty"`
	// InFlight is the number of tasks currently running from this queue.
	InFlight int32            `json:"inFlight,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// BrainQueueList contains a list of BrainQueue.
type BrainQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BrainQueue `json:"items"`
}
