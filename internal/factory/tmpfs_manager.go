package factory

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// TmpfsManager handles tmpfs mounting and unmounting for workspace acceleration.
type TmpfsManager struct {
	config *WorkspaceConfig
}

// NewTmpfsManager creates a new tmpfs manager.
func NewTmpfsManager(config *WorkspaceConfig) *TmpfsManager {
	return &TmpfsManager{
		config: config,
	}
}

// MountForWorkspace creates a tmpfs mount for the given workspace path if enabled and sufficient memory is available.
// Returns (tmpfsPath, sizeMB, cleanupFunc, error).
func (tm *TmpfsManager) MountForWorkspace(workspacePath string, workspaceClass WorkspaceClass) (string, int, func(), error) {
	// Check if tmpfs is enabled
	if !tm.config.Tmpfs.Enabled {
		return "", 0, func() {}, nil
	}

	// Check if workspace class can use tmpfs
	if !CanUseTmpfs(workspaceClass, tm.config.Tmpfs) {
		return "", 0, func() {}, nil
	}

	// Get available memory
	availableMemMB, err := tm.getAvailableMemoryMB()
	if err != nil {
		return "", 0, func() {}, fmt.Errorf("failed to get available memory: %w", err)
	}

	// Check minimum memory requirement
	if availableMemMB < tm.config.Tmpfs.MinMemoryMB {
		return "", 0, func() {}, fmt.Errorf("insufficient memory for tmpfs: %dMB available, %dMB required",
			availableMemMB, tm.config.Tmpfs.MinMemoryMB)
	}

	// Apply safety margin and calculate tmpfs size
	usableMemMB := float64(availableMemMB) * (1.0 - tm.config.Tmpfs.SafetyMargin)
	tmpfsSizeMB := int(usableMemMB * tm.config.Tmpfs.UsageRatio)

	// Warn if memory is marginal
	if availableMemMB < tm.config.Tmpfs.MinMemoryMB*2 {
		fmt.Printf("[TmpfsManager] Warning: Marginal memory (%dMB) for tmpfs (%dMB)\n",
			availableMemMB, tmpfsSizeMB)
	}

	// Create tmpfs mount
	tmpfsPath, err := tm.mountTmpfs(workspacePath, tmpfsSizeMB)
	if err != nil {
		return "", 0, func() {}, fmt.Errorf("failed to mount tmpfs: %w", err)
	}

	cleanup := func() {
		tm.umountTmpfs(tmpfsPath)
	}

	return tmpfsPath, tmpfsSizeMB, cleanup, nil
}

// getAvailableMemoryMB returns available system memory in MB.
func (tm *TmpfsManager) getAvailableMemoryMB() (int, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get total system memory from /proc/meminfo (Linux) or similar
	// For cross-platform, use a simple heuristic
	totalMemMB := int(m.Sys / 1024 / 1024) // Convert bytes to MB

	// Available = Total - Used - Safety margin
	usedMemMB := int(m.Alloc / 1024 / 1024)
	availableMemMB := totalMemMB - usedMemMB

	if availableMemMB < 0 {
		availableMemMB = 0
	}

	return availableMemMB, nil
}

// mountTmpfs creates and mounts a tmpfs at the given path.
func (tm *TmpfsManager) mountTmpfs(workspacePath string, sizeMB int) (string, error) {
	// Create mount point if it doesn't exist
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create tmpfs mount point: %w", err)
	}

	// Mount tmpfs
	// On Linux: mount -t tmpfs -o size=<size>M tmpfs <mount_point>
	// On other platforms: skip (tmpfs is Linux-only)
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("tmpfs is Linux-only (current OS: %s)", runtime.GOOS)
	}

	sizeStr := fmt.Sprintf("%dM", sizeMB)
	cmd := exec.Command("mount", "-t", "tmpfs", "-o", "size="+sizeStr, "zen-tmpfs", workspacePath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to mount tmpfs: %w", err)
	}

	return workspacePath, nil
}

// umountTmpfs unmounts a tmpfs at the given path.
func (tm *TmpfsManager) umountTmpfs(path string) error {
	if runtime.GOOS != "linux" {
		// Not a tmpfs mount
		return nil
	}

	cmd := exec.Command("umount", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to umount tmpfs: %w", err)
	}

	return nil
}
