package factory

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/intelligence"
	"github.com/kube-zen/zen-brain1/internal/llm"
	llmcontracts "github.com/kube-zen/zen-brain1/pkg/llm"
	"github.com/kube-zen/zen-brain1/internal/mlq"
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
	llmGenerator           *LLMGenerator      // Optional LLM generator for code generation (default fallback)
	llmEnabled             bool               // Whether LLM templates are enabled
	mlq                    *mlq.MLQ            // Multi-Level Queue for backend selection
	taskExecutor           *mlq.TaskExecutor   // Task-level retry/escalation executor
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

// SetLLMGenerator sets the LLM generator for code generation.
// When set, the Factory will use LLM-powered templates instead of shell scripts.
func (f *FactoryImpl) SetLLMGenerator(generator *LLMGenerator) {
	f.llmGenerator = generator
	f.llmEnabled = generator != nil
	if generator != nil {
		// Register LLM templates
		f.templateManager.registry.RegisterLLMTemplates(generator)
		log.Printf("[Factory] LLM-powered templates enabled")
	}
}

// EnableLLM enables LLM-powered code generation with the given provider.
// This is a convenience method that creates an LLMGenerator with default config.
func (f *FactoryImpl) EnableLLM(provider interface{}) error {
	// Type assertion would happen here when using proper llm.Provider type
	// For now, log that LLM mode is requested
	log.Printf("[Factory] LLM mode requested (provider type: %T) - use SetLLMGenerator for full support", provider)
	return nil
}

// EnableMLQ enables Multi-Level Queue for backend selection and task-level retry/escalation.
func (f *FactoryImpl) EnableMLQ(configPath string) error {
	m, err := mlq.NewMLQFromConfig(configPath)
	if err != nil {
		return fmt.Errorf("load MLQ config: %w", err)
	}
	f.mlq = m

	// Create worker pools for each enabled level
	pools := make(map[int]*mlq.WorkerPool)
	for _, levelNum := range m.ListLevels() {
		level, ok := m.GetLevel(levelNum)
		if !ok {
			continue
		}
		// Use the configured endpoint as the primary worker
		endpoints := []string{level.Backend.APIEndpoint}
		pools[levelNum] = mlq.NewWorkerPool(level, endpoints)
	}

	f.taskExecutor = mlq.NewTaskExecutor(m, pools)
	log.Printf("[Factory] MLQ enabled with task-level retry/escalation (config=%s, levels=%v, pools=%d)",
		configPath, m.ListLevels(), len(pools))
	return nil
}

// IsLLMEnabled returns true if LLM-powered templates are enabled.
func (f *FactoryImpl) IsLLMEnabled() bool {
	return f.llmEnabled && f.llmGenerator != nil
}

// IsMLQEnabled returns true if MLQ routing is enabled.
func (f *FactoryImpl) IsMLQEnabled() bool {
	return f.mlq != nil
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

	// ZB-027G: Enforce hard task-level timeout as outer deadline
	// This wraps the entire execution (preflight, LLM generation, validation, postflight)
	if spec.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(spec.TimeoutSeconds)*time.Second)
		defer cancel()
		log.Printf("[Factory] Task timeout set: task_id=%s timeout=%ds", spec.ID, spec.TimeoutSeconds)
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

	var result *ExecutionResult

	// Check if we should use LLM execution (empty steps + LLM enabled)
	if len(steps) == 0 && f.llmEnabled && f.shouldUseLLMTemplate(spec) {
		// Execute with LLM-powered code generation
		log.Printf("[Factory] Using LLM-powered execution for task %s (work_type=%s, model=%s)", spec.ID, spec.WorkType, f.llmGenerator.config.Model)
		filesCreated, llmErr := f.executeWithLLM(ctx, spec, workspaceMetadata.Path)
		
		result = &ExecutionResult{
			TaskID:         spec.ID,
			SessionID:      spec.SessionID,
			WorkItemID:     spec.WorkItemID,
			WorkspacePath:  workspaceMetadata.Path,
			TemplateKey:    spec.SelectedTemplate,
			Status:         ExecutionStatusCompleted,
			Success:        llmErr == nil,
			CompletedAt:    time.Now(),
			Duration:       time.Since(startTime),
			FilesChanged:   filesCreated,
			// ZB-022D: Add observability for execution mode
			Metadata: map[string]string{
				"execution_mode": "llm",
				"llm_model":      f.llmGenerator.config.Model,
				"llm_provider":   f.llmGenerator.config.Provider.Name(),
			},
		}

		if llmErr != nil {
			log.Printf("[Factory] LLM execution failed: task_id=%s error=%v", spec.ID, llmErr)
			result.Status = ExecutionStatusFailed
			result.Error = llmErr.Error()
			result.Success = false
			// Set proof-of-work path even on failure
			result.ProofOfWorkPath = filepath.Join(workspaceMetadata.Path, "proof-of-work.json")
			return result, llmErr
		}

		log.Printf("[Factory] LLM execution completed: task_id=%s files=%d", spec.ID, len(filesCreated))

	} else {
		// Execute bounded loop with shell steps
		var err error
		result, err = f.executor.ExecutePlan(ctx, steps, workspaceMetadata.Path)
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
			// Collect all failed checks with reasons
			failedReasons := []string{}
			criticalFailures := []string{}
			for _, check := range report.Checks {
				if !check.Passed {
					log.Printf("[Factory]   - %s: %s", check.Name, check.Message)
					failedReasons = append(failedReasons, fmt.Sprintf("%s: %s", check.Name, check.Message))
					// Critical checks that must hard-fail even in non-strict mode
					switch check.Name {
					case "execution_completed", "files_verified", "proof_of_work":
						criticalFailures = append(criticalFailures, check.Name)
					}
				}
			}

			if len(criticalFailures) > 0 {
				// HARD FAILURE: critical postflight checks failed
				log.Printf("[Factory] HARD FAILURE: critical postflight checks failed: task_id=%s critical=%v all_failed=%d",
					spec.ID, criticalFailures, len(failedReasons))
				result.Success = false
				result.Status = ExecutionStatusFailed
				result.VerificationFailed = true
				result.Recommendation = "retry"
				result.Error = fmt.Sprintf("critical postflight failure: %s", strings.Join(failedReasons, "; "))
			} else {
				log.Printf("[Factory] Postflight checks failed (non-fatal): task_id=%s failed=%d", spec.ID, len(failedReasons))
				result.VerificationFailed = true
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
// When LLM mode is enabled, returns empty steps (LLM execution handled separately).
func (f *FactoryImpl) createExecutionPlan(spec *FactoryTaskSpec) []*ExecutionStep {
	// PHASE 1: Add hard observability around the LLM decision point
	// Log all decision criteria before any branching
	normalizedWorkType := strings.TrimSpace(strings.ToLower(string(spec.WorkType)))
	normalizedWorkDomain := strings.TrimSpace(strings.ToLower(string(spec.WorkDomain)))
	shouldUseLLM := f.shouldUseLLMTemplate(spec)

	log.Printf("[Factory] llm gate: task_id=%s work_type=%s (normalized=%s) work_domain=%s (normalized=%s) llmEnabled=%v generator=%v shouldUseLLM=%v",
		spec.ID,
		spec.WorkType,
		normalizedWorkType,
		spec.WorkDomain,
		normalizedWorkDomain,
		f.llmEnabled,
		f.llmGenerator != nil,
		shouldUseLLM)

	// Log current template state before decision
	if spec.SelectedTemplate != "" {
		log.Printf("[Factory] llm gate: task_id=%s pre_decision_template=%s pre_decision_source=%s pre_decision_confidence=%.2f",
			spec.ID, spec.SelectedTemplate, spec.SelectionSource, spec.SelectionConfidence)
	} else {
		log.Printf("[Factory] llm gate: task_id=%s pre_decision_template=(empty)", spec.ID)
	}

	// PHASE 2: Make implementation tasks deterministic
	// For bounded implementation-capable work types on the active local CPU path:
	// if llmEnabled && llmGenerator != nil && work type is in the LLM allowlist
	// then force LLM path directly
	if f.llmEnabled && f.llmGenerator != nil && shouldUseLLM {
		log.Printf("[Factory] llm gate: task_id=%s FORCING_LLM_PATH work_type=%s model=%s",
			spec.ID, spec.WorkType, f.llmGenerator.config.Model)
		spec.SelectedTemplate = fmt.Sprintf("%s:llm", spec.WorkType)
		spec.SelectionSource = "llm_generator"
		spec.SelectionConfidence = 1.0
		// Return empty steps - actual execution via executeWithLLM
		return []*ExecutionStep{}
	}

	log.Printf("[Factory] Using shell-based template for task %s (work_type=%s, template=%s)", spec.ID, spec.WorkType, spec.SelectedTemplate)

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

// shouldUseLLMTemplate determines if LLM template should be used for a task.
func (f *FactoryImpl) shouldUseLLMTemplate(spec *FactoryTaskSpec) bool {
	if !f.llmEnabled || f.llmGenerator == nil {
		return false
	}

	// PHASE 3: Normalize work type matching
	// Guard against string drift: trim spaces, lowercase, ensure aliases
	normalizedWorkType := strings.TrimSpace(strings.ToLower(string(spec.WorkType)))

	// Use LLM for code generation tasks
	llmWorkTypes := map[string]bool{
		"implementation": true,
		"feature":        true,
		"bugfix":         true,
		"debug":          true,
		"refactor":       true,
		"test":           true,
		"migration":      true,
	}

	// Also support aliases for robustness
	llmWorkAliases := map[string]string{
		"implementation": "implementation",
		"implement":     "implementation",
		"impl":          "implementation",
		"feature":       "feature",
		"new":           "feature",
		"bugfix":        "bugfix",
		"fix":           "bugfix",
		"bug":           "bugfix",
		"debug":         "debug",
		"refactor":      "refactor",
		"refactoring":   "refactor",
		"test":          "test",
		"testing":       "test",
		"unit_test":     "test",
		"integration_test": "test",
		"migration":     "migration",
		"migrate":       "migration",
	}

	// Check direct match first
	if llmWorkTypes[normalizedWorkType] {
		log.Printf("[Factory] llm gate: task_id=%s work_type=%s normalized=%s -> LLM_CAPABLE (direct_match)", spec.ID, spec.WorkType, normalizedWorkType)
		return true
	}

	// Check alias match
	if canonicalType, ok := llmWorkAliases[normalizedWorkType]; ok {
		if llmWorkTypes[canonicalType] {
			log.Printf("[Factory] llm gate: task_id=%s work_type=%s normalized=%s -> LLM_CAPABLE (alias_match -> %s)", spec.ID, spec.WorkType, normalizedWorkType, canonicalType)
			return true
		}
	}

	log.Printf("[Factory] llm gate: task_id=%s work_type=%s normalized=%s -> NOT_LLM_CAPABLE", spec.ID, spec.WorkType, normalizedWorkType)
	return false
}

// executeWithLLM executes a task using LLM-powered code generation.
// When MLQ is enabled, routes through TaskExecutor for retry/escalation.
func (f *FactoryImpl) executeWithLLM(ctx context.Context, spec *FactoryTaskSpec, workspacePath string) ([]string, error) {
	if f.llmGenerator == nil {
		return nil, fmt.Errorf("LLM generator not configured")
	}

	// Determine task class from work type
	taskClass := string(spec.WorkType)
	if strings.Contains(taskClass, "implementation") {
		taskClass = "implementation"
	}

	// If TaskExecutor is available, use retry/escalation path
	if f.taskExecutor != nil {
		return f.executeWithLLMRetry(ctx, spec, workspacePath, taskClass)
	}

	// Legacy single-shot path (no MLQ or TaskExecutor not initialized)
	return f.executeWithLLMSingle(ctx, spec, workspacePath, taskClass)
}

// executeWithLLMSingle is the legacy single-shot MLQ selection path.
func (f *FactoryImpl) executeWithLLMSingle(ctx context.Context, spec *FactoryTaskSpec, workspacePath, taskClass string) ([]string, error) {
	generator, selectedProvider, selectedModel, selectedBaseURL, _, err :=
		f.createGeneratorForLevel(ctx, spec, taskClass, "")
	if err != nil {
		return nil, err
	}
	log.Printf("[Factory] Single-shot LLM: task_id=%s provider=%s model=%s url=%s",
		spec.ID, selectedProvider, selectedModel, selectedBaseURL)

	return f.runLLMTemplate(ctx, generator, spec, workspacePath)
}

// executeWithLLMRetry routes task execution through TaskExecutor for retry/escalation.
func (f *FactoryImpl) executeWithLLMRetry(ctx context.Context, spec *FactoryTaskSpec, workspacePath, taskClass string) ([]string, error) {
	var lastFiles []string
	var lastErr error

	telemetry := f.taskExecutor.ExecuteWithRetry(
		ctx, spec.ID, taskClass, spec.WorkItemID,
		func(execCtx context.Context, workerEndpoint string) (string, error) {
			generator, provider, model, baseURL, _, err :=
				f.createGeneratorForLevel(execCtx, spec, taskClass, workerEndpoint)
			if err != nil {
				return "", err
			}

			log.Printf("[Factory] LLM attempt: task_id=%s provider=%s model=%s url=%s",
				spec.ID, provider, model, baseURL)

			files, err := f.runLLMTemplate(execCtx, generator, spec, workspacePath)
			lastFiles = files
			lastErr = err
			if err != nil {
				return "", err
			}
			return workspacePath, nil
		},
	)

	// Log telemetry summary
	log.Printf("[MLQ-Telemetry] task_id=%s class=%s initial=%d final=%d result=%s attempts=%d retries=%d escalated=%v",
		telemetry.TaskID, telemetry.TaskClass, telemetry.InitialLevel, telemetry.FinalLevel,
		telemetry.FinalResult, len(telemetry.Attempts), telemetry.TotalRetries, telemetry.Escalated)

	if lastErr != nil {
		return nil, lastErr
	}
	return lastFiles, nil
}

// createGeneratorForLevel creates an LLM generator for a given worker endpoint.
// If workerEndpoint is empty, uses the default MLQ-selected level.
func (f *FactoryImpl) createGeneratorForLevel(ctx context.Context, spec *FactoryTaskSpec, taskClass, workerEndpoint string) (*LLMGenerator, string, string, string, int, error) {
	var selectedProvider, selectedModel, selectedBaseURL string
	var selectedTimeout int

	if f.mlq != nil && workerEndpoint == "" {
		// Use MLQ selection to determine the initial level
		level, err := f.mlq.SelectLevel(spec.ID, spec.WorkItemID, taskClass)
		if err != nil {
			log.Printf("[Factory] MLQ selection failed, using fallback: task_id=%s error=%v", spec.ID, err)
			return nil, "", "", "", 0, fmt.Errorf("MLQ selection failed: %w", err)
		}
		provider, model, baseURL, timeout := level.GetBackend()
		selectedProvider = provider
		selectedModel = model
		selectedBaseURL = baseURL
		selectedTimeout = timeout
	} else if workerEndpoint != "" {
		// Use the provided worker endpoint — determine provider from MLQ levels
		selectedBaseURL = workerEndpoint
		selectedTimeout = 2700 // default
		// Find the matching level
		if f.mlq != nil {
			for _, levelNum := range f.mlq.ListLevels() {
				if level, ok := f.mlq.GetLevel(levelNum); ok {
					if level.Backend.APIEndpoint == workerEndpoint {
						selectedProvider = level.Backend.Provider
						selectedModel = level.Backend.Name
						selectedTimeout = level.Backend.TimeoutSeconds
						break
					}
				}
			}
		}
		if selectedProvider == "" {
			selectedProvider = "llama-cpp"
			selectedModel = "unknown"
		}
	} else {
		// No MLQ, use default generator's provider info
		selectedProvider = f.llmGenerator.config.Provider.Name()
		selectedModel = f.llmGenerator.config.Model
		selectedTimeout = int(f.llmGenerator.config.Timeout.Seconds())
		return f.llmGenerator, selectedProvider, selectedModel, "", selectedTimeout, nil
	}

	// Create provider instance
	var providerProvider llmcontracts.Provider
	switch selectedProvider {
	case "ollama":
		providerProvider = llm.NewOllamaProvider(selectedBaseURL, selectedModel, selectedTimeout, "45m")
	case "llama-cpp":
		providerProvider = llm.NewOpenAICompatibleProviderWithTimeout(
			"llama-cpp", selectedBaseURL, selectedModel, "",
			time.Duration(selectedTimeout)*time.Second,
		)
	default:
		return nil, "", "", "", 0, fmt.Errorf("unsupported MLQ provider: %s", selectedProvider)
	}

	// Create generator
	genConfig := &LLMGeneratorConfig{
		Provider:       providerProvider,
		Model:          selectedModel,
		Temperature:    0.3,
		MaxTokens:      4096,
		EnableThinking: false,
		Timeout:        time.Duration(selectedTimeout) * time.Second,
	}

	generator, err := NewLLMGenerator(genConfig)
	if err != nil {
		return nil, "", "", "", 0, fmt.Errorf("create generator: %w", err)
	}

	return generator, selectedProvider, selectedModel, selectedBaseURL, selectedTimeout, nil
}

// runLLMTemplate executes the LLM template with the given generator.
func (f *FactoryImpl) runLLMTemplate(ctx context.Context, generator *LLMGenerator, spec *FactoryTaskSpec, workspacePath string) ([]string, error) {
	templateType := LLMTemplateImplementation
	switch spec.WorkType {
	case "bugfix", "debug":
		templateType = LLMTemplateBugFix
	case "refactor":
		templateType = LLMTemplateRefactor
	case "test":
		templateType = LLMTemplateTest
	case "migration":
		templateType = LLMTemplateMigration
	}

	config := &LLMTemplateConfig{
		Type:              templateType,
		WorkType:          string(spec.WorkType),
		WorkDomain:        string(spec.WorkDomain),
		ValidateCode:      true,
		CreateTests:       true,
		CreateDocs:        false,
		GenerationTimeout: 120 * time.Second,
	}

	executor, err := NewLLMTemplateExecutor(generator, config)
	if err != nil {
		return nil, fmt.Errorf("create LLM executor: %w", err)
	}

	files, err := executor.Execute(ctx, spec, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("LLM execution: %w", err)
	}

	return files, nil
}
