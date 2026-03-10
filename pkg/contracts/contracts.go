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
	DomainOffice         WorkDomain = "office"
	DomainFactory        WorkDomain = "factory"
	DomainSDK            WorkDomain = "sdk"
	DomainPolicy         WorkDomain = "policy"
	DomainMemory         WorkDomain = "memory"
	DomainObservability  WorkDomain = "observability"
	DomainInfrastructure WorkDomain = "infrastructure"
	DomainIntegration    WorkDomain = "integration"
	DomainCore           WorkDomain = "core"
)

// Priority represents normalized priority (not Jira-native).
type Priority string

const (
	PriorityCritical   Priority = "critical"
	PriorityHigh       Priority = "high"
	PriorityMedium     Priority = "medium"
	PriorityLow        Priority = "low"
	PriorityBackground Priority = "background"
)

// ExecutionMode represents the level of human oversight.
type ExecutionMode string

const (
	ModeAutonomous       ExecutionMode = "autonomous"
	ModeApprovalRequired ExecutionMode = "approval_required"
	ModeReadOnly         ExecutionMode = "read_only"
	ModeSimulationOnly   ExecutionMode = "simulation_only"
	ModeSupervised       ExecutionMode = "supervised"
)

// WorkStatus represents the canonical lifecycle state.
type WorkStatus string

const (
	StatusRequested       WorkStatus = "requested"
	StatusAnalyzing       WorkStatus = "analyzing"
	StatusAnalyzed        WorkStatus = "analyzed"
	StatusPlanning        WorkStatus = "planning"
	StatusPlanned         WorkStatus = "planned"
	StatusPendingApproval WorkStatus = "pending_approval"
	StatusApproved        WorkStatus = "approved"
	StatusQueued          WorkStatus = "queued"
	StatusRunning         WorkStatus = "running"
	StatusBlocked         WorkStatus = "blocked"
	StatusCompleted       WorkStatus = "completed"
	StatusFailed          WorkStatus = "failed"
	StatusCanceled        WorkStatus = "canceled"
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
	SREDU1DynamicProvisioning   SREDTag = "u1_dynamic_provisioning"
	SREDU2SecurityGates         SREDTag = "u2_security_gates"
	SREDU3DeterministicDelivery SREDTag = "u3_deterministic_delivery"
	SREDU4Backpressure          SREDTag = "u4_backpressure"
	SREDExperimentalGeneral     SREDTag = "experimental_general"
)

// ApprovalState represents the approval state of a work item.
type ApprovalState string

const (
	ApprovalPending     ApprovalState = "pending"
	ApprovalApproved    ApprovalState = "approved"
	ApprovalRejected    ApprovalState = "rejected"
	ApprovalNotRequired ApprovalState = "not_required"
)

// AIAttribution represents structured AI attribution for Jira content.
type AIAttribution struct {
	AgentRole string    `json:"agent_role"` // "planner-v1", "worker-debug", etc.
	ModelUsed string    `json:"model_used"` // "glm-4.7", "claude-sonnet-4-6", etc.
	SessionID string    `json:"session_id"` // Session UUID
	TaskID    string    `json:"task_id"`    // Task UUID
	Timestamp time.Time `json:"timestamp"`  // When content was generated
}

// SourceMetadata preserves origin information (not execution-critical).
type SourceMetadata struct {
	System    string    `json:"system"`     // "jira", "linear", "github", "slack"
	IssueKey  string    `json:"issue_key"`  // "PROJ-123"
	Project   string    `json:"project"`    // "PROJECT"
	IssueType string    `json:"issue_type"` // "Task", "Bug", "Story", "Epic"
	ParentKey string    `json:"parent_key"` // For subtasks
	EpicKey   string    `json:"epic_key"`
	Reporter  string    `json:"reporter"`
	Assignee  string    `json:"assignee"`
	Sprint    string    `json:"sprint"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExecutionConstraints represents runtime constraints for work execution.
type ExecutionConstraints struct {
	MaxCostUSD       float64  `json:"max_cost_usd,omitempty"`
	TimeoutSeconds   int64    `json:"timeout_seconds,omitempty"`
	AllowedClusters  []string `json:"allowed_clusters,omitempty"`
	RequiredApproval bool     `json:"required_approval,omitempty"`
}

// WorkTags is a structured tag model that replaces generic Labels.
// Categories are defined in taxonomy package.
type WorkTags struct {
	HumanOrg  []string  `json:"human_org,omitempty"`
	Routing   []string  `json:"routing,omitempty"`
	Policy    []string  `json:"policy,omitempty"`
	Analytics []string  `json:"analytics,omitempty"`
	SRED      []SREDTag `json:"sred,omitempty"`
}

// WorkItem is the canonical work representation that all Office connectors map to,
// and the Factory operates on exclusively.
type WorkItem struct {
	// Identity
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"` // Full description

	// Classification
	WorkType      WorkType      `json:"work_type"`
	WorkDomain    WorkDomain    `json:"work_domain"`
	Priority      Priority      `json:"priority"`
	ExecutionMode ExecutionMode `json:"execution_mode"`

	// Lifecycle
	Status    WorkStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Context
	ClusterID  string `json:"cluster_id,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`

	// Structured tags (replaces Labels)
	Tags WorkTags `json:"tags,omitempty"`

	// Requirements
	EvidenceRequirement EvidenceRequirement `json:"evidence_requirement"`

	// SR&ED
	SREDDisabled bool `json:"sred_disabled,omitempty"`

	// Source (preserved but not execution-critical)
	Source SourceMetadata `json:"source"`

	// Attribution (injected by ZenOffice adapter)
	Attribution *AIAttribution `json:"attribution,omitempty"`

	// Relationships
	ParentID  string   `json:"parent_id,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`

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

// BrainTaskSpec represents a specification for an AI task.
// This is produced by the Intent Analyzer and consumed by the Factory
// to create BrainTask CRDs.
type BrainTaskSpec struct {
	// Identity
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`

	// Source reference
	WorkItemID string `json:"work_item_id"`
	SourceKey  string `json:"source_key"` // e.g., "PROJ-123"

	// Classification
	WorkType   WorkType   `json:"work_type"`
	WorkDomain WorkDomain `json:"work_domain"`
	Priority   Priority   `json:"priority"`

	// Requirements
	Objective          string   `json:"objective"` // What needs to be done
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Constraints        []string `json:"constraints,omitempty"`

	// Evidence requirements
	EvidenceRequirement EvidenceRequirement `json:"evidence_requirement"`

	// SR&ED
	SREDTags   []SREDTag `json:"sred_tags,omitempty"`
	Hypothesis string    `json:"hypothesis,omitempty"` // SR&ED hypothesis framing

	// Execution
	EstimatedCostUSD float64 `json:"estimated_cost_usd,omitempty"`
	TimeoutSeconds   int64   `json:"timeout_seconds,omitempty"`
	MaxRetries       int     `json:"max_retries,omitempty"`

	// Dependencies
	DependsOn []string `json:"depends_on,omitempty"`

	// Knowledge
	KBScopes []string `json:"kb_scopes,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AnalysisResult represents the output of the Intent Analyzer.
type AnalysisResult struct {
	WorkItem              *WorkItem       `json:"work_item"`
	BrainTaskSpecs        []BrainTaskSpec `json:"brain_task_specs"`
	Confidence            float64         `json:"confidence"` // 0.0-1.0
	AnalysisNotes         string          `json:"analysis_notes"`
	RequiresApproval      bool            `json:"requires_approval"`
	RecommendedModel      string          `json:"recommended_model,omitempty"`
	EstimatedTotalCostUSD float64         `json:"estimated_total_cost_usd,omitempty"`
}

// SessionState represents the lifecycle state of a work session.
type SessionState string

const (
	SessionStateCreated    SessionState = "created"
	SessionStateAnalyzed   SessionState = "analyzed"
	SessionStateScheduled  SessionState = "scheduled"
	SessionStateInProgress SessionState = "in_progress"
	SessionStateCompleted  SessionState = "completed"
	SessionStateFailed     SessionState = "failed"
	SessionStateBlocked    SessionState = "blocked"
	SessionStateCanceled   SessionState = "canceled"
)

// Session represents a work execution session.
// A session tracks the progress of a WorkItem through the Zen‑Brain pipeline.
type Session struct {
	// Identity
	ID         string `json:"id"`
	WorkItemID string `json:"work_item_id"`
	SourceKey  string `json:"source_key"` // e.g., "PROJ-123"

	// State
	State        SessionState      `json:"state"`
	StateHistory []StateTransition `json:"state_history,omitempty"`

	// Content
	WorkItem       *WorkItem       `json:"work_item,omitempty"`
	AnalysisResult *AnalysisResult `json:"analysis_result,omitempty"`
	BrainTaskSpecs []BrainTaskSpec `json:"brain_task_specs,omitempty"`

	// Evidence (SR&ED)
	EvidenceItems []EvidenceItem `json:"evidence_items,omitempty"`

	// Execution
	AssignedAgent string     `json:"assigned_agent,omitempty"`
	AssignedModel string     `json:"assigned_model,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Error         string     `json:"error,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StateTransition records a state change in a session.
type StateTransition struct {
	FromState SessionState `json:"from_state"`
	ToState   SessionState `json:"to_state"`
	Timestamp time.Time    `json:"timestamp"`
	Reason    string       `json:"reason,omitempty"`
	Agent     string       `json:"agent,omitempty"` // Who triggered the transition
}

// EvidenceItem represents a piece of SR&ED evidence collected during a session.
type EvidenceItem struct {
	ID          string            `json:"id"`
	SessionID   string            `json:"session_id"`
	Type        EvidenceType      `json:"type"`
	Content     string            `json:"content"` // Text, JSON, or reference
	Metadata    map[string]string `json:"metadata,omitempty"`
	CollectedAt time.Time         `json:"collected_at"`
	CollectedBy string            `json:"collected_by"` // Agent/model
}

// EvidenceType categorizes SR&ED evidence.
type EvidenceType string

const (
	EvidenceTypeHypothesis    EvidenceType = "hypothesis"
	EvidenceTypeExperiment   EvidenceType = "experiment"
	EvidenceTypeObservation  EvidenceType = "observation"
	EvidenceTypeMeasurement  EvidenceType = "measurement"
	EvidenceTypeAnalysis     EvidenceType = "analysis"
	EvidenceTypeConclusion   EvidenceType = "conclusion"
	EvidenceTypeProofOfWork  EvidenceType = "proof_of_work"  // Factory proof-of-work artifact
	EvidenceTypeExecutionLog EvidenceType = "execution_log"  // Command/execution audit trail
)
