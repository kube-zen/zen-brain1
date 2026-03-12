package factory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestLLMIntegration_SimpleGeneration tests LLM generation with real LLM provider.
//
// This is an integration test that requires:
//   - OLLAMA_BASE_URL environment variable set
//   - Ollama service running
//
// Run with:
//   OLLAMA_BASE_URL=http://localhost:11434 go test -run TestLLMIntegration
func TestLLMIntegration_SimpleGeneration(t *testing.T) {
	// Skip if Ollama is not available
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		t.Skip("OLLAMA_BASE_URL not set - skipping integration test")
		return
	}

	// Create LLM gateway
	gw, err := llm.NewGateway(&llm.GatewayConfig{
		LocalWorkerModel: "qwen3.5:0.8b", // Use smaller model for faster tests
		PlannerModel:     "glm-4.7",
		FallbackModel:    "glm-4.7",
		RequestTimeout:   120,
	})

	if err != nil {
		t.Fatalf("Failed to create LLM gateway: %v", err)
	}

	// Get Ollama provider (returns Provider, bool)
	provider, exists := gw.GetProvider("ollama")
	if !exists {
		t.Fatalf("Ollama provider not found")
	}
	if provider == nil {
		t.Fatalf("Ollama provider is nil")
	}

	// Create LLM generator
	config := DefaultLLMGeneratorConfig(provider)
	config.EnableThinking = false // Disable for faster tests
	config.Timeout = 60 * time.Second
	config.MaxTokens = 2000 // Limit for faster tests

	generator, err := NewLLMGenerator(config)
	if err != nil {
		t.Fatalf("Failed to create LLM generator: %v", err)
	}

	t.Logf("LLM provider: %s", provider.Name())
	t.Logf("Model: %s", config.Model)
	t.Logf("Base URL: %s", baseURL)

	// Test 1: Simple Go implementation
	t.Run("simple_go_implementation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		req := &GenerationRequest{
			WorkItemID:  "INTG-TEST-001",
			Title:         "Add greeting function",
			Objective:     "Create a simple Greeting function that returns 'Hello, World!'",
			WorkType:      "implementation",
			ProjectType:   "go",
			PackageName:   "main",
		}

		result, err := generator.GenerateImplementation(ctx, req)
		if err != nil {
			t.Fatalf("GenerateImplementation() error: %v", err)
		}

		if result.Code == "" {
			t.Fatal("GenerateImplementation() returned empty code")
		}

		if result.Language != "go" {
			t.Errorf("Language = %v, want go", result.Language)
		}

		// Verify generated code contains expected elements
		code := result.Code
		if len(code) == 0 {
			t.Fatal("Generated code is empty")
		}

		// Should contain package declaration
		if !strings.Contains(code, "package main") && !strings.Contains(code, "package greeting") {
			t.Error("Generated code doesn't contain package declaration")
		}

		// Should contain function declaration
		if !strings.Contains(code, "func Greeting") && !strings.Contains(code, "func Hello") {
			t.Error("Generated code doesn't contain expected function")
		}

		t.Logf("Generated code (%d tokens):\n%s", result.TokensUsed, result.Code)
		t.Logf("Model used: %s", result.Model)
	})

	// Test 2: Bug fix generation
	t.Run("bug_fix_generation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		req := &GenerationRequest{
			WorkItemID:  "INTG-TEST-002",
			Title:         "Fix nil pointer check",
			Objective:     "Fix nil pointer dereference in user lookup",
			WorkType:      "bugfix",
			ProjectType:   "go",
			PackageName:   "user",
			ExistingCode:  "func GetUser(id int) *User {\n    return users[id]\n}",
		}

		result, err := generator.GenerateImplementation(ctx, req)
		if err != nil {
			t.Fatalf("GenerateImplementation() error: %v", err)
		}

		if result.Code == "" {
			t.Fatal("GenerateImplementation() returned empty code")
		}

		t.Logf("Generated bug fix (%d tokens):\n%s", result.TokensUsed, result.Code)
	})

	// Test 3: Test generation
	t.Run("test_generation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		req := &GenerationRequest{
			WorkItemID:  "INTG-TEST-003",
			Title:         "Add tests for counter",
			Objective:     "Create tests for Increment() and Decrement() functions",
			WorkType:      "test",
			ProjectType:   "go",
			PackageName:   "counter",
			ExistingCode:  "package counter\n\ntype Counter struct { value int }\n\nfunc (c *Counter) Increment() { c.value++ }\nfunc (c *Counter) Decrement() { c.value-- }",
		}

		result, err := generator.GenerateImplementation(ctx, req)
		if err != nil {
			t.Fatalf("GenerateImplementation() error: %v", err)
		}

		if result.Code == "" {
			t.Fatal("GenerateImplementation() returned empty code")
		}

		// Should contain test function
		if !strings.Contains(result.Code, "func Test") {
			t.Error("Generated code doesn't contain test functions")
		}

		t.Logf("Generated tests (%d tokens):\n%s", result.TokensUsed, result.Code)
	})
}

// TestLLMIntegration_TemplateExecutor tests full template execution workflow.
//
// This tests the LLMTemplateExecutor with:
//   - Workspace creation
//   - Project detection
//   - Code generation
//   - File writing
//   - Validation
func TestLLMIntegration_TemplateExecutor(t *testing.T) {
	// Skip if Ollama is not available
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		t.Skip("OLLAMA_BASE_URL not set - skipping integration test")
		return
	}

	// Create LLM gateway
	gw, err := llm.NewGateway(&llm.GatewayConfig{
		LocalWorkerModel: "qwen3.5:0.8b",
		PlannerModel:     "glm-4.7",
		FallbackModel:    "glm-4.7",
		RequestTimeout:   120,
	})

	if err != nil {
		t.Fatalf("Failed to create LLM gateway: %v", err)
	}

	provider, exists := gw.GetProvider("ollama")
	if !exists {
		t.Fatalf("Ollama provider not found")
	}
	if provider == nil {
		t.Fatalf("Ollama provider is nil")
	}

	// Create LLM generator
	config := DefaultLLMGeneratorConfig(provider)
	config.EnableThinking = false
	config.Timeout = 60 * time.Second

	generator, err := NewLLMGenerator(config)
	if err != nil {
		t.Fatalf("Failed to create LLM generator: %v", err)
	}

	// Create temporary workspace
	workspacePath := t.TempDir()

	// Create fake Go project structure
	goModPath := filepath.Join(workspacePath, "go.mod")
	goModContent := `module example.com/integration-test

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create factory config
	templateConfig := &LLMTemplateConfig{
		Type:             LLMTemplateImplementation,
		WorkType:         "implementation",
		WorkDomain:       "core",
		ValidateCode:     false, // Skip validation for test
		CreateTests:      false, // Don't create tests for speed
		CreateDocs:       false,
		GenerationTimeout: 60 * time.Second,
	}

	executor, err := NewLLMTemplateExecutor(generator, templateConfig)
	if err != nil {
		t.Fatalf("Failed to create template executor: %v", err)
	}

	// Create task spec
	spec := &FactoryTaskSpec{
		ID:            "INTG-TEST-004",
		WorkItemID:    "TEST-INTG-004",
		SessionID:     "SESSION-INTG-001",
		Title:         "Add service",
		Objective:     "Create a service with Get method",
		WorkType:      contracts.WorkTypeImplementation,
		WorkDomain:    "core",
		Priority:      contracts.PriorityHigh,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Execute LLM template
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	filesCreated, err := executor.Execute(ctx, spec, workspacePath)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if len(filesCreated) == 0 {
		t.Fatal("Execute() returned no files")
	}

	// Verify files were created
	for _, file := range filesCreated {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("File not created: %s: %v", file, err)
		} else {
			t.Logf("✓ File created: %s", file)
		}
	}

	// Verify generated code
	for _, file := range filesCreated {
		if filepath.Ext(file) == ".go" {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Errorf("Failed to read %s: %v", file, err)
				continue
			}

			code := string(content)
			if len(code) == 0 {
				t.Errorf("File %s is empty", file)
			} else {
				t.Logf("Generated code in %s (%d bytes)", file, len(code))
			}
		}
	}

	t.Logf("Total files created: %d", len(filesCreated))
}

// TestLLMIntegration_EndToEnd tests complete Factory workflow with LLM.
//
// This is the most comprehensive integration test, covering:
//   - Factory creation with LLM
//   - Task execution
//   - Workspace management
//   - Proof-of-work generation
func TestLLMIntegration_EndToEnd(t *testing.T) {
	// Skip if Ollama is not available
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		t.Skip("OLLAMA_BASE_URL not set - skipping integration test")
		return
	}

	t.Logf("Running end-to-end LLM integration test")
	t.Logf("Ollama URL: %s", baseURL)

	// Create LLM gateway
	gw, err := llm.NewGateway(&llm.GatewayConfig{
		LocalWorkerModel: "qwen3.5:0.8b",
		PlannerModel:     "glm-4.7",
		FallbackModel:    "glm-4.7",
		RequestTimeout:   120,
	})

	if err != nil {
		t.Fatalf("Failed to create LLM gateway: %v", err)
	}

	provider, exists := gw.GetProvider("ollama")
	if !exists {
		t.Fatalf("Ollama provider not found")
	}
	if provider == nil {
		t.Fatalf("Ollama provider is nil")
	}

	// Create Factory components
	runtimeDir := t.TempDir()
	workspaceManager := NewWorkspaceManager(runtimeDir)
	executor := NewBoundedExecutor()
	proofManager := NewProofOfWorkManager(runtimeDir)

	// Create Factory
	factoryInst := NewFactory(workspaceManager, executor, proofManager, runtimeDir)

	// Create LLM generator
	llmConfig := DefaultLLMGeneratorConfig(provider)
	llmConfig.EnableThinking = false
	llmConfig.Timeout = 60 * time.Second
	llmConfig.MaxTokens = 2000

	generator, err := NewLLMGenerator(llmConfig)
	if err != nil {
		t.Fatalf("Failed to create LLM generator: %v", err)
	}

	// Enable LLM mode
	factoryInst.SetLLMGenerator(generator)

	if !factoryInst.IsLLMEnabled() {
		t.Fatal("LLM mode not enabled after SetLLMGenerator()")
	}

	t.Log("✓ LLM mode enabled")

	// Create task spec
	spec := &FactoryTaskSpec{
		ID:            "TASK-E2E-001",
		SessionID:      "SESSION-E2E-001",
		WorkItemID:    "E2E-TEST-001",
		Title:         "Add calculator service",
		Objective:     "Implement a Calculator service with Add, Subtract, Multiply, Divide methods",
		WorkType:      contracts.WorkTypeImplementation,
		WorkDomain:    "service",
		Priority:      contracts.PriorityHigh,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Execute task
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	result, err := factoryInst.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteTask() error: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("ExecuteTask() returned nil result")
	}

	if !result.Success {
		t.Errorf("Task failed: %s", result.Status)
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, ExecutionStatusCompleted)
	}

	// Verify files were generated
	if len(result.FilesChanged) == 0 {
		t.Error("No files generated by LLM")
	} else {
		t.Logf("✓ Files generated: %d", len(result.FilesChanged))
		for _, file := range result.FilesChanged {
			if file != "" {
				t.Logf("  - %s", file)
			}
		}
	}

	// Verify generated code
	for _, file := range result.FilesChanged {
		if file != "" && filepath.Ext(file) == ".go" {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Errorf("Failed to read %s: %v", file, err)
				continue
			}

			code := string(content)
			if len(code) < 10 {
				t.Errorf("File %s has insufficient content (%d bytes)", file, len(code))
			} else {
				t.Logf("✓ %s: %d bytes generated", filepath.Base(file), len(code))
			}
		}
	}

	// Generate proof-of-work
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		t.Logf("Warning: failed to generate proof-of-work: %v", err)
	} else {
		t.Logf("✓ Proof-of-work: %s (duration: %v)", proof.WorkspacePath, proof.Duration)
	}

	t.Log("✓ End-to-end LLM integration test passed")
}
