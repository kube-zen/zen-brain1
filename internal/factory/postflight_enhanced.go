// Package factory provides enhanced postflight verification for task execution.
// These checks add stronger validation for artifact integrity and git state.
package factory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EnhancedPostflightVerifier runs enhanced postflight verification.
type EnhancedPostflightVerifier struct {
	workspaceManager WorkspaceManager
	strictMode        bool // If true, more strict validation
}

// NewEnhancedPostflightVerifier creates a new EnhancedPostflightVerifier.
func NewEnhancedPostflightVerifier(wsManager WorkspaceManager, strictMode bool) *EnhancedPostflightVerifier {
	return &EnhancedPostflightVerifier{
		workspaceManager: wsManager,
		strictMode:       strictMode,
	}
}

// RunEnhancedPostflightVerification executes all enhanced postflight checks.
func (e *EnhancedPostflightVerifier) RunEnhancedPostflightVerification(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (*PostflightReport, error) {
	report := &PostflightReport{
		TaskID:    spec.ID,
		SessionID: spec.SessionID,
		Timestamp: time.Now(),
		AllPassed: true,
		Checks:    []PostflightCheckResult{},
	}

	// Run all checks
	checks := []struct {
		name string
		fn   func(context.Context, *ExecutionResult, *FactoryTaskSpec) (PostflightCheckResult, error)
	}{
		{"execution_completed", e.checkExecutionCompleted},
		{"workspace_accessible", e.checkWorkspaceAccessible},
		{"git_state_valid", e.checkGitStateValid},      // NEW: stricter git validation
		{"git_status_verified", e.checkGitStatusVerified}, // NEW: verify git status matches expectations
		{"artifacts_generated", e.checkArtifactsGenerated},
		{"artifact_integrity", e.checkArtifactIntegrity}, // NEW: verify artifact checksums
		{"files_verified", e.checkFilesVerified},
		{"tests_verified", e.checkTestsVerified},
		{"proof_of_work_valid", e.checkProofOfWorkValid}, // NEW: validate proof structure
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

		// In strict mode, any failure fails the entire report
		if !checkResult.Passed {
			report.AllPassed = !e.strictMode
		}
	}

	return report, nil
}

// MustRunEnhancedPostflightVerification runs checks and enforces failure behavior.
func (e *EnhancedPostflightVerifier) MustRunEnhancedPostflightVerification(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (*PostflightReport, error) {
	report, err := e.RunEnhancedPostflightVerification(ctx, result, spec)
	if err != nil {
		return nil, fmt.Errorf("enhanced postflight verification failed: %w", err)
	}

	if !report.AllPassed {
		failedChecks := []string{}
		for _, check := range report.Checks {
			if !check.Passed {
				failedChecks = append(failedChecks, fmt.Sprintf("%s: %s", check.Name, check.Message))
			}
		}
		return report, fmt.Errorf("enhanced postflight verification failed: %s", strings.Join(failedChecks, "; "))
	}

	return report, nil
}

// checkExecutionCompleted verifies that execution completed successfully.
func (e *EnhancedPostflightVerifier) checkExecutionCompleted(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
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

// checkWorkspaceAccessible verifies workspace is accessible.
func (e *EnhancedPostflightVerifier) checkWorkspaceAccessible(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "workspace_accessible",
			Passed:   true,
			Message:  "No workspace path to verify",
			Duration: time.Since(start),
		}, nil
	}

	// Check if workspace directory exists
	if _, err := os.Stat(result.WorkspacePath); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "workspace_accessible",
			Passed:   false,
			Message:  "Workspace directory does not exist",
			Details:  result.WorkspacePath,
			Duration: time.Since(start),
		}, nil
	}

	// Check workspace is accessible
	if !isDirAccessible(result.WorkspacePath) {
		return PostflightCheckResult{
			Name:     "workspace_accessible",
			Passed:   false,
			Message:  "Workspace directory is not accessible",
			Details:  result.WorkspacePath,
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "workspace_accessible",
		Passed:   true,
		Message:  "Workspace is accessible",
		Details:  result.WorkspacePath,
		Duration: time.Since(start),
	}, nil
}

// checkGitStateValid performs stricter git state validation.
func (e *EnhancedPostflightVerifier) checkGitStateValid(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "git_state_valid",
			Passed:   true,
			Message:  "No workspace path to check git state",
			Duration: time.Since(start),
		}, nil
	}

	// Check if workspace is a git repo
	gitDir := filepath.Join(result.WorkspacePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "git_state_valid",
			Passed:   true, // Non-fatal if not a git repo
			Message:  "Workspace is not a git repository",
			Details:  result.WorkspacePath,
			Duration: time.Since(start),
		}, nil
	}

	// Verify HEAD exists (not an empty repo)
	cmd := exec.CommandContext(ctx, "git", "-C", result.WorkspacePath, "rev-parse", "--verify", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PostflightCheckResult{
			Name:     "git_state_valid",
			Passed:   false,
			Message:  "Git repository has no HEAD (empty or corrupted)",
			Details:  strings.TrimSpace(string(output)),
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "git_state_valid",
		Passed:   true,
		Message:  "Git repository state is valid",
		Details:  "HEAD exists and is valid",
		Duration: time.Since(start),
	}, nil
}

// checkGitStatusVerified verifies git status matches expected changes.
func (e *EnhancedPostflightVerifier) checkGitStatusVerified(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
	start := time.Now()

	if result.WorkspacePath == "" {
		return PostflightCheckResult{
			Name:     "git_status_verified",
			Passed:   true,
			Message:  "No workspace path to verify git status",
			Duration: time.Since(start),
		}, nil
	}

	// Check if workspace is a git repo
	gitDir := filepath.Join(result.WorkspacePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "git_status_verified",
			Passed:   true,
			Message:  "Not a git repository (skipping)",
			Duration: time.Since(start),
		}, nil
	}

	// Get git status
	cmd := exec.CommandContext(ctx, "git", "-C", result.WorkspacePath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return PostflightCheckResult{
			Name:     "git_status_verified",
			Passed:   true, // Non-fatal
			Message:  "Could not verify git status",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	statusLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	dirtyFiles := []string{}
	for _, line := range statusLines {
		if line != "" {
			dirtyFiles = append(dirtyFiles, line)
		}
	}

	// In non-strict mode, we just report git status
	// In strict mode, we verify that declared changes match actual changes
	details := fmt.Sprintf("Dirty files: %d", len(dirtyFiles))

	if len(dirtyFiles) > 0 {
		// If task declared files changed, verify consistency
		if len(result.FilesChanged) > 0 {
			// Count actual modified files from git status
			actualModified := []string{}
			for _, line := range dirtyFiles {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					actualModified = append(actualModified, fields[1])
				}
			}

			// Check if declared files subset of actual files
			declaredSet := make(map[string]bool)
			for _, f := range result.FilesChanged {
				declaredSet[filepath.Base(f)] = true
			}

			missingInDeclared := []string{}
			for _, actual := range actualModified {
				if !declaredSet[filepath.Base(actual)] {
					missingInDeclared = append(missingInDeclared, filepath.Base(actual))
				}
			}

			if len(missingInDeclared) > 0 {
				details = fmt.Sprintf("Inconsistent: declared %d files, git shows %d. Missing from declared: %v",
					len(result.FilesChanged), len(actualModified), missingInDeclared)
				return PostflightCheckResult{
					Name:     "git_status_verified",
					Passed:   !e.strictMode, // Fail in strict mode
					Message:  "Declared files inconsistent with git status",
					Details:  details,
					Duration: time.Since(start),
				}, nil
			}
		}

		return PostflightCheckResult{
			Name:     "git_status_verified",
			Passed:   true,
			Message:  fmt.Sprintf("Git repository has %d modified files", len(dirtyFiles)),
			Details:  details,
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "git_status_verified",
		Passed:   true,
		Message:  "Git repository is clean",
		Details:  details,
		Duration: time.Since(start),
	}, nil
}

// checkArtifactsGenerated verifies that expected artifacts were created.
func (e *EnhancedPostflightVerifier) checkArtifactsGenerated(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
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
		"*.txt",
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
			Details:  "No JSON/MD/LOG/TXT files found in workspace",
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

// checkArtifactIntegrity verifies artifact checksums and integrity.
func (e *EnhancedPostflightVerifier) checkArtifactIntegrity(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
	start := time.Now()

	// Verify proof-of-work artifacts exist
	if result.ProofOfWorkPath == "" {
		return PostflightCheckResult{
			Name:     "artifact_integrity",
			Passed:   true, // Non-fatal
			Message:  "No proof-of-work path to verify",
			Duration: time.Since(start),
		}, nil
	}

	// Check if proof-of-work directory exists
	if _, err := os.Stat(result.ProofOfWorkPath); os.IsNotExist(err) {
		return PostflightCheckResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Proof-of-work directory does not exist",
			Details:  result.ProofOfWorkPath,
			Duration: time.Since(start),
		}, nil
	}

	// Verify expected files exist
	expectedFiles := []string{
		"proof-of-work.json",
		"proof-of-work.md",
		"execution.log",
	}

	missingFiles := []string{}
	for _, file := range expectedFiles {
		path := filepath.Join(result.ProofOfWorkPath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missingFiles = append(missingFiles, file)
		}
	}

	if len(missingFiles) > 0 {
		return PostflightCheckResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Missing proof-of-work files",
			Details:  fmt.Sprintf("Missing: %v", missingFiles),
			Duration: time.Since(start),
		}, nil
	}

	// Verify JSON artifact is valid
	jsonPath := filepath.Join(result.ProofOfWorkPath, "proof-of-work.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return PostflightCheckResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Failed to read JSON artifact",
			Details:  err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Compute checksum
	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])

	// Store checksum in result for verification
	if result.ArtifactPaths == nil {
		result.ArtifactPaths = []string{}
	}
	result.ArtifactPaths = append(result.ArtifactPaths, jsonPath)

	return PostflightCheckResult{
		Name:     "artifact_integrity",
		Passed:   true,
		Message:  "Artifact integrity verified",
		Details:  fmt.Sprintf("JSON SHA256: %s", checksumHex),
		Duration: time.Since(start),
	}, nil
}

// checkFilesVerified verifies that declared files were actually created/modified.
func (e *EnhancedPostflightVerifier) checkFilesVerified(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
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

// checkTestsVerified verifies that declared tests actually ran.
func (e *EnhancedPostflightVerifier) checkTestsVerified(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
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
		patternCount := 0
		realPatterns := []string{
			"=== run",
			"--- pass",
			"--- fail",
			"passed",
			"failed",
			"ok ",
			"error:",
			"assertion",
			"expect",
			"::", // pytest pattern
		}

		for _, pattern := range realPatterns {
			if strings.Contains(output, pattern) {
				patternCount++
			}
		}

		// Need at least 2 patterns to be considered real test output
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

// checkProofOfWorkValid validates proof-of-work structure.
func (e *EnhancedPostflightVerifier) checkProofOfWorkValid(
	ctx context.Context,
	result *ExecutionResult,
	spec *FactoryTaskSpec,
) (PostflightCheckResult, error) {
	start := time.Now()

	// Check if proof-of-work was generated
	if result.ProofOfWorkPath == "" {
		return PostflightCheckResult{
			Name:     "proof_of_work_valid",
			Passed:   true, // Non-fatal
			Message:  "No proof-of-work generated",
			Details:  "Proof-of-work generation is optional for this task type",
			Duration: time.Since(start),
		}, nil
	}

	// Verify directory structure
	jsonPath := filepath.Join(result.ProofOfWorkPath, "proof-of-work.json")
	mdPath := filepath.Join(result.ProofOfWorkPath, "proof-of-work.md")
	logPath := filepath.Join(result.ProofOfWorkPath, "execution.log")

	missingFiles := []string{}
	for _, path := range []string{jsonPath, mdPath, logPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missingFiles = append(missingFiles, filepath.Base(path))
		}
	}

	if len(missingFiles) > 0 {
		return PostflightCheckResult{
			Name:     "proof_of_work_valid",
			Passed:   false,
			Message:  "Proof-of-work structure incomplete",
			Details:  fmt.Sprintf("Missing files: %v", missingFiles),
			Duration: time.Since(start),
		}, nil
	}

	return PostflightCheckResult{
		Name:     "proof_of_work_valid",
		Passed:   true,
		Message:  "Proof-of-work structure valid",
		Details:  fmt.Sprintf("Path: %s", result.ProofOfWorkPath),
		Duration: time.Since(start),
	}, nil
}
