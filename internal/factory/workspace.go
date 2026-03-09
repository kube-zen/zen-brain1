package factory

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// staleLockThreshold defines how old a lock file can be before considered stale.
const staleLockThreshold = 1 * time.Hour

// staleWorkspaceThreshold defines how old a workspace directory can be before considered stale.
const staleWorkspaceThreshold = 24 * time.Hour

// WorkspaceManagerImpl implements WorkspaceManager.
type WorkspaceManagerImpl struct {
	homeDir       string
	workspacesDir  string
	lockMap        map[string]*sync.Mutex
	lockMapMutex   sync.RWMutex
}

// NewWorkspaceManager creates a new workspace manager.
func NewWorkspaceManager(homeDir string) *WorkspaceManagerImpl {
	workspacesDir := filepath.Join(homeDir, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create workspaces directory: %v", err))
	}
	return &WorkspaceManagerImpl{
		homeDir:      homeDir,
		workspacesDir: workspacesDir,
		lockMap:       make(map[string]*sync.Mutex),
	}
}

func (w *WorkspaceManagerImpl) CreateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error) {
	// Validate task ID and session ID
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Build workspace path
	workspacePath := filepath.Join(w.workspacesDir, sessionID, taskID)

	// Safety check 1: Canonicalized path validation before creation
	if err := validatePathCanonical(workspacePath, w.workspacesDir); err != nil {
		return nil, fmt.Errorf("workspace safety check failed: %w", err)
	}

	// Safety check 2: Ensure workspaces directory exists and is safe
	if err := os.MkdirAll(w.workspacesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspaces root directory %s: %w", w.workspacesDir, err)
	}

	// Safety check 3: Create workspace directory with safe permissions
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory %s: %w", workspacePath, err)
	}

	// Safety check 4: Validate workspace ownership
	if err := validateWorkspaceOwnership(workspacePath, taskID, sessionID); err != nil {
		return nil, fmt.Errorf("workspace ownership validation failed: %w", err)
	}

	metadata := &WorkspaceMetadata{
		TaskID:      taskID,
		SessionID:   sessionID,
		Path:        workspacePath,
		Initialized: false,
		Clean:       true,
		Locked:      false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	log.Printf("[WorkspaceManager] Workspace created: task_id=%s session_id=%s path=%s", taskID, sessionID, workspacePath)
	return metadata, nil
}

func (w *WorkspaceManagerImpl) ValidateWorkspace(ctx context.Context, path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (w *WorkspaceManagerImpl) LockWorkspace(ctx context.Context, path string) error {
	w.lockMapMutex.Lock()
	defer w.lockMapMutex.Unlock()
	if _, exists := w.lockMap[path]; !exists {
		w.lockMap[path] = &sync.Mutex{}
	}
	w.lockMap[path].Lock()
	
	lockPath := filepath.Join(path, ".zen-lock")
	
	// Check for existing lock file
	if info, err := os.Stat(lockPath); err == nil {
		// Lock file exists, check if stale
		if time.Since(info.ModTime()) > staleLockThreshold {
			log.Printf("[WorkspaceManager] Removing stale lock file: path=%s age=%v", lockPath, time.Since(info.ModTime()))
			if err := os.Remove(lockPath); err != nil {
				w.lockMap[path].Unlock()
				return fmt.Errorf("failed to remove stale lock file: %w", err)
			}
		} else {
			// Lock is still valid, workspace is locked
			w.lockMap[path].Unlock()
			return fmt.Errorf("workspace is locked: %s (locked at %v)", lockPath, info.ModTime().Format(time.RFC3339))
		}
	}
	
	// Create lock file atomically to prevent race conditions between processes
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		w.lockMap[path].Unlock()
		return fmt.Errorf("failed to create lock file (possibly race condition): %w", err)
	}
	// Write timestamp
	timestamp := time.Now().Format(time.RFC3339)
	if _, err := file.Write([]byte(timestamp)); err != nil {
		file.Close()
		os.Remove(lockPath)
		w.lockMap[path].Unlock()
		return fmt.Errorf("failed to write lock timestamp: %w", err)
	}
	file.Close()
	
	log.Printf("[WorkspaceManager] Workspace locked: path=%s", path)
	return nil
}

func (w *WorkspaceManagerImpl) UnlockWorkspace(ctx context.Context, path string) error {
	w.lockMapMutex.RLock()
	mutex, exists := w.lockMap[path]
	w.lockMapMutex.RUnlock()
	if !exists {
		return fmt.Errorf("workspace not locked: %s", path)
	}
	mutex.Unlock()
	os.Remove(filepath.Join(path, ".zen-lock"))
	log.Printf("[WorkspaceManager] Workspace unlocked: path=%s", path)
	return nil
}

func (w *WorkspaceManagerImpl) GetWorkspaceMetadata(ctx context.Context, path string) (*WorkspaceMetadata, error) {
	metadata := &WorkspaceMetadata{
		Path:      path,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	markerPath := filepath.Join(path, ".zen-workspace")
	if _, err := os.Stat(markerPath); err == nil {
		metadata.Initialized = true
	}
	lockPath := filepath.Join(path, ".zen-lock")
	if _, err := os.Stat(lockPath); err == nil {
		metadata.Locked = true
	}
	metadata.Clean = true
	return metadata, nil
}

func (w *WorkspaceManagerImpl) DeleteWorkspace(ctx context.Context, path string) error {
	// Safety check 1: Canonicalized path validation
	if err := validatePathCanonical(path, w.workspacesDir); err != nil {
		return fmt.Errorf("workspace safety check failed: %w", err)
	}

	// Safety check 2: Verify path is within workspaces directory
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	absWorkspacesDir, err := filepath.Abs(filepath.Clean(w.workspacesDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute workspaces dir: %w", err)
	}
	withinRoot, err := isPathWithinRoot(absPath, absWorkspacesDir)
	if err != nil {
		return fmt.Errorf("failed to check path containment: %w", err)
	}
	if !withinRoot {
		return fmt.Errorf("refusing to delete workspace outside workspaces dir: %s", path)
	}

	// Safety check 3: Prevent deleting entire workspaces directory
	if absPath == absWorkspacesDir {
		return fmt.Errorf("refusing to delete workspaces root directory: %s", path)
	}

	// Safety check 4: Release workspace lock before deletion
	w.lockMapMutex.RLock()
	mutex, exists := w.lockMap[path]
	w.lockMapMutex.RUnlock()
	if exists {
		mutex.Lock()
		defer mutex.Unlock()
	}

	// Safety check 5: Log destructive action before execution
	log.Printf("[WorkspaceManager] Deleting workspace: path=%s canonical=%s", path, absPath)

	// Safety check 6: RemoveAll is the only destructive operation (no shell commands)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete workspace %s: %w", path, err)
	}

	log.Printf("[WorkspaceManager] Workspace deleted successfully: path=%s", path)

	return nil
}

// CleanupStaleWorkspaces removes workspace directories that are older than staleWorkspaceThreshold.
// It skips directories that are currently locked.
func (w *WorkspaceManagerImpl) CleanupStaleWorkspaces(ctx context.Context) (int, error) {
	cleaned := 0
	err := filepath.WalkDir(w.workspacesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip errors for individual entries
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Check if this is a task directory (two levels deep from workspacesDir)
		relPath, err := filepath.Rel(w.workspacesDir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) != 2 {
			// Not a task directory (session level or deeper)
			return nil
		}
		// Check directory age
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if time.Since(info.ModTime()) <= staleWorkspaceThreshold {
			return nil
		}
		// Check for lock file
		lockPath := filepath.Join(path, ".zen-lock")
		if lockInfo, err := os.Stat(lockPath); err == nil {
			// Lock file exists, check if stale
			if time.Since(lockInfo.ModTime()) > staleLockThreshold {
				// Stale lock, remove it
				os.Remove(lockPath)
				log.Printf("[WorkspaceManager] Removed stale lock: %s", lockPath)
			} else {
				// Still locked, skip deletion
				log.Printf("[WorkspaceManager] Skipping locked workspace: %s", path)
				return nil
			}
		}
		// Delete workspace directory
		if err := w.DeleteWorkspace(ctx, path); err != nil {
			log.Printf("[WorkspaceManager] Failed to delete stale workspace %s: %v", path, err)
			return nil
		}
		cleaned++
		log.Printf("[WorkspaceManager] Deleted stale workspace: %s", path)
		return nil
	})
	if err != nil {
		return cleaned, fmt.Errorf("error walking workspaces directory: %w", err)
	}
	return cleaned, nil
}

// GitWorkspaceManager integrates with zen-brain's worktree manager.
type GitWorkspaceManager struct {
	worktreeManager interface{}
}

func NewGitWorkspaceManager(worktreeManager interface{}) *GitWorkspaceManager {
	return &GitWorkspaceManager{
		worktreeManager: worktreeManager,
	}
}

func (g *GitWorkspaceManager) CreateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error) {
	metadata := &WorkspaceMetadata{
		TaskID:      taskID,
		SessionID:   sessionID,
		Path:        "",
		Initialized: false,
		Clean:       false,
		Locked:      false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	log.Printf("[GitWorkspaceManager] Git workspace creation deferred to zen-brain worktree manager: task_id=%s session_id=%s", taskID, sessionID)
	return metadata, nil
}

func (g *GitWorkspaceManager) ValidateWorkspace(ctx context.Context, path string) (bool, error) {
	return true, nil
}

func (g *GitWorkspaceManager) LockWorkspace(ctx context.Context, path string) error {
	lockPath := filepath.Join(path, ".zen-lock")
	if err := os.WriteFile(lockPath, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		return fmt.Errorf("failed to create lock marker: %w", err)
	}
	log.Printf("[GitWorkspaceManager] Git workspace locked: path=%s", path)
	return nil
}

func (g *GitWorkspaceManager) UnlockWorkspace(ctx context.Context, path string) error {
	os.Remove(filepath.Join(path, ".zen-lock"))
	log.Printf("[GitWorkspaceManager] Git workspace unlocked: path=%s", path)
	return nil
}

func (g *GitWorkspaceManager) GetWorkspaceMetadata(ctx context.Context, path string) (*WorkspaceMetadata, error) {
	metadata := &WorkspaceMetadata{
		Path:        path,
		Initialized: true,
		Clean:       true,
		Locked:      false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return metadata, nil
}

func (g *GitWorkspaceManager) DeleteWorkspace(ctx context.Context, path string) error {
	log.Printf("[GitWorkspaceManager] Git workspace deletion deferred to zen-brain worktree manager: path=%s", path)
	return nil
}
