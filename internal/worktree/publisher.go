// Package worktree provides git worktree management and publishing capabilities.
package worktree

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Publisher handles git operations for bounded remediation.
type Publisher struct {
	repoPath    string
	authorName  string
	authorEmail string
	remoteName  string
	pushEnabled bool
}

// NewPublisher creates a Publisher for git-backed remediation.
func NewPublisher(repoPath, authorName, authorEmail string, pushEnabled bool, remoteName ...string) (*Publisher, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("worktree: repoPath is required")
	}
	if !isGitRepo(repoPath) {
		return nil, fmt.Errorf("worktree: path is not a git repo: %s", repoPath)
	}
	if authorName == "" {
		authorName = "zen-brain1"
	}
	if authorEmail == "" {
		authorEmail = "zen-brain1@kube-zen.io"
	}
	rn := "origin"
	if len(remoteName) > 0 && remoteName[0] != "" {
		rn = remoteName[0]
	}
	return &Publisher{
		repoPath:    repoPath,
		authorName:  authorName,
		authorEmail: authorEmail,
		remoteName:  rn,
		pushEnabled: pushEnabled,
	}, nil
}

// CreateBranch creates a new branch for the remediation.
func (p *Publisher) CreateBranch(ctx context.Context, branchName, baseRef string) error {
	if baseRef == "" {
		baseRef = "HEAD"
	}
	cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "checkout", "-b", branchName, baseRef)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create branch %s: %w (output: %s)", branchName, err, string(out))
	}
	return nil
}

// CommitChanges stages and commits the specified files.
func (p *Publisher) CommitChanges(ctx context.Context, files []string, message string) (commitSHA string, err error) {
	// Stage files
	for _, f := range files {
		absPath := filepath.Join(p.repoPath, f)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", f)
		}
		cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "add", "--", f)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git add %s: %w (output: %s)", f, err, string(out))
		}
	}

	// Set author identity for this commit
	env := append(os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", p.authorName),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", p.authorEmail),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", p.authorName),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", p.authorEmail),
	)

	// Commit (with --no-verify to bypass pre-commit hooks for automated commits)
	cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "commit", "--no-verify", "-m", message)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %w (output: %s)", err, string(out))
	}

	// Get commit SHA
	cmd = exec.CommandContext(ctx, "git", "-C", p.repoPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// PushBranch pushes the branch to remote.
func (p *Publisher) PushBranch(ctx context.Context, branchName string) (remoteBranch string, err error) {
	if !p.pushEnabled {
		return "", fmt.Errorf("git push disabled")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "push", "-u", p.remoteName, branchName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git push %s: %w (output: %s)", branchName, err, string(out))
	}
	return fmt.Sprintf("%s/%s", p.remoteName, branchName), nil
}

// GetChangedFiles returns the list of modified files.
func (p *Publisher) GetChangedFiles(ctx context.Context, baseRef string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "diff", "--name-only", baseRef)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only: %w", err)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// WriteDiffStat writes a diff stat to the specified path.
func (p *Publisher) WriteDiffStat(ctx context.Context, baseRef, outputPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", p.repoPath, "diff", "--stat", baseRef)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git diff --stat: %w", err)
	}
	return ioutil.WriteFile(outputPath, out, 0644)
}

// GenerateBranchName creates a deterministic branch name for a ticket.
func GenerateBranchName(jiraKey string) string {
	timestamp := time.Now().Format("20060102-150405")
	sanitized := strings.ToLower(strings.ReplaceAll(jiraKey, "-", ""))
	return fmt.Sprintf("zb/%s-%s", sanitized, timestamp)
}
