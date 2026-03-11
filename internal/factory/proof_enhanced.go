package factory

import (
	"fmt"
	"strings"
	"time"
)

// StructuredInputs represents structured input data for a task.
type StructuredInputs struct {
	Spec          *FactoryTaskSpec   `json:"spec,omitempty"`
	Template      *WorkTypeTemplate  `json:"template,omitempty"`
	Objective     string            `json:"objective"`
	WorkType      string            `json:"work_type"`
	WorkDomain    string            `json:"work_domain"`
	Context       map[string]any     `json:"context,omitempty"`
	Constraints   []string          `json:"constraints,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
}

// StructuredOutputs represents structured output data for a task.
type StructuredOutputs struct {
	Status          ExecutionStatus `json:"status"`
	Success         bool           `json:"success"`
	ExitCode        int            `json:"exit_code,omitempty"`
	FilesChanged    []string       `json:"files_changed,omitempty"`
	FilesCreated    []string       `json:"files_created,omitempty"`
	FilesDeleted    []string       `json:"files_deleted,omitempty"`
	TestsRun        []string       `json:"tests_run,omitempty"`
	TestsPassed     bool           `json:"tests_passed,omitempty"`
	Artifacts       []string       `json:"artifacts,omitempty"`
	DiffSummary     string         `json:"diff_summary,omitempty"`
	DetailedDiff    string         `json:"detailed_diff,omitempty"`
}

// FailureAnalysis represents detailed failure analysis.
type FailureAnalysis struct {
	FailureMode    string    `json:"failure_mode,omitempty"`         // test, timeout, validation, runtime, workspace, policy, infra
	FailureReason  string    `json:"failure_reason"`             // Human-readable explanation
	FailedStep     string    `json:"failed_step,omitempty"`        // Name of the failed step
	FailedAt       time.Time `json:"failed_at"`
	RetryCount     int       `json:"retry_count"`
	SuggestedFixes []string  `json:"suggested_fixes,omitempty"`
	RecoveryPath   string    `json:"recovery_path,omitempty"`      // "manual", "retry", "escalate", "alternate_template"
	Recoverable    bool      `json:"recoverable"`
}

// StepExecutionSummary represents a summary of step execution.
type StepExecutionSummary struct {
	StepID         string        `json:"step_id"`
	Name           string        `json:"name"`
	Command        string        `json:"command,omitempty"`
	Status         StepStatus    `json:"status"`
	StartedAt      time.Time     `json:"started_at"`
	CompletedAt    time.Time     `json:"completed_at"`
	Duration       time.Duration `json:"duration"`
	ExitCode       int           `json:"exit_code,omitempty"`
	Success        bool          `json:"success"`
	OutputSummary  string        `json:"output_summary,omitempty"`     // First 200 chars of output
	ErrorSummary   string        `json:"error_summary,omitempty"`      // First 200 chars of error
	RetryCount     int           `json:"retry_count,omitempty"`
	ResourceUsage  *ResourceUsage `json:"resource_usage,omitempty"`
}

// ResourceUsage represents resource usage for a step.
type ResourceUsage struct {
	MaxMemoryMB  int64         `json:"max_memory_mb,omitempty"`
	MaxCPUPercent float64       `json:"max_cpu_percent,omitempty"`
	Duration      time.Duration `json:"duration"`
}

// ExecutionTimeline represents a timeline of task execution.
type ExecutionTimeline struct {
	TaskStarted   time.Time               `json:"task_started"`
	TaskCompleted time.Time               `json:"task_completed"`
	TotalDuration time.Duration          `json:"total_duration"`
	Steps         []*StepExecutionSummary `json:"steps"`
	CriticalPath  []string               `json:"critical_path,omitempty"`  // Step IDs on critical path
	Checkpoints   []*ExecutionCheckpoint  `json:"checkpoints,omitempty"`
}

// ExecutionCheckpoint represents a checkpoint during execution.
type ExecutionCheckpoint struct {
	CheckpointID  string    `json:"checkpoint_id"`
	Timestamp     time.Time `json:"timestamp"`
	Description   string    `json:"description"`
	State         string    `json:"state"`
	SnapshotPath  string    `json:"snapshot_path,omitempty"`
}

// EnhancedProofOfWorkSummary extends the base summary with enhanced fields.
type EnhancedProofOfWorkSummary struct {
	*ProofOfWorkSummary

	// Enhanced fields
	Inputs         *StructuredInputs     `json:"inputs,omitempty"`
	Outputs        *StructuredOutputs    `json:"outputs,omitempty"`
	Failure        *FailureAnalysis      `json:"failure,omitempty"`
	Timeline       *ExecutionTimeline    `json:"timeline,omitempty"`
	ProofQuality   *ProofQualityMetrics `json:"proof_quality,omitempty"`
}

// ProofQualityMetrics represents quality metrics for a proof.
type ProofQualityMetrics struct {
	TimestampAccuracy    float64 `json:"timestamp_accuracy"`    // 0.0 to 1.0 - how accurate timestamps are
	CompletenessScore  float64 `json:"completeness_score"`   // 0.0 to 1.0 - how complete the proof is
	VerifiabilityScore float64 `json:"verifiability_score"` // 0.0 to 1.0 - how verifiable the proof is
	DataIntegrity     float64 `json:"data_integrity"`      // 0.0 to 1.0 - checksum verification passed
	Reproducibility   float64 `json:"reproducibility"`    // 0.0 to 1.0 - can it be reproduced?
	OverallScore      float64 `json:"overall_score"`       // 0.0 to 1.0 - overall proof quality
}

// GenerateStructuredInputs creates structured inputs from result and spec.
func GenerateStructuredInputs(result *ExecutionResult, spec *FactoryTaskSpec) *StructuredInputs {
	inputs := &StructuredInputs{
		Spec:        spec,
		Objective:    spec.Objective,
		WorkType:     string(spec.WorkType),
		WorkDomain:   string(spec.WorkDomain),
		Context:      make(map[string]any),
		Constraints:  spec.Constraints,
		Dependencies: []string{},
	}

	// Add context
	if result.SessionID != "" {
		inputs.Context["session_id"] = result.SessionID
	}
	if result.WorkItemID != "" {
		inputs.Context["work_item_id"] = result.WorkItemID
	}
	if result.TemplateKey != "" {
		inputs.Context["template"] = result.TemplateKey
	}

	return inputs
}

// GenerateStructuredOutputs creates structured outputs from result.
func GenerateStructuredOutputs(result *ExecutionResult) *StructuredOutputs {
	outputs := &StructuredOutputs{
		Status:       result.Status,
		Success:      result.Success,
		FilesChanged: result.FilesChanged,
		TestsRun:     result.TestsRun,
		TestsPassed:  result.TestsPassed,
		Artifacts:    []string{},
	}

	// Calculate exit code from failed steps
	if result.Status == ExecutionStatusFailed && len(result.FailedSteps) > 0 {
		outputs.ExitCode = result.FailedSteps[0].ExitCode
	} else if result.Status == ExecutionStatusCompleted {
		outputs.ExitCode = 0
	}

	// Categorize files
	outputs.FilesCreated = []string{}
	outputs.FilesDeleted = []string{}

	for _, file := range result.FilesChanged {
		if strings.HasPrefix(file, "NEW:") {
			outputs.FilesCreated = append(outputs.FilesCreated, strings.TrimPrefix(file, "NEW:"))
		} else if strings.HasPrefix(file, "DEL:") {
			outputs.FilesDeleted = append(outputs.FilesDeleted, strings.TrimPrefix(file, "DEL:"))
		}
	}

	// Add diff summary
	if result.DiffPath != "" {
		outputs.DetailedDiff = "See " + result.DiffPath
	}

	// Add artifacts
	if result.ProofOfWorkPath != "" {
		outputs.Artifacts = append(outputs.Artifacts, result.ProofOfWorkPath)
	}
	if result.LogPath != "" {
		outputs.Artifacts = append(outputs.Artifacts, result.LogPath)
	}

	return outputs
}

// GenerateFailureAnalysis creates detailed failure analysis from result.
func GenerateFailureAnalysis(result *ExecutionResult) *FailureAnalysis {
	if result.Status == ExecutionStatusCompleted {
		return nil
	}

	analysis := &FailureAnalysis{
		FailedAt:    result.CompletedAt,
		RetryCount:   0,
		Recoverable:  true,
		RecoveryPath: "retry",
	}

	// Determine failure mode and reason
	if len(result.FailedSteps) > 0 {
		failedStep := result.FailedSteps[0]
		analysis.FailedStep = failedStep.Name

		// Classify failure mode
		if strings.Contains(strings.ToLower(failedStep.Command), "test") {
			analysis.FailureMode = "test"
			analysis.FailureReason = fmt.Sprintf("Test execution failed: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Review test logs for failure details",
				"Check if tests are flaky or have dependencies",
				"Consider adding retries or fixing test logic",
			}
		} else if failedStep.ExitCode >= 128 || (failedStep.ExitCode != 0 && strings.Contains(strings.ToLower(failedStep.Output), "timeout")) {
			analysis.FailureMode = "timeout"
			analysis.FailureReason = fmt.Sprintf("Command timed out: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Increase timeout threshold",
				"Optimize command execution time",
				"Check for infinite loops or blocking operations",
			}
		} else if strings.Contains(strings.ToLower(failedStep.Error), "validation") ||
			strings.Contains(strings.ToLower(failedStep.Error), "invalid") {
			analysis.FailureMode = "validation"
			analysis.FailureReason = fmt.Sprintf("Validation failed: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Review input validation rules",
				"Check schema compatibility",
				"Verify data format requirements",
			}
		} else if strings.Contains(strings.ToLower(failedStep.Command), "git") ||
			strings.Contains(strings.ToLower(failedStep.Error), "git") ||
			strings.Contains(strings.ToLower(failedStep.Error), "repository") {
			analysis.FailureMode = "workspace"
			analysis.FailureReason = fmt.Sprintf("Workspace/Git error: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Clean git state (reset, stash, or rebase)",
				"Check repository permissions",
				"Verify git configuration",
			}
			// Only escalate for exit codes >= 128 for workspace errors
			if failedStep.ExitCode >= 128 {
				analysis.RecoveryPath = "escalate"
				analysis.Recoverable = false
			} else {
				analysis.RecoveryPath = "manual"
			}
		} else if strings.Contains(strings.ToLower(failedStep.Error), "policy") ||
			strings.Contains(strings.ToLower(failedStep.Error), "authorized") {
			analysis.FailureMode = "policy"
			analysis.FailureReason = fmt.Sprintf("Policy violation: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Review policy requirements",
				"Request necessary permissions",
				"Check approval workflows",
			}
		} else if strings.Contains(strings.ToLower(failedStep.Error), "connection") ||
			strings.Contains(strings.ToLower(failedStep.Error), "network") ||
			strings.Contains(strings.ToLower(failedStep.Error), "dial tcp") {
			analysis.FailureMode = "infra"
			analysis.FailureReason = fmt.Sprintf("Infrastructure error: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Check network connectivity",
				"Verify service availability",
				"Review firewall/VPN configuration",
			}
		} else {
			analysis.FailureMode = "runtime"
			analysis.FailureReason = fmt.Sprintf("Runtime error: %s", truncateStringNew(failedStep.Error, 200))
			analysis.SuggestedFixes = []string{
				"Review error logs for details",
				"Check code for bugs or edge cases",
				"Verify dependencies and configuration",
			}
		}
	} else {
		analysis.FailureMode = "unknown"
		analysis.FailureReason = "Task failed but no failed steps recorded"
		analysis.SuggestedFixes = []string{
			"Review execution logs for details",
			"Check system resources and configuration",
		}
	}

	return analysis
}

// GenerateExecutionTimeline creates a timeline of task execution.
func GenerateExecutionTimeline(result *ExecutionResult) *ExecutionTimeline {
	timeline := &ExecutionTimeline{
		TaskStarted:   result.CompletedAt.Add(-result.Duration),
		TaskCompleted: result.CompletedAt,
		TotalDuration: result.Duration,
		Steps:        []*StepExecutionSummary{},
		Checkpoints:   []*ExecutionCheckpoint{},
	}

	// Add step summaries
	for _, step := range result.ExecutionSteps {
		summary := &StepExecutionSummary{
			StepID:       step.StepID,
			Name:         step.Name,
			Command:      step.Command,
			Status:       step.Status,
			ExitCode:     step.ExitCode,
			Success:      step.Status == StepStatusCompleted,
			OutputSummary: truncateStringNew(step.Output, 200),
			ErrorSummary:  truncateStringNew(step.Error, 200),
			RetryCount:   0,
		}

		// Set timestamps
		if step.StartedAt != nil {
			summary.StartedAt = *step.StartedAt
		}
		if step.CompletedAt != nil {
			summary.CompletedAt = *step.CompletedAt
		}

		// Calculate duration
		if !summary.StartedAt.IsZero() && !summary.CompletedAt.IsZero() {
			summary.Duration = summary.CompletedAt.Sub(summary.StartedAt)
		}

		timeline.Steps = append(timeline.Steps, summary)
	}

	// Identify critical path (failed steps)
	criticalPath := []string{}
	for _, step := range timeline.Steps {
		if step.Status == StepStatusFailed {
			criticalPath = append(criticalPath, step.StepID)
		}
	}

	timeline.CriticalPath = criticalPath

	return timeline
}

// CalculateProofQuality calculates quality metrics for a proof.
func CalculateProofQuality(result *ExecutionResult) *ProofQualityMetrics {
	metrics := &ProofQualityMetrics{}

	// Timestamp accuracy (1.0 if all steps have timestamps)
	timestampCount := 0
	for _, step := range result.ExecutionSteps {
		if step.StartedAt != nil && step.CompletedAt != nil {
			timestampCount++
		}
	}
	if len(result.ExecutionSteps) > 0 {
		metrics.TimestampAccuracy = float64(timestampCount) / float64(len(result.ExecutionSteps))
	}

	// Completeness score (has files changed, tests run, etc.)
	completeness := 0.0
	if result.FilesChanged != nil && len(result.FilesChanged) > 0 {
		completeness += 0.3
	}
	if result.TestsRun != nil && len(result.TestsRun) > 0 {
		completeness += 0.3
	}
	if len(result.ExecutionSteps) > 0 {
		completeness += 0.2
	}
	if result.LogPath != "" {
		completeness += 0.2
	}
	metrics.CompletenessScore = completeness

	// Verifiability (has checksums, signatures, etc.)
	verifiability := 0.5 // Base score
	if result.ArtifactPaths != nil && len(result.ArtifactPaths) > 0 {
		verifiability += 0.3
	}
	// Checksums verified later
	metrics.VerifiabilityScore = verifiability

	// Data integrity (will be set after checksum verification)
	metrics.DataIntegrity = 1.0 // Default to 1.0, will be verified

	// Reproducibility (has template, git info, etc.)
	reproducibility := 0.0
	if result.TemplateKey != "" {
		reproducibility += 0.3
	}
	if result.GitBranch != "" {
		reproducibility += 0.3
	}
	if result.GitCommit != "" {
		reproducibility += 0.4
	}
	metrics.Reproducibility = reproducibility

	// Overall score (average of all metrics)
	metrics.OverallScore = (metrics.TimestampAccuracy +
		metrics.CompletenessScore +
		metrics.VerifiabilityScore +
		metrics.DataIntegrity +
		metrics.Reproducibility) / 5.0

	return metrics
}

// truncateStringNew truncates a string to max length with ellipsis.
// Named with "New" suffix to avoid conflict with existing truncateString.
func truncateStringNew(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
