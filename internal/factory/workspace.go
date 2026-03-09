package factory

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
	if err := os.WriteFile(lockPath, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		w.lockMap[path].Unlock()
		return fmt.Errorf("failed to create lock marker: %w", err)
	}
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
