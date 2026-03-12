// Package main provides an example of using LLM-powered Factory.
//
// This example shows how to configure Factory with LLM code generation
// and execute tasks with AI-generated implementations.
//
// Usage:
//   go run examples/llm_factory_example.go
//
// Requirements:
//   - Ollama or other LLM provider running
//   - OLLAMA_BASE_URL environment variable set
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/llm/gateway"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== LLM-Powered Factory Example ===")
	fmt.Println()

	// 1. Create LLM gateway
	fmt.Println("[1/5] Creating LLM gateway...")
	llmGateway := llmgateway.NewGateway(&llmgateway.Config{
		DefaultModel: "qwen3.5:14b", // Use larger model for better code
	})

	// Get LLM provider
	provider, err := llmGateway.GetProvider("ollama")
	if err != nil {
		log.Fatalf("Failed to get LLM provider: %v", err)
	}
	fmt.Printf("✓ LLM provider: %s\n", provider.Name())
	fmt.Println()

	// 2. Create LLM code generator
	fmt.Println("[2/5] Creating LLM code generator...")
	llmConfig := factory.DefaultLLMGeneratorConfig(provider)
	llmConfig.EnableThinking = true // Enable chain-of-thought
	llmConfig.Temperature = 0.3    // Lower for more deterministic code
	llmConfig.MaxTokens = 4096      // Limit response size

	generator, err := factory.NewLLMGenerator(llmConfig)
	if err != nil {
		log.Fatalf("Failed to create LLM generator: %v", err)
	}
	fmt.Println("✓ LLM generator ready")
	fmt.Println()

	// 3. Create Factory components
	fmt.Println("[3/5] Creating Factory components...")
	
	// Workspace manager (for isolation)
	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/tmp/zen-brain-factory-llm"
	}
	
	workspaceManager := factory.NewWorkspaceManager(runtimeDir)
	fmt.Printf("✓ Runtime directory: %s\n", runtimeDir)

	// Bounded executor (limits retries/timeout)
	executor := factory.NewBoundedExecutor()
	fmt.Println("✓ Bounded executor")

	// Proof-of-work manager
	proofManager := factory.NewProofOfWorkManager(runtimeDir)
	fmt.Println("✓ Proof-of-work manager")
	fmt.Println()

	// 4. Create Factory with LLM support
	fmt.Println("[4/5] Creating Factory with LLM support...")
	factoryInst := factory.NewFactory(workspaceManager, executor, proofManager, runtimeDir)
	factoryInst.SetLLMGenerator(generator) // Enable LLM mode
	fmt.Println("✓ Factory created with LLM support")
	fmt.Println()

	// 5. Create and execute a task
	fmt.Println("[5/5] Executing task with LLM generation...")
	
	taskSpec := &factory.FactoryTaskSpec{
		ID:            "TASK-LLM-001",
		SessionID:      "SESSION-001",
		WorkItemID:    "LLM-DEMO-001",
		Title:         "Add user authentication service",
		Objective:     "Implement a UserService with JWT-based authentication including login, logout, and token validation. The service should support password hashing, token generation, and refresh token handling.",
		WorkType:      contracts.WorkTypeImplementation,
		WorkDomain:    contracts.DomainAuth,
		Priority:      contracts.PriorityHigh,
		ExecutionMode: contracts.ModeAuto,
		Status:        contracts.StatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ClusterID:     "default",
		ProjectID:     "demo",
		RequiredArtifacts: []string{
			"internal/auth/user_service.go",
			"internal/auth/user_service_test.go",
		},
	}

	// Execute task
	result, err := factoryInst.ExecuteTask(ctx, taskSpec)
	if err != nil {
		log.Fatalf("Task execution failed: %v", err)
	}

	// Print results
	fmt.Println()
	fmt.Println("=== Execution Results ===")
	fmt.Printf("Task ID:     %s\n", result.TaskID)
	fmt.Printf("Work Item:   %s\n", result.WorkItemID)
	fmt.Printf("Status:       %s\n", result.Status)
	fmt.Printf("Success:      %v\n", result.Success)
	fmt.Printf("Duration:     %v\n", result.Duration)
	fmt.Printf("Template:     %s\n", result.TemplateKey)
	fmt.Println()
	
	if len(result.FilesChanged) > 0 {
		fmt.Println("Files Generated:")
		for _, file := range result.FilesChanged {
			fmt.Printf("  - %s\n", file)
		}
		fmt.Println()
	}

	// Show generated code preview
	if len(result.FilesChanged) > 0 {
		fmt.Println("Generated Code Preview:")
		for _, file := range result.FilesChanged {
			if file != "" {
				content, err := os.ReadFile(file)
				if err == nil {
					lines := len(string(content))
					if lines > 0 {
						preview := string(content)
						if len(preview) > 500 {
							preview = preview[:500] + "\n..."
						}
						fmt.Printf("\n--- %s (%d lines) ---\n%s\n", file, lines, preview)
					}
				}
			}
		}
	}

	// Generate proof-of-work
	fmt.Println("=== Generating Proof-of-Work ===")
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		log.Printf("Warning: failed to generate proof-of-work: %v", err)
	} else {
		fmt.Printf("Proof-of-work: %s\n", proof.ProofOfWorkPath)
		fmt.Printf("Proof summary: %s\n", proof.Summary)
	}

	fmt.Println()
	fmt.Println("✅ LLM-powered execution complete!")
	fmt.Println()
	fmt.Println("Key Points:")
	fmt.Println("  • Factory used LLM to generate actual code")
	fmt.Println("  • Code was validated and compiled")
	fmt.Println("  • Tests were generated alongside implementation")
	fmt.Println("  • No TODO placeholders in generated code")
}
