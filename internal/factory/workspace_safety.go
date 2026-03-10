package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePathCanonical checks that a path is safe and canonical.
// It prevents path traversal, symlink attacks, and ensures paths are
// within the allowed root directory.
func validatePathCanonical(path, rootDir string) error {
	// Clean and make absolute
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cleanRoot := filepath.Clean(rootDir)
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute root: %w", err)
	}

	// Check for path traversal attempts
	relPath, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("path %s is outside root %s: %w", path, rootDir, err)
	}

	// Check if relative path starts with ".." (parent directory traversal)
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path %s attempts parent directory traversal: %s -> %s", path, relPath, absPath)
	}

	// Resolve symbolic links to prevent symlink-based attacks (only if path exists)
	resolvedPath := absPath
	if _, err := os.Stat(absPath); err == nil {
		// Path exists, resolve symlinks
		resolvedPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve symlinks for %s: %w", path, err)
		}

		// Double-check containment after symlink resolution
		resolvedRel, err := filepath.Rel(absRoot, resolvedPath)
		if err != nil {
			return fmt.Errorf("resolved path %s is outside root %s (via symlink): %w", resolvedPath, rootDir, err)
		}

		if strings.HasPrefix(resolvedRel, "..") {
			return fmt.Errorf("resolved path %s attempts parent directory traversal (via symlink): %s", resolvedPath, resolvedRel)
		}

		// Prevent deleting the root directory itself
		if resolvedPath == absRoot {
			return fmt.Errorf("refusing to delete root directory: %s", absRoot)
		}

		// Ensure path is a subdirectory of root (not root itself)
		if filepath.Dir(resolvedPath) == absRoot && filepath.Base(resolvedPath) != filepath.Base(absPath) {
			return fmt.Errorf("path %s is not a direct subdirectory of root %s", path, rootDir)
		}
	} else {
		// Path doesn't exist yet (e.g., during CreateWorkspace)
		// Still check that it wouldn't be the root directory
		if absPath == absRoot {
			return fmt.Errorf("refusing to create workspace at root directory: %s", absRoot)
		}
	}

	return nil
}

// validateWorkspaceOwnership checks if the workspace belongs to the expected
// task/session by verifying directory name structure.
func validateWorkspaceOwnership(path, expectedTaskID, expectedSessionID string) error {
	// Clean path
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract directory name
	base := filepath.Base(absPath)

	// For MVP, we only verify path is not empty
	if base == "" || base == "." || base == "/" {
		return fmt.Errorf("invalid workspace path: %s", path)
	}

	// In production, verify taskID/sessionID match expected pattern
	// Expected pattern: .../workspaces/{sessionID}/{taskID}
	// Extract last two components
	dir := filepath.Dir(absPath)
	sessionDir := filepath.Base(dir)
	workspacesDir := filepath.Dir(dir)

	// Check if parent directory name matches expected session ID
	if sessionDir != expectedSessionID {
		return fmt.Errorf("workspace session ID mismatch: path contains %s, expected %s", sessionDir, expectedSessionID)
	}

	// Check if workspace directory name matches expected task ID
	if base != expectedTaskID {
		return fmt.Errorf("workspace task ID mismatch: path contains %s, expected %s", base, expectedTaskID)
	}

	// Verify workspaces directory name is "workspaces" (optional, but good safety)
	if filepath.Base(workspacesDir) != "workspaces" {
		// Not a hard requirement, but log warning
		// Continue anyway
	}

	return nil
}

// isPathWithinRoot checks if path is within root directory.
func isPathWithinRoot(path, root string) (bool, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return false, err
	}

	// Path is within root if relative path doesn't start with ".."
	return !strings.HasPrefix(relPath, ".."), nil
}
