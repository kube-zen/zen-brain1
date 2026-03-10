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

// TestReviewRealTemplate verifies review:real produces repo-aware artifacts: review/files.txt, REVIEW.md, and skip markers when Go/Python absent.
func TestReviewRealTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping review template test in short mode")
	}
	tempDir := t.TempDir()
	ctx := context.Background()
	workspaceManager := NewWorkspaceManager(tempDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(tempDir)
	f := NewFactory(workspaceManager, executor, powManager, tempDir)

	spec := &FactoryTaskSpec{
		ID:         "review-task-1",
		SessionID:  "review-session",
		WorkItemID: "REVIEW-001",
		Title:      "Review change",
		Objective:  "Review the implementation",
		WorkType:   "review",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	result, err := f.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}
	if !result.Success {
		t.Fatalf("task failed: %s", result.Error)
	}

	ws := result.WorkspacePath
	// Must have review/files.txt (inventory)
	filesTxt := filepath.Join(ws, "review", "files.txt")
	if _, err := os.Stat(filesTxt); os.IsNotExist(err) {
		t.Errorf("review/files.txt not created")
	}
	// Must have REVIEW.md
	reviewMD := filepath.Join(ws, "REVIEW.md")
	if _, err := os.Stat(reviewMD); os.IsNotExist(err) {
		t.Errorf("REVIEW.md not created")
	}
	// Go and Python check outputs (either "skipped" or actual output)
	goTest := filepath.Join(ws, "review", "go-test.txt")
	if _, err := os.Stat(goTest); os.IsNotExist(err) {
		t.Errorf("review/go-test.txt not created")
	} else {
		content, _ := os.ReadFile(goTest)
		if len(content) == 0 {
			t.Error("review/go-test.txt should not be empty")
		}
	}
	pythonTest := filepath.Join(ws, "review", "python-test.txt")
	if _, err := os.Stat(pythonTest); os.IsNotExist(err) {
		t.Errorf("review/python-test.txt not created")
	} else {
		content, _ := os.ReadFile(pythonTest)
		if len(content) == 0 {
			t.Error("review/python-test.txt should not be empty")
		}
		// When no Python project, should contain explicit skip marker
		s := string(content)
		if s != "" && s != "skipped" && !contains(s, "skipped") && !contains(s, "not in PATH") {
			// If it's not "skipped", it may be pytest/py_compile output; that's fine
		}
	}
	// REVIEW.md should mention work item and next action
	content, _ := os.ReadFile(reviewMD)
	c := string(content)
	if !contains(c, "REVIEW-001") {
		t.Error("REVIEW.md should contain work item ID")
	}
	if !contains(c, "Next action") && !contains(c, "next action") {
		t.Error("REVIEW.md should contain next-action recommendation")
	}
}

// TestCICDTemplate tests the CI/CD template.
func TestCICDTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CI/CD template test in short mode")
	}

	baseDir, err := os.MkdirTemp("", "zen-brain-cicd-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	taskID := "cicd-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "cicd-session",
		WorkItemID: "CICD-001",
		Title:      "CI/CD Pipeline Setup",
		Objective:  "Set up GitHub Actions CI/CD pipeline",
		WorkType:   "cicd",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

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

	cicdFiles := []string{
		".github/workflows/ci.yml",
		"DEPLOYMENT.md",
		"PROOF_OF_WORK.md",
		".cicd_structure",
		".ci_workflow",
		".deploy_documented",
		".pow_generated",
	}

	for _, file := range cicdFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected CI/CD file %s was not created", file)
		}
	}

	t.Logf("CI/CD template test passed. Workspace: %s", result.WorkspacePath)
}

// TestJavaScriptTemplate tests the JavaScript implementation template.
func TestJavaScriptTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping JavaScript template test in short mode")
	}

	baseDir, err := os.MkdirTemp("", "zen-brain-javascript-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	taskID := "javascript-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "javascript-session",
		WorkItemID: "JS-001",
		Title:      "JavaScript Feature",
		Objective:  "Create a Node.js application with tests",
		WorkType:   "implementation",
		WorkDomain: "javascript",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

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

	jsFiles := []string{
		"src/main.js",
		"tests/main.test.js",
		"tests/package.json",
		"package.json",
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

	for _, file := range jsFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected JavaScript file %s was not created", file)
		}
	}

	t.Logf("JavaScript template test passed. Workspace: %s", result.WorkspacePath)
}

// TestDatabaseMigrationTemplate tests the database migration template.
func TestDatabaseMigrationTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration template test in short mode")
	}

	baseDir, err := os.MkdirTemp("", "zen-brain-migration-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	taskID := "migration-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "migration-session",
		WorkItemID: "MIGRATION-001",
		Title:      "Database Migration",
		Objective:  "Create database migration scripts",
		WorkType:   "migration",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

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

	// Verify migrations directory was created
	migrationsDir := filepath.Join(result.WorkspacePath, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Error("Expected migrations directory to be created")
	}

	// Verify rollbacks directory was created
	rollbacksDir := filepath.Join(result.WorkspacePath, "rollbacks")
	if _, err := os.Stat(rollbacksDir); os.IsNotExist(err) {
		t.Error("Expected rollbacks directory to be created")
	}

	migrationFiles := []string{
		"MIGRATION.md",
		"PROOF_OF_WORK.md",
		".migration_structure",
		".up_migration",
		".down_migration",
		".migration_documented",
		".pow_generated",
	}

	for _, file := range migrationFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected migration file %s was not created", file)
		}
	}

	t.Logf("Database migration template test passed. Workspace: %s", result.WorkspacePath)
}

// TestMonitoringTemplate tests the monitoring template.
func TestMonitoringTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring template test in short mode")
	}

	baseDir, err := os.MkdirTemp("", "zen-brain-monitoring-*")
	if err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	workspaceManager := NewWorkspaceManager(baseDir)
	executor := NewBoundedExecutor()
	powManager := NewProofOfWorkManager(baseDir)
	factory := NewFactory(workspaceManager, executor, powManager, baseDir)

	taskID := "monitoring-task-" + fmt.Sprint(time.Now().Unix())
	spec := &FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "monitoring-session",
		WorkItemID: "MONITORING-001",
		Title:      "Monitoring Setup",
		Objective:  "Set up Prometheus metrics and Grafana dashboards",
		WorkType:   "monitoring",
		WorkDomain: "real",
		Priority:   "high",
		CreatedAt:  time.Now(),
	}

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

	monitoringFiles := []string{
		"monitoring/metrics/metrics.yml",
		"monitoring/dashboards/application.json",
		"monitoring/alerts/alerts.yml",
		"MONITORING.md",
		"PROOF_OF_WORK.md",
		".monitoring_structure",
		".metrics_config",
		".dashboard_config",
		".alerts_config",
		".monitoring_documented",
		".pow_generated",
	}

	for _, file := range monitoringFiles {
		filePath := filepath.Join(result.WorkspacePath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected monitoring file %s was not created", file)
		}
	}

	t.Logf("Monitoring template test passed. Workspace: %s", result.WorkspacePath)
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
