// Package foreman defines the TaskDispatcher interface for worker dispatch (Block 4.3).
package foreman

import (
	"context"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// TaskDispatcher dispatches a scheduled BrainTask to a worker (Block 4.3).
//
// When nil, Foreman only updates status to Scheduled; no actual dispatch occurs.
// This is the canonical way to disable dispatch (e.g., for testing or dry-run mode).
//
// The real implementation is Worker (see worker.go), which implements TaskDispatcher
// with a goroutine pool, session affinity, and context binding support.
type TaskDispatcher interface {
	// Dispatch sends the task to a worker. Called after admission and status transition to Scheduled.
	// Implementations must be non-blocking; enqueue the task and return immediately.
	Dispatch(ctx context.Context, task *v1alpha1.BrainTask) error
}
