// Package office provides the ZenOffice interface for work ingress.
// ZenOffice is the abstract interface that defines how zen-brain
// interacts with external planning systems (Jira, Linear, Slack, etc.)
//
// The Factory operates on canonical WorkItem types only.
// All Office-specific concepts are translated at the ZenOffice boundary.
// No Factory type, API, CRD, or event schema may import Jira-specific models.
package office

import (
	"context"
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

// WorkItem is the canonical work representation that all Office connectors map to.
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
	Labels      []string `json:"labels,omitempty"`

	// Requirements
	EvidenceRequirement EvidenceRequirement `json:"evidence_requirement"`

	// SR&ED
	SREDTags    []SREDTag `json:"sred_tags,omitempty"`
	SREDDisabled bool     `json:"sred_disabled,omitempty"`

	// Source (preserved but not execution-critical)
	Source      SourceMetadata `json:"source"`

	// Attribution (injected by ZenOffice adapter)
	Attribution *AIAttribution `json:"attribution,omitempty"`
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

// ZenOffice is the interface for work ingress from external systems.
// Implementations: Jira connector, Linear connector, Slack connector, etc.
type ZenOffice interface {
	// Fetch retrieves a work item by ID.
	Fetch(ctx context.Context, clusterID, workItemID string) (*WorkItem, error)

	// FetchBySourceKey retrieves a work item by its source system key (e.g., "PROJ-123").
	FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*WorkItem, error)

	// UpdateStatus updates the status of a work item.
	UpdateStatus(ctx context.Context, clusterID, workItemID string, status WorkStatus) error

	// AddComment adds a comment to a work item.
	// AI-generated comments must include attribution.
	AddComment(ctx context.Context, clusterID, workItemID string, comment *Comment) error

	// AddAttachment attaches evidence to a work item.
	AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *Attachment, content []byte) error

	// Search searches for work items matching criteria.
	Search(ctx context.Context, clusterID string, query string) ([]WorkItem, error)

	// Watch returns a channel for receiving work item events.
	Watch(ctx context.Context, clusterID string) (<-chan WorkItemEvent, error)
}

// WorkItemEvent represents an event from the Office system.
type WorkItemEvent struct {
	Type      WorkEventType `json:"type"`
	WorkItem  *WorkItem     `json:"work_item"`
	Timestamp time.Time     `json:"timestamp"`
}

// WorkEventType represents the type of work item event.
type WorkEventType string

const (
	WorkItemCreated   WorkEventType = "created"
	WorkItemUpdated   WorkEventType = "updated"
	WorkItemCommented WorkEventType = "commented"
	WorkItemDeleted   WorkEventType = "deleted"
)
