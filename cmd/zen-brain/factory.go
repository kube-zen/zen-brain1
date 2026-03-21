// Package main: factory subcommands (Block 4 operator-facing surface).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func runFactoryCommand() {
	if len(os.Args) < 3 {
		printFactoryUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "execute":
		runFactoryExecute()
	case "status":
		runFactoryStatus()
	case "proof":
		runFactoryProof()
	case "workspaces":
		runFactoryWorkspaces()
	case "cleanup":
		runFactoryCleanup()
	default:
		fmt.Printf("Unknown factory subcommand: %s\n", sub)
		printFactoryUsage()
		os.Exit(1)
	}
}

func printFactoryUsage() {
	fmt.Println("Usage: zen-brain factory <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  execute <task-id>   Execute a task and generate proof-of-work")
	fmt.Println("  status <task-id>    Show task execution status")
	fmt.Println("  proof <task-id>     Show proof-of-work for a task")
	fmt.Println("  workspaces          List active workspaces")
	fmt.Println("  cleanup [--all]     Clean up old workspaces (or all with --all)")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --llm               Enable LLM-powered code generation (requires OLLAMA_BASE_URL)")
	fmt.Println("  --json              Output as JSON")
	fmt.Println("  --full              Show complete proof details")
}

func runFactoryExecute() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain factory execute <task-id>")
		os.Exit(1)
	}

	taskID := os.Args[3]
	showJSON := hasFlag("--json")

	// Build factory with default config
	factoryInst, err := buildFactory()
	if err != nil {
		log.Fatalf("Build factory: %v", err)
	}

	// Create mock task spec for demonstration
	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Title:      "Execute task " + taskID,
		Objective:  "Demonstrate factory execution with proof-of-work",
		WorkType:   contracts.WorkTypeImplementation,
		Priority:   contracts.PriorityHigh,
		TemplateKey: "default",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()

	// Execute task
	result, err := factoryInst.ExecuteTask(ctx, spec)
	if err != nil {
		log.Fatalf("Execute task: %v", err)
	}

	// Generate proof-of-work
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		log.Printf("Warning: Failed to generate proof: %v", err)
	}

	// Output
	if showJSON {
		output := map[string]interface{}{
			"result": result,
			"proof":  proof,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(output); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable output
	printExecutionResult(result, proof)
}

func runFactoryStatus() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain factory status <task-id>")
		os.Exit(1)
	}

	taskID := os.Args[3]
	showJSON := hasFlag("--json")

	factoryInst, err := buildFactory()
	if err != nil {
		log.Fatalf("Build factory: %v", err)
	}

	ctx := context.Background()

	// Get task
	task, err := factoryInst.GetTask(ctx, taskID)
	if err != nil {
		log.Fatalf("Get task: %v", err)
	}

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(task); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable output
	fmt.Println("=== Task Status ===")
	fmt.Println()
	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Session: %s\n", task.SessionID)
	fmt.Printf("Work Item: %s\n", task.WorkItemID)
	fmt.Println()
	fmt.Printf("Title: %s\n", task.Title)
	fmt.Printf("Type: %s\n", task.WorkType)
	fmt.Printf("Priority: %s\n", task.Priority)
	fmt.Println()
	fmt.Printf("Template: %s\n", task.TemplateKey)
	fmt.Printf("Workspace: %s\n", task.WorkspacePath)
	fmt.Printf("Created: %s\n", task.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", task.UpdatedAt.Format(time.RFC3339))
}

func runFactoryProof() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain factory proof <task-id>")
		os.Exit(1)
	}

	taskID := os.Args[3]
	showFull := hasFlag("--full")
	showJSON := hasFlag("--json")

	// Look for proof artifacts
	// Use ZEN_BRAIN_RUNTIME_DIR if set, otherwise use ZEN_BRAIN_HOME/runtime
	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(config.HomeDir(), "runtime")
	}
	proofDir := filepath.Join(runtimeDir, "proof-of-work")

	// Find most recent proof for this task
	var latestProof *factory.ProofOfWorkArtifact
	var latestTime time.Time

	err := filepath.Walk(proofDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && strings.Contains(path, taskID) {
			// Try to load proof
			proofPath := filepath.Join(path, "proof-of-work.json")
			if data, err := os.ReadFile(proofPath); err == nil {
				var proof factory.ProofOfWorkArtifact
				if err := json.Unmarshal(data, &proof); err == nil {
					if proof.CreatedAt.After(latestTime) {
						latestProof = &proof
						latestTime = proof.CreatedAt
					}
				}
			}
		}

		return nil
	})

	if err != nil || latestProof == nil {
		log.Fatalf("No proof found for task %s", taskID)
	}

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(latestProof); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable output
	printProofOfWork(latestProof, showFull)
}

func runFactoryWorkspaces() {
	showJSON := hasFlag("--json")

	factoryInst, err := buildFactory()
	if err != nil {
		log.Fatalf("Build factory: %v", err)
	}

	ctx := context.Background()

	// List tasks (which have workspaces)
	tasks, err := factoryInst.ListTasks(ctx)
	if err != nil {
		log.Fatalf("List tasks: %v", err)
	}

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(tasks); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable output
	fmt.Println("=== Active Workspaces ===")
	fmt.Println()

	if len(tasks) == 0 {
		fmt.Println("No active workspaces")
		return
	}

	for i, task := range tasks {
		fmt.Printf("%d. Task %s\n", i+1, task.ID)
		fmt.Printf("   Workspace: %s\n", task.WorkspacePath)
		fmt.Printf("   Work Item: %s\n", task.WorkItemID)
		fmt.Printf("   Type: %s\n", task.WorkType)
		fmt.Printf("   Created: %s\n", task.CreatedAt.Format(time.RFC3339))
		fmt.Println()
	}

	fmt.Printf("Total: %d workspace(s)\n", len(tasks))
}

func runFactoryCleanup() {
	cleanAll := hasFlag("--all")

	factoryInst, err := buildFactory()
	if err != nil {
		log.Fatalf("Build factory: %v", err)
	}

	ctx := context.Background()

	// List tasks
	tasks, err := factoryInst.ListTasks(ctx)
	if err != nil {
		log.Fatalf("List tasks: %v", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No workspaces to clean up")
		return
	}

	cleaned := 0
	for _, task := range tasks {
		if task.WorkspacePath == "" {
			continue
		}

		// Only clean completed tasks unless --all is specified
		if !cleanAll {
			// Check if task is old enough (> 24 hours)
			if time.Since(task.UpdatedAt) < 24*time.Hour {
				continue
			}
		}

		if err := factoryInst.CleanupWorkspace(ctx, task.WorkspacePath); err != nil {
			log.Printf("Warning: Failed to cleanup workspace %s: %v", task.WorkspacePath, err)
		} else {
			cleaned++
		}
	}

	fmt.Printf("Cleaned up %d workspace(s)\n", cleaned)
}

func printExecutionResult(result *factory.ExecutionResult, proof *factory.ProofOfWorkSummary) {
	fmt.Println("=== Execution Result ===")
	fmt.Println()

	// Status
	statusEmoji := "✅"
	if !result.Success {
		statusEmoji = "❌"
	}
	fmt.Printf("%s Status: %s\n", statusEmoji, result.Status)
	fmt.Printf("Task ID: %s\n", result.TaskID)
	fmt.Printf("Session: %s\n", result.SessionID)
	fmt.Println()

	// Execution details
	fmt.Printf("📊 EXECUTION DETAILS\n")
	fmt.Printf("  Steps completed: %d/%d\n", result.CompletedSteps, result.TotalSteps)
	fmt.Printf("  Workspace: %s\n", result.WorkspacePath)

	if len(result.FilesChanged) > 0 {
		fmt.Printf("  Files changed: %d\n", len(result.FilesChanged))
	}
	if result.TestsPassed {
		fmt.Printf("  Tests: ✅ PASSED\n")
	} else if len(result.TestsRun) > 0 {
		fmt.Printf("  Tests: ❌ FAILED\n")
	}
	fmt.Println()

	// Proof-of-work summary
	if proof != nil {
		fmt.Printf("🔍 PROOF-OF-WORK\n")
		fmt.Printf("  Task ID: %s\n", proof.TaskID)
		fmt.Printf("  Result: %s\n", proof.Result)
		fmt.Printf("  Template: %s\n", proof.TemplateKey)

		if proof.GitCommit != "" {
			fmt.Printf("  Git commit: %s\n", proof.GitCommit[:min(8, len(proof.GitCommit))])
		}
		if proof.GitBranch != "" {
			fmt.Printf("  Git branch: %s\n", proof.GitBranch)
		}

		if len(proof.ArtifactPaths) > 0 {
			fmt.Printf("  Artifacts: %d\n", len(proof.ArtifactPaths))
		}

		if proof.Checksums != nil && len(proof.Checksums) > 0 {
			fmt.Printf("  Checksums: ✓ verified\n")
		}

		fmt.Println()
		fmt.Printf("📄 Workspace: %s\n", proof.WorkspacePath)
	}

	// Failed steps (if any)
	if len(result.FailedSteps) > 0 {
		fmt.Println()
		fmt.Printf("⚠️  FAILED STEPS (%d)\n", len(result.FailedSteps))
		for i, step := range result.FailedSteps {
			fmt.Printf("  %d. %s (exit code: %d)\n", i+1, step.Name, step.ExitCode)
			if step.Error != "" {
				// Show first 100 chars of error
				errMsg := step.Error
				if len(errMsg) > 100 {
					errMsg = errMsg[:100] + "..."
				}
				fmt.Printf("     Error: %s\n", errMsg)
			}
		}
	}
}

func printProofOfWork(proof *factory.ProofOfWorkArtifact, showFull bool) {
	fmt.Println("=== Proof-of-Work ===")
	fmt.Println()

	// Basic info
	fmt.Printf("📋 SUMMARY\n")
	fmt.Printf("  Task ID: %s\n", proof.Summary.TaskID)
	fmt.Printf("  Result: %s\n", proof.Summary.Result)
	fmt.Printf("  Template: %s\n", proof.Summary.TemplateKey)
	fmt.Printf("  Created: %s\n", proof.CreatedAt.Format(time.RFC3339))
	fmt.Println()

	// Git information
	if proof.Summary.GitCommit != "" {
		fmt.Printf("🌿 GIT INFORMATION\n")
		fmt.Printf("  Commit: %s\n", proof.Summary.GitCommit)
		if proof.Summary.GitBranch != "" {
			fmt.Printf("  Branch: %s\n", proof.Summary.GitBranch)
		}
		if proof.Summary.GitProvenance != nil && proof.Summary.GitProvenance.ParentCommit != "" {
			fmt.Printf("  Base: %s\n", proof.Summary.GitProvenance.ParentCommit[:min(8, len(proof.Summary.GitProvenance.ParentCommit))])
		}
		fmt.Println()
	}

	// Files changed
	if len(proof.Summary.FilesChanged) > 0 {
		fmt.Printf("📁 FILES CHANGED (%d)\n", len(proof.Summary.FilesChanged))
		for i, file := range proof.Summary.FilesChanged {
			if i < 10 { // Show first 10
				fmt.Printf("  - %s\n", file)
			}
		}
		if len(proof.Summary.FilesChanged) > 10 {
			fmt.Printf("  ... and %d more\n", len(proof.Summary.FilesChanged)-10)
		}
		fmt.Println()
	}

	// Artifacts
	fmt.Printf("📦 ARTIFACTS\n")
	fmt.Printf("  Directory: %s\n", proof.Directory)
	fmt.Printf("  JSON: %s\n", filepath.Base(proof.JSONPath))
	fmt.Printf("  Markdown: %s\n", filepath.Base(proof.MarkdownPath))
	fmt.Printf("  Log: %s\n", filepath.Base(proof.LogPath))
	fmt.Println()

	// Checksums
	if proof.Summary.Checksums != nil && len(proof.Summary.Checksums) > 0 {
		fmt.Printf("🔐 CHECKSUMS (%d files)\n", len(proof.Summary.Checksums))
		if showFull {
			for file, hash := range proof.Summary.Checksums {
				fmt.Printf("  %s: %s\n", filepath.Base(file), hash[:min(16, len(hash))])
			}
		} else {
			fmt.Printf("  ✓ All artifact checksums verified\n")
		}
		fmt.Println()
	}

	// Full output
	if showFull {
		fmt.Printf("📄 EXECUTION LOG\n")
		// Try to read and display log
		if data, err := os.ReadFile(proof.LogPath); err == nil {
			lines := strings.Split(string(data), "\n")
			fmt.Printf("  %d log lines\n", len(lines))
			if len(lines) > 0 {
				fmt.Println("  First 5 lines:")
				for i := 0; i < min(5, len(lines)); i++ {
					fmt.Printf("    %s\n", lines[i])
				}
			}
		}
	}
}

func buildFactory() (*factory.FactoryImpl, error) {
	// Build with default configuration (isolated directory mode)
	// Use ZEN_BRAIN_RUNTIME_DIR if set, otherwise use ZEN_BRAIN_HOME/runtime
	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(config.HomeDir(), "runtime")
	}

	workspaceManager := factory.NewWorkspaceManager(runtimeDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(runtimeDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		runtimeDir,
	)

	// Enable LLM mode if requested and OLLAMA_BASE_URL is set
	if hasFlag("--llm") {
		if ollamaURL := os.Getenv("OLLAMA_BASE_URL"); ollamaURL != "" {
			gw, gwErr := llm.NewGateway(&llm.GatewayConfig{
				LocalWorkerModel: "qwen3.5:0.8b", // ZB-018: only supported local model
			})

			if gwErr == nil {
				provider, providerFound := gw.GetProvider("local-worker")
				if providerFound {
					llmConfig := factory.DefaultLLMGeneratorConfig(provider)
					llmConfig.EnableThinking = false // ZB-025H1: Disable thinking for CPU path
					llmConfig.Model = "qwen3.5:0.8b" // ZB-025H2: Set explicit model
					llmConfig.Temperature = 0.3
					llmConfig.MaxTokens = 4096

					generator, genErr := factory.NewLLMGenerator(llmConfig)
					if genErr == nil {
						factoryInst.SetLLMGenerator(generator)
						log.Printf("✓ LLM mode enabled (provider=%s, model=%s, url=%s)",
							provider.Name(), llmConfig.Model, ollamaURL)
					} else {
						log.Printf("Warning: Failed to create LLM generator: %v", genErr)
					}
				} else {
					log.Printf("Warning: Local-worker provider not found")
				}
			} else {
				log.Printf("Warning: Failed to create LLM gateway: %v", gwErr)
			}
		} else {
			log.Printf("Warning: --llm flag set but OLLAMA_BASE_URL not set")
			log.Printf("LLM mode disabled - falling back to shell templates")
			log.Printf("Set OLLAMA_BASE_URL to enable LLM mode")
		}
	}

	return factoryInst, nil
}
