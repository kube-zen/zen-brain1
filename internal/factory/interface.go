package factory

import (
	"context"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// Factory defines the interface for task execution.
// Factory is responsible for accepting tasks, managing workspaces,
// and executing bounded work with proof-of-work generation.
type Factory interface {
	// ExecuteTask runs a task in an isolated workspace.
	// This is the main entry point for task execution.
	ExecuteTask(ctx context.Context, spec *FactoryTaskSpec) (*ExecutionResult, error)

	// AllocateWorkspace creates or retrieves an isolated workspace for a task.
	// Workspace path is deterministic based on task/session IDs.
	AllocateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error)

	// CleanupWorkspace removes a workspace and associated resources.
	// This is safe to call multiple times (idempotent).
	CleanupWorkspace(ctx context.Context, workspacePath string) error

	// GetWorkspaceMetadata returns current workspace state.
	GetWorkspaceMetadata(ctx context.Context, workspacePath string) (*WorkspaceMetadata, error)

	// GenerateProofOfWork creates a structured proof-of-work summary.
	GenerateProofOfWork(ctx context.Context, result *ExecutionResult) (*ProofOfWorkSummary, error)

	// ListTasks returns all tasks known to the Factory.
	ListTasks(ctx context.Context) ([]*FactoryTaskSpec, error)

	// GetTask retrieves a specific task by ID.
	GetTask(ctx context.Context, taskID string) (*FactoryTaskSpec, error)

	// CancelTask cancels a running task.
	CancelTask(ctx context.Context, taskID string) error
}

// WorkspaceManager handles workspace lifecycle.
// Workspaces provide isolated execution environments for tasks.
type WorkspaceManager interface {
	// CreateWorkspace creates a new isolated workspace.
	CreateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error)

	// ValidateWorkspace checks if a workspace is valid and clean.
	ValidateWorkspace(ctx context.Context, path string) (bool, error)

	// LockWorkspace locks a workspace for exclusive access.
	LockWorkspace(ctx context.Context, path string) error

	// UnlockWorkspace releases a workspace lock.
	UnlockWorkspace(ctx context.Context, path string) error

	// GetWorkspaceMetadata returns workspace state.
	GetWorkspaceMetadata(ctx context.Context, path string) (*WorkspaceMetadata, error)

	// DeleteWorkspace removes a workspace and all its contents.
	DeleteWorkspace(ctx context.Context, path string) error
}

// Executor runs bounded execution steps.
// Executor enforces timeout, retry, and safety limits.
type Executor interface {
	// ExecuteStep runs a single bounded execution step.
	ExecuteStep(ctx context.Context, step *ExecutionStep, workspacePath string) (*ExecutionStep, error)

	// ExecutePlan runs a sequence of steps as a bounded execution loop.
	ExecutePlan(ctx context.Context, steps []*ExecutionStep, workspacePath string) (*ExecutionResult, error)
}

// ProofOfWorkGenerator creates structured proof-of-work summaries.
type ProofOfWorkGenerator interface {
	// Generate creates a proof-of-work summary from execution result.
	Generate(ctx context.Context, result *ExecutionResult) (*ProofOfWorkSummary, error)

	// SerializeToJSON converts proof-of-work to JSON format.
	SerializeToJSON(proof *ProofOfWorkSummary) ([]byte, error)

	// SerializeToMarkdown converts proof-of-work to human-readable markdown.
	SerializeToMarkdown(proof *ProofOfWorkSummary) (string, error)
}

// ProofOfWorkManager manages proof-of-work artifact generation and storage.
// It creates structured evidence bundles for task execution.
type ProofOfWorkManager interface {
	// CreateProofOfWork creates a complete proof-of-work bundle.
	// Generates both JSON and markdown formats for easy consumption.
	CreateProofOfWork(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (*ProofOfWorkArtifact, error)

	// GenerateJiraComment creates a Jira comment from proof-of-work summary.
	// Returns a contracts.Comment that can be posted to Jira via office connector.
	GenerateJiraComment(ctx context.Context, artifact *ProofOfWorkArtifact) (*contracts.Comment, error)

	// ListProofOfWorks returns all proof-of-work artifacts for a task.
	ListProofOfWorks(ctx context.Context, taskID string) ([]*ProofOfWorkArtifact, error)

	// GetProofOfWork retrieves a specific proof-of-work artifact.
	GetProofOfWork(ctx context.Context, artifactDir string) (*ProofOfWorkArtifact, error)

	// CleanupProofOfWorks removes old proof-of-work artifacts.
	CleanupProofOfWorks(ctx context.Context, olderThan time.Duration) error
}
