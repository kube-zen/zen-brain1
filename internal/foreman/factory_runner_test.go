package foreman

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFactoryTaskRunner_MapsBrainTaskToSpec(t *testing.T) {
	// Use config-based constructor to get a runner with PreferRealTemplates
	cfg := FactoryTaskRunnerConfig{
		RuntimeDir:          t.TempDir(),
		WorkspaceHome:       t.TempDir(),
		PreferRealTemplates: true,
	}
	r, err := NewFactoryTaskRunner(cfg)
	if err != nil {
		t.Fatalf("NewFactoryTaskRunner: %v", err)
	}
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-1", Namespace: "default"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-1",
			SessionID:  "session-1",
			Title:      "Implement feature",
			Objective:  "Do the work",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: "", // empty -> should map to real when PreferRealTemplates
			Priority:   contracts.PriorityHigh,
			TimeoutSeconds: 300,
			MaxRetries:     2,
			KBScopes:       []string{"scope1"},
		},
	}
	spec := r.brainTaskToFactorySpec(task)
	if spec.ID != "task-1" {
		t.Errorf("ID: got %q", spec.ID)
	}
	if spec.SessionID != "session-1" {
		t.Errorf("SessionID: got %q", spec.SessionID)
	}
	if spec.WorkItemID != "WI-1" {
		t.Errorf("WorkItemID: got %q", spec.WorkItemID)
	}
	if spec.Title != "Implement feature" {
		t.Errorf("Title: got %q", spec.Title)
	}
	if spec.Objective != "Do the work" {
		t.Errorf("Objective: got %q", spec.Objective)
	}
	if string(spec.WorkType) != "implementation" {
		t.Errorf("WorkType: got %q", spec.WorkType)
	}
	// PreferRealTemplates + empty domain -> real
	if string(spec.WorkDomain) != "real" {
		t.Errorf("WorkDomain: expected real when PreferRealTemplates and empty, got %q", spec.WorkDomain)
	}
	if spec.TimeoutSeconds != 300 {
		t.Errorf("TimeoutSeconds: got %d", spec.TimeoutSeconds)
	}
	if spec.MaxRetries != 2 {
		t.Errorf("MaxRetries: got %d", spec.MaxRetries)
	}
	if len(spec.KBScopes) != 1 || spec.KBScopes[0] != "scope1" {
		t.Errorf("KBScopes: got %v", spec.KBScopes)
	}
}

func TestFactoryTaskRunner_ReturnsOutcomeWithWorkspacePath(t *testing.T) {
	wsHome := t.TempDir()
	runtimeDir := t.TempDir()

	// Initialize git in the workspace home directory so templates can validate
	setupGitInDir(t, wsHome)

	cfg := FactoryTaskRunnerConfig{
		RuntimeDir:          runtimeDir,
		WorkspaceHome:       wsHome,
		PreferRealTemplates: false, // Use stub templates that don't require project structure
	}
	r, err := NewFactoryTaskRunner(cfg)
	if err != nil {
		t.Fatalf("NewFactoryTaskRunner: %v", err)
	}
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-1", Namespace: "default"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-1",
			SessionID:  "session-1",
			Title:      "Test",
			Objective:  "Test objective",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: contracts.WorkDomain("stub"), // use stub template
		},
	}
	ctx := context.Background()
	outcome, err := r.Run(ctx, task)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome == nil {
		t.Fatal("outcome should not be nil")
	}
	if outcome.WorkspacePath == "" {
		t.Error("outcome.WorkspacePath should be non-empty")
	}
	if outcome.ResultStatus != "completed" {
		t.Errorf("ResultStatus: got %q", outcome.ResultStatus)
	}
	if outcome.TemplateKey == "" {
		t.Error("TemplateKey should be set")
	}
}

// setupGitInDir initializes a git repo in the given directory for template validation.
func setupGitInDir(t *testing.T, dir string) {
	t.Helper()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name: %v", err)
	}

	// Create initial commit so git validation passes
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Workspace\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func TestNewFactoryTaskRunner_FromConfig(t *testing.T) {
	cfg := FactoryTaskRunnerConfig{
		RuntimeDir:          t.TempDir(),
		WorkspaceHome:       t.TempDir(),
		PreferRealTemplates: true,
	}
	r, err := NewFactoryTaskRunner(cfg)
	if err != nil {
		t.Fatalf("NewFactoryTaskRunner: %v", err)
	}
	if r == nil {
		t.Fatal("runner should not be nil")
	}
	if r.Factory == nil {
		t.Fatal("Factory should be set")
	}
}

func TestNewFactoryTaskRunner_WithFactory(t *testing.T) {
	wsHome := t.TempDir()
	runtimeDir := t.TempDir()
	w := factory.NewWorkspaceManager(wsHome)
	exec := factory.NewBoundedExecutor()
	pow := factory.NewProofOfWorkManager(runtimeDir)
	f := factory.NewFactory(w, exec, pow, runtimeDir)
	r := NewFactoryTaskRunnerWithFactory(f)
	if r == nil {
		t.Fatal("runner should not be nil")
	}
	if r.Factory != f {
		t.Error("Factory should be the passed instance")
	}
}
