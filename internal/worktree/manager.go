// Package worktree provides a worktree manager for task execution (Block 4).
// A worktree is a directory prepared for a task; the manager allocates and cleans up.
package worktree

import (
	"context"
	"os"
	"path/filepath"
)

// Manager prepares and cleans work trees for BrainTasks (Block 4 worktree manager).
type Manager interface {
	// Prepare creates a work tree for the given task/session and returns its path and a cleanup function.
	// Call cleanup when the task is done (or on failure).
	Prepare(ctx context.Context, taskID, sessionID string) (workDir string, cleanup func(), err error)
}

// StubManager is a Manager that uses a temp directory per Prepare (Block 4 stub).
type StubManager struct {
	// Prefix is used for the temp dir name; default "zen-worktree-".
	Prefix string
}

// NewStubManager returns a Manager that uses os.MkdirTemp with an optional prefix.
func NewStubManager(prefix string) *StubManager {
	if prefix == "" {
		prefix = "zen-worktree-"
	}
	return &StubManager{Prefix: prefix}
}

// Prepare returns a new temp dir and a cleanup that removes it.
func (s *StubManager) Prepare(ctx context.Context, taskID, sessionID string) (string, func(), error) {
	dir, err := os.MkdirTemp("", s.Prefix+"*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	return filepath.Clean(dir), cleanup, nil
}
