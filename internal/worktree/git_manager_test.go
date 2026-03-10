package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitManager_PrepareAndCleanup(t *testing.T) {
	// Create a temp git repo
	repoDir := t.TempDir()
	run(t, repoDir, "git", "init")
	run(t, repoDir, "git", "config", "user.email", "test@test")
	run(t, repoDir, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(repoDir, "README"), "hello")
	run(t, repoDir, "git", "add", "README")
	run(t, repoDir, "git", "commit", "-m", "initial")

	baseDir := t.TempDir()
	cfg := GitManagerConfig{
		RepoPath:     repoDir,
		BasePath:     baseDir,
		DefaultRef:   "HEAD",
		BranchPrefix: "ai",
	}
	mgr, err := NewGitManager(cfg)
	if err != nil {
		t.Fatalf("NewGitManager: %v", err)
	}

	ctx := context.Background()
	workDir, cleanup, err := mgr.Prepare(ctx, "task-1", "session-1")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	if workDir == "" {
		t.Fatal("workDir is empty")
	}
	absBase, _ := filepath.Abs(baseDir)
	absWork, _ := filepath.Abs(workDir)
	if !isPathWithinRoot(absWork, absBase) {
		t.Errorf("worktree path %s not under BasePath %s", workDir, baseDir)
	}
	if _, err := os.Stat(filepath.Join(workDir, "README")); err != nil {
		t.Errorf("README not in worktree: %v", err)
	}

	cleanup()
	// After cleanup, worktree should be removed
	if worktreeExists(repoDir, workDir) {
		t.Error("worktree still exists after cleanup")
	}
}

func TestGitManager_InvalidRepoFails(t *testing.T) {
	notRepo := t.TempDir()
	baseDir := t.TempDir()
	cfg := GitManagerConfig{RepoPath: notRepo, BasePath: baseDir}
	mgr, err := NewGitManager(cfg)
	if err != nil {
		t.Fatalf("NewGitManager: %v", err)
	}
	_, _, err = mgr.Prepare(context.Background(), "t", "s")
	if err == nil {
		t.Fatal("expected error for non-git path")
	}
	if !strings.Contains(err.Error(), "not a git repo") {
		t.Errorf("error should mention not a git repo: %v", err)
	}
}

func TestGitManager_PathStaysUnderBasePath(t *testing.T) {
	repoDir := t.TempDir()
	run(t, repoDir, "git", "init")
	run(t, repoDir, "git", "config", "user.email", "t@t")
	run(t, repoDir, "git", "config", "user.name", "T")
	writeFile(t, filepath.Join(repoDir, "f"), "x")
	run(t, repoDir, "git", "add", "f")
	run(t, repoDir, "git", "commit", "-m", "x")

	baseDir := t.TempDir()
	mgr, err := NewGitManager(GitManagerConfig{RepoPath: repoDir, BasePath: baseDir})
	if err != nil {
		t.Fatalf("NewGitManager: %v", err)
	}
	workDir, cleanup, err := mgr.Prepare(context.Background(), "task", "session")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()
	absBase, _ := filepath.Abs(baseDir)
	absWork, _ := filepath.Abs(workDir)
	if !isPathWithinRoot(absWork, absBase) {
		t.Errorf("path %s not under %s", absWork, absBase)
	}
}

func TestSanitizeBranchPart(t *testing.T) {
	if got := sanitizeBranchPart("a/b:c"); got != "a-b-c" {
		t.Errorf("sanitizeBranchPart: got %q", got)
	}
	if got := sanitizeBranchPart("  x-y  "); got != "x-y" {
		t.Errorf("sanitizeBranchPart: got %q", got)
	}
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

