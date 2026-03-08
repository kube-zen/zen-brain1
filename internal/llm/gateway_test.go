package llm

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

func TestNewGateway(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, err := NewGateway(config)

	if err != nil {
		t.Fatalf("NewGateway failed: %v", err)
	}

	if gateway == nil {
		t.Fatal("Gateway should not be nil")
	}

	// Verify providers are registered
	providers := gateway.ListProviders()
	expectedProviders := []string{"local-worker", "planner", "fallback"}
	
	for _, expected := range expectedProviders {
		found := false
		for _, actual := range providers {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider %q not found in registered providers: %v", expected, providers)
		}
	}
}

func TestGateway_Chat_LocalWorkerRoute(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RoutingPolicy = "simple"
	gateway, _ := NewGateway(config)

	// Create a simple request that should route to local worker
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello, can you help me with a simple task?"},
		},
		Tools: []llm.Tool{
			{
				Name:        "simple_tool",
				Description: "A simple tool for testing",
				Parameters:  map[string]interface{}{},
			},
		},
		SessionID: "test-session",
	}

	ctx := context.Background()
	resp, err := gateway.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	if resp.Content == "" {
		t.Error("Response content should not be empty")
	}

	// Verify it's a reasonable response
	if len(resp.Content) < 10 {
		t.Error("Response content should be meaningful")
	}

	// Check that usage stats are populated
	if resp.Usage == nil {
		t.Error("Usage stats should not be nil")
	}

	if resp.Usage.TotalTokens <= 0 {
		t.Error("Total tokens should be positive")
	}
}

func TestGateway_Chat_PlannerRoute(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RoutingPolicy = "simple"
	config.AutoEscalateComplexTasks = true
	gateway, _ := NewGateway(config)

	// Create a complex request that should route to planner
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "I need to design a complete microservices architecture for a new e-commerce platform. Please provide a detailed plan including technology stack, deployment strategy, and risk assessment."},
		},
		TaskID: "complex-task-1",
		SessionID: "complex-session",
	}

	ctx := context.Background()
	resp, err := gateway.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	// Planner responses should be more comprehensive
	if len(resp.Content) < 100 {
		t.Error("Planner response should be detailed")
	}

	// Planner might include reasoning content
	if resp.ReasoningContent != "" {
		if len(resp.ReasoningContent) < 50 {
			t.Error("Reasoning content should be meaningful if present")
		}
	}
}

func TestGateway_Chat_Timeout(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RequestTimeout = 1 // 1 second timeout
	gateway, _ := NewGateway(config)

	// Create a simple request
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Simple request"},
		},
	}

	ctx := context.Background()
	start := time.Now()
	_, err := gateway.Chat(ctx, req)
	elapsed := time.Since(start)

	// Request should complete quickly (not timeout)
	// Our mock providers take ~50ms
	if elapsed > 500*time.Millisecond {
		t.Errorf("Request should complete quickly, took %v", elapsed)
	}

	if err != nil {
		t.Errorf("Request should succeed, got error: %v", err)
	}
}

func TestGateway_Route_SimplePolicy(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RoutingPolicy = "simple"
	gateway, _ := NewGateway(config)

	// Test 1: Simple task → local worker
	simpleReq := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Simple help request"},
		},
	}

	provider, reason, err := gateway.Route(context.Background(), simpleReq)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if provider.Name() != "local-worker" {
		t.Errorf("Expected local-worker, got %s", provider.Name())
	}

	if reason == "" {
		t.Error("Routing reason should not be empty")
	}

	// Test 2: Complex task → planner
	complexReq := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Design a complex architecture plan with risk analysis"},
		},
		TaskID: "task-1",
	}

	provider, reason, err = gateway.Route(context.Background(), complexReq)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if provider.Name() != "planner" {
		t.Errorf("Expected planner for complex task, got %s", provider.Name())
	}
}

func TestGateway_Route_CostAwarePolicy(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RoutingPolicy = "cost_aware"
	config.LocalWorkerMaxCost = 0.05  // $0.05 max for local worker
	config.PlannerMinCost = 0.10      // $0.10 min for planner
	gateway, _ := NewGateway(config)

	// Note: Our simple cost estimator is very basic
	// These tests verify the routing logic structure
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Test message"},
		},
	}

	provider, reason, err := gateway.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	if reason == "" {
		t.Error("Routing reason should not be empty")
	}
}

func TestGateway_SupportsTools(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	if !gateway.SupportsTools() {
		t.Error("Gateway should support tools (at least one provider does)")
	}
}

func TestGateway_CreateProvider(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	// Test creating local-worker provider
	provider, err := gateway.CreateProvider("local-worker")
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	if provider.Name() != "local-worker" {
		t.Errorf("Expected local-worker, got %s", provider.Name())
	}

	// Test creating non-existent provider
	_, err = gateway.CreateProvider("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestLocalWorkerProvider_Chat(t *testing.T) {
	provider := NewLocalWorkerProvider("qwen3.5:0.8b", 30)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello, can you help me call a tool?"},
		},
		Tools: []llm.Tool{
			{
				Name:        "test_tool",
				Description: "Test tool",
				Parameters:  map[string]interface{}{},
			},
		},
	}

	ctx := context.Background()
	resp, err := provider.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Response content should not be empty")
	}

	if !provider.SupportsTools() {
		t.Error("Local worker should support tools")
	}

	// Test with normal context (should succeed)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	_, err = provider.Chat(ctx, req)
	if err != nil {
		t.Errorf("Chat with normal context should succeed, got: %v", err)
	}
}

func TestPlannerProvider_Chat(t *testing.T) {
	provider := NewPlannerProvider("glm-4.7", 60)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Please analyze this architecture design and provide recommendations."},
		},
		TaskID:    "planning-task-1",
		SessionID: "planning-session",
		Tools: []llm.Tool{
			{
				Name:        "analyze_architecture",
				Description: "Analyze software architecture",
				Parameters:  map[string]interface{}{},
			},
		},
	}

	ctx := context.Background()
	resp, err := provider.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Response content should not be empty")
	}

	// Planner responses should be more detailed
	if len(resp.Content) < 100 {
		t.Error("Planner response should be comprehensive")
	}

	if !provider.SupportsTools() {
		t.Error("Planner should support tools")
	}

	// Planner might generate tool calls for complex tasks
	// (Our mock implementation may or may not generate them)
}

func TestGateway_StatsTracking(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	// Make several requests
	reqs := []llm.ChatRequest{
		{
			Messages: []llm.Message{{Role: "user", Content: "Simple request 1"}},
		},
		{
			Messages: []llm.Message{{Role: "user", Content: "Complex architecture design"}},
			TaskID:   "task-1",
		},
		{
			Messages: []llm.Message{{Role: "user", Content: "Simple request 2"}},
		},
	}

	ctx := context.Background()
	for _, req := range reqs {
		_, err := gateway.Chat(ctx, req)
		if err != nil {
			t.Logf("Chat error (may be expected): %v", err)
		}
	}

	// Check stats
	stats := gateway.GetStats()
	if stats.TotalRequests < 3 {
		t.Errorf("Expected at least 3 total requests, got %d", stats.TotalRequests)
	}

	// Should have some local worker and planner requests
	if stats.LocalWorkerRequests == 0 && stats.PlannerRequests == 0 {
		t.Error("Expected some requests to be routed to local worker or planner")
	}

	if stats.TotalLatencyMs <= 0 {
		t.Error("Total latency should be positive")
	}
}

func TestGateway_ChatStream(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Test streaming request"},
		},
		Stream: true,
	}

	// Simple callback that just counts tokens
	tokenCount := 0
	callback := func(token string) {
		tokenCount++
	}

	ctx := context.Background()
	resp, err := gateway.ChatStream(ctx, req, callback)

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	// In MVP, streaming falls back to regular chat
	// So we just verify it works without error
}

func TestGateway_EmbeddingNotSupported(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	req := llm.EmbeddingRequest{
		Input: "Test embedding input",
	}

	ctx := context.Background()
	_, err := gateway.Embed(ctx, req)

	if err != llm.ErrEmbeddingNotSupported {
		t.Errorf("Expected ErrEmbeddingNotSupported, got %v", err)
	}
}

func TestGateway_ComplexTaskDetection(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	// Test cases
	testCases := []struct {
		name     string
		req      llm.ChatRequest
		expected bool
	}{
		{
			name: "Simple short message",
			req: llm.ChatRequest{
				Messages: []llm.Message{{Role: "user", Content: "Hello"}},
			},
			expected: false,
		},
		{
			name: "Long message",
			req: llm.ChatRequest{
				Messages: []llm.Message{{Role: "user", Content: "This is a very long message that goes on and on about various topics and requirements and specifications and details and considerations and factors and elements and components and aspects and dimensions and perspectives and viewpoints and approaches and methodologies and techniques and strategies and tactics and plans and designs and architectures"}},
			},
			expected: true,
		},
		{
			name: "Planning keyword",
			req: llm.ChatRequest{
				Messages: []llm.Message{{Role: "user", Content: "Let's plan the architecture"}},
			},
			expected: true,
		},
		{
			name: "Task with ID",
			req: llm.ChatRequest{
				Messages: []llm.Message{{Role: "user", Content: "Do something"}},
				TaskID:   "task-123",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// We can't directly call isComplexTask as it's private
			// Instead, test through routing
			provider, _, err := gateway.Route(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Route failed: %v", err)
			}

			// Complex tasks should route to planner
			if tc.expected && provider.Name() != "planner" {
				t.Errorf("Expected planner for complex task, got %s", provider.Name())
			}
			// Note: Simple tasks might still go to planner if local worker unavailable
			// or based on other routing factors
		})
	}
}

func TestGateway_MultipleMessages(t *testing.T) {
	config := DefaultGatewayConfig()
	gateway, _ := NewGateway(config)

	// Conversation with multiple messages
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there! How can I help you?"},
			{Role: "user", Content: "I need help with a programming problem."},
		},
	}

	ctx := context.Background()
	resp, err := gateway.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Response content should not be empty")
	}

	if resp.Usage == nil {
		t.Error("Usage stats should not be nil")
	}

	if resp.Usage.InputTokens <= 0 {
		t.Error("Input tokens should be positive for multi-message request")
	}
}

func TestGateway_ToolSupportMatching(t *testing.T) {
	config := DefaultGatewayConfig()
	config.RoutingPolicy = "simple"
	gateway, _ := NewGateway(config)

	// Request with tools
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Please use the analyze tool on this data"},
		},
		Tools: []llm.Tool{
			{
				Name:        "analyze_data",
				Description: "Analyze structured data",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	ctx := context.Background()
	resp, err := gateway.Chat(ctx, req)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Should not error even with tools
	// Our mock providers handle tools appropriately
	if resp == nil {
		t.Fatal("Response should not be nil")
	}
}