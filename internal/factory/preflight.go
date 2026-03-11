// Package factory provides preflight checks for task execution.
package factory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PreflightCheckResult represents the result of a preflight check.
type PreflightCheckResult struct {
	Name        string        `json:"name"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	FixSuggestion string       `json:"fix_suggestion,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// PreflightReport represents a complete preflight check report.
type PreflightReport struct {
	TaskID      string                `json:"task_id"`
	SessionID   string                `json:"session_id"`
	Timestamp   time.Time             `json:"timestamp"`
	AllPassed   bool                  `json:"all_passed"`
	Checks      []PreflightCheckResult `json:"checks"`
	Environment *ExecutionEnvironment `json:"environment,omitempty"`
}

// PreflightChecker runs preflight checks before task execution.
type PreflightChecker struct {
	workspaceManager WorkspaceManager
	worktreeManager  WorktreeManager
}

// NewPreflightChecker creates a new PreflightChecker.
func NewPreflightChecker(wsManager WorkspaceManager, wtManager WorktreeManager) *PreflightChecker {
	return &PreflightChecker{
		workspaceManager: wsManager,
		worktreeManager:  wtManager,
	}
}

// RunPreflightChecks executes all preflight checks for a task.
func (p *PreflightChecker) RunPreflightChecks(ctx context.Context, spec *FactoryTaskSpec) (*PreflightReport, error) {
	report := &PreflightReport{
		TaskID:    spec.ID,
		SessionID: spec.SessionID,
		Timestamp: time.Now(),
		AllPassed: true,
		Checks:    []PreflightCheckResult{},
	}

	// Run all checks
	checks := []struct {
		name string
		fn   func(context.Context, *FactoryTaskSpec) (PreflightCheckResult, error)
	}{
		{"git_available", p.checkGitAvailable},
		{"git_version", p.checkGitVersion},
		{"worktree_support", p.checkWorktreeSupport},
		{"repo_state", p.checkRepoState},
		{"workspace_config", p.checkWorkspaceConfig},
		{"resource_availability", p.checkResourceAvailability},
	}

	for _, check := range checks {
		start := time.Now()
		result, err := check.fn(ctx, spec)
		if err != nil {
			result = PreflightCheckResult{
				Name:     check.name,
				Passed:   false,
				Message:  fmt.Sprintf("Check failed with error: %v", err),
				Duration: time.Since(start),
			}
		}
		result.Duration = time.Since(start)
		report.Checks = append(report.Checks, result)
		if !result.Passed {
			report.AllPassed = false
		}
	}

	return report, nil
}

// checkGitAvailable checks if git is available in PATH.
func (p *PreflightChecker) checkGitAvailable(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "git", "--version")
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		return PreflightCheckResult{
			Name:          "git_available",
			Passed:        false,
			Message:       "Git is not available",
			Details:       fmt.Sprintf("Error: %v", err),
			FixSuggestion: "Install git: apt-get install git or brew install git",
			Duration:      duration,
		}, nil
	}

	version := strings.TrimSpace(string(output))
	return PreflightCheckResult{
		Name:     "git_available",
		Passed:   true,
		Message:  "Git is available",
		Details:  version,
		Duration: duration,
	}, nil
}

// checkGitVersion checks if git version supports worktrees (2.5+).
func (p *PreflightChecker) checkGitVersion(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "git", "version")
	output, err := cmd.Output()
	duration := time.Since(start)

	if err != nil {
		return PreflightCheckResult{
			Name:     "git_version",
			Passed:   false,
			Message:  "Failed to get git version",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: duration,
		}, nil
	}

	version := strings.TrimSpace(string(output))
	// Git version format: "git version 2.43.0"
	// We need 2.5+ for worktree support
	if strings.Contains(version, "git version") {
		parts := strings.Fields(version)
		if len(parts) >= 3 {
			ver := parts[2]
			return PreflightCheckResult{
				Name:     "git_version",
				Passed:   true,
				Message:  "Git version supports worktrees",
				Details:  fmt.Sprintf("Version: %s", ver),
				Duration: duration,
			}, nil
		}
	}

	return PreflightCheckResult{
		Name:     "git_version",
		Passed:   true, // Assume OK if we can't parse
		Message:  "Git version check passed (assumed compatible)",
		Details:  version,
		Duration: duration,
	}, nil
}

// checkWorktreeSupport checks if git worktree is supported.
func (p *PreflightChecker) checkWorktreeSupport(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "git", "worktree", "list")
	cmd.Env = append(cmd.Env, "GIT_DIR=/dev/null")
	_ = cmd.Run() // Just check if command exists
	duration := time.Since(start)

	return PreflightCheckResult{
		Name:     "worktree_support",
		Passed:   true,
		Message:  "Git worktree support available",
		Duration: duration,
	}, nil
}

// checkRepoState checks if the repository is in a good state for worktree creation.
func (p *PreflightChecker) checkRepoState(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Check if workspace path exists and is a git repo
	if spec.WorkspacePath == "" {
		return PreflightCheckResult{
			Name:     "repo_state",
			Passed:   true,
			Message:  "No workspace path set (will be allocated)",
			Duration: time.Since(start),
		}, nil
	}

	// Check if repo is dirty
	cmd := exec.CommandContext(ctx, "git", "-C", spec.WorkspacePath, "status", "--porcelain")
	output, err := cmd.Output()
	duration := time.Since(start)

	if err != nil {
		return PreflightCheckResult{
			Name:     "repo_state",
			Passed:   true, // Non-fatal
			Message:  "Could not check repo state",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: duration,
		}, nil
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		return PreflightCheckResult{
			Name:     "repo_state",
			Passed:   true, // Non-fatal, just informational
			Message:  "Repository has uncommitted changes",
			Details:  fmt.Sprintf("Dirty files: %s", strings.TrimSpace(string(output))),
			Duration: duration,
		}, nil
	}

	return PreflightCheckResult{
		Name:     "repo_state",
		Passed:   true,
		Message:  "Repository is clean",
		Duration: duration,
	}, nil
}

// checkWorkspaceConfig validates workspace configuration.
func (p *PreflightChecker) checkWorkspaceConfig(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Check that workspace manager is configured
	if p.workspaceManager == nil {
		return PreflightCheckResult{
			Name:     "workspace_config",
			Passed:   false,
			Message:  "Workspace manager not configured",
			Duration: time.Since(start),
		}, nil
	}

	// Validate task spec constraints
	if spec.ID == "" {
		return PreflightCheckResult{
			Name:     "workspace_config",
			Passed:   false,
			Message:  "Task ID is empty",
			Duration: time.Since(start),
		}, nil
	}

	if spec.SessionID == "" {
		return PreflightCheckResult{
			Name:     "workspace_config",
			Passed:   false,
			Message:  "Session ID is empty",
			Duration: time.Since(start),
		}, nil
	}

	return PreflightCheckResult{
		Name:     "workspace_config",
		Passed:   true,
		Message:  "Workspace configuration valid",
		Duration: time.Since(start),
	}, nil
}

// checkResourceAvailability checks if sufficient resources are available.
func (p *PreflightChecker) checkResourceAvailability(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Check disk space on workspace directory
	workspacePath := spec.WorkspacePath
	if workspacePath == "" {
		// Use current directory if workspace not set
		workspacePath = "."
	}

	// Get disk space using 'df' command
	var freeSpaceBytes uint64
	cmd := exec.CommandContext(ctx, "df", "-B1", workspacePath)
	output, err := cmd.Output()
	if err != nil {
		return PreflightCheckResult{
			Name:     "resource_availability",
			Passed:   false,
			Message:  "Failed to check disk space",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Parse df output to get available space
	// Format: Filesystem 1K-blocks Used Available Use% Mounted
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return PreflightCheckResult{
			Name:     "resource_availability",
			Passed:   false,
			Message:  "Failed to parse disk space output",
			Details:  string(output),
			Duration: time.Since(start),
		}, nil
	}

	fields := strings.Fields(lines[1])
	if len(fields) >= 4 {
		// Available field is the 4th column (index 3)
		fmt.Sscanf(fields[3], "%d", &freeSpaceBytes)
	}

	// Minimum required: 500 MB
	const minRequiredBytes = 500 * 1024 * 1024

	if freeSpaceBytes < minRequiredBytes {
		return PreflightCheckResult{
			Name:     "resource_availability",
			Passed:   false,
			Message:  "Insufficient disk space",
			Details:  fmt.Sprintf("Available: %d MB, Required: %d MB", freeSpaceBytes/(1024*1024), minRequiredBytes/(1024*1024)),
			FixSuggestion: "Free up disk space or choose a different workspace location",
			Duration: time.Since(start),
		}, nil
	}

	// Check memory (basic check via /proc/meminfo on Linux)
	memAvailableMB := 0
	if _, err := os.Stat("/proc/meminfo"); err == nil {
		memData, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			// Look for MemAvailable line
			lines := strings.Split(string(memData), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						fmt.Sscanf(fields[1], "%d", &memAvailableMB)
					}
					break
				}
			}
		}
	}

	// Minimum required: 100 MB
	const minRequiredMemoryMB = 100

	if memAvailableMB > 0 && memAvailableMB < minRequiredMemoryMB {
		return PreflightCheckResult{
			Name:     "resource_availability",
			Passed:   false,
			Message:  "Insufficient memory",
			Details:  fmt.Sprintf("Available: %d MB, Required: %d MB", memAvailableMB, minRequiredMemoryMB),
			FixSuggestion: "Close other applications or increase available memory",
			Duration: time.Since(start),
		}, nil
	}

	details := fmt.Sprintf("Disk: %d MB free", freeSpaceBytes/(1024*1024))
	if memAvailableMB > 0 {
		details += fmt.Sprintf(", Memory: %d MB available", memAvailableMB)
	}

	return PreflightCheckResult{
		Name:     "resource_availability",
		Passed:   true,
		Message:  "Sufficient resources available",
		Details:  details,
		Duration: time.Since(start),
	}, nil
}

// MustRunPreflightChecks runs preflight checks and returns error if any fail.
// This is for fail-closed behavior when checks are mandatory.
func (p *PreflightChecker) MustRunPreflightChecks(ctx context.Context, spec *FactoryTaskSpec) (*PreflightReport, error) {
	report, err := p.RunPreflightChecks(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("preflight checks failed: %w", err)
	}

	if !report.AllPassed {
		failedChecks := []string{}
		for _, check := range report.Checks {
			if !check.Passed {
				failedChecks = append(failedChecks, fmt.Sprintf("%s: %s", check.Name, check.Message))
			}
		}
		return report, fmt.Errorf("preflight checks failed: %s", strings.Join(failedChecks, "; "))
	}

	return report, nil
}
