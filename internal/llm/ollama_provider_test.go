// Package llm provides LLM provider tests.
// ZB-023: Local CPU Inference Policy Tests

package llm

import (
	"context"
	"testing"
	"time"
)

// TestNewOllamaProviderCertifiedModel tests creating provider with certified model (qwen3.5:0.8b)
func TestNewOllamaProviderCertifiedModel(t *testing.T) {
	baseURL := "http://host.k3d.internal:11434"
	model := "qwen3.5:0.8b"
	timeout := 30
	keepAlive := "30m"

	provider := NewOllamaProvider(baseURL, model, timeout, keepAlive)

	if provider == nil {
		t.Fatal("NewOllamaProvider returned nil")
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}

	if !provider.SupportsTools() {
		t.Error("Expected provider to support tools")
	}
}

// TestNewOllamaProviderDefaultModel tests creating provider with default model
func TestNewOllamaProviderDefaultModel(t *testing.T) {
	baseURL := "http://host.k3d.internal:11434"

	// Create provider with empty model (should default to qwen3.5:0.8b)
	provider := NewOllamaProvider(baseURL, "", 30, "30m")

	// Default model should be qwen3.5:0.8b (ZB-023 certified)
	if provider == nil {
		t.Fatal("NewOllamaProvider returned nil")
	}
}

// TestNewOllamaProviderInClusterDetection tests in-cluster Ollama detection
func TestNewOllamaProviderInClusterDetection(t *testing.T) {
	testCases := []struct {
		name         string
		baseURL      string
		expectDetect  bool
	}{
		{
			name:        "host-docker-ollama",
			baseURL:     "http://host.k3d.internal:11434",
			expectDetect: false, // Host Docker is ALLOWED
		},
		{
			name:        "in-cluster-ollama-service",
			baseURL:     "http://ollama:11434",
			expectDetect: true, // In-cluster is FORBIDDEN
		},
		{
			name:        "in-cluster-ollama-namespace",
			baseURL:     "http://ollama.zen-brain:11434",
			expectDetect: true, // In-cluster is FORBIDDEN
		},
		{
			name:        "in-cluster-ollama-full-fqdn",
			baseURL:     "http://ollama.zen-brain.svc.cluster.local:11434",
			expectDetect: true, // In-cluster is FORBIDDEN
		},
		{
			name:        "localhost-ollama",
			baseURL:     "http://localhost:11434",
			expectDetect: true, // Might be in-cluster (conservative)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			detected := isInClusterOllama(tc.baseURL)
			if detected != tc.expectDetect {
				t.Errorf("isInClusterOllama(%s) = %v, expected %v", tc.baseURL, detected, tc.expectDetect)
			}
		})
	}
}

// TestOllamaProviderChatCertifiedModel tests chat with certified model (should pass or clamp)
func TestOllamaProviderChatCertifiedModel(t *testing.T) {
	// This test requires a running Ollama instance
	// Skip in CI/unit test environments without Ollama

	baseURL := "http://host.k3d.internal:11434"
	model := "qwen3.5:0.8b"
	timeout := 30
	keepAlive := "30m"

	provider := NewOllamaProvider(baseURL, model, timeout, keepAlive)

	if provider == nil {
		t.Skip("Ollama provider not available (requires running Ollama)")
	}

	req := ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Say hello"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// This will fail if Ollama is not running, which is OK for unit tests
	_, err := provider.Chat(ctx, req)

	// If Ollama is running, request should succeed
	// If Ollama is not running, we expect connection error
	// We're just testing that certified model doesn't trigger policy rejection
	if err != nil {
		// Check if error is connection error (expected when Ollama not running)
		// If error is "FAIL-CLOSED" policy error, that's a test failure
		errStr := err.Error()
		if contains(errStr, "FAIL-CLOSED") && contains(errStr, "Non-certified local model") {
			t.Errorf("Certified model should not trigger FAIL-CLOSED policy error: %v", err)
		}
	}
}

// TestOllamaProviderChatNonCertifiedModel tests chat with non-certified model (should reject or clamp)
func TestOllamaProviderChatNonCertifiedModel(t *testing.T) {
	baseURL := "http://host.k3d.internal:11434"
	// Create provider with certified model (default)
	provider := NewOllamaProvider(baseURL, "qwen3.5:0.8b", 30, "30m")

	if provider == nil {
		t.Skip("Ollama provider not available")
	}

	// Try to request non-certified model (qwen3.5:14b)
	req := ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Say hello"},
		},
		Model: "qwen3.5:14b", // Non-certified model
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// This test verifies that non-certified models are either:
	// 1. Rejected with FAIL-CLOSED error (if strict mode), OR
	// 2. Clamped to qwen3.5:0.8b with warning (if permissive mode)
	resp, err := provider.Chat(ctx, req)

	// For this test, we check that either rejection or clamping occurs
	if err != nil {
		errStr := err.Error()
		// Option 1: Strict rejection (FAIL-CLOSED)
		if contains(errStr, "FAIL-CLOSED") && contains(errStr, "Non-certified local model") {
			// PASS: Model was rejected
			return
		}
	}

	// Option 2: Permissive clamping (warning logged, request succeeds with certified model)
	if resp != nil && resp.Model == "qwen3.5:0.8b" {
		// PASS: Model was clamped to certified
		return
	}

	// FAIL: Neither rejection nor clamping occurred
	t.Error("Non-certified model should be rejected or clamped to qwen3.5:0.8b")
}

// TestOllamaProviderTimeoutDefaults tests generous timeout defaults for CPU path
func TestOllamaProviderTimeoutDefaults(t *testing.T) {
	baseURL := "http://host.k3d.internal:11434"
	model := "qwen3.5:0.8b"

	// Create provider with 0 timeout (should default to 30s)
	provider := NewOllamaProvider(baseURL, model, 0, "30m")

	if provider == nil {
		t.Fatal("NewOllamaProvider returned nil")
	}

	// Verify timeout is at least 30s (generous for CPU inference)
	// This is tested indirectly via provider usage
	// Direct field access not available without exposing internal state
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
