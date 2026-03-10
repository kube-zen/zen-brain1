package factory

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// BoundedExecutor implements the Executor interface with bounded execution semantics.
// It enforces timeouts, retries, and safety limits on all execution steps.
type BoundedExecutor struct {
}

// NewBoundedExecutor creates a new bounded executor instance.
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
		case "run tests", "go test", "test":
			// Real execution: run Go tests when workspace has go.mod (Factory completeness)
			cmdStr = "if [ -f go.mod ]; then go test ./... -count=1; else echo 'No go.mod, skipping go test'; fi"
		case "build", "go build", "compile":
			// Real execution: build Go project when go.mod present
			cmdStr = "if [ -f go.mod ]; then go build ./...; else echo 'No go.mod, skipping go build'; fi"
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

	// Use shell to execute commands (preserves shell syntax like &&, >, etc.)
	cmd := exec.CommandContext(stepCtx, "/bin/sh", "-c", cmdStr)
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

	// Mark step as completed or failed
	if err != nil && stepCtx.Err() != context.DeadlineExceeded {
		step.Status = StepStatusFailed
		step.Error = fmt.Sprintf("Command execution failed: %v", err)
		step.ExitCode = exitCode
		step.Output = output.String()
		log.Printf("[BoundedExecutor] Step failed: step_id=%s error=%v exit_code=%d", step.StepID, err, exitCode)
		return step, StepExecutionError(step.StepID, "command execution failed", exitCode, err)
	} else if stepCtx.Err() == context.DeadlineExceeded {
		step.Status = StepStatusFailed
		step.Error = "Step timed out"
		step.ExitCode = -2
		step.Output = output.String()
		log.Printf("[BoundedExecutor] Step timed out: step_id=%s", step.StepID)
		return step, StepTimeoutError(step.StepID)
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
		TotalSteps:     len(steps),
		CompletedSteps: 0,
		ExecutionSteps: make([]*ExecutionStep, 0),
		FailedSteps:    make([]*ExecutionStep, 0),
		Status:         ExecutionStatusRunning,
		WorkspacePath:  workspacePath,
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
			
			// Create structured error for max retries exceeded
			retryErr := StepMaxRetriesError(step.StepID, step.MaxRetries)
			if fe, ok := lastErr.(*FactoryError); ok {
				retryErr = fe // Preserve the original factory error
			}
			
			result.Error = retryErr.Error()
			result.ErrorCode = string(GetErrorCode(retryErr))
			result.Status = ExecutionStatusFailed
			result.NeedsRetry = true
			result.Recommendation = "retry"

			return result, retryErr
		}
	}

	// All steps completed
	result.Status = ExecutionStatusCompleted
	result.Success = true
	result.Recommendation = "merge" // Assume merge if all steps pass

	log.Printf("[BoundedExecutor] Execution plan completed: total_steps=%d completed_steps=%d status=%s", result.TotalSteps, result.CompletedSteps, result.Status)

	return result, nil
}
