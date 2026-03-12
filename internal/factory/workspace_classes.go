package factory

import (
	"path/filepath"
	"strings"
)

// WorkspaceClass represents the isolation and access control level of a workspace.
type WorkspaceClass string

const (
	// WorkspaceClassSandbox - Complete isolation, no git integration, no access to production codebases
	WorkspaceClassSandbox WorkspaceClass = "sandbox"

	// WorkspaceClassProtected - Restricted access to trusted codebases with controlled modifications
	WorkspaceClassProtected WorkspaceClass = "protected"

	// WorkspaceClassProduction - Full access for trusted operations with minimal restrictions
	WorkspaceClassProduction WorkspaceClass = "production"
)

// TrustLevel represents the level of trust for an agent or session.
type TrustLevel string

const (
	// TrustNone - No write access, read-only within workspace
	TrustNone TrustLevel = "trust:none"

	// TrustLow - Read/write within workspace, approval required for all writes
	TrustLow TrustLevel = "trust:low"

	// TrustMedium - Read/write in protected paths, approval required for writes to protected paths
	TrustMedium TrustLevel = "trust:medium"

	// TrustHigh - Read/write in protected paths, approval only for critical paths
	TrustHigh TrustLevel = "trust:high"

	// TrustFull - Full access including destructive ops, no approval needed
	TrustFull TrustLevel = "trust:full"
)

// TrustLevelValue returns a numeric value for comparison (0-4).
// Higher values represent more trust.
func TrustLevelValue(level TrustLevel) int {
	switch level {
	case TrustNone:
		return 0
	case TrustLow:
		return 1
	case TrustMedium:
		return 2
	case TrustHigh:
		return 3
	case TrustFull:
		return 4
	default:
		return 0 // Default to no trust
	}
}

// ProtectedRepoConfig defines protection rules for a repository.
type ProtectedRepoConfig struct {
	// Path is the absolute path to the repository
	Path string `yaml:"path"`

	// Class is the workspace class assigned to this repo
	Class WorkspaceClass `yaml:"class"`

	// AllowedPaths is an allowlist of paths that can be written to
	// Supports glob patterns: internal/**, src/saas/back/api/**
	AllowedPaths []string `yaml:"allowed_paths"`

	// ForbiddenPaths is an explicit denylist that blocks write access
	// Supports glob patterns: src/saas/front/**, infrastructure/**
	ForbiddenPaths []string `yaml:"forbidden_paths"`

	// CriticalPaths are high-risk paths requiring special approval
	// Examples: auth/, infrastructure/, secrets/
	CriticalPaths []string `yaml:"critical_paths"`
}

// TmpfsConfig defines tmpfs acceleration settings.
type TmpfsConfig struct {
	// Enabled activates tmpfs acceleration for eligible workspace classes
	Enabled bool `yaml:"enabled"`

	// MinMemoryMB is the minimum required RAM in MB to enable tmpfs
	MinMemoryMB int `yaml:"min_memory_mb"`

	// SafetyMargin is the headroom margin (0.0-1.0, e.g., 0.2 = 20%)
	SafetyMargin float64 `yaml:"safety_margin"`

	// UsageRatio is the percentage of available memory to use for tmpfs (0.0-1.0)
	UsageRatio float64 `yaml:"usage_ratio"`

	// EnabledClasses specifies which workspace classes can use tmpfs
	// Production class is excluded by default for safety (data loss risk)
	EnabledClasses []WorkspaceClass `yaml:"enabled_classes"`
}

// WorkspaceConfig contains all workspace-related configuration.
type WorkspaceConfig struct {
	// DefaultClass is the default workspace class for new workspaces
	DefaultClass WorkspaceClass `yaml:"default_class"`

	// DefaultTrustLevel is the default trust level for new sessions
	DefaultTrustLevel TrustLevel `yaml:"default_trust_level"`

	// ProtectedRepos defines protection rules for repositories
	ProtectedRepos []ProtectedRepoConfig `yaml:"protected_repos"`

	// Tmpfs defines tmpfs acceleration settings
	Tmpfs TmpfsConfig `yaml:"tmpfs"`
}

// DefaultWorkspaceConfig returns a default workspace configuration.
func DefaultWorkspaceConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		DefaultClass:      WorkspaceClassSandbox,
		DefaultTrustLevel: TrustLow,
		ProtectedRepos:     []ProtectedRepoConfig{},
		Tmpfs: TmpfsConfig{
			Enabled:      false,
			MinMemoryMB: 1024,
			SafetyMargin: 0.2,
			UsageRatio:   0.5,
			EnabledClasses: []WorkspaceClass{
				WorkspaceClassSandbox,
				WorkspaceClassProtected,
			},
		},
	}
}

// CanWriteToPath checks if a path can be written to based on trust level
// and protected repo configuration.
func CanWriteToPath(path string, trustLevel TrustLevel, protectedRepos []ProtectedRepoConfig) (bool, string) {
	trustValue := TrustLevelValue(trustLevel)

	// trust:none has no write access
	if trustValue == 0 {
		return false, "trust:none has no write access"
	}

	// Check if path is within any protected repo
	for _, repo := range protectedRepos {
		if isPathWithinRepo(path, repo.Path) {
			// Check forbidden paths first (denylist)
			for _, forbidden := range repo.ForbiddenPaths {
				if matchesGlobPattern(path, repo.Path, forbidden) {
					return false, "path is in forbidden list"
				}
			}

			// Check allowed paths (allowlist)
			if len(repo.AllowedPaths) > 0 {
				allowed := false
				for _, allowedPattern := range repo.AllowedPaths {
					if matchesGlobPattern(path, repo.Path, allowedPattern) {
						allowed = true
						break
					}
				}
				if !allowed {
					return false, "path is not in allowed list"
				}
			}

			// Check critical paths
			for _, critical := range repo.CriticalPaths {
				if matchesGlobPattern(path, repo.Path, critical) {
					// Critical paths require trust:high or higher
					if trustValue < 3 {
						return false, "critical path requires trust:high or higher"
					}
					// Even with high trust, still need approval
					if trustValue < 4 {
						return true, "critical path requires approval"
					}
					// trust:full can write without approval
					return true, ""
				}
			}

			// Non-critical protected paths
			if trustValue < 2 {
				return false, "protected path requires trust:medium or higher"
			}
			// trust:medium requires approval
			if trustValue < 3 {
				return true, "protected path requires approval"
			}
			// trust:high can write without approval
			return true, ""
		}
	}

	// Path is not in any protected repo - allow based on trust level
	if trustValue >= 1 {
		return true, ""
	}

	return false, "insufficient trust level"
}

// CanDeletePath checks if a path can be deleted based on trust level,
// workspace class, and protected repo configuration.
func CanDeletePath(path string, trustLevel TrustLevel, workspaceClass WorkspaceClass, protectedRepos []ProtectedRepoConfig) (bool, string) {
	trustValue := TrustLevelValue(trustLevel)

	// Check if path is within any protected repo
	for _, repo := range protectedRepos {
		if isPathWithinRepo(path, repo.Path) {
			// Cannot delete protected repo root
			if path == repo.Path {
				return false, "cannot delete protected repository root"
			}

			// Check critical paths
			for _, critical := range repo.CriticalPaths {
				if matchesGlobPattern(path, repo.Path, critical) {
					// Critical paths always require approval
					if trustValue < 4 {
						return false, "critical path deletion requires trust:full and approval"
					}
					return true, "critical path deletion requires approval"
				}
			}

			// Production class requires approval for any delete
			if workspaceClass == WorkspaceClassProduction && trustValue < 4 {
				return false, "production workspace deletion requires approval"
			}

			// Non-critical protected paths
			if trustValue < 3 {
				return false, "protected path deletion requires trust:high or higher"
			}
			// trust:high requires approval for non-critical paths
			if trustValue < 4 {
				return true, "protected path deletion requires approval"
			}
			return true, ""
		}
	}

	// Path is not in any protected repo - allow based on workspace class
	if workspaceClass == WorkspaceClassSandbox {
		return true, ""
	}

	// Protected and production classes require some trust
	if trustValue < 1 {
		return false, "insufficient trust level for deletion"
	}

	return true, ""
}

// isPathWithinRepo checks if a path is within a repository path.
func isPathWithinRepo(path, repoPath string) bool {
	rel, err := filepath.Rel(repoPath, path)
	if err != nil {
		return false
	}
	// Path is within repo if relative path doesn't start with ".."
	return !strings.HasPrefix(rel, "..")
}

// matchesGlobPattern checks if a path matches a glob pattern relative to a base path.
// Supports patterns like: internal/**, src/saas/back/api/**
func matchesGlobPattern(path, basePath, pattern string) bool {
	// Get relative path from base
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return false
	}

	// Simple glob matching for MVP
	// ** matches any number of directories
	// * matches any sequence within a directory
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(relPath, "/")

	for i, patternPart := range patternParts {
		if i >= len(pathParts) {
			return false
		}

		pathPart := pathParts[i]
		if patternPart == "**" {
			// ** matches remaining path
			return true
		}

		matched, _ := filepath.Match(patternPart, pathPart)
		if !matched {
			return false
		}
	}

	// Pattern exhausted but path has more parts
	return len(patternParts) == len(pathParts)
}

// WorkspaceClassSelector selects a workspace class based on trust level.
func SelectWorkspaceClass(trustLevel TrustLevel, defaultClass WorkspaceClass) WorkspaceClass {
	trustValue := TrustLevelValue(trustLevel)

	switch {
	case trustValue == 0:
		return WorkspaceClassSandbox
	case trustValue == 1:
		return WorkspaceClassSandbox
	case trustValue == 2:
		return WorkspaceClassProtected
	case trustValue == 3:
		return WorkspaceClassProtected
	case trustValue == 4:
		return WorkspaceClassProduction
	default:
		return defaultClass
	}
}

// CanUseTmpfs checks if a workspace class can use tmpfs acceleration.
func CanUseTmpfs(workspaceClass WorkspaceClass, tmpfsConfig TmpfsConfig) bool {
	if !tmpfsConfig.Enabled {
		return false
	}

	for _, enabledClass := range tmpfsConfig.EnabledClasses {
		if enabledClass == workspaceClass {
			return true
		}
	}

	return false
}
