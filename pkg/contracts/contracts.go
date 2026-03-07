// Package contracts defines the canonical data types used across zen-brain.
// These are the shared language that all components must agree on.
// No component-specific types should live here.
package contracts

import (
	"time"
)

// WorkType represents the kind of work.
type WorkType string

const (
	WorkTypeResearch       WorkType = "research"
	WorkTypeDesign         WorkType = "design"
	WorkTypeImplementation WorkType = "implementation"
	WorkTypeDebug          WorkType = "debug"
	WorkTypeRefactor       WorkType = "refactor"
	WorkTypeDocumentation  WorkType = "documentation"
	WorkTypeAnalysis       WorkType = "analysis"
	WorkTypeOperations     WorkType = "operations"
	WorkTypeSecurity       WorkType = "security"
	WorkTypeTesting        WorkType = "testing"
)

// WorkDomain represents which part of the system this affects.
type WorkDomain string

const (
	DomainOffice       WorkDomain = "office"
	DomainFactory      WorkDomain = "factory"
	DomainSDK          WorkDomain = "sdk"
	DomainPolicy       WorkDomain = "policy"
	DomainMemory       WorkDomain = "memory"
	DomainObservability WorkDomain = "observability"
	DomainInfrastructure WorkDomain = "infrastructure"
	DomainIntegration  WorkDomain = "integration"
	DomainCore         WorkDomain = "core"
)

// Priority represents normalized priority (not Jira-native).
type Priority string

const (
	PriorityCritical  Priority = "critical"
	PriorityHigh      Priority = "high"
	PriorityMedium    Priority = "medium"
	PriorityLow       Priority = "low"
	PriorityBackground Priority = "background"
)

// ExecutionMode represents the level of human oversight.
type ExecutionMode string

const (
	ModeAutonomous     ExecutionMode = "autonomous"
	ModeApprovalRequired ExecutionMode = "approval_required"
	ModeReadOnly       ExecutionMode = "read_only"
	ModeSimulationOnly ExecutionMode = "simulation_only"
	ModeSupervised     ExecutionMode = "supervised"
)

// WorkStatus represents the canonical lifecycle state.
type WorkStatus string

const (
	StatusRequested      WorkStatus = "requested"
	StatusAnalyzing      WorkStatus = "analyzing"
	StatusAnalyzed       WorkStatus = "analyzed"
	StatusPlanning       WorkStatus = "planning"
	StatusPlanned        WorkStatus = "planned"
	StatusPendingApproval WorkStatus = "pending_approval"
	StatusApproved       WorkStatus = "approved"
	StatusQueued         WorkStatus = "queued"
	StatusRunning        WorkStatus = "running"
	StatusBlocked        WorkStatus = "blocked"
	StatusCompleted      WorkStatus = "completed"
	StatusFailed         WorkStatus = "failed"
	StatusCanceled       WorkStatus = "canceled"
)

// EvidenceRequirement represents what proof of work is needed.
type EvidenceRequirement string

const (
	EvidenceNone         EvidenceRequirement = "none"
	EvidenceSummary      EvidenceRequirement = "summary"
	EvidenceLogs         EvidenceRequirement = "logs"
	EvidenceDiff         EvidenceRequirement = "diff"
	EvidenceTestResults  EvidenceRequirement = "test_results"
	EvidenceFullArtifact EvidenceRequirement = "full_artifact"
)

// SREDTag represents SR&ED uncertainty categories.
type SREDTag string

const (
	SREDU1DynamicProvisioning  SREDTag = "u1_dynamic_provisioning"
	SREDU2SecurityGates        SREDTag = "u2_security_gates"
	SREDU3DeterministicDelivery SREDTag = "u3_deterministic_delivery"
	SREDU4Backpressure         SREDTag = "u4_backpressure"
	SREDExperimentalGeneral    SREDTag = "experimental_general"
)

// ApprovalState represents the approval state of a work item.
type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalRejected ApprovalState = "rejected"
	ApprovalNotRequired ApprovalState = "not_required"
)

// AIAttribution represents structured AI attribution for Jira content.
type AIAttribution struct {
	AgentRole  string    `json:"agent_role"`  // "planner-v1", "worker-debug", etc.
	ModelUsed  string    `json:"model_used"`  // "glm-4.7", "claude-sonnet-4-6", etc.
	SessionID  string    `json:"session_id"`  // Session UUID
	TaskID     string    `json:"task_id"`     // Task UUID
	Timestamp  time.Time `json:"timestamp"`   // When content was generated
}

// SourceMetadata preserves origin information (not execution-critical).
type SourceMetadata struct {
	System     string    `json:"system"`      // "jira", "linear", "github", "slack"
	IssueKey   string    `json:"issue_key"`   // "PROJ-123"
	Project    string    `json:"project"`     // "PROJECT"
	IssueType  string    `json:"issue_type"`  // "Task", "Bug", "Story", "Epic"
	ParentKey  string    `json:"parent_key"`  // For subtasks
	EpicKey    string    `json:"epic_key"`
	Reporter   string    `json:"reporter"`
	Assignee   string    `json:"assignee"`
	Sprint     string    `json:"sprint"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ExecutionConstraints represents runtime constraints for work execution.
type ExecutionConstraints struct {
	MaxCostUSD       float64 `json:"max_cost_usd,omitempty"`
	TimeoutSeconds   int64   `json:"timeout_seconds,omitempty"`
	AllowedClusters  []string `json:"allowed_clusters,omitempty"`
	RequiredApproval bool    `json:"required_approval,omitempty"`
}

// WorkTags is a structured tag model that replaces generic Labels.
// Categories are defined in taxonomy package.
type WorkTags struct {
	HumanOrg  []string   `json:"human_org,omitempty"`
	Routing   []string   `json:"routing,omitempty"`
	Policy    []string   `json:"policy,omitempty"`
	Analytics []string   `json:"analytics,omitempty"`
	SRED      []SREDTag  `json:"sred,omitempty"`
}

// WorkItem is the canonical work representation that all Office connectors map to,
// and the Factory operates on exclusively.
type WorkItem struct {
	// Identity
	ID         string `json:"id"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Body       string `json:"body"`       // Full description

	// Classification
	WorkType    WorkType    `json:"work_type"`
	WorkDomain  WorkDomain  `json:"work_domain"`
	Priority    Priority    `json:"priority"`
	ExecutionMode ExecutionMode `json:"execution_mode"`

	// Lifecycle
	Status      WorkStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Context
	ClusterID   string   `json:"cluster_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	WorkingDir  string   `json:"working_dir,omitempty"`

	// Structured tags (replaces Labels)
	Tags        WorkTags `json:"tags,omitempty"`

	// Requirements
	EvidenceRequirement EvidenceRequirement `json:"evidence_requirement"`

	// SR&ED
	SREDDisabled bool     `json:"sred_disabled,omitempty"`

	// Source (preserved but not execution-critical)
	Source      SourceMetadata `json:"source"`

	// Attribution (injected by ZenOffice adapter)
	Attribution *AIAttribution `json:"attribution,omitempty"`

	// Relationships
	ParentID   string   `json:"parent_id,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`

	// Request and Approval
	RequestedBy   string        `json:"requested_by,omitempty"`
	ApprovalState ApprovalState `json:"approval_state,omitempty"`
	PolicyClass   string        `json:"policy_class,omitempty"`

	// Evidence and References
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	SourceRefs   []string `json:"source_refs,omitempty"`

	// Execution Constraints
	ExecutionConstraints ExecutionConstraints `json:"execution_constraints,omitempty"`

	// Knowledge Base Scopes
	KBScopes []string `json:"kb_scopes,omitempty"`
}

// Comment represents a comment on a work item.
type Comment struct {
	ID          string         `json:"id"`
	WorkItemID  string         `json:"work_item_id"`
	Body        string         `json:"body"`
	Author      string         `json:"author"`
	CreatedAt   time.Time      `json:"created_at"`
	Attribution *AIAttribution `json:"attribution,omitempty"`
}

// Attachment represents an attachment on a work item.
type Attachment struct {
	ID          string    `json:"id"`
	WorkItemID  string    `json:"work_item_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}