// Package v1alpha1 contains API Schema definitions for the zen.kube-zen.com API group
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ZenProjectSpec defines the desired state of ZenProject
type ZenProjectSpec struct {
	// DisplayName is the human-readable name of the project
	DisplayName string `json:"display_name"`

	// ClusterRef is the name of the ZenCluster where this project runs
	ClusterRef string `json:"cluster_ref"`

	// RepoURLs are the Git repository URLs for this project
	RepoURLs []string `json:"repo_urls,omitempty"`

	// KBScopes are the knowledge base scopes for this project
	// Example: ["zen-brain", "general", "company"]
	KBScopes []string `json:"kb_scopes,omitempty"`

	// SREDTags are the SR&ED uncertainty categories for this project
	SREDTags []contracts.SREDTag `json:"sred_tags,omitempty"`

	// FundingPrograms are the funding programs this project participates in
	FundingPrograms []string `json:"funding_programs,omitempty"`

	// SREDDisabled indicates whether SR&ED evidence collection is disabled for this project
	SREDDisabled bool `json:"sred_disabled,omitempty"`

	// AutoGenerateFundingReports indicates whether funding reports should be auto-generated
	AutoGenerateFundingReports bool `json:"auto_generate_funding_reports,omitempty"`

	// CostBudgetUSD is the monthly budget limit in USD
	CostBudgetUSD float64 `json:"cost_budget_usd,omitempty"`

	// Metadata contains additional project metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ZenProjectStatus defines the observed state of ZenProject
type ZenProjectStatus struct {
	// Phase is the current phase of the project
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the project's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime is the last time the project was synced
	LastSyncTime *metav1.Time `json:"last_sync_time,omitempty"`

	// CostSpentUSD is the total cost spent this month
	CostSpentUSD float64 `json:"cost_spent_usd,omitempty"`

	// TaskCount is the total number of tasks executed
	TaskCount int64 `json:"task_count,omitempty"`

	// SessionCount is the total number of active sessions
	SessionCount int64 `json:"session_count,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.display_name"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.cluster_ref"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Cost",type="number",JSONPath=".status.cost_spent_usd"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ZenProject is the Schema for the zenprojects API
type ZenProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZenProjectSpec   `json:"spec,omitempty"`
	Status ZenProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZenProjectList contains a list of ZenProject
type ZenProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZenProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZenProject{}, &ZenProjectList{})
}