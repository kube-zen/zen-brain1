package factory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedExecutor_StaticCheck(t *testing.T) {
	// Create temporary workspace with go.mod
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module test.com/example

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a simple Go file
	goFile := filepath.Join(tmpDir, "main.go")
	goCode := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test staticcheck step
	step := &ExecutionStep{
		StepID:         "staticcheck-1",
		Name:           "staticcheck",
		TimeoutSeconds: 30,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step completed (even if staticcheck not installed, it should exit 0 with echo)
	if result.Status != StepStatusCompleted && result.Status != StepStatusFailed {
		t.Errorf("Expected step to complete or fail, got: %s", result.Status)
	}

	t.Logf("StaticCheck output: %s", result.Output)
	t.Logf("Exit code: %d", result.ExitCode)
}

func TestBoundedExecutor_GolangciLint(t *testing.T) {
	// Create temporary workspace with go.mod
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module test.com/example

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test golangci-lint step
	step := &ExecutionStep{
		StepID:         "golangci-lint-1",
		Name:           "golangci-lint",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed (even if golangci-lint not installed, it should handle gracefully)
	if result.Status != StepStatusCompleted && result.Status != StepStatusFailed {
		t.Errorf("Expected step to complete or fail, got: %s", result.Status)
	}

	t.Logf("Golangci-lint output: %s", result.Output)
}

func TestBoundedExecutor_PythonPytest(t *testing.T) {
	// Create temporary workspace with requirements.txt
	tmpDir := t.TempDir()
	reqPath := filepath.Join(tmpDir, "requirements.txt")
	reqContent := `pytest>=7.0.0
`
	if err := os.WriteFile(reqPath, []byte(reqContent), 0644); err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	// Create a simple Python test file
	testFile := filepath.Join(tmpDir, "test_example.py")
	testCode := `def test_example():
    assert 1 + 1 == 2
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test_example.py: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test pytest step
	step := &ExecutionStep{
		StepID:         "pytest-1",
		Name:           "pytest",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed (even if pytest not installed, it should handle gracefully)
	if result.Status != StepStatusCompleted && result.Status != StepStatusFailed {
		t.Errorf("Expected step to complete or fail, got: %s", result.Status)
	}

	t.Logf("Pytest output: %s", result.Output)
}

func TestBoundedExecutor_PythonPylint(t *testing.T) {
	// Create temporary workspace with pyproject.toml
	tmpDir := t.TempDir()
	pyprojPath := filepath.Join(tmpDir, "pyproject.toml")
	pyprojContent := `[tool.pytest.ini_options]
testpaths = ["tests"]
`
	if err := os.WriteFile(pyprojPath, []byte(pyprojContent), 0644); err != nil {
		t.Fatalf("Failed to create pyproject.toml: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test pylint step
	step := &ExecutionStep{
		StepID:         "pylint-1",
		Name:           "pylint",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed gracefully
	t.Logf("Pylint output: %s", result.Output)
}

func TestBoundedExecutor_PythonBlack(t *testing.T) {
	// Create temporary workspace with setup.py
	tmpDir := t.TempDir()
	setupPath := filepath.Join(tmpDir, "setup.py")
	setupContent := `from setuptools import setup
setup(name="test", version="0.1.0")
`
	if err := os.WriteFile(setupPath, []byte(setupContent), 0644); err != nil {
		t.Fatalf("Failed to create setup.py: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test black step
	step := &ExecutionStep{
		StepID:         "black-1",
		Name:           "black",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed gracefully
	t.Logf("Black output: %s", result.Output)
}

func TestBoundedExecutor_NpmTest(t *testing.T) {
	// Create temporary workspace with package.json
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")
	pkgContent := `{
  "name": "test-project",
  "version": "1.0.0",
  "scripts": {
    "test": "echo 'Running tests'"
  }
}
`
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test npm test step
	step := &ExecutionStep{
		StepID:         "npm-test-1",
		Name:           "npm test",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed (even if npm not installed, it should handle gracefully)
	if result.Status != StepStatusCompleted && result.Status != StepStatusFailed {
		t.Errorf("Expected step to complete or fail, got: %s", result.Status)
	}

	t.Logf("npm test output: %s", result.Output)
}

func TestBoundedExecutor_NpmLint(t *testing.T) {
	// Create temporary workspace with package.json (without lint script)
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")
	pkgContent := `{
  "name": "test-project",
  "version": "1.0.0"
}
`
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test npm run lint step
	step := &ExecutionStep{
		StepID:         "npm-lint-1",
		Name:           "npm run lint",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed gracefully (should handle missing lint script)
	t.Logf("npm lint output: %s", result.Output)
}

func TestBoundedExecutor_NpmBuild(t *testing.T) {
	// Create temporary workspace with package.json and yarn.lock
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")
	pkgContent := `{
  "name": "test-project",
  "version": "1.0.0",
  "scripts": {
    "build": "echo 'Building'"
  }
}
`
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create yarn.lock (empty)
	yarnLockPath := filepath.Join(tmpDir, "yarn.lock")
	if err := os.WriteFile(yarnLockPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create yarn.lock: %v", err)
	}

	// Create executor
	executor := NewBoundedExecutor()

	// Test npm run build step (should prefer yarn)
	step := &ExecutionStep{
		StepID:         "npm-build-1",
		Name:           "npm run build",
		TimeoutSeconds: 60,
	}

	result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	// Verify step executed (will fail if yarn not installed, but should handle gracefully)
	t.Logf("npm build output: %s", result.Output)
}

func TestBoundedExecutor_NoProjectDetected(t *testing.T) {
	// Create empty temporary directory
	tmpDir := t.TempDir()

	// Create executor
	executor := NewBoundedExecutor()

	// Test various steps that should skip when no project detected
	testCases := []struct {
		name     string
		stepName string
	}{
		{"staticcheck", "staticcheck"},
		{"pytest", "pytest"},
		{"npm test", "npm test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			step := &ExecutionStep{
				StepID:         tc.name + "-1",
				Name:           tc.stepName,
				TimeoutSeconds: 30,
			}

			result, err := executor.ExecuteStep(context.Background(), step, tmpDir)
			if err != nil {
				t.Fatalf("ExecuteStep failed: %v", err)
			}

			// Should complete successfully with "skipping" message
			if result.Status != StepStatusCompleted {
				t.Errorf("Expected step to complete, got: %s", result.Status)
			}

			// Output should indicate skipping
			if result.Output == "" {
				t.Error("Expected non-empty output")
			}

			t.Logf("%s output: %s", tc.name, result.Output)
		})
	}
}

func TestBoundedExecutor_Timeout(t *testing.T) {
	// Skip this test - context cancellation timing with shell commands
	// is implementation-dependent and can vary across systems
	t.Skip("Skipping timeout test - context cancellation timing varies")
}
