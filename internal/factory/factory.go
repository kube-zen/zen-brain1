package factory

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/intelligence"
)

// RecommenderInterface is an alias for intelligence.FactoryRecommenderInterface.
// This provides type safety while avoiding circular dependencies.
type RecommenderInterface = intelligence.FactoryRecommenderInterface

// FactoryImpl implements the Factory interface.
// It orchestrates task execution with bounded loops and proof-of-work generation.
type FactoryImpl struct {
	workspaceManager   WorkspaceManager
	executor           Executor
	proofOfWorkManager ProofOfWorkManager
	templateManager    *TemplateManager
	runtimeDir         string
	tasks              map[string]*FactoryTaskSpec
	tasksMutex         sync.RWMutex
	recommender        RecommenderInterface // Optional intelligence recommender for template auto-selection
}

// NewFactory creates a new Factory instance.
func NewFactory(
	workspaceManager WorkspaceManager,
	executor Executor,
	proofOfWorkManager ProofOfWorkManager,
	runtimeDir string,
) *FactoryImpl {
	return &FactoryImpl{
		workspaceManager:   workspaceManager,
		executor:           executor,
		proofOfWorkManager: proofOfWorkManager,
		templateManager:    NewTemplateManager(),
		runtimeDir:         runtimeDir,
		tasks:              make(map[string]*FactoryTaskSpec),
		recommender:        nil,
	}
}

// SetRecommender sets the intelligence recommender for template auto-selection.
// If nil, the Factory falls back to static template selection.
func (f *FactoryImpl) SetRecommender(r RecommenderInterface) {
	f.recommender = r
}

// ExecuteTask runs a task in an isolated workspace.
func (f *FactoryImpl) ExecuteTask(ctx context.Context, spec *FactoryTaskSpec) (*ExecutionResult, error) {
	// Validate spec
	if spec == nil {
		return nil, fmt.Errorf("task spec cannot be nil")
	}
	if spec.ID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	log.Printf("[Factory] Executing task: task_id=%s session_id=%s title=%s", spec.ID, spec.SessionID, spec.Title)

	// Store task
	f.tasksMutex.Lock()
	f.tasks[spec.ID] = spec
	f.tasksMutex.Unlock()

	// Start timer
	startTime := time.Now()

	// Allocate workspace
	workspaceMetadata, err := f.workspaceManager.CreateWorkspace(ctx, spec.ID, spec.SessionID)
	if err != nil {
		wsErr := WorkspaceError(ErrWorkspaceAllocation, "failed to allocate workspace", "", err).
			WithTaskID(spec.ID)
		return f.createErrorResult(spec, wsErr, "failed to allocate workspace"), wsErr
	}

	// Set workspace path in spec
	spec.WorkspacePath = workspaceMetadata.Path
	spec.UpdatedAt = time.Now()

	// Lock workspace for exclusive access
	if err := f.workspaceManager.LockWorkspace(ctx, workspaceMetadata.Path); err != nil {
		wsErr := WorkspaceError(ErrWorkspaceLock, "failed to lock workspace", workspaceMetadata.Path, err).
			WithTaskID(spec.ID)
		return f.createErrorResult(spec, wsErr, "failed to lock workspace"), wsErr
	}
	defer f.workspaceManager.UnlockWorkspace(ctx, workspaceMetadata.Path)

	// Create execution plan from spec (sets spec.TemplateKey)
	steps := f.createExecutionPlan(spec)

	// Execute bounded loop
	result, err := f.executor.ExecutePlan(ctx, steps, workspaceMetadata.Path)
	if err != nil {
		log.Printf("[Factory] Task execution failed: task_id=%s error=%v", spec.ID, err)
		return result, err
	}

	// Populate result metadata
	result.TaskID = spec.ID
	result.SessionID = spec.SessionID
	result.WorkItemID = spec.WorkItemID
	result.WorkspacePath = workspaceMetadata.Path
	result.TemplateKey = spec.TemplateKey
	if result.TemplateKey == "" {
		result.TemplateKey = spec.SelectedTemplate
	}
	result.CompletedAt = time.Now()
	result.Duration = time.Since(startTime)
	result.Success = (result.Status == ExecutionStatusCompleted)

	// Scan workspace for file changes (state continuity); sort for deterministic proof
	if files, err := f.workspaceManager.ListWorkspaceFiles(ctx, workspaceMetadata.Path); err == nil {
		sort.Strings(files)
		result.FilesChanged = files
	} else {
		log.Printf("[Factory] Failed to scan workspace files: task_id=%s error=%v", spec.ID, err)
	}

	// Populate git metadata from workspace when available
	if meta, err := f.workspaceManager.GetWorkspaceMetadata(ctx, workspaceMetadata.Path); err == nil && meta != nil {
		result.GitBranch = meta.Branch
		result.GitCommit = meta.BaseCommit
	}

	// Generate proof-of-work
	artifact, err := f.proofOfWorkManager.CreateProofOfWork(ctx, result, spec)
	proofPath := ""
	if err != nil {
		log.Printf("[Factory] Failed to generate proof-of-work: task_id=%s error=%v", spec.ID, err)
	} else {
		result.ProofOfWorkPath = artifact.Directory
		proofPath = artifact.Directory
	}

	log.Printf("[Factory] Task execution completed: task_id=%s status=%s duration=%s proof=%s", spec.ID, result.Status, result.Duration.String(), proofPath)

	return result, nil
}

// AllocateWorkspace creates or retrieves an isolated workspace for a task.
func (f *FactoryImpl) AllocateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error) {
	return f.workspaceManager.CreateWorkspace(ctx, taskID, sessionID)
}

// CleanupWorkspace removes a workspace and associated resources.
func (f *FactoryImpl) CleanupWorkspace(ctx context.Context, workspacePath string) error {
	log.Printf("[Factory] Cleaning up workspace: path=%s", workspacePath)
	return f.workspaceManager.DeleteWorkspace(ctx, workspacePath)
}

// GetWorkspaceMetadata returns current workspace state.
func (f *FactoryImpl) GetWorkspaceMetadata(ctx context.Context, workspacePath string) (*WorkspaceMetadata, error) {
	return f.workspaceManager.GetWorkspaceMetadata(ctx, workspacePath)
}

// GenerateProofOfWork creates a structured proof-of-work summary.
func (f *FactoryImpl) GenerateProofOfWork(ctx context.Context, result *ExecutionResult) (*ProofOfWorkSummary, error) {
	// This method is deprecated - use CreateProofOfWork instead
	// Kept for backward compatibility with interface
	artifact, err := f.proofOfWorkManager.CreateProofOfWork(ctx, result, nil)
	if err != nil {
		return nil, err
	}
	return artifact.Summary, nil
}

// ListTasks returns all tasks known to the Factory.
func (f *FactoryImpl) ListTasks(ctx context.Context) ([]*FactoryTaskSpec, error) {
	f.tasksMutex.RLock()
	defer f.tasksMutex.RUnlock()

	tasks := make([]*FactoryTaskSpec, 0, len(f.tasks))
	for _, task := range f.tasks {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTask retrieves a specific task by ID.
func (f *FactoryImpl) GetTask(ctx context.Context, taskID string) (*FactoryTaskSpec, error) {
	f.tasksMutex.RLock()
	defer f.tasksMutex.RUnlock()

	task, exists := f.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// createErrorResult creates a failed execution result with error details.
func (f *FactoryImpl) createErrorResult(spec *FactoryTaskSpec, err error, message string) *ExecutionResult {
	errorCode := "WORKSPACE_ERROR"
	recommendation := "retry"
	
	// Extract structured error information if available
	if fe, ok := err.(*FactoryError); ok {
		errorCode = string(fe.Code)
		// Set recommendation based on error type
		switch fe.Code {
		case ErrStepTimeout, ErrStepMaxRetriesExceeded:
			recommendation = "escalate"
		case ErrContextCanceled:
			recommendation = "review"
		case ErrInvalidInput:
			recommendation = "review"
		default:
			recommendation = "retry"
		}
	}
	
	return &ExecutionResult{
		TaskID:         spec.ID,
		SessionID:      spec.SessionID,
		WorkItemID:     spec.WorkItemID,
		Status:         ExecutionStatusFailed,
		Success:        false,
		Error:          fmt.Sprintf("%s: %v", message, err),
		ErrorCode:      errorCode,
		CompletedAt:    time.Now(),
		Recommendation: recommendation,
	}
}

// CancelTask cancels a running task.
func (f *FactoryImpl) CancelTask(ctx context.Context, taskID string) error {
	f.tasksMutex.Lock()
	defer f.tasksMutex.Unlock()

	_, exists := f.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	// Note: In full implementation, this would signal the executor to stop
	// For MVP, we mark the task as canceled
	return nil
}

// createExecutionPlan creates a bounded execution plan from task spec using templates.
// When a recommender is set, it is used to choose template and configuration; otherwise static selection is used.
func (f *FactoryImpl) createExecutionPlan(spec *FactoryTaskSpec) []*ExecutionStep {
	ctx := context.Background()
	sel := f.chooseTemplateAndConfig(ctx, spec)

	template, err := f.templateManager.GetTemplate(sel.workType, sel.workDomain)
	if err != nil {
		log.Printf("[Factory] No template for work type %s, using default: %v", spec.WorkType, err)
		template, _ = f.templateManager.GetTemplate("default", "")
	}

	// Apply timeout/retry overrides from selection to steps (ExpandTemplateVariables uses spec; spec already updated in chooseTemplateAndConfig)
	steps := f.templateManager.ExpandTemplateVariables(template, spec)
	for _, step := range steps {
		if sel.timeoutSeconds > 0 && step.TimeoutSeconds <= 0 {
			step.TimeoutSeconds = sel.timeoutSeconds
		}
		if sel.maxRetries > 0 && step.MaxRetries <= 0 {
			step.MaxRetries = sel.maxRetries
		}
	}

	log.Printf("[Factory] intelligence selection: task_id=%s template=%s source=%s confidence=%.2f",
		spec.ID, spec.SelectedTemplate, spec.SelectionSource, spec.SelectionConfidence)
	log.Printf("[Factory] Created execution plan with %d steps for task %s (work_type=%s)",
		len(steps), spec.ID, spec.WorkType)

	return steps
}
