package factory

import (
	"fmt"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestGenerateStructuredInputs(t *testing.T) {
	spec := &FactoryTaskSpec{
		Objective:   "Test objective",
		WorkType:    contracts.WorkTypeImplementation,
		WorkDomain:  contracts.DomainCore,
		Constraints: []string{"req1", "req2"},
	}

	result := &ExecutionResult{
		SessionID:  "session-123",
		WorkItemID: "ZB-123",
	}

	inputs := GenerateStructuredInputs(result, spec)

	if inputs.Objective != "Test objective" {
		t.Errorf("Expected objective 'Test objective', got: %s", inputs.Objective)
	}

	if inputs.WorkType != "implementation" {
		t.Errorf("Expected work type 'implementation', got: %s", inputs.WorkType)
	}

	if inputs.Context["session_id"] != "session-123" {
		t.Errorf("Expected session_id 'session-123', got: %v", inputs.Context["session_id"])
	}

	if len(inputs.Constraints) != 2 {
		t.Errorf("Expected 2 constraints, got: %d", len(inputs.Constraints))
	}

	t.Logf("✅ Structured inputs generated correctly")
}

func TestGenerateStructuredOutputs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		result := &ExecutionResult{
			Status:       ExecutionStatusCompleted,
			Success:      true,
			FilesChanged: []string{"NEW:file1.txt", "DEL:file2.txt", "MOD:file3.txt"},
			TestsRun:     []string{"Test1", "Test2"},
			TestsPassed:  true,
			ProofOfWorkPath: "/tmp/proof.json",
			LogPath:        "/tmp/log.txt",
			ExecutionSteps: []*ExecutionStep{
				{Status: StepStatusCompleted},
			},
			FailedSteps: []*ExecutionStep{},
		}

		outputs := GenerateStructuredOutputs(result)

		if outputs.Status != ExecutionStatusCompleted {
			t.Errorf("Expected status 'completed', got: %s", outputs.Status)
		}

		if outputs.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got: %d", outputs.ExitCode)
		}

		if len(outputs.FilesCreated) != 1 {
			t.Errorf("Expected 1 file created, got: %d", len(outputs.FilesCreated))
		}

		if outputs.FilesCreated[0] != "file1.txt" {
			t.Errorf("Expected created file 'file1.txt', got: %s", outputs.FilesCreated[0])
		}

		if len(outputs.FilesDeleted) != 1 {
			t.Errorf("Expected 1 file deleted, got: %d", len(outputs.FilesDeleted))
		}

		if len(outputs.Artifacts) != 2 {
			t.Errorf("Expected 2 artifacts, got: %d", len(outputs.Artifacts))
		}

		t.Logf("✅ Structured outputs (success) generated correctly")
	})

	t.Run("Failure", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:       ExecutionStatusFailed,
			Success:      false,
			CompletedAt:  now,
			FailedSteps: []*ExecutionStep{
				{
					Name:    "Test",
					ExitCode: 1,
					Error:   "Test failed",
				},
			},
		}

		outputs := GenerateStructuredOutputs(result)

		if outputs.Status != ExecutionStatusFailed {
			t.Errorf("Expected status 'failed', got: %s", outputs.Status)
		}

		if outputs.ExitCode != 1 {
			t.Errorf("Expected exit code 1, got: %d", outputs.ExitCode)
		}

		t.Logf("✅ Structured outputs (failure) generated correctly")
	})
}

func TestGenerateFailureAnalysis(t *testing.T) {
	t.Run("Test failure", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:      ExecutionStatusFailed,
			CompletedAt: now,
			FailedSteps: []*ExecutionStep{
				{
					Name:    "Run tests",
					Command: "go test ./...",
					ExitCode: 1,
					Error:   "FAIL: TestX (0.05s)",
				},
			},
		}

		analysis := GenerateFailureAnalysis(result)

		if analysis == nil {
			t.Fatal("Expected failure analysis, got nil")
		}

		if analysis.FailureMode != "test" {
			t.Errorf("Expected failure mode 'test', got: %s", analysis.FailureMode)
		}

		if analysis.FailedStep != "Run tests" {
			t.Errorf("Expected failed step 'Run tests', got: %s", analysis.FailedStep)
		}

		if !analysis.Recoverable {
			t.Error("Expected test failure to be recoverable")
		}

		if analysis.RecoveryPath != "retry" {
			t.Errorf("Expected recovery path 'retry', got: %s", analysis.RecoveryPath)
		}

		if len(analysis.SuggestedFixes) == 0 {
			t.Error("Expected suggested fixes")
		}

		t.Logf("✅ Test failure analysis generated correctly")
	})

	t.Run("Timeout failure", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:      ExecutionStatusFailed,
			CompletedAt: now,
			FailedSteps: []*ExecutionStep{
				{
					Name:    "Build",
					Command: "make build",
					ExitCode: 124, // timeout
					Output:  "Build timed out after 10m",
				},
			},
		}

		analysis := GenerateFailureAnalysis(result)

		if analysis.FailureMode != "timeout" {
			t.Errorf("Expected failure mode 'timeout', got: %s", analysis.FailureMode)
		}

		t.Logf("✅ Timeout failure analysis generated correctly")
	})

	t.Run("Workspace failure", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:      ExecutionStatusFailed,
			CompletedAt: now,
			FailedSteps: []*ExecutionStep{
				{
					Name:    "Git status",
					Command: "git status",
					ExitCode: 128,
					Error:   "fatal: not a git repository",
				},
			},
		}

		analysis := GenerateFailureAnalysis(result)

		if analysis.FailureMode != "workspace" {
			t.Errorf("Expected failure mode 'workspace', got: %s", analysis.FailureMode)
		}

		if analysis.RecoveryPath != "manual" {
			t.Errorf("Expected recovery path 'manual' for workspace error, got: %s", analysis.RecoveryPath)
		}

		t.Logf("✅ Workspace failure analysis generated correctly")
	})

	t.Run("Infra failure", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:      ExecutionStatusFailed,
			CompletedAt: now,
			FailedSteps: []*ExecutionStep{
				{
					Name:    "Download deps",
					Command: "go mod download",
					ExitCode: 1,
					Error:   "dial tcp: lookup go.uber.org on [::1]:53: read udp [::1]:53: read: connection refused",
				},
			},
		}

		analysis := GenerateFailureAnalysis(result)

		if analysis.FailureMode != "infra" {
			t.Errorf("Expected failure mode 'infra', got: %s", analysis.FailureMode)
		}

		if !analysis.Recoverable {
			t.Error("Expected infra failure to be recoverable")
		}

		t.Logf("✅ Infra failure analysis generated correctly")
	})

	t.Run("Success (no analysis)", func(t *testing.T) {
		now := time.Now()
		result := &ExecutionResult{
			Status:      ExecutionStatusCompleted,
			CompletedAt: now,
			FailedSteps: []*ExecutionStep{},
		}

		analysis := GenerateFailureAnalysis(result)

		if analysis != nil {
			t.Error("Expected no failure analysis for success, got analysis")
		}

		t.Logf("✅ Success correctly returns no failure analysis")
	})
}

func TestGenerateExecutionTimeline(t *testing.T) {
	now := time.Now()
	startTime := now.Add(-30 * time.Second)

	result := &ExecutionResult{
		CompletedAt: now,
		Duration:    30 * time.Second,
		ExecutionSteps: []*ExecutionStep{
			{
				StepID:      "step-1",
				Name:        "Format",
				Status:       StepStatusCompleted,
				StartedAt:    &startTime,
				CompletedAt:  &now,
				Output:       "formatted",
			},
		},
	}

	timeline := GenerateExecutionTimeline(result)

	if timeline.TaskStarted.IsZero() {
		t.Error("Expected non-zero task started time")
	}

	if timeline.TaskCompleted.IsZero() {
		t.Error("Expected non-zero task completed time")
	}

	if timeline.TotalDuration != 30*time.Second {
		t.Errorf("Expected duration 30s, got: %s", timeline.TotalDuration)
	}

	if len(timeline.Steps) != 1 {
		t.Errorf("Expected 1 step, got: %d", len(timeline.Steps))
	}

	step := timeline.Steps[0]
	if step.StepID != "step-1" {
		t.Errorf("Expected step ID 'step-1', got: %s", step.StepID)
	}

	if !step.Success {
		t.Error("Expected step success")
	}

	t.Logf("✅ Execution timeline generated correctly")
}

func TestCalculateProofQuality(t *testing.T) {
	t.Run("High quality proof", func(t *testing.T) {
		result := &ExecutionResult{
			FilesChanged:   []string{"file1.txt"},
			TestsRun:       []string{"Test1"},
			ExecutionSteps: []*ExecutionStep{
				{StartedAt: &time.Time{}, CompletedAt: &time.Time{}},
			},
			LogPath:        "/tmp/log.txt",
			ArtifactPaths:   []string{"/tmp/artifact.json"},
			TemplateKey:     "implementation:real",
			GitBranch:       "main",
			GitCommit:       "abc123",
		}

		quality := CalculateProofQuality(result)

		if quality.TimestampAccuracy <= 0.9 {
			t.Errorf("Expected high timestamp accuracy, got: %.2f", quality.TimestampAccuracy)
		}

		if quality.CompletenessScore <= 0.9 {
			t.Errorf("Expected high completeness score, got: %.2f", quality.CompletenessScore)
		}

		if quality.VerifiabilityScore <= 0.5 {
			t.Errorf("Expected verifiability score >= 0.5, got: %.2f", quality.VerifiabilityScore)
		}

		if quality.Reproducibility <= 0.9 {
			t.Errorf("Expected high reproducibility, got: %.2f", quality.Reproducibility)
		}

		if quality.OverallScore <= 0.7 {
			t.Errorf("Expected high overall score, got: %.2f", quality.OverallScore)
		}

		t.Logf("✅ High quality proof: overall=%.2f", quality.OverallScore)
	})

	t.Run("Low quality proof", func(t *testing.T) {
		result := &ExecutionResult{
			FilesChanged:   nil,
			TestsRun:       nil,
			ExecutionSteps: []*ExecutionStep{
				{StartedAt: nil, CompletedAt: nil},
			},
			LogPath:        "",
			ArtifactPaths:   nil,
			TemplateKey:     "",
			GitBranch:       "",
			GitCommit:       "",
		}

		quality := CalculateProofQuality(result)

		if quality.TimestampAccuracy != 0.0 {
			t.Errorf("Expected zero timestamp accuracy, got: %.2f", quality.TimestampAccuracy)
		}

		// With one step (even with nil timestamps), completeness gets 0.2
		if quality.CompletenessScore != 0.2 {
			t.Errorf("Expected completeness score 0.2 (one step), got: %.2f", quality.CompletenessScore)
		}

		// Verifiability has base score of 0.5
		if quality.VerifiabilityScore != 0.5 {
			t.Errorf("Expected base verifiability score 0.5, got: %.2f", quality.VerifiabilityScore)
		}

		if quality.Reproducibility != 0.0 {
			t.Errorf("Expected zero reproducibility, got: %.2f", quality.Reproducibility)
		}

		// Overall = (0 + 0.2 + 0.5 + 1.0 + 0) / 5 = 0.34
		if quality.OverallScore < 0.3 || quality.OverallScore > 0.4 {
			t.Errorf("Expected overall score ~0.34, got: %.2f", quality.OverallScore)
		}

		t.Logf("✅ Low quality proof: overall=%.2f", quality.OverallScore)
	})
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"a very long string that should be truncated", 10, "a very ..."},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			result := truncateStringNew(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected '%s', got: '%s'", tt.expected, result)
			}
		})
	}

	t.Logf("✅ String truncation works correctly")
}
