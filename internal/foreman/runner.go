// Package foreman defines TaskRunner for executing a BrainTask (Block 4.3).
package foreman

import (
	"context"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// TaskRunner executes a single BrainTask. Used by Worker after setting status to Running.
type TaskRunner interface {
	// Run executes the task. The task has already been patched to Running.
	Run(ctx context.Context, task *v1alpha1.BrainTask) error
}

// PlaceholderRunner is a TaskRunner that does no work and succeeds (Block 4.3 placeholder).
type PlaceholderRunner struct{}

// Run returns nil without performing work.
func (PlaceholderRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) error {
	return nil
}
