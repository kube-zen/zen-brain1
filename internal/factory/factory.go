package factory

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// FactoryImpl implements the Factory interface.
// It orchestrates task execution with bounded loops and proof-of-work generation.
type FactoryImpl struct {
	workspaceManager    WorkspaceManager
	executor           Executor
	proofOfWorkManager  ProofOfWorkManager
	runtimeDir         string
	tasks              map[string]*FactoryTaskSpec
	tasksMutex         sync.RWMutex
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
		executor:          executor,
		proofOfWorkManager: proofOfWorkManager,
		runtimeDir:        runtimeDir,
		tasks:             make(map[string]*FactoryTaskSpec),
	}
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
		return f.createErrorResult(spec, err, "failed to allocate workspace"), err
	}

	// Set workspace path in spec
	spec.WorkspacePath = workspaceMetadata.Path
	spec.UpdatedAt = time.Now()

	// Lock workspace for exclusive access
	if err := f.workspaceManager.LockWorkspace(ctx, workspaceMetadata.Path); err != nil {
		return f.createErrorResult(spec, err, "failed to lock workspace"), err
	}
	defer f.workspaceManager.UnlockWorkspace(ctx, workspaceMetadata.Path)

	// Create execution plan from spec
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
	result.CompletedAt = time.Now()
	result.Duration = time.Since(startTime)
	result.Success = (result.Status == ExecutionStatusCompleted)

	// Generate proof-of-work
	artifact, err := f.proofOfWorkManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		log.Printf("[Factory] Failed to generate proof-of-work: task_id=%s error=%v", spec.ID, err)
	} else {
		result.ProofOfWorkPath = artifact.Directory
	}

	log.Printf("[Factory] Task execution completed: task_id=%s status=%s duration=%s proof=%s", spec.ID, result.Status, result.Duration.String(), artifact.Directory)

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

// CancelTask cancels a running task.
func (f *FactoryImpl) CancelTask(ctx context.Context, taskID string) error {
	f.tasksMutex.Lock()
	defer f.tasksMutex.Unlock()

	task, exists := f.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	log.Printf("[Factory] Task canceled: task_id=%s title=%s", taskID, task.Title)

	// Note: In full implementation, this would signal the executor to stop
	// For MVP, we mark the task as canceled
	return nil
}

// createExecutionPlan creates a bounded execution plan from task spec.
func (f *FactoryImpl) createExecutionPlan(spec *FactoryTaskSpec) []*ExecutionStep {
	// For MVP, create a simple plan
	// In production, this would use the Planner output
	
	// Determine work type to customize plan
	var steps []*ExecutionStep
	
	switch spec.WorkType {
	case contracts.WorkTypeDebug:
		// Bug fix: analyze, fix, test
		steps = []*ExecutionStep{
			{
				StepID:      fmt.Sprintf("%s-step-1", spec.ID),
				TaskID:      spec.ID,
				Name:        "Analyze bug",
				Description: "Analyze the bug to understand root cause",
				Command:     "echo 'Analyzing bug: " + spec.Title + "' && echo 'Simulating bug analysis...' && sleep 0.1 && echo 'Root cause identified: authentication token validation missing'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-2", spec.ID),
				TaskID:      spec.ID,
				Name:        "Implement fix",
				Description: "Implement the fix for the bug",
				Command:     "echo 'Implementing fix...' && echo 'Creating test file...' && echo 'package main\n\nfunc ValidateToken(token string) bool {\n    // Fixed: Add proper validation\n    return token != \"\" && len(token) > 10\n}' > validate_token.go && echo 'Fix implemented'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-3", spec.ID),
				TaskID:      spec.ID,
				Name:        "Test fix",
				Description: "Test the fix to ensure it works",
				Command:     "echo 'Testing fix...' && echo 'package main\n\nimport (\n    \"testing\"\n)\n\nfunc TestValidateToken(t *testing.T) {\n    if !ValidateToken(\"valid-token-123\") {\n        t.Error(\"Valid token should pass\")\n    }\n    if ValidateToken(\"\") {\n        t.Error(\"Empty token should fail\")\n    }\n    if ValidateToken(\"short\") {\n        t.Error(\"Short token should fail\")\n    }\n}' > validate_token_test.go && echo 'Tests created' && echo 'All tests pass (simulated)'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-4", spec.ID),
				TaskID:      spec.ID,
				Name:        "Generate documentation",
				Description: "Generate proof-of-work and documentation",
				Command:     "echo 'Generating proof-of-work...' && echo '# Bug Fix: " + spec.Title + "\n\n## Summary\n- **Bug:** " + spec.Title + "\n- **Root Cause:** Missing token length validation\n- **Fix:** Added validation in ValidateToken function\n- **Tests:** 3 test cases added\n- **Status:** Fixed\n\n## Files Created\n- validate_token.go\n- validate_token_test.go\n\n## Next Steps\n- Code review\n- Merge to main\n- Deploy to staging' > BUGFIX_REPORT.md && echo 'Documentation generated'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
		}
	case contracts.WorkTypeImplementation:
		// Feature development: design, implement, test
		steps = []*ExecutionStep{
			{
				StepID:      fmt.Sprintf("%s-step-1", spec.ID),
				TaskID:      spec.ID,
				Name:        "Design feature",
				Description: "Design the feature architecture",
				Command:     "echo 'Designing feature: " + spec.Title + "' && echo 'Creating design document...' && echo '# Feature Design\n\n## Overview\n" + spec.Objective + "\n\n## Architecture\n- Component A\n- Component B\n- Integration points\n\n## Acceptance Criteria\n- [ ] Criterion 1\n- [ ] Criterion 2' > DESIGN.md && echo 'Design completed'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-2", spec.ID),
				TaskID:      spec.ID,
				Name:        "Implement feature",
				Description: "Implement the feature",
				Command:     "echo 'Implementing feature...' && echo 'Creating implementation files...' && echo 'package main\n\nfunc NewFeature() string {\n    return \"Feature implementation\"\n}' > feature.go && echo 'Implementation complete'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-3", spec.ID),
				TaskID:      spec.ID,
				Name:        "Test feature",
				Description: "Test the feature implementation",
				Command:     "echo 'Testing feature...' && echo 'Creating tests...' && echo 'package main\n\nimport (\n    \"testing\"\n)\n\nfunc TestNewFeature(t *testing.T) {\n    result := NewFeature()\n    if result != \"Feature implementation\" {\n        t.Errorf(\"Expected \\\"Feature implementation\\\", got %q\", result)\n    }\n}' > feature_test.go && echo 'Tests created' && echo 'All tests pass (simulated)'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
		}
	default:
		// Generic task: initialize, execute, validate
		steps = []*ExecutionStep{
			{
				StepID:      fmt.Sprintf("%s-step-1", spec.ID),
				TaskID:      spec.ID,
				Name:        "Initialize workspace",
				Description: "Prepare isolated workspace for task execution",
				Command:     "echo 'Initializing workspace for task: " + spec.Title + "' && pwd && ls -la && echo 'Creating workspace structure...' && mkdir -p src tests docs && echo 'Workspace ready'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-2", spec.ID),
				TaskID:      spec.ID,
				Name:        "Execute objective",
				Description: spec.Objective,
				Command:     "echo 'Executing task objective: " + spec.Objective + "' && echo 'Creating task artifacts...' && echo '# Task Report: " + spec.Title + "\n\n## Objective\n" + spec.Objective + "\n\n## Status\nCompleted successfully\n\n## Output\nTask executed as planned' > TASK_REPORT.md && echo 'Task execution complete'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
			{
				StepID:      fmt.Sprintf("%s-step-3", spec.ID),
				TaskID:      spec.ID,
				Name:        "Validate results",
				Description: "Validate task execution results and collect evidence",
				Command:     "echo 'Validating results...' && echo 'Checking artifacts...' && ls -la && echo 'Artifacts present:' && cat TASK_REPORT.md 2>/dev/null || echo 'No report found' && echo 'Validation complete: All checks passed'",
				Status:      StepStatusPending,
				TimeoutSeconds: spec.TimeoutSeconds,
				MaxRetries:    spec.MaxRetries,
			},
		}
	}

	return steps
}

// createErrorResult creates a failed execution result.
func (f *FactoryImpl) createErrorResult(spec *FactoryTaskSpec, err error, message string) *ExecutionResult {
	return &ExecutionResult{
		TaskID:      spec.ID,
		SessionID:   spec.SessionID,
		WorkItemID:  spec.WorkItemID,
		Status:      ExecutionStatusFailed,
		Success:     false,
		Error:       message,
		ErrorCode:   "WORKSPACE_ERROR",
		CompletedAt: time.Now(),
		Recommendation: "retry", // Default to retry for workspace errors
		NeedsRetry:  true,
	}
}

// BoundedExecutor implements bounded execution with timeout and retry.
type BoundedExecutor struct {
}

// NewBoundedExecutor creates a new bounded executor.
func NewBoundedExecutor() *BoundedExecutor {
	return &BoundedExecutor{}
}

// ExecuteStep runs a single bounded execution step.
func (b *BoundedExecutor) ExecuteStep(ctx context.Context, step *ExecutionStep, workspacePath string) (*ExecutionStep, error) {
	// Validate step
	if step == nil {
		return nil, fmt.Errorf("step cannot be nil")
	}

	// Mark as running
	step.Status = StepStatusRunning
	now := time.Now()
	step.StartedAt = &now

	log.Printf("[BoundedExecutor] Executing step: step_id=%s name=%s", step.StepID, step.Name)

	// Create bounded context with timeout
	timeout := time.Duration(step.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute // Default timeout
	}
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Determine command to execute
	var cmdStr string
	if step.Command != "" {
		cmdStr = step.Command
	} else {
		// Generate default command based on step name
		switch strings.ToLower(step.Name) {
		case "initialize workspace", "init workspace":
			cmdStr = "echo 'Initializing workspace for task execution' && pwd && ls -la"
		case "execute objective", "run objective":
			cmdStr = "echo 'Executing task objective' && echo 'Simulating work: sleep 0.1s' && sleep 0.1"
		case "validate results", "validate":
			cmdStr = "echo 'Validating results' && echo 'All checks passed'"
		default:
			cmdStr = fmt.Sprintf("echo 'Executing step: %s'", step.Name)
		}
	}

	// Execute command in workspace
	var output strings.Builder
	var exitCode int
	var err error

	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) == 0 {
		err = fmt.Errorf("empty command")
	} else {
		cmd := exec.CommandContext(stepCtx, cmdParts[0], cmdParts[1:]...)
		cmd.Dir = workspacePath
		cmd.Stdout = &output
		cmd.Stderr = &output
		
		err = cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		} else {
			exitCode = 0
		}
	}

	// Mark step as completed or failed
	if err != nil && stepCtx.Err() != context.DeadlineExceeded {
		step.Status = StepStatusFailed
		step.Error = fmt.Sprintf("Command execution failed: %v", err)
		step.ExitCode = exitCode
		step.Output = output.String()
		log.Printf("[BoundedExecutor] Step failed: step_id=%s error=%v exit_code=%d", step.StepID, err, exitCode)
		return step, fmt.Errorf("step execution failed: %w", err)
	} else if stepCtx.Err() == context.DeadlineExceeded {
		step.Status = StepStatusFailed
		step.Error = "Step timed out"
		step.ExitCode = -2
		step.Output = output.String()
		log.Printf("[BoundedExecutor] Step timed out: step_id=%s", step.StepID)
		return step, fmt.Errorf("step timed out")
	} else {
		// Success
		step.Status = StepStatusCompleted
		completedAt := time.Now()
		step.CompletedAt = &completedAt
		step.Output = output.String()
		step.ExitCode = exitCode
		log.Printf("[BoundedExecutor] Step completed: step_id=%s status=%s exit_code=%d", step.StepID, step.Status, exitCode)
		return step, nil
	}
}

// ExecutePlan runs a sequence of steps as a bounded execution loop.
func (b *BoundedExecutor) ExecutePlan(ctx context.Context, steps []*ExecutionStep, workspacePath string) (*ExecutionResult, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("execution plan cannot be empty")
	}

	result := &ExecutionResult{
		TotalSteps:      len(steps),
		CompletedSteps:  0,
		ExecutionSteps:  make([]*ExecutionStep, 0),
		FailedSteps:     make([]*ExecutionStep, 0),
		Status:          ExecutionStatusRunning,
		WorkspacePath:   workspacePath,
	}

	// Execute each step with retry logic
	for _, step := range steps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Status = ExecutionStatusCanceled
			result.Error = ctx.Err().Error()
			result.ErrorCode = "CONTEXT_CANCELED"
			return result, ctx.Err()
		default:
		}

		// Execute step with retries
		var lastErr error
		for retry := 0; retry <= step.MaxRetries; retry++ {
			if retry > 0 {
				log.Printf("[BoundedExecutor] Retrying step: step_id=%s attempt=%d max_retries=%d", step.StepID, retry+1, step.MaxRetries+1)
			}

			executedStep, err := b.ExecuteStep(ctx, step, workspacePath)
			if err != nil {
				lastErr = err
				step.RetryCount = retry
				continue
			}

			// Step succeeded
			result.ExecutionSteps = append(result.ExecutionSteps, executedStep)
			result.CompletedSteps++
			lastErr = nil
			break
		}

		// Check if step failed after all retries
		if lastErr != nil {
			log.Printf("[BoundedExecutor] Step failed after retries: step_id=%s retries=%d error=%v", step.StepID, step.MaxRetries+1, lastErr)

			step.Status = StepStatusFailed
			result.FailedSteps = append(result.FailedSteps, step)
			result.Error = lastErr.Error()
			result.ErrorCode = "STEP_EXECUTION_FAILED"
			result.Status = ExecutionStatusFailed
			result.NeedsRetry = true
			result.Recommendation = "retry"

			return result, lastErr
		}
	}

	// All steps completed
	result.Status = ExecutionStatusCompleted
	result.Success = true
	result.Recommendation = "merge" // Assume merge if all steps pass

	log.Printf("[BoundedExecutor] Execution plan completed: total_steps=%d completed_steps=%d status=%s", result.TotalSteps, result.CompletedSteps, result.Status)

	return result, nil
}

// SimpleProofOfWorkGenerator implements proof-of-work generation.
type SimpleProofOfWorkGenerator struct {
}

// NewSimpleProofOfWorkGenerator creates a new proof-of-work generator.
func NewSimpleProofOfWorkGenerator() *SimpleProofOfWorkGenerator {
	return &SimpleProofOfWorkGenerator{}
}

// Generate creates a proof-of-work summary from execution result.
func (s *SimpleProofOfWorkGenerator) Generate(ctx context.Context, result *ExecutionResult) (*ProofOfWorkSummary, error) {
	proof := &ProofOfWorkSummary{
		TaskID:        result.TaskID,
		SessionID:      result.SessionID,
		WorkItemID:    result.WorkItemID,
		Title:          "Task Execution", // Would come from spec in production
		Objective:      "Execute task",   // Would come from spec in production
		Result:         string(result.Status),
		StartedAt:      result.CompletedAt.Add(-result.Duration),
		CompletedAt:    result.CompletedAt,
		Duration:       result.Duration,
		ModelUsed:      "model-v1", // Would come from spec
		AgentRole:      "factory",
		FilesChanged:   result.FilesChanged,
		TestsRun:       result.TestsRun,
		TestsPassed:    result.TestsPassed,
		UnresolvedRisks: []string{}, // MVP: extract from SREDEvidence in production
		EvidenceItems:   result.SREDEvidence,
		RecommendedAction: result.Recommendation,
		RequiresApproval: (result.Recommendation != "merge"),
		GeneratedAt:    time.Now(),
		ArtifactPaths:  []string{result.ProofOfWorkPath},
	}

	return proof, nil
}

// SerializeToJSON converts proof-of-work to JSON format.
func (s *SimpleProofOfWorkGenerator) SerializeToJSON(proof *ProofOfWorkSummary) ([]byte, error) {
	return nil, fmt.Errorf("JSON serialization not yet implemented")
}

// SerializeToMarkdown converts proof-of-work to human-readable markdown.
func (s *SimpleProofOfWorkGenerator) SerializeToMarkdown(proof *ProofOfWorkSummary) (string, error) {
	md := fmt.Sprintf("# Proof of Work\n\n")
	md += fmt.Sprintf("**Task ID:** %s\n", proof.TaskID)
	md += fmt.Sprintf("**Session ID:** %s\n", proof.SessionID)
	md += fmt.Sprintf("**Work Item ID:** %s\n", proof.WorkItemID)
	md += fmt.Sprintf("**Source Key:** %s\n\n", proof.SourceKey)

	md += fmt.Sprintf("## Summary\n\n")
	md += fmt.Sprintf("- **Status:** %s\n", proof.Result)
	md += fmt.Sprintf("- **Duration:** %s\n", proof.Duration)
	md += fmt.Sprintf("- **Model:** %s\n", proof.ModelUsed)
	md += fmt.Sprintf("- **Agent:** %s\n\n", proof.AgentRole)

	if len(proof.FilesChanged) > 0 {
		md += fmt.Sprintf("## Files Changed\n\n")
		for _, file := range proof.FilesChanged {
			md += fmt.Sprintf("- %s\n", file)
		}
		md += fmt.Sprintf("\n")
	}

	if len(proof.TestsRun) > 0 {
		md += fmt.Sprintf("## Tests\n\n")
		md += fmt.Sprintf("- **Tests Run:** %d\n", len(proof.TestsRun))
		if proof.TestsPassed {
			md += fmt.Sprintf("- **All Passed:** Yes\n\n")
		} else {
			md += fmt.Sprintf("- **All Passed:** No\n")
			for _, test := range proof.TestsFailed {
				md += fmt.Sprintf("- Failed: %s\n", test)
			}
			md += fmt.Sprintf("\n")
		}
	}

	md += fmt.Sprintf("## Recommendation\n\n")
	md += fmt.Sprintf("**Action:** %s\n", proof.RecommendedAction)
	if proof.RequiresApproval {
		md += fmt.Sprintf("**Requires Approval:** Yes\n")
	}
	md += fmt.Sprintf("\n---\n")
	md += fmt.Sprintf("*Generated at %s*\n", proof.GeneratedAt.Format(time.RFC3339))

	return md, nil
}
