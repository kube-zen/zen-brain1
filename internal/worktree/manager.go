// Package worktree provides a worktree.Manager interface and GitManager implementation (Block 4).
package worktree

import (
	"context"
)

// Manager is the interface for git worktree management.
// Implementations must be safe for concurrent use.
type Manager interface {
	// Prepare creates or reuses a worktree for the given task/session and returns the work directory path.
	// The returned cleanup function must be called when the worktree is no longer needed.
	Prepare(ctx context.Context, taskID, sessionID string) (workDir string, cleanup func(), err error)
}
