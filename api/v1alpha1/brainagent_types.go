// Package v1alpha1 defines the BrainAgent CRD (Block 4.1).
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ba
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BrainAgent represents a worker agent that executes BrainTasks (Block 4.3).
type BrainAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BrainAgentSpec   `json:"spec,omitempty"`
	Status BrainAgentStatus `json:"status,omitempty"`
}

// BrainAgentSpec defines the desired state of a worker agent.
type BrainAgentSpec struct {
	// ModelID is the LLM model this agent uses (e.g. qwen3.5:0.8b, glm-4.7).
	// +kubebuilder:validation:MinLength=1
	ModelID string `json:"modelID"`
	// Role is the agent role (e.g. implementer, planner).
	// +optional
	Role string `json:"role,omitempty"`
	// MaxConcurrentTasks is the maximum number of tasks this agent can run at once.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxConcurrentTasks int `json:"maxConcurrentTasks,omitempty"`
}

// BrainAgentPhase is the phase of a BrainAgent.
type BrainAgentPhase string

const (
	BrainAgentPhaseIdle      BrainAgentPhase = "Idle"
	BrainAgentPhaseRunning   BrainAgentPhase = "Running"
	BrainAgentPhaseDraining  BrainAgentPhase = "Draining"
	BrainAgentPhaseUnhealthy BrainAgentPhase = "Unhealthy"
)

// BrainAgentStatus defines the observed state of BrainAgent.
type BrainAgentStatus struct {
	Phase   BrainAgentPhase `json:"phase,omitempty"`
	Message string          `json:"message,omitempty"`
	// CurrentTaskCount is the number of tasks currently being executed.
	CurrentTaskCount int   `json:"currentTaskCount,omitempty"`
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// BrainAgentList contains a list of BrainAgent.
type BrainAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BrainAgent `json:"items"`
}
