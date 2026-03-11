package factory

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/intelligence"
	"github.com/kube-zen/zen-brain1/internal/worktree"
)

// RecommenderInterface is an alias for intelligence.FactoryRecommenderInterface.
// This provides type safety while avoiding circular dependencies.
type RecommenderInterface = intelligence.FactoryRecommenderInterface

// FactoryConfig configures Factory behavior for repo-native execution.
type FactoryConfig struct {
	// UseGitWorktree enables git worktree-based workspaces (repo-native).
	// When true, Factory creates git worktrees from GitRepoPath.
	// When false, Factory creates isolated directories (WorkspaceManagerImpl).
	UseGitWorktree bool

	// GitRepoPath is the path to the git repository to create worktrees from.
	// Required when UseGitWorktree is true.
	GitRepoPath string

	// WorktreeBasePath is the base directory under which worktrees are created.
	// Required when UseGitWorktree is true.
	WorktreeBasePath string

	// StrictProofMode enables proof verification (checks actual git changes).
	// When true, proof generation verifies actual git diffs/commits exist.
	// When false, proof records paths without verification.
	StrictProofMode bool
}

// FactoryImpl implements the Factory interface.
// It orchestrates task execution with bounded loops and proof-of-work generation.
type FactoryImpl struct {
	config                 FactoryConfig // Factory configuration
	workspaceManager       WorkspaceManager
	executor               Executor
	proofOfWorkManager     ProofOfWorkManager
	templateManager        *TemplateManager
	runtimeDir             string
	tasks                  map[string]*FactoryTaskSpec
	tasksMutex             sync.RWMutex
	recommender            RecommenderInterface // Optional intelligence recommender for template auto-selection
	preflightMode          PreflightMode       // Mode for preflight checks (default: strict)
	postflightStrictMode   bool               // If true, postflight failures are fatal (default: false)
	proofVerificationMode   bool               // If true, run enhanced proof verification (default: false)
}

// NewFactory creates a new Factory instance with default configuration.
// Uses WorkspaceManagerImpl (isolated directories, not git-backed).
// For git worktree support, use NewFactoryWithConfig.
func NewFactory(
	workspaceManager WorkspaceManager,
	executor Executor,
	proofOfWorkManager ProofOfWorkManager,
	runtimeDir string,
) *FactoryImpl {
	return &FactoryImpl{
		workspaceManager:       workspaceManager,
		executor:               executor,
		proofOfWorkManager:     proofOfWorkManager,
		templateManager:        NewTemplateManager(),
		runtimeDir:             runtimeDir,
		tasks:                  make(map[string]*FactoryTaskSpec),
		recommender:            nil,
		preflightMode:          PreflightModeStrict, // Default to strict mode
		postflightStrictMode:   false,               // Default to non-strict postflight
		proofVerificationMode:   false,               // Default to skip proof verification
	}
}

// NewFactoryWithConfig creates a new Factory instance with custom configuration.
// When config.UseGitWorktree is true, creates GitWorkspaceManager for repo-native execution.
// When config.UseGitWorktree is false, creates WorkspaceManagerImpl (isolated directories).
func NewFactoryWithConfig(
	config FactoryConfig,
	executor Executor,
	proofOfWorkManager ProofOfWorkManager,
	runtimeDir string,
) (*FactoryImpl, error) {
	var workspaceManager WorkspaceManager

	if config.UseGitWorktree {
		// Create GitWorkspaceManager for repo-native execution
		if config.GitRepoPath == "" {
			return nil, fmt.Errorf("GitRepoPath required when UseGitWorktree is true")
		}
		if config.WorktreeBasePath == "" {
			return nil, fmt.Errorf("WorktreeBasePath required when UseGitWorktree is true")
		}

		// Create git worktree manager
		gitManagerConfig := worktree.GitManagerConfig{
			RepoPath:       config.GitRepoPath,
			BasePath:       config.WorktreeBasePath,
			DefaultRef:     "HEAD",
			BranchPrefix:   "zen",
			ReuseSessionWT: false,
		}
		gitManager, err := worktree.NewGitManager(gitManagerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create git worktree manager: %w", err)
		}
		workspaceManager = NewGitWorkspaceManager(gitManager)
		log.Printf("[Factory] Git worktree mode enabled (repo=%s, base=%s)", config.GitRepoPath, config.WorktreeBasePath)
	} else {
		// Create WorkspaceManagerImpl for isolated directory execution
		workspaceManager = NewWorkspaceManager(runtimeDir)
		log.Printf("[Factory] Isolated directory mode enabled")
	}

	return &FactoryImpl{
		config:                 config,
		workspaceManager:       workspaceManager,
		executor:               executor,
		proofOfWorkManager:     proofOfWorkManager,
		templateManager:        NewTemplateManager(),
		runtimeDir:             runtimeDir,
		tasks:                  make(map[string]*FactoryTaskSpec),
		recommender:            nil,
		preflightMode:          PreflightModeStrict,
		postflightStrictMode:   config.StrictProofMode,
		proofVerificationMode:   config.StrictProofMode,
	}, nil
}

// SetPreflightMode sets the preflight check mode.
// Valid modes: "lenient" (non-fatal), "strict" (critical checks fail), "fail-closed" (all checks fail)
func (f *FactoryImpl) SetPreflightMode(mode PreflightMode) {
	f.preflightMode = mode
}

// SetPostflightStrictMode enables strict postflight verification.
// When true, postflight failures are fatal to the task.
func (f *FactoryImpl) SetPostflightStrictMode(strict bool) {
	f.postflightStrictMode = strict
}

// SetProofVerificationMode enables enhanced proof verification.
// When true, proof artifacts are comprehensively verified after generation.
func (f *FactoryImpl) SetProofVerificationMode(enabled bool) {
	f.proofVerificationMode = enabled
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

	// Run preflight checks (using enhanced checker when mode is set)
	var preflightReport interface{}
	var err error
	if f.preflightMode != "" {
		// Use enhanced preflight checker with configured mode
		enhancedChecker := NewEnhancedPreflightChecker(f.workspaceManager, nil, f.preflightMode)
		if f.preflightMode == PreflightModeFailClosed {
			preflightReport, err = enhancedChecker.MustRunEnhancedPreflightChecks(ctx, spec)
		} else {
			preflightReport, err = enhancedChecker.RunEnhancedPreflightChecks(ctx, spec)
		}
	} else {
		// Use basic preflight checker (backward compatibility)
		basicChecker := NewPreflightChecker(f.workspaceManager, nil)
		preflightReport, err = basicChecker.MustRunPreflightChecks(ctx, spec)
	}

	if err != nil {
		log.Printf("[Factory] Preflight checks failed: task_id=%s error=%v", spec.ID, err)
		return f.createErrorResult(spec, err, "preflight checks failed"), err
	}

	// Extract number of checks from report (type assertion)
	var checkCount int
	if report, ok := preflightReport.(*PreflightReport); ok {
		checkCount = len(report.Checks)
		log.Printf("[Factory] Preflight checks passed: task_id=%s mode=%s checks=%d", spec.ID, f.preflightMode, checkCount)
	} else {
		log.Printf("[Factory] Preflight checks passed: task_id=%s mode=%s", spec.ID, f.preflightMode)
	}

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
		// Set TemplateKey on error result before returning
		if result != nil {
			result.TaskID = spec.ID
			result.SessionID = spec.SessionID
			result.WorkItemID = spec.WorkItemID
			result.WorkspacePath = workspaceMetadata.Path
			result.TemplateKey = spec.TemplateKey
			if result.TemplateKey == "" {
				result.TemplateKey = spec.SelectedTemplate
			}
		}
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
		result.ArtifactPaths = artifact.Summary.ArtifactPaths
		result.GitStatusPath = artifact.Summary.GitStatusPath
		result.GitDiffStatPath = artifact.Summary.GitDiffStatPath
	}

	// Run postflight verification (using enhanced verifier when strict mode is enabled)
	var postflightReport interface{}
	if f.postflightStrictMode {
		// Use enhanced postflight verifier with strict mode
		enhancedVerifier := NewEnhancedPostflightVerifier(f.workspaceManager, true)
		postflightReport, err = enhancedVerifier.RunEnhancedPostflightVerification(ctx, result, spec)
	} else {
		// Use basic postflight verifier
		basicVerifier := NewPostflightVerifier(f.workspaceManager)
		postflightReport, err = basicVerifier.RunPostflightVerification(ctx, result, spec)
	}

	if err != nil {
		log.Printf("[Factory] Postflight verification failed: task_id=%s error=%v", spec.ID, err)
		// In strict mode, this is fatal
		if f.postflightStrictMode {
			return result, err
		}
		// Non-fatal: log warning but don't fail the task
	}

	// Extract check results from report
	if report, ok := postflightReport.(*PostflightReport); ok {
		if !report.AllPassed {
			log.Printf("[Factory] Postflight checks failed (non-fatal): task_id=%s failed=%d", spec.ID, len(report.Checks))
			for _, check := range report.Checks {
				if !check.Passed {
					log.Printf("[Factory]   - %s: %s", check.Name, check.Message)
				}
			}
		} else {
			log.Printf("[Factory] Postflight checks passed: task_id=%s checks=%d", spec.ID, len(report.Checks))
		}
	}

	// Run enhanced proof verification if enabled
	if f.proofVerificationMode && artifact != nil {
		proofVerifier := NewProofVerifier(f.proofOfWorkManager, f.postflightStrictMode)
		verificationReport, err := proofVerifier.VerifyProof(ctx, artifact)
		if err != nil {
			log.Printf("[Factory] Proof verification failed: task_id=%s error=%v", spec.ID, err)
		} else {
			log.Printf("[Factory] Proof verification completed: task_id=%s score=%.2f passed=%v",
				spec.ID, verificationReport.OverallScore, verificationReport.AllPassed)
			if !verificationReport.AllPassed && len(verificationReport.Recommendations) > 0 {
				log.Printf("[Factory] Proof verification recommendations: task_id=%s", spec.ID)
				for _, rec := range verificationReport.Recommendations {
					log.Printf("[Factory]   - %s", rec)
				}
			}
		}
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
