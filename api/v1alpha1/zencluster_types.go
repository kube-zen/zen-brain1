// Package v1alpha1 contains API Schema definitions for the zen.kube-zen.com API group
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZenClusterSpec defines the desired state of ZenCluster
type ZenClusterSpec struct {
	// Endpoint is the Kubernetes API server endpoint
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`

	// AuthRef is a reference to a Kubernetes Secret containing kubeconfig
	// +kubebuilder:validation:MinLength=1
	AuthRef string `json:"auth_ref"`

	// Capacity defines the cluster's resource capacity
	// +optional
	Capacity ClusterCapacity `json:"capacity,omitempty"`

	// Status indicates whether the cluster is active, inactive, or draining
	// +optional
	Status string `json:"status,omitempty"`

	// Location indicates where the cluster is located (local, cloud, edge)
	// +optional
	Location string `json:"location,omitempty"`

	// Labels are key-value pairs for cluster selection
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Metadata contains additional cluster metadata
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ClusterCapacity defines resource capacity
type ClusterCapacity struct {
	// CPUCores is the total CPU cores available
	// +optional
	// +kubebuilder:validation:Minimum=0
	CPUCores int64 `json:"cpu_cores,omitempty"`

	// MemoryGB is the total memory in GB available
	// +optional
	// +kubebuilder:validation:Minimum=0
	MemoryGB int64 `json:"memory_gb,omitempty"`

	// GPUs is the number of GPUs available
	// +optional
	// +kubebuilder:validation:Minimum=0
	GPUs int64 `json:"gpus,omitempty"`

	// StorageGB is the total storage in GB available
	// +optional
	// +kubebuilder:validation:Minimum=0
	StorageGB int64 `json:"storage_gb,omitempty"`
}

// ZenClusterStatus defines the observed state of ZenCluster
type ZenClusterStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Phase is the current phase of the cluster
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the cluster's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastHeartbeatTime is the last time a heartbeat was received
	LastHeartbeatTime *metav1.Time `json:"last_heartbeat_time,omitempty"`

	// AvailableCapacity is the currently available capacity
	AvailableCapacity ClusterCapacity `json:"available_capacity,omitempty"`

	// NodeCount is the number of nodes in the cluster
	NodeCount int32 `json:"node_count,omitempty"`

	// Version is the Kubernetes version
	Version string `json:"version,omitempty"`

	// ZenBrainAgentVersion is the version of the zen-brain-agent running on this cluster
	ZenBrainAgentVersion string `json:"zen_brain_agent_version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.endpoint"
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".spec.status"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ZenCluster is the Schema for the zenclusters API
type ZenCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZenClusterSpec   `json:"spec,omitempty"`
	Status ZenClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZenClusterList contains a list of ZenCluster
type ZenClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZenCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZenCluster{}, &ZenClusterList{})
}