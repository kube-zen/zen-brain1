// Package foreman defines the TaskDispatcher interface for worker dispatch (Block 4.3).
package foreman

import (
	"context"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// TaskDispatcher dispatches a scheduled BrainTask to a worker (Block 4.3).
// When nil, Foreman only updates status to Scheduled; no actual dispatch.
type TaskDispatcher interface {
	// Dispatch sends the task to a worker. Called after admission and status transition to Scheduled.
	Dispatch(ctx context.Context, task *v1alpha1.BrainTask) error
}

// NoOpDispatcher is a TaskDispatcher that does nothing (placeholder until workers are implemented).
type NoOpDispatcher struct{}

// Dispatch is a no-op.
func (NoOpDispatcher) Dispatch(ctx context.Context, task *v1alpha1.BrainTask) error {
	return nil
}
