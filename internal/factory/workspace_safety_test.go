package factory

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidatePathCanonical_PathTraversal attempts to delete parent directory
func TestValidatePathCanonical_PathTraversal(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspaces")
	os.MkdirAll(workspaceDir, 0755)

	// Try to validate path that traverses to parent
	parentPath := filepath.Join(workspaceDir, "..", "parent")
	err := validatePathCanonical(parentPath, workspaceDir)
	if err == nil {
		t.Errorf("Expected error for parent directory traversal, got nil")
	}
	t.Logf("Correctly rejected parent traversal: %v", err)
}

// TestValidatePathCanonical_SymlinkAttack attempts to escape via symlink
func TestValidatePathCanonical_SymlinkAttack(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspaces")
	os.MkdirAll(workspaceDir, 0755)

	// Create target directory outside workspaces
	outsideDir := filepath.Join(tmpDir, "outside")
	os.MkdirAll(outsideDir, 0755)

	// Create symlink inside workspaces pointing outside
	symlinkPath := filepath.Join(workspaceDir, "escape-link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks on this system: %v", err)
		return
	}

	// Try to validate symlink path
	err := validatePathCanonical(symlinkPath, workspaceDir)
	if err == nil {
		t.Errorf("Expected error for symlink escape, got nil")
	}
	t.Logf("Correctly rejected symlink escape: %v", err)
}

// TestValidatePathCanonical_DeleteRoot attempts to delete root directory
func TestValidatePathCanonical_DeleteRoot(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspaces")
	os.MkdirAll(workspaceDir, 0755)

	// Try to validate root directory itself
	err := validatePathCanonical(workspaceDir, workspaceDir)
	if err == nil {
		t.Errorf("Expected error for deleting root directory, got nil")
	}
	t.Logf("Correctly rejected root directory deletion: %v", err)
}

// TestValidatePathCanonical_ValidPath accepts valid workspace path
func TestValidatePathCanonical_ValidPath(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspaces")
	os.MkdirAll(workspaceDir, 0755)

	// Create valid workspace
	validPath := filepath.Join(workspaceDir, "session-123", "task-456")
	if err := os.MkdirAll(validPath, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	// Validate valid path
	err := validatePathCanonical(validPath, workspaceDir)
	if err != nil {
		t.Errorf("Expected nil error for valid path, got: %v", err)
	}
}

// TestIsPathWithinRoot checks containment logic
func TestIsPathWithinRoot(t *testing.T) {
	root := "/home/user/workspaces"

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "path within root",
			path:     "/home/user/workspaces/session/task",
			expected: true,
		},
		{
			name:     "path is root itself",
			path:     "/home/user/workspaces",
			expected: true,
		},
		{
			name:     "path outside root (parent)",
			path:     "/home/user",
			expected: false,
		},
		{
			name:     "path outside root (sibling)",
			path:     "/home/user/other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := isPathWithinRoot(tt.path, root)
			if err != nil {
				t.Errorf("isPathWithinRoot(%s, %s) returned error: %v", tt.path, root, err)
				return
			}
			if result != tt.expected {
				t.Errorf("isPathWithinRoot(%s, %s) = %v, want %v", tt.path, root, result, tt.expected)
			}
		})
	}
}

// TestWorkspaceManager_DeleteWorkspace_Safety checks safety guards in deletion
func TestWorkspaceManager_DeleteWorkspace_Safety(t *testing.T) {
	manager := NewWorkspaceManager(t.TempDir())

	// Create a workspace
	metadata, err := manager.CreateWorkspace(nil, "task-123", "session-456")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Try to delete parent directory (should fail)
	parentPath := filepath.Dir(filepath.Dir(metadata.Path))
	err = manager.DeleteWorkspace(nil, parentPath)
	if err == nil {
		t.Errorf("Expected error when trying to delete parent directory, got nil")
	}
	t.Logf("Correctly rejected parent directory deletion: %v", err)
}

// TestWorkspaceManager_DeleteWorkspace_Valid checks valid deletion works
func TestWorkspaceManager_DeleteWorkspace_Valid(t *testing.T) {
	manager := NewWorkspaceManager(t.TempDir())

	// Create a workspace
	metadata, err := manager.CreateWorkspace(nil, "task-123", "session-456")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Verify workspace exists
	if _, err := os.Stat(metadata.Path); os.IsNotExist(err) {
		t.Fatalf("Workspace was not created: %s", metadata.Path)
	}

	// Delete workspace
	err = manager.DeleteWorkspace(nil, metadata.Path)
	if err != nil {
		t.Errorf("Failed to delete workspace: %v", err)
	}

	// Verify workspace is deleted
	if _, err := os.Stat(metadata.Path); !os.IsNotExist(err) {
		t.Errorf("Workspace still exists after deletion: %s", metadata.Path)
	}
}
