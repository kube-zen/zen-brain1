package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// Build-time variables (set via Makefile)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	fmt.Printf("zen-brain %s (built %s)\n", Version, BuildTime)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "test":
		runTestQuery()

	case "vertical-slice":
		runVerticalSlice()

	case "version":
		printVersion()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: zen-brain <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  test           Run a simple LLM Gateway test query")
	fmt.Println("  vertical-slice Run end-to-end vertical slice (Jira → plan → execute → update)")
	fmt.Println("  version        Print version information")
	fmt.Println()
	fmt.Println("For vertical-slice command:")
	fmt.Println("  zen-brain vertical-slice <jira-key>   Process a Jira ticket by key")
	fmt.Println("  zen-brain vertical-slice --mock          Use mock work item instead of real Jira")
}

func printVersion() {
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Built: %s\n", BuildTime)
}

func runTestQuery() {
	fmt.Println("Initializing LLM Gateway...")

	gatewayConfig := &llmgateway.GatewayConfig{
		LocalWorkerModel:        "qwen3.5:0.8b",
		PlannerModel:            "glm-4.7",
		FallbackModel:           "glm-4.7",
		LocalWorkerMaxCost:     0.01,
		PlannerMinCost:          0.10,
		LocalWorkerTimeout:       30,
		PlannerTimeout:           60,
		RequestTimeout:           120,
		LocalWorkerSupportsTools: true,
		PlannerSupportsTools:     true,
		AutoEscalateComplexTasks:   true,
		RoutingPolicy:            "simple",
		EnableFallbackChain:     true,
		StrictPreferred:         false,
	}

	gateway, err := llmgateway.NewGateway(gatewayConfig)
	if err != nil {
		log.Fatalf("Error creating gateway: %v", err)
	}

	fmt.Println("✓ LLM Gateway initialized")
	fmt.Printf("  - Local worker: %s\n", gatewayConfig.LocalWorkerModel)
	fmt.Printf("  - Planner: %s\n", gatewayConfig.PlannerModel)
	fmt.Printf("  - Fallback chain: %v\n", gatewayConfig.EnableFallbackChain)

	ctx := context.Background()
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are zen-brain, an AI assistant for software engineering tasks."},
			{Role: "user", Content: "Hello! What can you help with?"},
		},
		SessionID: "test-session-mvp",
	}

	resp, err := gateway.Chat(ctx, req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("\n✓ Test query successful\n")
	fmt.Printf("  Response: %s\n", resp.Content[:min(200, len(resp.Content))])
	fmt.Printf("  Tokens: %d\n", resp.Usage.TotalTokens)
	fmt.Printf("  Latency: %dms\n", resp.LatencyMs)
}

func runVerticalSlice() {
	fmt.Println("=== Zen-Brain Vertical Slice ===")
	fmt.Println()
	fmt.Println("This command demonstrates the end-to-end pipeline:")
	fmt.Println("  1. Fetch work item from Jira (or use mock)")
	fmt.Println("  2. Analyze intent and complexity")
	fmt.Println("  3. Plan execution steps")
	fmt.Println("  4. Execute in isolated workspace")
	fmt.Println("  5. Generate proof-of-work")
	fmt.Println("  6. Update session state")
	fmt.Println("  7. Update Jira with status and comments")
	fmt.Println()

	// Parse arguments
	useMock := false
	jiraKey := ""
	if len(os.Args) > 2 {
		if os.Args[2] == "--mock" {
			useMock = true
			fmt.Println("Mode: Using mock work item (no Jira required)")
		} else {
			jiraKey = os.Args[2]
			fmt.Printf("Mode: Fetching real Jira ticket: %s\n", jiraKey)
		}
	} else {
		useMock = true
		fmt.Println("Mode: Using mock work item (no Jira required)")
	}

	fmt.Println()
	fmt.Println("Initializing components...")

	// Step 1: Initialize LLM Gateway
	fmt.Println("[1/7] Initializing LLM Gateway...")
	gatewayConfig := &llmgateway.GatewayConfig{
		LocalWorkerModel:        "qwen3.5:0.8b",
		PlannerModel:            "glm-4.7",
		FallbackModel:           "glm-4.7",
		LocalWorkerMaxCost:     0.01,
		PlannerMinCost:          0.10,
		LocalWorkerTimeout:       30,
		PlannerTimeout:           60,
		RequestTimeout:           120,
		LocalWorkerSupportsTools: true,
		PlannerSupportsTools:     true,
		AutoEscalateComplexTasks:   true,
		RoutingPolicy:            "simple",
		EnableFallbackChain:     true,
		StrictPreferred:         false,
	}

	_, err := llmgateway.NewGateway(gatewayConfig)
	if err != nil {
		log.Fatalf("Error creating gateway: %v", err)
	}
	fmt.Println("✓ LLM Gateway initialized")

	// Step 2: Get work item
	fmt.Println("[2/7] Fetching work item...")
	var workItem *contracts.WorkItem

	if useMock {
		workItem = createMockWorkItem()
	} else {
		// TODO: Initialize Office Manager with Jira connector
		// TODO: Fetch real work item from Jira by jiraKey
		log.Fatalf("Jira connector integration not yet implemented in main command")
	}

	fmt.Printf("✓ Work item: %s - %s\n", workItem.ID, workItem.Title)
	fmt.Printf("  Type: %s, Priority: %s\n", workItem.WorkType, workItem.Priority)

	// Step 3: Analyze work item
	fmt.Println("[3/7] Analyzing work item...")
	// TODO: Initialize Analyzer with LLM Gateway
	// TODO: Run intent analysis
	fmt.Println("✓ Analysis complete (TODO: implement analyzer integration)")
	fmt.Println("  Complexity: medium")
	fmt.Println("  Estimated effort: 2 hours")

	// Step 4: Create execution plan
	fmt.Println("[4/7] Creating execution plan...")
	// TODO: Initialize Factory
	// TODO: Create FactoryTaskSpec from analysis
	// TODO: Generate execution steps
	fmt.Println("✓ Execution plan created (TODO: implement factory integration)")
	fmt.Println("  Steps: 5")
	fmt.Println("  Estimated cost: $0.05")

	// Step 5: Execute in isolated workspace
	fmt.Println("[5/7] Executing in isolated workspace...")
	// TODO: Execute task using Factory
	// TODO: Track execution progress
	fmt.Println("✓ Execution complete (TODO: implement factory execution)")
	fmt.Println("  Duration: 5s")
	fmt.Println("  Files changed: 3")
	fmt.Println("  Tests passed: 5/5")

	// Step 6: Generate proof-of-work
	fmt.Println("[6/7] Generating proof-of-work...")
	// TODO: Use ProofOfWorkGenerator to create artifact
	// TODO: Generate both JSON and Markdown formats
	fmt.Println("✓ Proof-of-work generated (TODO: implement proof-of-work integration)")
	fmt.Println("  Artifact: /tmp/zen-brain/pow/task-123.json")
	fmt.Println("  Markdown: /tmp/zen-brain/pow/task-123.md")

	// Step 7: Update session state
	fmt.Println("[7/7] Updating session state...")
	// TODO: Initialize Session Manager
	// TODO: Update session with execution results
	fmt.Println("✓ Session state updated (TODO: implement session manager integration)")

	// Step 8: Update Jira
	if !useMock {
		fmt.Println("[8/8] Updating Jira with status and comments...")
		// TODO: Add proof-of-work comment to Jira ticket
		// TODO: Update ticket status to completed
		fmt.Println("✓ Jira updated (TODO: implement office connector integration)")
	}

	fmt.Println()
	fmt.Println("=== Vertical Slice Complete ===")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Println("  Work item processed: " + workItem.ID)
	fmt.Println("  Status: completed")
	fmt.Println("  Proof-of-work: generated")
	fmt.Println("  Session: updated")
	fmt.Println()
	fmt.Println("Note: This is a demonstration of the pipeline structure.")
	fmt.Println("Full integration requires:")
	fmt.Println("  - Office Manager with Jira connector initialization")
	fmt.Println("  - Analyzer with LLM Gateway integration")
	fmt.Println("  - Factory execution with workspace management")
	fmt.Println("  - Proof-of-work generation and storage")
	fmt.Println("  - Session Manager with ZenContext persistence")
	fmt.Println("  - Office connector status/comment updates")
}

func createMockWorkItem() *contracts.WorkItem {
	now := time.Now()
	return &contracts.WorkItem{
		ID:          "MOCK-001",
		Title:       "Fix authentication bug in login flow",
		Summary:     "Users are unable to login when using special characters in passwords",
		Body:        "## Problem\n\nSeveral users have reported login failures when their passwords contain special characters (!@#$%). The error message is 'Invalid credentials' even though the password is correct.\n\n## Reproduction\n\n1. Navigate to login page\n2. Enter username\n3. Enter password with special characters\n4. Click login\n5. Error occurs\n\n## Expected Behavior\n\nUsers should be able to login with any valid password, including those with special characters.",
		WorkType:    contracts.WorkTypeDebug,
		WorkDomain:  contracts.DomainCore,
		Priority:    contracts.PriorityHigh,
		ExecutionMode: contracts.ModeApprovalRequired,
		Status:      contracts.StatusRequested,
		CreatedAt:   now,
		UpdatedAt:   now,
		ClusterID:   "default",
		ProjectID:   "MOCK",
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
