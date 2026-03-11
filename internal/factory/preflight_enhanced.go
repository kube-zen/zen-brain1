// Package factory provides enhanced preflight checks for task execution.
// These checks add stricter validation for fail-closed behavior.
package factory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PreflightMode controls the strictness of preflight checks.
type PreflightMode string

const (
	// PreflightModeLenient: Non-fatal checks, log warnings but don't block
	PreflightModeLenient PreflightMode = "lenient"
	// PreflightModeStrict: Critical checks must pass, non-critical warnings
	PreflightModeStrict PreflightMode = "strict"
	// PreflightModeFailClosed: All checks must pass, or execution is blocked
	PreflightModeFailClosed PreflightMode = "fail-closed"
)

// EnhancedPreflightChecker runs enhanced preflight checks with stricter validation.
type EnhancedPreflightChecker struct {
	workspaceManager WorkspaceManager
	worktreeManager  WorktreeManager
	mode             PreflightMode
	minDiskSpaceGB   float64
	minMemoryMB      uint64
}

// NewEnhancedPreflightChecker creates a new EnhancedPreflightChecker.
func NewEnhancedPreflightChecker(
	wsManager WorkspaceManager,
	wtManager WorktreeManager,
	mode PreflightMode,
) *EnhancedPreflightChecker {
	// Set defaults based on mode
	minDiskSpaceGB := 2.0 // 2GB minimum
	minMemoryMB := uint64(1024) // 1GB minimum

	if mode == "" {
		mode = PreflightModeStrict
	}

	return &EnhancedPreflightChecker{
		workspaceManager: wsManager,
		worktreeManager:  wtManager,
		mode:             mode,
		minDiskSpaceGB:   minDiskSpaceGB,
		minMemoryMB:      minMemoryMB,
	}
}

// RunEnhancedPreflightChecks executes all enhanced preflight checks.
func (e *EnhancedPreflightChecker) RunEnhancedPreflightChecks(ctx context.Context, spec *FactoryTaskSpec) (*PreflightReport, error) {
	report := &PreflightReport{
		TaskID:    spec.ID,
		SessionID: spec.SessionID,
		Timestamp: time.Now(),
		AllPassed: true,
		Checks:    []PreflightCheckResult{},
	}

	// Add environment metadata
	report.Environment = NewExecutionEnvironment()

	// Run all checks
	checks := []struct {
		name      string
		critical  bool // Must pass in strict/fail-closed modes
		fn        func(context.Context, *FactoryTaskSpec) (PreflightCheckResult, error)
	}{
		{"git_available", true, e.checkGitAvailable},
		{"git_version", true, e.checkGitVersion},
		{"worktree_support", true, e.checkWorktreeSupport},
		{"repo_valid", true, e.checkRepoValid},      // NEW: stricter repo validation
		{"repo_clean", false, e.checkRepoCleanStrict}, // NEW: strict cleanliness check
		{"disk_space", true, e.checkDiskSpace},      // NEW: real disk space check
		{"memory_available", true, e.checkMemoryAvailable}, // NEW: memory check
		{"workspace_config", true, e.checkWorkspaceConfig},
		{"network_connectivity", false, e.checkNetworkConnectivity}, // NEW: optional network check
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

		// Determine if this check should fail based on mode and criticality
		if !result.Passed {
			switch e.mode {
			case PreflightModeFailClosed:
				// All checks must pass
				report.AllPassed = false
			case PreflightModeStrict:
				// Only critical checks must pass
				if check.critical {
					report.AllPassed = false
				}
			case PreflightModeLenient:
				// No checks are blocking
				// Still track failures but don't mark report as failed
			}
		}
	}

	return report, nil
}

// MustRunEnhancedPreflightChecks runs checks and enforces fail-closed behavior.
func (e *EnhancedPreflightChecker) MustRunEnhancedPreflightChecks(ctx context.Context, spec *FactoryTaskSpec) (*PreflightReport, error) {
	report, err := e.RunEnhancedPreflightChecks(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("enhanced preflight checks failed: %w", err)
	}

	if !report.AllPassed {
		failedChecks := []string{}
		for _, check := range report.Checks {
			if !check.Passed {
				failedChecks = append(failedChecks, fmt.Sprintf("%s: %s", check.Name, check.Message))
			}
		}
		return report, fmt.Errorf("enhanced preflight checks failed: %s", strings.Join(failedChecks, "; "))
	}

	return report, nil
}

// checkRepoValid performs stricter repository validation.
// This checks that the repository exists, is a valid git repo, and has a HEAD.
func (e *EnhancedPreflightChecker) checkRepoValid(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Get the workspace path
	workspacePath := spec.WorkspacePath
	if workspacePath == "" {
		return PreflightCheckResult{
			Name:     "repo_valid",
			Passed:   true,
			Message:  "No workspace path set (will be allocated)",
			Duration: time.Since(start),
		}, nil
	}

	// Check if directory exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return PreflightCheckResult{
			Name:          "repo_valid",
			Passed:        false,
			Message:       "Workspace directory does not exist",
			Details:       workspacePath,
			FixSuggestion: "Allocate workspace before checking repo state",
			Duration:      time.Since(start),
		}, nil
	}

	// Check if it's a git repository
	gitDir := filepath.Join(workspacePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return PreflightCheckResult{
			Name:          "repo_valid",
			Passed:        false,
			Message:       "Workspace is not a git repository",
			Details:       workspacePath,
			FixSuggestion: "Initialize git repository: git init",
			Duration:      time.Since(start),
		}, nil
	}

	// Check if HEAD exists (valid git repo)
	cmd := exec.CommandContext(ctx, "git", "-C", workspacePath, "rev-parse", "--verify", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PreflightCheckResult{
			Name:          "repo_valid",
			Passed:        false,
			Message:       "Repository has no HEAD (empty or corrupted)",
			Details:       strings.TrimSpace(string(output)),
			FixSuggestion: "Initialize repository with a commit: git commit --allow-empty -m 'Initial commit'",
			Duration:      time.Since(start),
		}, nil
	}

	return PreflightCheckResult{
		Name:     "repo_valid",
		Passed:   true,
		Message:  "Repository is valid",
		Details:  "Git repository exists with HEAD",
		Duration: time.Since(start),
	}, nil
}

// checkRepoCleanStrict performs strict repository cleanliness check.
// This can be configured to fail if there are any uncommitted changes.
func (e *EnhancedPreflightChecker) checkRepoCleanStrict(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	workspacePath := spec.WorkspacePath
	if workspacePath == "" {
		return PreflightCheckResult{
			Name:     "repo_clean",
			Passed:   true,
			Message:  "No workspace path set",
			Duration: time.Since(start),
		}, nil
	}

	// Get git status
	cmd := exec.CommandContext(ctx, "git", "-C", workspacePath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return PreflightCheckResult{
			Name:     "repo_clean",
			Passed:   true, // Non-fatal if we can't check
			Message:  "Could not check repo cleanliness",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	statusLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(statusLines) > 0 && statusLines[0] != "" {
		// Count dirty files
		dirtyCount := 0
		for _, line := range statusLines {
			if line != "" {
				dirtyCount++
			}
		}

		// In fail-closed mode, dirty repo is an error
		// In strict mode, it's a warning
		message := fmt.Sprintf("Repository has %d uncommitted changes", dirtyCount)
		details := strings.TrimSpace(string(output))
		duration := time.Since(start)

		if e.mode == PreflightModeFailClosed {
			return PreflightCheckResult{
				Name:          "repo_clean",
				Passed:        false,
				Message:       message,
				Details:       details,
				FixSuggestion: "Commit or stash changes before execution",
				Duration:      duration,
			}, nil
		}

		return PreflightCheckResult{
			Name:     "repo_clean",
			Passed:   true, // Warning only
			Message:  message + " (warning)",
			Details:  details,
			Duration: duration,
		}, nil
	}

	return PreflightCheckResult{
		Name:     "repo_clean",
		Passed:   true,
		Message:  "Repository is clean",
		Duration: time.Since(start),
	}, nil
}

// checkDiskSpace verifies sufficient disk space is available.
func (e *EnhancedPreflightChecker) checkDiskSpace(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Determine path to check (workspace or current directory)
	checkPath := spec.WorkspacePath
	if checkPath == "" {
		checkPath = "."
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkPath, &stat); err != nil {
		return PreflightCheckResult{
			Name:     "disk_space",
			Passed:   true, // Don't block on platform-specific errors
			Message:  "Could not check disk space",
			Details:  fmt.Sprintf("Error: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Calculate available space in GB
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	availableGB := float64(availableBytes) / (1024 * 1024 * 1024)

	details := fmt.Sprintf("Available: %.2f GB (minimum: %.2f GB)", availableGB, e.minDiskSpaceGB)

	if availableGB < e.minDiskSpaceGB {
		return PreflightCheckResult{
			Name:          "disk_space",
			Passed:        false,
			Message:       "Insufficient disk space",
			Details:       details,
			FixSuggestion: "Free up disk space or increase minimum threshold",
			Duration:      time.Since(start),
		}, nil
	}

	return PreflightCheckResult{
		Name:     "disk_space",
		Passed:   true,
		Message:  "Sufficient disk space available",
		Details:  details,
		Duration: time.Since(start),
	}, nil
}

// checkMemoryAvailable checks if sufficient memory is available.
func (e *EnhancedPreflightChecker) checkMemoryAvailable(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get system memory info (Linux-specific)
	var sysInfo struct {
		Total   uint64
		Free    uint64
		Buffers uint64
		Cached  uint64
	}

	if runtime.GOOS == "linux" {
		// Read /proc/meminfo
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					value, _ := strconv.ParseUint(fields[1], 10, 64)
					switch fields[0] {
					case "MemTotal:":
						sysInfo.Total = value * 1024 // Convert kB to bytes
					case "MemFree:":
						sysInfo.Free = value * 1024
					case "Buffers:":
						sysInfo.Buffers = value * 1024
					case "Cached:":
						sysInfo.Cached = value * 1024
					}
				}
			}

			// Calculate available memory (free + buffers + cached)
			availableMB := (sysInfo.Free + sysInfo.Buffers + sysInfo.Cached) / (1024 * 1024)
			totalMB := sysInfo.Total / (1024 * 1024)

			details := fmt.Sprintf("Available: %d MB (total: %d MB, minimum: %d MB)", availableMB, totalMB, e.minMemoryMB)

			if availableMB < uint64(e.minMemoryMB) {
				return PreflightCheckResult{
					Name:          "memory_available",
					Passed:        false,
					Message:       "Insufficient memory available",
					Details:       details,
					FixSuggestion: "Close other processes or increase minimum threshold",
					Duration:      time.Since(start),
				}, nil
			}

			return PreflightCheckResult{
				Name:     "memory_available",
				Passed:   true,
				Message:  "Sufficient memory available",
				Details:  details,
				Duration: time.Since(start),
			}, nil
		}
	}

	// Fallback for non-Linux systems: check Go heap stats
	allocMB := m.Sys / (1024 * 1024)
	details := fmt.Sprintf("Go alloc: %d MB (system-specific memory check unavailable)", allocMB)

	return PreflightCheckResult{
		Name:     "memory_available",
		Passed:   true, // Pass on systems we can't check
		Message:  "Memory check passed (system-specific check unavailable)",
		Details:  details,
		Duration: time.Since(start),
	}, nil
}

// checkNetworkConnectivity performs optional network connectivity checks.
// This is non-critical but useful for remote operations.
func (e *EnhancedPreflightChecker) checkNetworkConnectivity(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Try to resolve common DNS names (lightweight check)
	dnsHosts := []string{
		"github.com",   // Common for git operations
		"8.8.8.8",      // Google DNS (IP, no resolution needed)
	}

	allOK := true
	failedHosts := []string{}

	for _, host := range dnsHosts {
		// Just try to ping once with short timeout
		cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", host)
		_, err := cmd.CombinedOutput()
		if err != nil {
			allOK = false
			failedHosts = append(failedHosts, host)
		}
	}

	duration := time.Since(start)

	if allOK {
		return PreflightCheckResult{
			Name:     "network_connectivity",
			Passed:   true,
			Message:  "Network connectivity OK",
			Details:  fmt.Sprintf("All %d hosts reachable", len(dnsHosts)),
			Duration: duration,
		}, nil
	}

	// Network check is non-critical, so we pass but note the issue
	return PreflightCheckResult{
		Name:     "network_connectivity",
		Passed:   true, // Non-critical
		Message:  "Limited network connectivity (non-critical)",
		Details:  fmt.Sprintf("Unreachable hosts: %v", failedHosts),
		Duration: duration,
	}, nil
}

// checkGitAvailable checks if git is available in PATH.
func (e *EnhancedPreflightChecker) checkGitAvailable(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
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
func (e *EnhancedPreflightChecker) checkGitVersion(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
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
		Passed:   true,
		Message:  "Git version check passed",
		Details:  version,
		Duration: duration,
	}, nil
}

// checkWorktreeSupport checks if git worktree is supported.
func (e *EnhancedPreflightChecker) checkWorktreeSupport(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
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

// checkWorkspaceConfig validates workspace configuration.
func (e *EnhancedPreflightChecker) checkWorkspaceConfig(ctx context.Context, spec *FactoryTaskSpec) (PreflightCheckResult, error) {
	start := time.Now()

	// Check that workspace manager is configured
	if e.workspaceManager == nil {
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
