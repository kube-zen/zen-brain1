// Package worktree provides a real git worktree manager for task execution (Block 4).
package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// GitManagerConfig configures the real git worktree manager.
type GitManagerConfig struct {
	RepoPath       string // Path to the git repo (must exist and be a git repo)
	BasePath       string // Base directory under which worktrees are created (must be under control)
	DefaultRef     string // e.g. "HEAD" or "main"
	BranchPrefix   string // e.g. "ai"
	ReuseSessionWT bool   // If true, reuse one worktree per session (task-scoped path within it or session-scoped path)
}

// GitManager creates and cleans real git worktrees.
type GitManager struct {
	cfg GitManagerConfig
}

// NewGitManager creates a GitManager. RepoPath and BasePath must be set and valid.
func NewGitManager(cfg GitManagerConfig) (*GitManager, error) {
	if cfg.RepoPath == "" {
		return nil, fmt.Errorf("worktree: RepoPath is required")
	}
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("worktree: BasePath is required")
	}
	repoAbs, err := filepath.Abs(filepath.Clean(cfg.RepoPath))
	if err != nil {
		return nil, fmt.Errorf("worktree: invalid RepoPath: %w", err)
	}
	baseAbs, err := filepath.Abs(filepath.Clean(cfg.BasePath))
	if err != nil {
		return nil, fmt.Errorf("worktree: invalid BasePath: %w", err)
	}
	if !isPathWithinRoot(baseAbs, repoAbs) && baseAbs != repoAbs {
		// BasePath should typically be a sibling or under a dedicated worktrees dir; allow any abs path
	}
	if cfg.DefaultRef == "" {
		cfg.DefaultRef = "HEAD"
	}
	if cfg.BranchPrefix == "" {
		cfg.BranchPrefix = "ai"
	}
	return &GitManager{cfg: GitManagerConfig{
		RepoPath:       repoAbs,
		BasePath:       baseAbs,
		DefaultRef:     cfg.DefaultRef,
		BranchPrefix:   cfg.BranchPrefix,
		ReuseSessionWT: cfg.ReuseSessionWT,
	}}, nil
}

// Prepare creates a git worktree for the given task/session and returns its path and a cleanup function.
func (g *GitManager) Prepare(ctx context.Context, taskID, sessionID string) (workDir string, cleanup func(), err error) {
	if !isGitRepo(g.cfg.RepoPath) {
		return "", nil, fmt.Errorf("worktree: path is not a git repo: %s", g.cfg.RepoPath)
	}
	if err := os.MkdirAll(g.cfg.BasePath, 0755); err != nil {
		return "", nil, fmt.Errorf("worktree: failed to create base path %s: %w", g.cfg.BasePath, err)
	}
	// Deterministic worktree path: BasePath/sessionID/taskID (sanitized)
	sanitizedSession := sanitizeBranchPart(sessionID)
	sanitizedTask := sanitizeBranchPart(taskID)
	if sanitizedSession == "" {
		sanitizedSession = "default"
	}
	if sanitizedTask == "" {
		sanitizedTask = "task"
	}
	worktreePath := filepath.Join(g.cfg.BasePath, sanitizedSession, sanitizedTask)
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return "", nil, fmt.Errorf("worktree: failed to resolve path: %w", err)
	}
	if !isPathWithinRoot(absPath, g.cfg.BasePath) {
		return "", nil, fmt.Errorf("worktree: worktree path would escape BasePath: %s", absPath)
	}
	// Remove stale path if it exists and is not a valid worktree (e.g. leftover dir)
	if dirExists(absPath) {
		if worktreeExists(g.cfg.RepoPath, absPath) {
			// Already a worktree; remove it so we can re-add (idempotent prepare)
			_ = removeStaleWorktree(g.cfg.RepoPath, absPath)
		} else {
			// Plain dir; remove so we can create worktree here
			_ = os.RemoveAll(absPath)
		}
	}
	ref := g.cfg.DefaultRef
	cmd := exec.CommandContext(ctx, "git", "-C", g.cfg.RepoPath, "worktree", "add", "--detach", absPath, ref)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("worktree: git worktree add failed: %w (output: %s)", err, string(out))
	}
	cleanupFn := func() {
		if worktreeExists(g.cfg.RepoPath, absPath) {
			_ = removeStaleWorktree(g.cfg.RepoPath, absPath)
		}
		_ = os.RemoveAll(absPath)
	}
	return absPath, cleanupFn, nil
}

// sanitizeBranchPart returns a safe string for use in paths/branch names.
func sanitizeBranchPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Allow alphanumeric, hyphen, underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// isGitRepo returns true if path is a git repository.
func isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// worktreeExists returns true if path is listed as a worktree of repo.
func worktreeExists(repo, path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	cmd := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			wt := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if wt == absPath {
				return true
			}
		}
	}
	return false
}

// removeStaleWorktree removes the worktree at path from repo. Best-effort.
func removeStaleWorktree(repo, path string) error {
	cmd := exec.Command("git", "-C", repo, "worktree", "remove", "--force", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w (output: %s)", err, string(out))
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isPathWithinRoot returns true if path is under root (or equal).
func isPathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
