package factory

import (
	"fmt"
)

// ErrorCode represents canonical error codes for factory operations.
type ErrorCode string

const (
	// Step execution errors
	ErrStepExecutionFailed  ErrorCode = "STEP_EXECUTION_FAILED"
	ErrStepTimeout          ErrorCode = "STEP_TIMEOUT"
	ErrStepInvalid          ErrorCode = "STEP_INVALID"
	ErrStepMaxRetriesExceeded ErrorCode = "STEP_MAX_RETRIES_EXCEEDED"
	
	// Workspace errors
	ErrWorkspaceAllocation  ErrorCode = "WORKSPACE_ALLOCATION_FAILED"
	ErrWorkspaceLock        ErrorCode = "WORKSPACE_LOCK_FAILED"
	ErrWorkspaceValidation  ErrorCode = "WORKSPACE_VALIDATION_FAILED"
	
	// Plan execution errors
	ErrPlanEmpty            ErrorCode = "PLAN_EMPTY"
	ErrPlanExecutionFailed  ErrorCode = "PLAN_EXECUTION_FAILED"
	ErrPlanTimeout          ErrorCode = "PLAN_TIMEOUT"
	
	// Proof-of-work errors
	ErrProofOfWorkGeneration ErrorCode = "PROOF_OF_WORK_GENERATION_FAILED"
	ErrProofOfWorkValidation ErrorCode = "PROOF_OF_WORK_VALIDATION_FAILED"
	
	// General errors
	ErrInvalidInput         ErrorCode = "INVALID_INPUT"
	ErrContextCanceled      ErrorCode = "CONTEXT_CANCELED"
	ErrInternal             ErrorCode = "INTERNAL_ERROR"
)

// FactoryError represents a structured error with code, message, and context.
type FactoryError struct {
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
	Details  string    `json:"details,omitempty"`
	StepID   string    `json:"step_id,omitempty"`
	TaskID   string    `json:"task_id,omitempty"`
	ExitCode int       `json:"exit_code,omitempty"`
	Cause    error     `json:"-"`
}

// Error implements the error interface.
func (e *FactoryError) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Details != "" {
		msg += fmt.Sprintf(": %s", e.Details)
	}
	if e.StepID != "" {
		msg += fmt.Sprintf(" (step: %s)", e.StepID)
	}
	if e.TaskID != "" {
		msg += fmt.Sprintf(" (task: %s)", e.TaskID)
	}
	if e.ExitCode != 0 {
		msg += fmt.Sprintf(" (exit code: %d)", e.ExitCode)
	}
	if e.Cause != nil {
		msg += fmt.Sprintf(" (cause: %v)", e.Cause)
	}
	return msg
}

// Unwrap returns the underlying cause error.
func (e *FactoryError) Unwrap() error {
	return e.Cause
}

// NewFactoryError creates a new FactoryError.
func NewFactoryError(code ErrorCode, message string) *FactoryError {
	return &FactoryError{
		Code:    code,
		Message: message,
	}
}

// WithDetails adds details to the error.
func (e *FactoryError) WithDetails(details string) *FactoryError {
	e.Details = details
	return e
}

// WithStepID adds step ID context.
func (e *FactoryError) WithStepID(stepID string) *FactoryError {
	e.StepID = stepID
	return e
}

// WithTaskID adds task ID context.
func (e *FactoryError) WithTaskID(taskID string) *FactoryError {
	e.TaskID = taskID
	return e
}

// WithExitCode adds exit code.
func (e *FactoryError) WithExitCode(exitCode int) *FactoryError {
	e.ExitCode = exitCode
	return e
}

// WithCause adds the underlying cause error.
func (e *FactoryError) WithCause(cause error) *FactoryError {
	e.Cause = cause
	return e
}

// IsFactoryError checks if an error is a FactoryError.
func IsFactoryError(err error) bool {
	_, ok := err.(*FactoryError)
	return ok
}

// GetErrorCode extracts the error code from an error.
func GetErrorCode(err error) ErrorCode {
	if fe, ok := err.(*FactoryError); ok {
		return fe.Code
	}
	return ErrInternal
}

// StepExecutionError creates a step execution error.
func StepExecutionError(stepID, message string, exitCode int, cause error) *FactoryError {
	return NewFactoryError(ErrStepExecutionFailed, message).
		WithStepID(stepID).
		WithExitCode(exitCode).
		WithCause(cause)
}

// StepTimeoutError creates a step timeout error.
func StepTimeoutError(stepID string) *FactoryError {
	return NewFactoryError(ErrStepTimeout, "step execution timed out").
		WithStepID(stepID)
}

// StepMaxRetriesError creates a max retries exceeded error.
func StepMaxRetriesError(stepID string, maxRetries int) *FactoryError {
	return NewFactoryError(ErrStepMaxRetriesExceeded, 
		fmt.Sprintf("step failed after %d retries", maxRetries)).
		WithStepID(stepID)
}

// WorkspaceError creates a workspace-related error.
func WorkspaceError(code ErrorCode, message, workspacePath string, cause error) *FactoryError {
	return NewFactoryError(code, message).
		WithDetails(fmt.Sprintf("workspace: %s", workspacePath)).
		WithCause(cause)
}