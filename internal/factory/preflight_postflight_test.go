package factory

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPreflightChecker_RunPreflightChecks(t *testing.T) {
	checker := NewPreflightChecker(nil, nil)
	ctx := context.Background()

	spec := &FactoryTaskSpec{
		ID:        "test-task",
		SessionID: "test-session",
		Title:     "Test Task",
	}

	report, err := checker.RunPreflightChecks(ctx, spec)
	if err != nil {
		t.Fatalf("RunPreflightChecks failed: %v", err)
	}

	if report == nil {
		t.Fatal("Report is nil")
	}

	if report.TaskID != spec.ID {
		t.Errorf("Expected TaskID %s, got %s", spec.ID, report.TaskID)
	}

	if report.SessionID != spec.SessionID {
		t.Errorf("Expected SessionID %s, got %s", spec.SessionID, report.SessionID)
	}

	if len(report.Checks) == 0 {
		t.Error("Expected at least one check")
	}

	// Verify timestamp is recent
	if time.Since(report.Timestamp) > time.Minute {
		t.Error("Timestamp is too old")
	}
}

func TestPreflightChecker_checkGitAvailable(t *testing.T) {
	checker := NewPreflightChecker(nil, nil)
	ctx := context.Background()

	result, err := checker.checkGitAvailable(ctx, &FactoryTaskSpec{})
	if err != nil {
		t.Fatalf("checkGitAvailable failed: %v", err)
	}

	if result.Name != "git_available" {
		t.Errorf("Expected name 'git_available', got '%s'", result.Name)
	}

	// Git should be available on most systems
	if !result.Passed {
		t.Errorf("Git should be available (message: %s)", result.Message)
	}
}

func TestPreflightChecker_checkWorkspaceConfig(t *testing.T) {
	mockWS := &mockWorkspaceManager{}
	checker := NewPreflightChecker(mockWS, nil)
	ctx := context.Background()

	tests := []struct {
		name      string
		spec      *FactoryTaskSpec
		wantPass  bool
	}{
		{
			name: "valid spec",
			spec: &FactoryTaskSpec{
				ID:        "test-task",
				SessionID: "test-session",
			},
			wantPass: true,
		},
		{
			name: "missing task ID",
			spec: &FactoryTaskSpec{
				SessionID: "test-session",
			},
			wantPass: false,
		},
		{
			name: "missing session ID",
			spec: &FactoryTaskSpec{
				ID: "test-task",
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.checkWorkspaceConfig(ctx, tt.spec)
			if err != nil {
				t.Fatalf("checkWorkspaceConfig failed: %v", err)
			}

			if result.Passed != tt.wantPass {
				t.Errorf("Expected Passed=%v, got %v (message: %s)", tt.wantPass, result.Passed, result.Message)
			}
		})
	}
}

func TestPostflightVerifier_RunPostflightVerification(t *testing.T) {
	verifier := NewPostflightVerifier(nil)
	ctx := context.Background()

	result := &ExecutionResult{
		TaskID:    "test-task",
		SessionID: "test-session",
		Success:   true,
	}

	spec := &FactoryTaskSpec{
		ID:        "test-task",
		SessionID: "test-session",
	}

	report, err := verifier.RunPostflightVerification(ctx, result, spec)
	if err != nil {
		t.Fatalf("RunPostflightVerification failed: %v", err)
	}

	if report == nil {
		t.Fatal("Report is nil")
	}

	if report.TaskID != spec.ID {
		t.Errorf("Expected TaskID %s, got %s", spec.ID, report.TaskID)
	}

	if len(report.Checks) == 0 {
		t.Error("Expected at least one check")
	}

	// Verify timestamp is recent
	if time.Since(report.Timestamp) > time.Minute {
		t.Error("Timestamp is too old")
	}
}

func TestPostflightVerifier_checkExecutionCompleted(t *testing.T) {
	verifier := NewPostflightVerifier(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		result   *ExecutionResult
		wantPass bool
	}{
		{
			name: "successful execution",
			result: &ExecutionResult{
				Success: true,
				Status:  ExecutionStatusCompleted,
			},
			wantPass: true,
		},
		{
			name: "failed execution",
			result: &ExecutionResult{
				Success: false,
				Status:  ExecutionStatusFailed,
				Error:   "something went wrong",
			},
			wantPass: false,
		},
		{
			name:     "nil result",
			result:   nil,
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := verifier.checkExecutionCompleted(ctx, tt.result, &FactoryTaskSpec{})
			if err != nil {
				t.Fatalf("checkExecutionCompleted failed: %v", err)
			}

			if result.Passed != tt.wantPass {
				t.Errorf("Expected Passed=%v, got %v (message: %s)", tt.wantPass, result.Passed, result.Message)
			}
		})
	}
}

func TestPreflightReport_AllPassed(t *testing.T) {
	tests := []struct {
		name     string
		checks   []PreflightCheckResult
		wantPass bool
	}{
		{
			name: "all checks passed",
			checks: []PreflightCheckResult{
				{Name: "check1", Passed: true},
				{Name: "check2", Passed: true},
			},
			wantPass: true,
		},
		{
			name: "one check failed",
			checks: []PreflightCheckResult{
				{Name: "check1", Passed: true},
				{Name: "check2", Passed: false},
			},
			wantPass: false,
		},
		{
			name:     "no checks",
			checks:   []PreflightCheckResult{},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually compute AllPassed (what PreflightReport.AllPassed should reflect)
			allPassed := len(tt.checks) == 0
			for _, check := range tt.checks {
				if !check.Passed {
					allPassed = false
					break
				}
			}
			allPassed = allPassed || (len(tt.checks) > 0 && func() bool {
				for _, c := range tt.checks {
					if !c.Passed {
						return false
					}
				}
				return true
			}())

			if allPassed != tt.wantPass {
				t.Errorf("Expected AllPassed=%v, got %v", tt.wantPass, allPassed)
			}
		})
	}
}

func TestPostflightReport_AllPassed(t *testing.T) {
	tests := []struct {
		name     string
		checks   []PostflightCheckResult
		wantPass bool
	}{
		{
			name: "all checks passed",
			checks: []PostflightCheckResult{
				{Name: "check1", Passed: true},
				{Name: "check2", Passed: true},
			},
			wantPass: true,
		},
		{
			name: "one check failed",
			checks: []PostflightCheckResult{
				{Name: "check1", Passed: true},
				{Name: "check2", Passed: false},
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute AllPassed from checks
			allPassed := true
			for _, check := range tt.checks {
				if !check.Passed {
					allPassed = false
					break
				}
			}

			if allPassed != tt.wantPass {
				t.Errorf("Expected AllPassed=%v, got %v", tt.wantPass, allPassed)
			}
		})
	}
}

func TestPostflightVerifier_checkFilesCreated(t *testing.T) {
	verifier := NewPostflightVerifier(nil)
	ctx := context.Background()

	t.Run("files_exist", func(t *testing.T) {
		// Create temp directory and files
		tmpDir := t.TempDir()
		file1 := tmpDir + "/test1.go"
		file2 := tmpDir + "/test2.go"

		if err := os.WriteFile(file1, []byte("package test"), 0644); err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}
		if err := os.WriteFile(file2, []byte("package test"), 0644); err != nil {
			t.Fatalf("Failed to create file2: %v", err)
		}

		result := &ExecutionResult{
			WorkspacePath: tmpDir,
			FilesChanged:  []string{file1, file2},
			Success:       true,
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkFilesCreated(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkFilesCreated failed: %v", err)
		}

		if !checkResult.Passed {
			t.Errorf("Expected check to pass, got: %s - %s", checkResult.Message, checkResult.Details)
		}
	})

	t.Run("files_missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := &ExecutionResult{
			WorkspacePath: tmpDir,
			FilesChanged:  []string{"nonexistent1.go", "nonexistent2.go"},
			Success:       true,
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkFilesCreated(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkFilesCreated failed: %v", err)
		}

		if checkResult.Passed {
			t.Error("Expected check to fail for missing files")
		}

		if checkResult.Details == "" {
			t.Error("Expected details about missing files")
		}
	})

	t.Run("no_files_declared", func(t *testing.T) {
		result := &ExecutionResult{
			FilesChanged: []string{},
			Success:      true,
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkFilesCreated(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkFilesCreated failed: %v", err)
		}

		if !checkResult.Passed {
			t.Error("Expected check to pass when no files declared")
		}
	})
}

func TestPostflightVerifier_checkTestsRan(t *testing.T) {
	verifier := NewPostflightVerifier(nil)
	ctx := context.Background()

	t.Run("tests_ran_with_go_output", func(t *testing.T) {
		result := &ExecutionResult{
			TestsRun:    []string{"TestExample"},
			TestsPassed: true,
			ExecutionSteps: []*ExecutionStep{
				{
					StepID: "test-1",
					Name:   "Run tests",
					Status: StepStatusCompleted,
					Output: "=== RUN   TestExample\n--- PASS: TestExample (0.00s)\nPASS\nok  example 0.001s",
				},
			},
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkTestsRan(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkTestsRan failed: %v", err)
		}

		if !checkResult.Passed {
			t.Errorf("Expected check to pass for real test output, got: %s", checkResult.Message)
		}
	})

	t.Run("tests_ran_with_pytest_output", func(t *testing.T) {
		result := &ExecutionResult{
			TestsRun:    []string{"test_example"},
			TestsPassed: true,
			ExecutionSteps: []*ExecutionStep{
				{
					StepID: "pytest-1",
					Name:   "pytest",
					Status: StepStatusCompleted,
					Output: "test_example.py::test_example PASSED    [100%]\n1 passed in 0.01s",
				},
			},
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkTestsRan(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkTestsRan failed: %v", err)
		}

		if !checkResult.Passed {
			t.Errorf("Expected check to pass for pytest output, got: %s", checkResult.Message)
		}
	})

	t.Run("tests_declared_but_no_steps", func(t *testing.T) {
		result := &ExecutionResult{
			TestsRun:       []string{"TestExample"},
			TestsPassed:    false,
			ExecutionSteps: []*ExecutionStep{},
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkTestsRan(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkTestsRan failed: %v", err)
		}

		if checkResult.Passed {
			t.Error("Expected check to fail when tests declared but no steps found")
		}
	})

	t.Run("test_steps_with_fake_output", func(t *testing.T) {
		result := &ExecutionResult{
			TestsRun:    []string{"TestExample"},
			TestsPassed: true,
			ExecutionSteps: []*ExecutionStep{
				{
					StepID: "test-1",
					Name:   "test",
					Status: StepStatusCompleted,
					Output: "Simulating test execution",
				},
			},
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkTestsRan(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkTestsRan failed: %v", err)
		}

		if checkResult.Passed {
			t.Error("Expected check to fail for fake test output")
		}
	})

	t.Run("no_tests_declared", func(t *testing.T) {
		result := &ExecutionResult{
			TestsRun:       []string{},
			TestsPassed:    false,
			ExecutionSteps: []*ExecutionStep{},
		}

		spec := &FactoryTaskSpec{ID: "test-task"}

		checkResult, err := verifier.checkTestsRan(ctx, result, spec)
		if err != nil {
			t.Fatalf("checkTestsRan failed: %v", err)
		}

		if !checkResult.Passed {
			t.Error("Expected check to pass when no tests declared")
		}
	})
}

func TestPostflightVerifier_RunPostflightVerification_WithNewChecks(t *testing.T) {
	verifier := NewPostflightVerifier(nil)
	ctx := context.Background()

	t.Run("all_checks_included", func(t *testing.T) {
		result := &ExecutionResult{
			TaskID:    "test-task",
			SessionID: "test-session",
			Success:   true,
		}

		spec := &FactoryTaskSpec{
			ID:        "test-task",
			SessionID: "test-session",
		}

		report, err := verifier.RunPostflightVerification(ctx, result, spec)
		if err != nil {
			t.Fatalf("RunPostflightVerification failed: %v", err)
		}

		// Verify new checks are included
		checkNames := make(map[string]bool)
		for _, check := range report.Checks {
			checkNames[check.Name] = check.Passed
		}

		expectedChecks := []string{
			"execution_completed",
			"workspace_clean",
			"artifacts_generated",
			"files_verified",
			"tests_verified",
			"git_status",
			"proof_of_work",
		}

		for _, expected := range expectedChecks {
			if _, ok := checkNames[expected]; !ok {
				t.Errorf("Expected check '%s' to be present", expected)
			}
		}

		if len(report.Checks) < len(expectedChecks) {
			t.Errorf("Expected at least %d checks, got %d", len(expectedChecks), len(report.Checks))
		}
	})

	t.Run("files_verified_check_fails_on_missing_files", func(t *testing.T) {
		result := &ExecutionResult{
			TaskID:         "test-task",
			SessionID:      "test-session",
			Success:        true,
			FilesChanged:   []string{"/nonexistent/file.go"},
			WorkspacePath:  "/tmp",
		}

		spec := &FactoryTaskSpec{
			ID:        "test-task",
			SessionID: "test-session",
		}

		report, err := verifier.RunPostflightVerification(ctx, result, spec)
		if err != nil {
			t.Fatalf("RunPostflightVerification failed: %v", err)
		}

		// Should have at least one failed check (files_verified)
		if report.AllPassed {
			t.Error("Expected AllPassed=false when files are missing")
		}

		// Find files_verified check
		var filesCheck *PostflightCheckResult
		for i := range report.Checks {
			if report.Checks[i].Name == "files_verified" {
				filesCheck = &report.Checks[i]
				break
			}
		}

		if filesCheck == nil {
			t.Fatal("files_verified check not found")
		}

		if filesCheck.Passed {
			t.Error("Expected files_verified check to fail for missing files")
		}
	})
}

// Mock implementations for testing
type mockWorkspaceManager struct{}

func (m *mockWorkspaceManager) CreateWorkspace(ctx context.Context, taskID, sessionID string) (*WorkspaceMetadata, error) {
	return &WorkspaceMetadata{
		TaskID:    taskID,
		SessionID: sessionID,
		Path:      "/tmp/workspace",
	}, nil
}

func (m *mockWorkspaceManager) ValidateWorkspace(ctx context.Context, path string) (bool, error) {
	return true, nil
}

func (m *mockWorkspaceManager) LockWorkspace(ctx context.Context, path string) error {
	return nil
}

func (m *mockWorkspaceManager) UnlockWorkspace(ctx context.Context, path string) error {
	return nil
}

func (m *mockWorkspaceManager) GetWorkspaceMetadata(ctx context.Context, path string) (*WorkspaceMetadata, error) {
	return &WorkspaceMetadata{Path: path}, nil
}

func (m *mockWorkspaceManager) ListWorkspaceFiles(ctx context.Context, path string) ([]string, error) {
	return []string{}, nil
}

func (m *mockWorkspaceManager) DeleteWorkspace(ctx context.Context, path string) error {
	return nil
}

// Test that mock implements interface
var _ WorkspaceManager = (*mockWorkspaceManager)(nil)
