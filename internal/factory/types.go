package factory

import (
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ProofSchemaVersion defines the current schema version for proof-of-work artifacts.
const ProofSchemaVersion = "2.0.0"

// ProofSchemaID defines the unique schema identifier.
const ProofSchemaID = "zen-brain-proof-of-work-v2"

// FactoryTaskSpec represents a task to be executed by the Factory.
// This is Factory's internal representation of work, mapped from contracts.BrainTaskSpec.
type FactoryTaskSpec struct {
	// Identity
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	WorkItemID string `json:"work_item_id"`

	// Work definition
	Title             string   `json:"title"`
	Objective         string   `json:"objective"`
	Description       string   `json:"description,omitempty"`
	Constraints       []string `json:"constraints,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

	// Classification
	WorkType   contracts.WorkType   `json:"work_type"`
	WorkDomain contracts.WorkDomain `json:"work_domain"`
	Priority   contracts.Priority   `json:"priority"`

	// Execution parameters
	TimeoutSeconds       int64                          `json:"timeout_seconds,omitempty"`
	MaxRetries           int                            `json:"max_retries,omitempty"`
	KBScopes             []string                       `json:"kb_scopes,omitempty"`
	ExecutionConstraints contracts.ExecutionConstraints `json:"execution_constraints,omitempty"`

	// Workspace
	WorkspacePath string `json:"workspace_path,omitempty"`

	// Template key selected for execution (e.g. "implementation:real" or "default")
	TemplateKey string `json:"template_key,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Intelligence selection (template/configuration from recommender)
	SelectedTemplate   string  `json:"selected_template,omitempty"`
	SelectionSource     string  `json:"selection_source,omitempty"`     // "static" | "recommended"
	SelectionConfidence float64 `json:"selection_confidence,omitempty"`
	SelectionReasoning  string  `json:"selection_reasoning,omitempty"`
}

// WorkspaceMetadata represents workspace state and configuration.
type WorkspaceMetadata struct {
	// Identity
	TaskID    string `json:"task_id"`
	SessionID string `json:"session_id"`
	Path      string `json:"path"`

	// State
	Initialized bool `json:"initialized"`
	Clean       bool `json:"clean"`
	Locked      bool `json:"locked"`

	// Workspace Class and Trust Level
	Class      WorkspaceClass `json:"class,omitempty"`
	TrustLevel TrustLevel     `json:"trust_level,omitempty"`

	// Git information
	Branch     string   `json:"branch,omitempty"`
	BaseCommit string   `json:"base_commit,omitempty"`
	DirtyFiles []string `json:"dirty_files,omitempty"`

	// Tmpfs acceleration
	TmpfsMounted bool `json:"tmpfs_mounted,omitempty"`
	TmpfsSizeMB int  `json:"tmpfs_size_mb,omitempty"`

	// Metrics
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExecutionStep represents a single bounded execution step.
type ExecutionStep struct {
	// Identity
	StepID string `json:"step_id"`
	TaskID string `json:"task_id"`

	// Step definition
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`

	// State
	Status      StepStatus `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Results
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`

	// Bounded execution limits
	TimeoutSeconds int64 `json:"timeout_seconds,omitempty"`
	MaxRetries     int   `json:"max_retries,omitempty"`
	RetryCount     int   `json:"retry_count,omitempty"`
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
	TaskID     string `json:"task_id"`
	SessionID  string `json:"session_id"`
	WorkItemID string `json:"work_item_id"`

	// Outcome
	Status      ExecutionStatus `json:"status"`
	Success     bool            `json:"success"`
	CompletedAt time.Time       `json:"completed_at"`

	// Execution details
	TotalSteps     int              `json:"total_steps"`
	CompletedSteps int              `json:"completed_steps"`
	FailedSteps    []*ExecutionStep `json:"failed_steps,omitempty"`
	ExecutionSteps []*ExecutionStep `json:"execution_steps,omitempty"`

	// Workspace state
	WorkspacePath string   `json:"workspace_path"`
	FilesChanged  []string `json:"files_changed,omitempty"`
	TemplateKey   string   `json:"template_key,omitempty"`
	TestsRun      []string `json:"tests_run,omitempty"`
	TestsPassed   bool     `json:"tests_passed,omitempty"`

	// Artifacts
	ProofOfWorkPath string   `json:"proof_of_work_path,omitempty"`
	LogPath         string   `json:"log_path,omitempty"`
	DiffPath        string   `json:"diff_path,omitempty"`
	ArtifactPaths   []string `json:"artifact_paths,omitempty"` // actual proof bundle paths (JSON, MD, log)
	GitStatusPath   string   `json:"git_status_path,omitempty"` // e.g. workspace/review/git-status.txt
	GitDiffStatPath string   `json:"git_diff_stat_path,omitempty"` // e.g. workspace/review/git-diff-stat.txt

	// Git metadata from workspace (when available)
	GitBranch string `json:"git_branch,omitempty"`
	GitCommit string `json:"git_commit,omitempty"`

	// ZB-022D: Execution mode metadata (observability)
	Metadata map[string]string `json:"metadata,omitempty"`

	// Error handling
	Error              string `json:"error,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
	NeedsRetry         bool   `json:"needs_retry,omitempty"`
	Recommendation     string `json:"recommendation,omitempty"` // merge, retry, escalate
	VerificationFailed bool   `json:"verification_failed,omitempty"` // postflight checks failed

	// SR&ED
	SREDEvidence []contracts.EvidenceItem `json:"sred_evidence,omitempty"`

	// Timing
	Duration time.Duration `json:"duration,omitempty"`
}

// ExecutionStatus represents the overall execution status.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCanceled  ExecutionStatus = "canceled"
	ExecutionStatusBlocked   ExecutionStatus = "blocked"
)

// GenerationRequest contains all context needed for code generation.
type GenerationRequest struct {
	// Work item details
	WorkItemID        string
	Title             string
	Objective         string
	Description       string   // Task description for context
	AcceptanceCriteria []string // Acceptance criteria from task spec
	WorkType          string
	WorkDomain        string
	JiraKey           string // For structured rescue prompts

	// Project context
	ProjectType   string // "go", "python", "node", etc.
	ModuleName    string // Go module name (if Go project)
	PackageName   string // Target package name
	TargetPath    string // Target file path

	// Code context
	ExistingCode  string            // Existing code in target file (if any)
	RelatedFiles  map[string]string // Related files for context
	Imports       []string          // Required imports

	// Generation constraints
	Constraints   []string // Additional constraints
	Style         string   // Code style preferences

	// Structured prompt builder (for rescue tasks)
	StructuredPrompt  bool     // Use structured prompt instead of generic
	AllowedPaths     []string // Allowed file paths
	ForbiddenPaths   []string // Forbidden file paths
	ContextFiles     []string // Files to read first
	TargetFiles      []string // Files to modify
	ExistingTypes    []string // Existing types/interfaces to use
	ExistingPackages []string // Existing packages to import
	TimeoutSec       int      // Timeout in seconds
	WorkTypeLabel    string   // Label for logging
}

// GenerationResult contains generated code and metadata.
type GenerationResult struct {
	Code         string            // Extracted code (without markdown)
	Language     string            // Detected language (go, python, etc.)
	FullResponse string            // Full LLM response (including reasoning)
	Model        string            // Model used
	TokensUsed   int               // Token count
	Metadata     map[string]string // Additional metadata
}

// ProofOfWorkSummary represents the proof-of-work bundle.
// This is generated by Factory and can be attached to any office system.
type ProofOfWorkSummary struct {
	// Schema version
	Version string `json:"version"`
	// Identity
	TaskID       string `json:"task_id"`
	SessionID    string `json:"session_id"`
	WorkItemID   string `json:"work_item_id"`
	SourceKey    string `json:"source_key"`              // e.g., "PROJ-123"
	SourceSystem string `json:"source_system,omitempty"` // e.g., "jira", "github"

	// Work classification
	WorkType   string `json:"work_type"`
	WorkDomain string `json:"work_domain"`

	// Work summary
	Title     string `json:"title"`
	Objective string `json:"objective"`
	Result    string `json:"result"` // "completed", "failed", "needs_review"

	// Workspace information
	WorkspacePath string `json:"workspace_path,omitempty"` // Absolute path to workspace

	// Execution details
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`

	// Model and agent
	ModelUsed string `json:"model_used"`
	AgentRole string `json:"agent_role"`

	// Template and selection metadata (actual template used; ModelUsed kept for backward compatibility)
	TemplateUsed        string  `json:"template_used,omitempty"`
	SelectionSource     string  `json:"selection_source,omitempty"`
	SelectionConfidence float64 `json:"selection_confidence,omitempty"`
	SelectionReasoning  string  `json:"selection_reasoning,omitempty"`

	// Changes
	FilesChanged  []string `json:"files_changed,omitempty"`
	NewFiles      []string `json:"new_files,omitempty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	DeletedFiles  []string `json:"deleted_files,omitempty"`
	LinesAdded    int      `json:"lines_added,omitempty"`
	LinesDeleted  int      `json:"lines_deleted,omitempty"`

	// Tests and validation
	TestsRun    []string `json:"tests_run,omitempty"`
	TestsPassed bool     `json:"tests_passed"`
	TestsFailed []string `json:"tests_failed,omitempty"`

	// Evidence
	CommandLog []string `json:"command_log,omitempty"`
	OutputLog  string   `json:"output_log,omitempty"`
	ErrorLog   string   `json:"error_log,omitempty"`

	// Structured inputs/outputs for audit trail
	StructuredInputs  map[string]interface{} `json:"structured_inputs,omitempty"`
	StructuredOutputs map[string]interface{} `json:"structured_outputs,omitempty"`
	StepExitStatuses  map[string]int               `json:"step_exit_statuses,omitempty"` // step_name -> exit_code
	OverallExitStatus int                           `json:"overall_exit_status"` // 0 = success
	TouchedFiles      []TouchedFile                `json:"touched_files,omitempty"` // files read/written

	// SR&ED
	EvidenceItems []contracts.EvidenceItem `json:"evidence_items,omitempty"`

	// Risk assessment
	UnresolvedRisks  []string `json:"unresolved_risks,omitempty"`
	KnownLimitations []string `json:"known_limitations,omitempty"`

	// Recommendation
	RecommendedAction string `json:"recommended_action"` // merge, review, retry, escalate
	RequiresApproval  bool   `json:"requires_approval,omitempty"`
	ReviewNotes       string `json:"review_notes,omitempty"`

	// Artifacts
	ArtifactPaths   []string `json:"artifact_paths,omitempty"`
	GitStatusPath   string   `json:"git_status_path,omitempty"`
	GitDiffStatPath string   `json:"git_diff_stat_path,omitempty"`
	TemplateKey     string   `json:"template_key,omitempty"`
	GitBranch       string   `json:"git_branch,omitempty"`
	GitCommit       string   `json:"git_commit,omitempty"`
	PRURL           string   `json:"pr_url,omitempty"`

	// Timestamps
	GeneratedAt time.Time `json:"generated_at"`

	// Enhanced metadata (v2.0+)
	SchemaID     string                 `json:"schema_id,omitempty"`
	Signature    *ArtifactSignature    `json:"signature,omitempty"`
	Checksums    map[string]string      `json:"checksums,omitempty"`
	Environment  *ExecutionEnvironment `json:"environment,omitempty"`
	MetadataTags map[string]string      `json:"metadata_tags,omitempty"`

	// Enhanced provenance (v2.0+)
	GitProvenance *GitProvenance `json:"git_provenance,omitempty"`
}

// GitProvenance represents cryptographic provenance from git commits.
// Provides verifiable chain of changes linking proof to git history.
type GitProvenance struct {
	CommitSHA      string `json:"commit_sha"`       // git commit SHA-256
	TreeSHA        string `json:"tree_sha"`        // git tree SHA-256
	ParentCommit   string `json:"parent_commit"`   // parent commit SHA-256
	CommitMessage  string `json:"commit_message"`   // commit message
	Committer      string `json:"committer"`         // committer name/email
	CommitTime     string `json:"commit_time"`     // commit timestamp
}

// ArtifactSignature represents cryptographic signature information for proof-of-work artifacts.
// Signature is optional but provides strong verification of authenticity.
type ArtifactSignature struct {
	Algorithm   string `json:"algorithm"`   // e.g., "rsa-sha256", "ecdsa-sha256"
	KeyID       string `json:"key_id"`      // Identifier of the signing key
	Signature   string `json:"signature"`   // Base64-encoded signature
	Signer      string `json:"signer"`      // Signer identity (e.g., "zen-brain@production")
	SignedAt    string `json:"signed_at"`   // Timestamp when signature was created (RFC3339)
	ProofDigest string `json:"proof_digest"` // Digest of the proof data that was signed
}

// ExecutionEnvironment captures environment metadata at execution time.
type ExecutionEnvironment struct {
	OS           string `json:"os"`             // e.g., "linux", "darwin", "windows"
	Architecture string `json:"architecture"`   // e.g., "amd64", "arm64"
	GoVersion    string `json:"go_version"`     // Go runtime version
	Hostname     string `json:"hostname"`      // Hostname where proof was generated
	FactoryVersion string `json:"factory_version"` // Factory version/tag
	Timestamp    string `json:"timestamp"`      // When environment was captured (RFC3339)
}

// TouchedFile represents a file that was read or written during execution.
type TouchedFile struct {
	Path         string    `json:"path"`
	Operation    string    `json:"operation"` // "read", "write", "delete"
	Size         int64     `json:"size,omitempty"`
	Checksum     string    `json:"checksum,omitempty"`
	ModifiedAt   time.Time `json:"modified_at,omitempty"`
}

// ArtifactChecksums stores checksums for all proof-of-work files.
type ArtifactChecksums struct {
	JSONArtifact     string `json:"json_artifact"`
	MarkdownArtifact  string `json:"markdown_artifact"`
	ExecutionLog      string `json:"execution_log"`
	WorkspaceFiles   map[string]string `json:"workspace_files,omitempty"`
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
