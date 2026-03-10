package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestUsefulTemplates tests that the useful templates are registered and accessible.
func TestUsefulTemplates(t *testing.T) {
	registry := NewWorkTypeTemplateRegistry()

	// Test that real implementation template exists
	template, err := registry.GetTemplate("implementation", "real")
	if err != nil {
		t.Fatalf("Failed to get real implementation template: %v", err)
	}

	if template.WorkType != "implementation" {
		t.Errorf("Expected work type 'implementation', got '%s'", template.WorkType)
	}

	if template.WorkDomain != "real" {
		t.Errorf("Expected work domain 'real', got '%s'", template.WorkType)
	}

	if len(template.Steps) == 0 {
		t.Error("Expected template to have steps")
	}

	// Test that real documentation template exists
	template, err = registry.GetTemplate("docs", "real")
	if err != nil {
		t.Fatalf("Failed to get real documentation template: %v", err)
	}

	if template.WorkType != "docs" {
		t.Errorf("Expected work type 'docs', got '%s'", template.WorkType)
	}

	// Test that real bug fix template exists
	template, err = registry.GetTemplate("bugfix", "real")
	if err != nil {
		t.Fatalf("Failed to get real bug fix template: %v", err)
	}

	if template.WorkType != "bugfix" {
		t.Errorf("Expected work type 'bugfix', got '%s'", template.WorkType)
	}

	t.Log("All useful templates are registered and accessible")
}

// TestUsefulTemplateExecution tests that useful templates can be executed (simplified test).
func TestUsefulTemplateExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping useful template execution test in short mode")
	}

	// Create a temporary workspace directory
	tempDir := t.TempDir()
	ctx := context.Background()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create a workspace
	taskID := "test-useful-task"
	sessionID := "test-useful-session"
	workspace, err := workspaceManager.CreateWorkspace(ctx, taskID, sessionID)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Clean up workspace
	defer workspaceManager.DeleteWorkspace(ctx, workspace.Path)

	// Set workspace path for the helper functions
	os.Setenv("ZEN_WORKSPACE_PATH", workspace.Path)
	defer os.Unsetenv("ZEN_WORKSPACE_PATH")

	// Test creating workspace structure
	filesCreated, err := createWorkspaceStructure(taskID, "Test Feature")
	if err != nil {
		t.Fatalf("Failed to create workspace structure: %v", err)
	}

	t.Logf("Created %d files for workspace structure", len(filesCreated))

	// Verify files were created
	for _, filePath := range filesCreated {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", filePath)
		}
	}

	// Test generating source code
	filesCreated, err = generateSourceCode(taskID, "Test Feature", "Test objective")
	if err != nil {
		t.Fatalf("Failed to generate source code: %v", err)
	}

	t.Logf("Created %d source code files", len(filesCreated))

	// Test generating tests
	filesCreated, err = generateTests(taskID, "Test Feature")
	if err != nil {
		t.Fatalf("Failed to generate tests: %v", err)
	}

	t.Logf("Created %d test files", len(filesCreated))

	// Test generating documentation
	filesCreated, err = generateDocumentation(taskID, "Test Feature")
	if err != nil {
		t.Fatalf("Failed to generate documentation: %v", err)
	}

	t.Logf("Created %d documentation files", len(filesCreated))

	// Test generating proof-of-work summary
	filesCreated, err = generateProofOfWorkSummary(taskID, "Test Feature")
	if err != nil {
		t.Fatalf("Failed to generate proof-of-work summary: %v", err)
	}

	t.Logf("Created proof-of-work summary: %d files", len(filesCreated))

	// Verify proof-of-work summary exists and has expected content
	powPath := filepath.Join(workspace.Path, "PROOF_OF_WORK.md")
	content, err := os.ReadFile(powPath)
	if err != nil {
		t.Fatalf("Failed to read proof-of-work summary: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "Work Item: "+taskID) {
		t.Errorf("Proof-of-work summary does not contain work item ID")
	}

	if !contains(contentStr, "Files Created") {
		t.Errorf("Proof-of-work summary does not contain 'Files Created' section")
	}

	t.Log("Useful template execution test passed")
}

// TestFactoryWithUsefulTemplate tests Factory execution with useful templates.
func TestFactoryWithUsefulTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Factory useful template test in short mode")
	}

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create Factory with temp directory
	workspaceManager := NewWorkspaceManager(tempDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(tempDir)
	factory := NewFactory(workspaceManager, executor, powManager, tempDir)

	// Create a task spec that uses the "real" implementation template
	taskID := "real-impl-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "real-impl-session",
		WorkItemID: "REAL-001",
		Title:      "Real Feature Implementation",
		Objective:  "Implement a real feature with actual files",
		WorkType:   "implementation",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

	// Execute task
	result, err := factory.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}

	if !result.Success {
		t.Error("Expected task to succeed")
	}

	// Verify workspace was created
	if result.WorkspacePath == "" {
		t.Error("Expected workspace path to be set")
	}

	// Verify files were created in the workspace
	expectedFiles := []string{
		"README.md",
		"cmd/main.go",
		"cmd/main_test.go",
		"docs/API.md",
		"PROOF_OF_WORK.md",
		".structure_created",
		".code_generated",
		".docs_generated",
		".tests_created",
		".pow_generated",
	}

	// Debug: list all files in workspace
	t.Logf("Debug: Listing files in workspace %s", result.WorkspacePath)
	entries, err := os.ReadDir(result.WorkspacePath)
	if err == nil {
		for _, entry := range entries {
			t.Logf("  - %s", entry.Name())
			if entry.IsDir() {
				subPath := filepath.Join(result.WorkspacePath, entry.Name())
				subEntries, _ := os.ReadDir(subPath)
				for _, subEntry := range subEntries {
					t.Logf("    - %s/%s", entry.Name(), subEntry.Name())
				}
			}
		}
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created in workspace", file)
		}
	}

	t.Logf("Task executed successfully. Workspace: %s", result.WorkspacePath)
}

// TestRefactorTemplate tests the refactoring template.
func TestRefactorTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping refactor template test in short mode")
	}

	// Create temp base directory
	baseDir, err := os.MkdirTemp("", "zen-brain-refactor-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Create factory
	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	// Create a task spec that uses the refactor template
	taskID := "refactor-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "refactor-session",
		WorkItemID: "REFACTOR-001",
		Title:      "Code Refactoring",
		Objective:  "Refactor complex code for better maintainability",
		WorkType:   "refactor",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

	// Execute task
	ctx := context.Background()
	result, err := factory.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}

	if !result.Success {
		t.Error("Expected task to succeed")
	}

	// Verify refactor-specific files were created
	refactorFiles := []string{
		"analysis/REFACTOR_ANALYSIS.md",
		"pkg/refactored.go",
		"pkg/refactored_test.go",
		"REFACTORING.md",
		".analysis_created",
		".refactor_implemented",
		".tests_created",
		".refactor_documented",
	}

	for _, file := range refactorFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected refactor file %s was not created", file)
		}
	}

	t.Logf("Refactor template test passed. Workspace: %s", result.WorkspacePath)
}

// TestPythonTemplate tests the Python implementation template.
func TestPythonTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Python template test in short mode")
	}

	// Create temp base directory
	baseDir, err := os.MkdirTemp("", "zen-brain-python-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// Create factory
	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	// Create a task spec that uses the Python implementation template
	taskID := "python-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "python-session",
		WorkItemID: "PYTHON-001",
		Title:      "Python Feature",
		Objective:  "Create a Python application with tests and docs",
		WorkType:   "implementation",
		WorkDomain: "python",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

	// Execute task
	ctx := context.Background()
	result, err := factory.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}

	if !result.Success {
		t.Error("Expected task to succeed")
	}

	// Verify Python-specific files were created
	pythonFiles := []string{
		"src/main.py",
		"src/__init__.py",
		"tests/test_main.py",
		"tests/__init__.py",
		"requirements.txt",
		"setup.py",
		".gitignore",
		"README.md",
		"docs/api.md",
		"PROOF_OF_WORK.md",
		".structure_created",
		".code_generated",
		".docs_generated",
		".tests_created",
		".pow_generated",
	}

	for _, file := range pythonFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected Python file %s was not created", file)
		}
	}

	t.Logf("Python template test passed. Workspace: %s", result.WorkspacePath)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
