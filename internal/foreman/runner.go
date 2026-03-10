// Package foreman defines TaskRunner for executing a BrainTask (Block 4.3).
package foreman

import (
	"context"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// TaskRunOutcome holds structured result of a task run (Block 4 execution).
type TaskRunOutcome struct {
	WorkspacePath   string
	ProofOfWorkPath string
	TemplateKey     string
	FilesChanged    int
	ResultStatus    string
	Recommendation  string
	DurationSeconds int64
}

// TaskRunner executes a single BrainTask. Used by Worker after setting status to Running.
type TaskRunner interface {
	// Run executes the task. The task has already been patched to Running.
	// Returns outcome (may be nil on error) and error.
	Run(ctx context.Context, task *v1alpha1.BrainTask) (*TaskRunOutcome, error)
}

// TaskRunnerWithContext is an optional extension: run with session context and return updated state for continuation (Block 5.3).
type TaskRunnerWithContext interface {
	TaskRunner
	// RunWithContext runs the task with current session context; returns updated session (State/Scratchpad) to store, or nil.
	RunWithContext(ctx context.Context, task *v1alpha1.BrainTask, sessionCtx *zenctx.SessionContext) (updated *zenctx.SessionContext, outcome *TaskRunOutcome, err error)
}

// ContextBinder provides session context for continuation and writes intermediate state (Block 5.3 agent-context binding).
// Implementations typically wrap ZenContext; pass from cmd when ZenContext is available.
type ContextBinder interface {
	GetForContinuation(ctx context.Context, clusterID, sessionID, taskID string) (*zenctx.SessionContext, error)
	WriteIntermediate(ctx context.Context, clusterID string, session *zenctx.SessionContext) error
}

// PlaceholderRunner is a TaskRunner that does no work and succeeds (Block 4.3 placeholder).
type PlaceholderRunner struct{}

// Run returns a completed outcome without performing work.
func (PlaceholderRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) (*TaskRunOutcome, error) {
	return &TaskRunOutcome{ResultStatus: "completed"}, nil
}
