// Test program for MLQ rescue with structured prompts
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/promptbuilder"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// Mock LLM Provider for testing
type MockLLMProvider struct{}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	// Return a mock response for testing
	return &llm.ChatResponse{
		Content: `Files changed:
- internal/mlq/selector.go

Verification run:
- go build ./...: SUCCESS
- go test ./...: SUCCESS

Result: SUCCESS

Blockers: (none)`,
		Model: "qwen3.5:0.8b",
		Usage: &llm.Usage{
			TotalTokens: 1000,
		},
	}, nil
}

func main() {
	log.Printf("=== MLQ Rescue Test with Structured Prompt ===\n")

	// Create a mock LLM provider
	mockProvider := &MockLLMProvider{}

	// Create LLM generator
	generatorConfig := &factory.LLMGeneratorConfig{
		Provider:       mockProvider,
		Model:          "qwen3.5:0.8b",
		Temperature:    0.3,
		MaxTokens:      4096,
		EnableThinking: true,
		Timeout:        120 * time.Second,
	}

	generator, err := factory.NewLLMGenerator(generatorConfig)
	if err != nil {
		log.Fatalf("Failed to create LLM generator: %v", err)
	}

	// Create a structured MLQ rescue request
	req := &factory.GenerationRequest{
		WorkItemID:       "ZB-MLQ-RESCUE",
		Title:            "MLQ Rescue: Port multi-level queue from zen-brain 0.1",
		Objective:        "Port the MLQ (multi-level queue) architecture from zen-brain 0.1 to zen-brain1",
		WorkType:         "implementation",
		WorkDomain:       "office",
		JiraKey:          "ZB-281",
		ProjectType:      "go",
		WorkTypeLabel:    "mlq_rescue",
		TimeoutSec:       2700,

		// Enable structured prompt
		StructuredPrompt: true,

		// Source file (zen-brain 0.1)
		ContextFiles: []string{
			"/home/neves/zen-old/zen-brain/internal/queue/multi_level_queue.go",
		},

		// Target files (zen-brain1)
		TargetFiles: []string{
			"internal/mlq/selector.go",
		},

		// Allowed paths - strict bounding
		AllowedPaths: []string{
			"internal/mlq/",
		},

		// Forbidden paths - what NOT to touch
		ForbiddenPaths: []string{
			"cmd/",
			"api/",
			"deploy/",
		},

		// Existing types to reuse
		ExistingTypes: []string{
			"github.com/kube-zen/zen-brain1/pkg/llm.Provider",
			"github.com/kube-zen/zen-brain1/pkg/llm.ChatRequest",
			"github.com/kube-zen/zen-brain1/pkg/llm.ChatResponse",
		},

		// Existing packages to import
		ExistingPackages: []string{
			"github.com/kube-zen/zen-brain1/internal/llm",
			"github.com/kube-zen/zen-brain1/pkg/llm",
			"github.com/kube-zen/zen-brain1/internal/mlq",
		},
	}

	log.Printf("\n--- Structured Prompt Configuration ---")
	log.Printf("Jira Key: %s", req.JiraKey)
	log.Printf("Work Type: %s", req.WorkTypeLabel)
	log.Printf("Timeout: %d seconds", req.TimeoutSec)
	log.Printf("Source Files: %v", req.ContextFiles)
	log.Printf("Target Files: %v", req.TargetFiles)
	log.Printf("Allowed Paths: %v", req.AllowedPaths)
	log.Printf("Forbidden Paths: %v", req.ForbiddenPaths)
	log.Printf("Existing Types: %v", req.ExistingTypes)
	log.Printf("Existing Packages: %v", req.ExistingPackages)

	// Build the structured prompt using promptbuilder
	packet := promptbuilder.TaskPacket{
		JiraKey:    req.JiraKey,
		Summary:    req.Title,
		WorkType:   req.WorkTypeLabel,
		TimeoutSec: req.TimeoutSec,

		AllowedPaths:   req.AllowedPaths,
		ForbiddenPaths: req.ForbiddenPaths,
		ContextFiles:   req.ContextFiles,
		TargetFiles:    req.TargetFiles,

		ExistingTypes:    req.ExistingTypes,
		ExistingPackages: req.ExistingPackages,

		NoCodeExamples:  false,
		NoFakeArtifacts: true,
		ReportFiles:     true,
		ReportBlockers:  true,
		OutputFormat:    "structured",

		CompileCmd: "go build ./...",
		TestCmd:    "go test ./...",
		VerifyCmds: []string{
			"grep -r 'type Provider interface' pkg/llm/",
		},
		StaticChecks: []string{
			"No fake imports",
			"No invented packages",
			"Stays in allowed paths",
		},
	}

	prompt, err := promptbuilder.BuildPrompt(packet)
	if err != nil {
		log.Fatalf("Failed to build structured prompt: %v", err)
	}

	log.Printf("\n--- Generated Structured Prompt ---\n")
	log.Printf("%s\n", prompt)

	// Execute generation with the mock provider
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := generator.GenerateImplementation(ctx, req)
	if err != nil {
		log.Fatalf("Failed to generate implementation: %v", err)
	}

	log.Printf("\n--- Generation Result ---")
	log.Printf("Code:\n%s\n", result.Code)
	log.Printf("Language: %s", result.Language)
	log.Printf("Model: %s", result.Model)
	log.Printf("Tokens Used: %d", result.TokensUsed)

	log.Printf("\n=== Test Complete ===")
}
