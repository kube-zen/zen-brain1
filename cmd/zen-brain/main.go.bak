package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// Build-time variables (set via Makefile)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	fmt.Printf("zen-brain %s (built %s)\n", Version, BuildTime)

	fmt.Printf("Home directory: %s\n", config.HomeDir())
	fmt.Printf("Configuration loaded\n")

	// Create LLM Gateway
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
		fmt.Fprintf(os.Stderr, "Error creating gateway: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ LLM Gateway initialized")
	fmt.Printf("  - Local worker: %s\n", gatewayConfig.LocalWorkerModel)
	fmt.Printf("  - Planner: %s\n", gatewayConfig.PlannerModel)
	fmt.Printf("  - Fallback chain: %v\n", gatewayConfig.EnableFallbackChain)

	// Run a simple test query
	if len(os.Args) > 1 && os.Args[1] == "test" {
		fmt.Println("\nRunning test query...")
		runTestQuery(gateway)
		return
	}

	fmt.Println("\nReady! Use 'zen-brain test' to run a simple query.")
}

func runTestQuery(gateway *llmgateway.Gateway) {
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Test query successful\n")
	fmt.Printf("  Response: %s\n", resp.Content[:min(200, len(resp.Content))])
	fmt.Printf("  Tokens: %d\n", resp.Usage.TotalTokens)
	fmt.Printf("  Latency: %dms\n", resp.LatencyMs)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
