package factory

import (
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// FactoryTaskSpec represents a task to be executed by the Factory.
// This is Factory's internal representation of work, mapped from contracts.BrainTaskSpec.
type FactoryTaskSpec struct {
	// Identity
	ID          string    `json:"id"`
	SessionID    string    `json:"session_id"`
	WorkItemID   string    `json:"work_item_id"`

	// Work definition
	Title       string    `json:"title"`
	Objective   string    `json:"objective"`
	Constraints []string  `json:"constraints,omitempty"`

	// Classification
	WorkType    contracts.WorkType    `json:"work_type"`
	WorkDomain  contracts.WorkDomain  `json:"work_domain"`
	Priority    contracts.Priority    `json:"priority"`

	// Execution parameters
	TimeoutSeconds   int64               `json:"timeout_seconds,omitempty"`
	MaxRetries       int                 `json:"max_retries,omitempty"`
	KBScopes        []string            `json:"kb_scopes,omitempty"`
	ExecutionConstraints contracts.ExecutionConstraints `json:"execution_constraints,omitempty"`

	// Workspace
	WorkspacePath  string    `json:"workspace_path,omitempty"`

	// Metadata
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// WorkspaceMetadata represents workspace state and configuration.
type WorkspaceMetadata struct {
	// Identity
	TaskID      string    `json:"task_id"`
	SessionID   string    `json:"session_id"`
	Path        string    `json:"path"`

	// State
	Initialized bool      `json:"initialized"`
	Clean       bool      `json:"clean"`
	Locked      bool      `json:"locked"`

	// Git information
	Branch      string    `json:"branch,omitempty"`
	BaseCommit  string    `json:"base_commit,omitempty"`
	DirtyFiles  []string  `json:"dirty_files,omitempty"`

	// Metrics
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExecutionStep represents a single bounded execution step.
type ExecutionStep struct {
	// Identity
	StepID      string    `json:"step_id"`
	TaskID      string    `json:"task_id"`

	// Step definition
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Command     string    `json:"command,omitempty"`

	// State
	Status      StepStatus `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Results
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`

	// Bounded execution limits
	TimeoutSeconds  int64  `json:"timeout_seconds,omitempty"`
	MaxRetries    int    `json:"max_retries,omitempty"`
	RetryCount    int    `json:"retry_count,omitempty"`
}

// StepStatus represents the state of an execution step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusCanceled  StepStatus = "canceled"
)

// ExecutionResult represents the structured output of task execution.
type ExecutionResult struct {
	// Identity
	TaskID      string    `json:"task_id"`
	SessionID   string    `json:"session_id"`
	WorkItemID  string    `json:"work_item_id"`

	// Outcome
	Status      ExecutionStatus `json:"status"`
	Success     bool          `json:"success"`
	CompletedAt time.Time     `json:"completed_at"`

	// Execution details
	TotalSteps       int                `json:"total_steps"`
	CompletedSteps  int                `json:"completed_steps"`
	FailedSteps     []*ExecutionStep  `json:"failed_steps,omitempty"`
	ExecutionSteps  []*ExecutionStep  `json:"execution_steps,omitempty"`

	// Workspace state
	WorkspacePath   string   `json:"workspace_path"`
	FilesChanged    []string `json:"files_changed,omitempty"`
	TestsRun        []string `json:"tests_run,omitempty"`
	TestsPassed     bool     `json:"tests_passed,omitempty"`

	// Artifacts
	ProofOfWorkPath string   `json:"proof_of_work_path,omitempty"`
	LogPath        string   `json:"log_path,omitempty"`
	DiffPath       string   `json:"diff_path,omitempty"`

	// Error handling
	Error          string   `json:"error,omitempty"`
	ErrorCode      string   `json:"error_code,omitempty"`
	NeedsRetry     bool     `json:"needs_retry,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"` // merge, retry, escalate

	// SR&ED
	SREDEvidence   []contracts.EvidenceItem `json:"sred_evidence,omitempty"`

	// Timing
	Duration       time.Duration `json:"duration,omitempty"`
}

// ExecutionStatus represents the overall execution status.
type ExecutionStatus string

const (
	ExecutionStatusPending    ExecutionStatus = "pending"
	ExecutionStatusRunning    ExecutionStatus = "running"
	ExecutionStatusCompleted  ExecutionStatus = "completed"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusCanceled   ExecutionStatus = "canceled"
	ExecutionStatusBlocked    ExecutionStatus = "blocked"
)

// ProofOfWorkSummary represents the proof-of-work bundle.
// This is generated by Factory and can be attached to Jira.
type ProofOfWorkSummary struct {
	// Identity
	TaskID      string    `json:"task_id"`
	SessionID   string    `json:"session_id"`
	WorkItemID  string    `json:"work_item_id"`
	SourceKey   string    `json:"source_key"` // e.g., "PROJ-123"
	SourceSystem string   `json:"source_system,omitempty"` // e.g., "jira", "github"

	// Work summary
	Title       string    `json:"title"`
	Objective   string    `json:"objective"`
	Result      string    `json:"result"` // "completed", "failed", "needs_review"

	// Workspace information
	WorkspacePath string    `json:"workspace_path,omitempty"` // Absolute path to workspace

	// Execution details
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`

	// Model and agent
	ModelUsed   string    `json:"model_used"`
	AgentRole   string    `json:"agent_role"`

	// Changes
	FilesChanged     []string `json:"files_changed,omitempty"`
	NewFiles        []string `json:"new_files,omitempty"`
	ModifiedFiles   []string `json:"modified_files,omitempty"`
	DeletedFiles    []string `json:"deleted_files,omitempty"`
	LinesAdded      int       `json:"lines_added,omitempty"`
	LinesDeleted    int       `json:"lines_deleted,omitempty"`

	// Tests and validation
	TestsRun        []string `json:"tests_run,omitempty"`
	TestsPassed     bool     `json:"tests_passed"`
	TestsFailed     []string `json:"tests_failed,omitempty"`

	// Evidence
	CommandLog      []string `json:"command_log,omitempty"`
	OutputLog       string   `json:"output_log,omitempty"`
	ErrorLog        string   `json:"error_log,omitempty"`

	// SR&ED
	EvidenceItems   []contracts.EvidenceItem `json:"evidence_items,omitempty"`

	// Risk assessment
	UnresolvedRisks []string `json:"unresolved_risks,omitempty"`
	KnownLimitations []string `json:"known_limitations,omitempty"`

	// Recommendation
	RecommendedAction string   `json:"recommended_action"` // merge, review, retry, escalate
	RequiresApproval bool     `json:"requires_approval,omitempty"`
	ReviewNotes      string   `json:"review_notes,omitempty"`

	// Artifacts
	ArtifactPaths   []string `json:"artifact_paths,omitempty"`
	GitBranch       string   `json:"git_branch,omitempty"`
	GitCommit       string   `json:"git_commit,omitempty"`
	PRURL           string   `json:"pr_url,omitempty"`

	// Timestamps
	GeneratedAt time.Time `json:"generated_at"`
}

// ProofOfWorkArtifact represents a complete proof-of-work artifact bundle.
type ProofOfWorkArtifact struct {
	Directory    string              `json:"directory"`
	JSONPath     string              `json:"json_path"`
	MarkdownPath string              `json:"markdown_path"`
	LogPath      string              `json:"log_path"`
	Summary      *ProofOfWorkSummary `json:"summary"`
	CreatedAt    time.Time           `json:"created_at"`
}
