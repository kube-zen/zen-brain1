// Package factory provides postflight verification for task execution.
package factory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PostflightCheckResult represents the result of a postflight check.
type PostflightCheckResult struct {
	Name        string        `json:"name"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// PostflightReport represents a complete postflight verification report.
type PostflightReport struct {
	TaskID         string                 `json:"task_id"`
	SessionID      string                 `json:"session_id"`
	Timestamp      time.Time              `json:"timestamp"`
	AllPassed      bool                   `json:"all_passed"`
	Checks         []PostflightCheckResult `json:"checks"`
	Artifacts      []string               `json:"artifacts,omitempty"`
	GitStatus      *GitStatusReport       `json:"git_status,omitempty"`
	ExecutionTrace []ExecutionTraceEntry  `json:"execution_trace,omitempty"`
}

// GitStatusReport represents git status information.
type GitStatusReport struct {
	Branch       string   `json:"branch"`
	Commit       string   `json:"commit"`
	DirtyFiles   []string `json:"dirty_files,omitempty"`
	StagedFiles  []string `json:"staged_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
}

// ExecutionTraceEntry represents a single step in the execution trace.
type ExecutionTraceEntry struct {
	StepID    string    `json:"step_id"`
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	ExitCode  int       `json:"exit_code"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// PostflightVerifier runs postflight checks after task execution.
type PostflightVerifier struct {
	workspaceManager WorkspaceManager
}

// NewPostflightVerifier creates a new PostflightVerifier.
func NewPostflightVerifier(wsManager WorkspaceManager) *PostflightVerifier {
	return &PostflightVerifier{
		workspaceManager: wsManager,
	}
}

// RunPostflightVerification executes all postflight checks for a task.
func (p *PostflightVerifier) RunPostflightVerification(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (*PostflightReport, error) {
	report := &PostflightReport{
		TaskID:    spec.ID,
		SessionID: spec.SessionID,
		Timestamp: time.Now(),
		AllPassed: true,
		Checks:    []PostflightCheckResult{},
	}

	// Run all postflight checks
	checks := []struct {
		name string
		fn   func(context.Context, *ExecutionResult, *FactoryTaskSpec) (PostflightCheckResult, error)
	}{
		{"execution_completed", p.checkExecutionCompleted},
		{"workspace_clean", p.checkWorkspaceClean},
		{"artifacts_generated", p.checkArtifactsGenerated},
		{"files_verified", p.checkFilesCreated},          // NEW: Verify files were actually created
		{"tests_verified", p.checkTestsRan},             // NEW: Verify tests actually ran
		{"git_status", p.checkGitStatus},
		{"proof_of_work", p.checkProofOfWork},
	}

	for _, check := range checks {
		start := time.Now()
		checkResult, err := check.fn(ctx, result, spec)
		if err != nil {
			checkResult = PostflightCheckResult{
				Name:     check.name,
				Passed:   false,
				Message:  fmt.Sprintf("Check failed with error: %v", err),
				Duration: time.Since(start),
			}
		}
		checkResult.Duration = time.Since(start)
		report.Checks = append(report.Checks, checkResult)
		if !checkResult.Passed {
			report.AllPassed = false
		}
	}

	return report, nil
}

	// checkExecutionCompleted verifies that execution completed successfully.
	func (p *PostflightVerifier) checkExecutionCompleted(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
		start := time.Now()

		if result == nil {
			return PostflightCheckResult{
				Name:     "execution_completed",
				Passed:   false,
				Message:  "Execution result is nil",
				Duration: time.Since(start),
			}, nil
		}

		if result.Error != "" {
			return PostflightCheckResult{
				Name:     "execution_completed",
				Passed:   false,
				Message:  "Execution failed with error",
				Details:  result.Error,
				Duration: time.Since(start),
			}, nil
		}

		if !result.Success {
			return PostflightCheckResult{
				Name:     "execution_completed",
				Passed:   false,
				Message:  "Execution marked as unsuccessful",
				Details:  fmt.Sprintf("Status: %s", result.Status),
				Duration: time.Since(start),
			}, nil
		}

		return PostflightCheckResult{
			Name:     "execution_completed",
			Passed:   true,
			Message:  "Execution completed successfully",
			Duration: time.Since(start),
		}, nil
	}

// checkWorkspaceClean verifies workspace state after execution.
func (p *PostflightVerifier) checkWorkspaceClean(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "workspace_clean",
			Passed:   true,
			Message:  "No workspace path to verify",
			Duration: time.Since(start),
		}, nil
	}

	// Check if workspace directory exists
	if _, err := os.Stat(result.WorkspacePath); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "workspace_clean",
			Passed:   false,
			Message:  "Workspace directory does not exist",
			Details:  result.WorkspacePath,
			Duration: time.Since(start),
		}, nil
	}

	// Check workspace is accessible
	if !isDirAccessible(result.WorkspacePath) {
		return PostflightCheckResult{
			Name:     "workspace_clean",
			Passed:   false,
			Message:  "Workspace directory is not accessible",
			Details:  result.WorkspacePath,
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "workspace_clean",
		Passed:   true,
		Message:  "Workspace is accessible",
		Details:  result.WorkspacePath,
		Duration: time.Since(start),
	}, nil
}

// checkArtifactsGenerated verifies that expected artifacts were created.
func (p *PostflightVerifier) checkArtifactsGenerated(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "artifacts_generated",
			Passed:   true,
			Message:  "No workspace path (no artifacts to verify)",
			Duration: time.Since(start),
		}, nil
	}

	// Check for common artifacts
	artifacts := []string{}
	artifactPatterns := []string{
		"*.json",
		"*.md",
		"*.log",
	}

	for _, pattern := range artifactPatterns {
		matches, err := filepath.Glob(filepath.Join(result.WorkspacePath, pattern))
		if err != nil {
			continue
		}
		artifacts = append(artifacts, matches...)
	}

	if len(artifacts) == 0 {
		return PostflightCheckResult{
			Name:     "artifacts_generated",
			Passed:   true, // Non-fatal
			Message:  "No artifacts generated",
			Details:  "No JSON/MD/LOG files found in workspace",
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "artifacts_generated",
		Passed:   true,
		Message:  fmt.Sprintf("Found %d artifacts", len(artifacts)),
		Details:  strings.Join(artifacts, ", "),
		Duration: time.Since(start),
	}, nil
}

// checkGitStatus verifies git repository state after execution.
func (p *PostflightVerifier) checkGitStatus(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "git_status",
			Passed:   true,
			Message:  "No workspace path to check git status",
			Duration: time.Since(start),
		}, nil
	}

	// Check if workspace is a git repo
	gitDir := filepath.Join(result.WorkspacePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "git_status",
			Passed:   true,
			Message:  "Workspace is not a git repository",
			Duration: time.Since(start),
		}, nil
	}

	// Get git status
	cmd := exec.CommandContext(ctx, "git", "-C", result.WorkspacePath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return PostflightCheckResult{
			Name:     "git_status",
			Passed:   true, // Non-fatal
			Message:  "Failed to get git status",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	dirtyFiles := strings.TrimSpace(string(output))
	if dirtyFiles != "" {
		files := strings.Split(dirtyFiles, "\n")
		return PostflightCheckResult{
			Name:     "git_status",
			Passed:   true, // Non-fatal, informational
			Message:  fmt.Sprintf("Git repository has %d modified files", len(files)),
			Details:  dirtyFiles,
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "git_status",
		Passed:   true,
		Message:  "Git repository is clean",
		Duration: time.Since(start),
	}, nil
}

// checkProofOfWork verifies that proof-of-work was generated.
func (p *PostflightVerifier) checkProofOfWork(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	// Proof-of-work is optional, so we just check if it exists
	// In the future, we could verify checksums, signatures, etc.

	return PostflightCheckResult{
		Name:     "proof_of_work",
		Passed:   true,
		Message:  "Proof-of-work verification not implemented yet",
		Details:  "Future: verify checksums, signatures, artifact integrity",
		Duration: time.Since(start),
	}, nil
}

// checkFilesCreated verifies that declared files were actually created/modified.
func (p *PostflightVerifier) checkFilesCreated(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if len(result.FilesChanged) == 0 {
		return PostflightCheckResult{
			Name:     "files_verified",
			Passed:   true,
			Message:  "No files declared as changed",
			Duration: time.Since(start),
		}, nil
	}

	// Verify each declared file exists
	missingFiles := []string{}
	for _, filePath := range result.FilesChanged {
		// Use absolute path
		absPath := filePath
		if !filepath.IsAbs(filePath) && result.WorkspacePath != "" {
			absPath = filepath.Join(result.WorkspacePath, filePath)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, filePath)
		}
	}

	if len(missingFiles) > 0 {
		return PostflightCheckResult{
			Name:     "files_verified",
			Passed:   false,
			Message:  fmt.Sprintf("%d files declared as changed but not found", len(missingFiles)),
			Details:  strings.Join(missingFiles, ", "),
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "files_verified",
		Passed:   true,
		Message:  fmt.Sprintf("All %d declared files verified", len(result.FilesChanged)),
		Duration: time.Since(start),
	}, nil
}

// checkTestsRan verifies that declared tests actually ran.
func (p *PostflightVerifier) checkTestsRan(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	// If no tests declared, pass
	if len(result.TestsRun) == 0 {
		return PostflightCheckResult{
			Name:     "tests_verified",
			Passed:   true,
			Message:  "No tests declared",
			Duration: time.Since(start),
		}, nil
	}

	// Check execution steps for test-related steps
	testSteps := []*ExecutionStep{}
	for _, step := range result.ExecutionSteps {
		stepName := strings.ToLower(step.Name)
		if strings.Contains(stepName, "test") || strings.Contains(stepName, "verify") {
			testSteps = append(testSteps, step)
		}
	}

	// If tests declared but no test steps found, that's suspicious
	if len(result.TestsRun) > 0 && len(testSteps) == 0 {
		return PostflightCheckResult{
			Name:     "tests_verified",
			Passed:   false,
			Message:  "Tests declared but no test execution steps found",
			Details:  fmt.Sprintf("Declared: %v", result.TestsRun),
			Duration: time.Since(start),
		}, nil
	}

	// Check if test steps have meaningful output (not just echo)
	for _, step := range testSteps {
		if step.Output == "" {
			continue
		}

		output := strings.ToLower(step.Output)

		// Exclude obvious simulation patterns
		if strings.Contains(output, "simulating") ||
			strings.Contains(output, "echo ") ||
			strings.Contains(output, "placeholder") {
			continue
		}

		// Real test output requires MULTIPLE patterns (not just one)
		// This distinguishes "test passed" from "simulating test execution"
		patternCount := 0
		realPatterns := []string{
			"=== run",      // Go test
			"--- pass",     // Go test
			"--- fail",     // Go test
			"passed",       // Generic
			"failed",       // Generic
			"ok ",          // Go test
			"error:",       // Error output
			"assertion",    // Test assertion
			"expect",       // Test expectation
			"::",           // pytest pattern (test_file.py::test_name)
		}

		for _, pattern := range realPatterns {
			if strings.Contains(output, pattern) {
				patternCount++
			}
		}

		// Need at least 2 patterns to be considered real test output
		// (e.g., "=== RUN" and "--- PASS", or "passed" and "ok")
		if patternCount >= 2 {
			// Test produced real output
			return PostflightCheckResult{
				Name:     "tests_verified",
				Passed:   result.TestsPassed,
				Message:  fmt.Sprintf("Test execution verified (%d test steps)", len(testSteps)),
				Details:  fmt.Sprintf("Tests passed: %v, Steps: %d", result.TestsPassed, len(testSteps)),
				Duration: time.Since(start),
			}, nil
		}
	}

	// Tests declared but no real test output found
	return PostflightCheckResult{
		Name:     "tests_verified",
		Passed:   false,
		Message:  "Tests declared but test steps lack real output",
		Details:  "Test steps found but output doesn't match real test patterns",
		Duration: time.Since(start),
	}, nil
}

// isDirAccessible checks if a directory is accessible.
func isDirAccessible(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// MustRunPostflightVerification runs postflight checks and returns error if any fail.
// This is for fail-closed behavior when checks are mandatory.
func (p *PostflightVerifier) MustRunPostflightVerification(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (*PostflightReport, error) {
	report, err := p.RunPostflightVerification(ctx, result, spec)
	if err != nil {
		return nil, fmt.Errorf("postflight verification failed: %w", err)
	}

	if !report.AllPassed {
		failedChecks := []string{}
		for _, check := range report.Checks {
			if !check.Passed {
				failedChecks = append(failedChecks, fmt.Sprintf("%s: %s", check.Name, check.Message))
			}
		}
		return report, fmt.Errorf("postflight checks failed: %s", strings.Join(failedChecks, "; "))
	}

	return report, nil
}
