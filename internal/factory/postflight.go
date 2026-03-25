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
		{"files_verified", p.checkFilesCreated},
		{"code_compiles", p.checkCodeCompiles},
		{"no_fake_imports", p.checkNoFakeImports},
		{"tests_verified", p.checkTestsRan},
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

	// Downgrade recommendation if critical checks failed
	// Critical checks: execution_completed, files_verified (if files declared), proof_of_work
	criticalChecks := map[string]bool{
		"execution_completed": true,
		"files_verified":      result.FilesChanged != nil && len(result.FilesChanged) > 0,
		"proof_of_work":       true,
		"code_compiles":       true, // ZB-281 C022: compile failure is always critical
		"no_fake_imports":     true, // ZB-281 C022: fake imports are always critical
	}

	failedCriticalChecks := []string{}
	for _, check := range report.Checks {
		if criticalChecks[check.Name] && !check.Passed {
			failedCriticalChecks = append(failedCriticalChecks, check.Name)
		}
	}

	if len(failedCriticalChecks) > 0 {
		// Downgrade recommendation based on what failed
		if checkContains(report.Checks, "execution_completed") && !checkPassed(report.Checks, "execution_completed") {
			result.Recommendation = "retry"
		} else if checkContains(report.Checks, "files_verified") && !checkPassed(report.Checks, "files_verified") {
			result.Recommendation = "investigate"
		} else if checkContains(report.Checks, "proof_of_work") && !checkPassed(report.Checks, "proof_of_work") {
			result.Recommendation = "investigate"
		}

		// Mark result as having verification failures
		result.VerificationFailed = true
	}

	return report, nil
}

// checkPassed checks if a specific check passed.
func checkPassed(checks []PostflightCheckResult, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return check.Passed
		}
	}
	return false
}

// checkContains checks if a check with the given name exists.
func checkContains(checks []PostflightCheckResult, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return true
		}
	}
	return false
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

// checkProofOfWork verifies that proof-of-work was generated and is complete.
func (p *PostflightVerifier) checkProofOfWork(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	// ZB-281 C029: Check ProofOfWorkPath (set by CreateProofOfWork) first,
	// then fall back to workspace scan for backward compatibility.
	proofPaths := []string{}

	if result.ProofOfWorkPath != "" {
		// PoW was created at the artifact directory — check for markdown or JSON
		proofPaths = append(proofPaths,
			filepath.Join(result.ProofOfWorkPath, "proof-of-work.md"),
			filepath.Join(result.ProofOfWorkPath, "PROOF_OF_WORK.md"),
			filepath.Join(result.ProofOfWorkPath, "proof-of-work.json"),
		)
	}

	// Also check workspace for legacy proof files
	if result.WorkspacePath != "" {
		proofPaths = append(proofPaths,
			filepath.Join(result.WorkspacePath, "PROOF_OF_WORK.md"),
			filepath.Join(result.WorkspacePath, "PROOF.md"),
			filepath.Join(result.WorkspacePath, "proof.md"),
		)
	}

	// Try each path
	var proofContent string
	for _, pp := range proofPaths {
		if content, err := os.ReadFile(pp); err == nil {
			proofContent = string(content)
			break
		}
	}

	if proofContent == "" {
		if result.ProofOfWorkPath != "" {
			// Artifact directory exists but no readable proof file — pass with note
			return PostflightCheckResult{
				Name:     "proof_of_work",
				Passed:   true,
				Message:  fmt.Sprintf("Proof-of-work artifact created at %s", result.ProofOfWorkPath),
				Duration: time.Since(start),
			}, nil
		}
		return PostflightCheckResult{
			Name:     "proof_of_work",
			Passed:   false,
			Message:  "No proof-of-work file generated",
			Details:  "No proof artifact or workspace proof file found",
			Duration: time.Since(start),
		}, nil
	}

	// Check for minimal substantive content (at least 50 characters)
	if len(proofContent) < 50 {
		return PostflightCheckResult{
			Name:     "proof_of_work",
			Passed:   false,
			Message:  "Proof file is too small",
			Details:  fmt.Sprintf("Content length: %d characters", len(proofContent)),
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "proof_of_work",
		Passed:   true,
		Message:  fmt.Sprintf("Proof-of-work verified (%d characters)", len(proofContent)),
		Details:  fmt.Sprintf("Proof file has substantive content (%d characters)", len(proofContent)),
		Duration: time.Since(start),
	}, nil
}

// checkFilesCreated verifies that declared files were actually created/modified.
func (p *PostflightVerifier) checkFilesCreated(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if len(result.FilesChanged) == 0 {
		return PostflightCheckResult{
			Name:     "files_verified",
			Passed:   false,
			Message:  "No files generated — hard failure",
			Details:  "Task completed but produced zero files",
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

// checkCodeCompiles verifies that generated Go code compiles successfully.
// ZB-281 C022: This is a critical hard gate when the toolchain is available.
func (p *PostflightVerifier) checkCodeCompiles(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "code_compiles",
			Passed:   false,
			Message:  "No workspace path — cannot verify compilation",
			Duration: time.Since(start),
		}, nil
	}

	// Check if go toolchain is available
	if _, err := exec.LookPath("go"); err != nil {
		return PostflightCheckResult{
			Name:     "code_compiles",
			Passed:   true,
			Message:  "Skipped: go toolchain not available in runtime container",
			Duration: time.Since(start),
		}, nil
	}

	// Run go build ./... in the workspace
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = result.WorkspacePath
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Truncate output to avoid huge logs
		outStr := string(output)
		if len(outStr) > 500 {
			outStr = outStr[:500] + "... (truncated)"
		}
		return PostflightCheckResult{
			Name:     "code_compiles",
			Passed:   false,
			Message:  fmt.Sprintf("go build ./... failed: %s", strings.TrimSpace(outStr)),
			Details:  outStr,
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "code_compiles",
		Passed:   true,
		Message:  "go build ./... passed",
		Duration: time.Since(start),
	}, nil
}

// checkNoFakeImports scans generated files for hallucinated imports.
// ZB-281 C022: Fake imports indicate the model invented code rather than adapted it.
func (p *PostflightVerifier) checkNoFakeImports(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (PostflightCheckResult, error) {
	start := time.Now()

	// Known fake/suspicious import patterns from previous hallucination evidence
	fakePatterns := []string{
		"github.com/alexmiller/",
		"github.com/tidwall/",
		"github.com/example.com/",
		"github.com/stretchr/testify/mock",
		"github.com/prometheus/client_golang/prometheus",
		"github.com/sirupsen/logrus",
	}

	if len(result.FilesChanged) == 0 {
		return PostflightCheckResult{
			Name:     "no_fake_imports",
			Passed:   false,
			Message:  "No files to scan for fake imports",
			Duration: time.Since(start),
		}, nil
	}

	violations := []string{}
	for _, fpath := range result.FilesChanged {
		absPath := fpath
		if !filepath.IsAbs(fpath) && result.WorkspacePath != "" {
			absPath = filepath.Join(result.WorkspacePath, fpath)
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			continue // File might not exist, skip
		}

		for _, pattern := range fakePatterns {
			if strings.Contains(string(content), pattern) {
				violations = append(violations, fmt.Sprintf("%s contains %q", filepath.Base(fpath), pattern))
			}
		}

		// Check for "command execution not implemented" marker (previous hallucination)
		if strings.Contains(string(content), "command execution not implemented") {
			violations = append(violations, fmt.Sprintf("%s contains placeholder marker", filepath.Base(fpath)))
		}
	}

	if len(violations) > 0 {
		return PostflightCheckResult{
			Name:     "no_fake_imports",
			Passed:   false,
			Message:  fmt.Sprintf("%d fake/disallowed import violations: %s", len(violations), strings.Join(violations, "; ")),
			Details:  strings.Join(violations, "\n"),
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "no_fake_imports",
		Passed:   true,
		Message:  fmt.Sprintf("No fake imports detected across %d files", len(result.FilesChanged)),
		Duration: time.Since(start),
	}, nil
}
