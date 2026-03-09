package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// mockProvider is a mock LLM provider for testing
type mockProvider struct {
	name      string
	shouldErr bool
	errMsg    string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) SupportsTools() bool { return true }
func (m *mockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.shouldErr {
		return nil, errors.New(m.errMsg)
	}
	return &llm.ChatResponse{
		Content:      "Mock response from " + m.name,
		FinishReason: "stop",
		Model:        "mock",
	}, nil
}
func (m *mockProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	return m.Chat(ctx, req)
}
func (m *mockProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}

func TestDefaultFallbackChain_ProviderOrder(t *testing.T) {
	config := DefaultFallbackConfig()
	chain := NewDefaultFallbackChain(config, func(name string) bool {
		// All providers available for this test
		return name == "local-worker" || name == "planner" || name == "fallback"
	})

	// Test with preferred provider
	order := chain.ProviderOrder("local-worker")
	if len(order) != 1 || order[0] != "local-worker" {
		t.Errorf("Expected [local-worker], got %v", order)
	}

	// Test without preferred (should use default + fallbacks)
	order = chain.ProviderOrder("")
	expected := []string{"local-worker", "planner", "fallback"}
	if len(order) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, order)
	}
	for i, provider := range expected {
		if order[i] != provider {
			t.Errorf("Position %d: expected %s, got %s", i, provider, order[i])
		}
	}
}

func TestDefaultFallbackChain_ProviderOrderForContext(t *testing.T) {
	config := DefaultFallbackConfig()
	chain := NewDefaultFallbackChain(config, func(name string) bool {
		return name == "local-worker" || name == "planner"
	})

	// Test with tokens within local-worker limit
	order := chain.ProviderOrderForContext("", 3000, nil, false)
	if len(order) == 0 || order[0] != "local-worker" {
		t.Errorf("Expected local-worker first for 3000 tokens, got %v", order)
	}

	// Test with tokens exceeding local-worker limit
	order = chain.ProviderOrderForContext("", 5000, nil, false)
	if len(order) == 0 {
		t.Error("Expected at least one provider for 5000 tokens")
	}
	// Should not include local-worker if it can't handle tokens
	for _, provider := range order {
		if provider == "local-worker" {
			t.Error("local-worker should not be in chain for 5000 tokens (max 4000)")
		}
	}
}

func TestDefaultFallbackChain_IsRetryable(t *testing.T) {
	config := DefaultFallbackConfig()
	chain := NewDefaultFallbackChain(config, func(name string) bool { return true })

	// Test retryable errors
	retryableErrors := []error{
		errors.New("timeout exceeded"),
		errors.New("rate limit exceeded"),
		errors.New("server error 500"),
		errors.New("connection refused"),
		errors.New("bad gateway"),
	}
	for _, err := range retryableErrors {
		if !chain.IsRetryable(err) {
			t.Errorf("Expected retryable: %v", err)
		}
	}

	// Test non-retryable errors
	nonRetryableErrors := []error{
		errors.New("invalid input"),
		errors.New("permission denied"),
		errors.New("not found"),
		errors.New("validation failed"),
	}
	for _, err := range nonRetryableErrors {
		if chain.IsRetryable(err) {
			t.Errorf("Expected non-retryable: %v", err)
		}
	}
}

func TestExecuteWithFallback(t *testing.T) {
	// Create providers (using providers from default config)
	providers := map[string]llm.Provider{
		"local-worker": &mockProvider{name: "local-worker", shouldErr: true, errMsg: "rate limit exceeded"},
		"planner":      &mockProvider{name: "planner", shouldErr: false},
	}

	// Create chain that checks provider availability
	config := DefaultFallbackConfig()
	chain := NewDefaultFallbackChain(config, func(name string) bool {
		_, ok := providers[name]
		return ok
	})

	// Create request
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Execute with fallback (prefers local-worker which fails, should fall back to planner)
	ctx := context.Background()
	resp, err := ExecuteWithFallback(ctx, chain, providers, req, "local-worker", nil, false)
	
	if err != nil {
		t.Fatalf("ExecuteWithFallback failed: %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected response from planner provider")
	}
	
	expectedContent := "Mock response from planner"
	if resp.Content != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, resp.Content)
	}
}

func TestRetryConfig(t *testing.T) {
	config := RetryConfig()
	
	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts=3, got %d", config.MaxAttempts)
	}
	
	if config.InitialDelay != 200*time.Millisecond {
		t.Errorf("Expected InitialDelay=200ms, got %v", config.InitialDelay)
	}
	
	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay=5s, got %v", config.MaxDelay)
	}
	
	if !config.Jitter {
		t.Error("Expected Jitter=true")
	}
	
	// Test retryable errors function
	retryableErr := errors.New("rate limit exceeded")
	if !config.RetryableErrors(retryableErr) {
		t.Error("Expected rate limit error to be retryable")
	}
	
	nonRetryableErr := errors.New("invalid input")
	if config.RetryableErrors(nonRetryableErr) {
		t.Error("Expected invalid input to be non-retryable")
	}
}

func TestSmartProviderOrder(t *testing.T) {
	config := &FallbackConfig{
		DefaultProvider: "local",
		FallbackOrder:   []string{"cloud", "fallback"},
		ProviderCapabilities: map[string]ProviderCapability{
			"local":    {MaxContextTokens: 4000},
			"cloud":    {MaxContextTokens: 128000},
			"fallback": {MaxContextTokens: 128000},
		},
		EnableSmartRouting: true,
	}
	
	chain := NewDefaultFallbackChain(config, func(name string) bool {
		return name == "local" || name == "cloud"
	})
	
	// Test with tokens within local limit
	order := chain.ProviderOrderForContext("", 3000, nil, false)
	if len(order) == 0 || order[0] != "local" {
		t.Errorf("Expected local first for 3000 tokens, got %v", order)
	}
	
	// Test with tokens exceeding local limit
	order = chain.ProviderOrderForContext("", 5000, nil, false)
	if len(order) == 0 || order[0] != "cloud" {
		t.Errorf("Expected cloud first for 5000 tokens, got %v", order)
	}
}

func TestSessionContextAwareRouting(t *testing.T) {
	config := DefaultFallbackConfig()
	chain := NewDefaultFallbackChain(config, func(name string) bool {
		return name == "local-worker" || name == "planner" || name == "special"
	})
	
	// Test with session context that includes special provider
	sessionContext := []string{"special", "planner"}
	order := chain.ProviderOrderForContext("", 3000, sessionContext, false)
	
	// Should prefer session context providers
	if len(order) == 0 {
		t.Error("Expected providers from session context")
	}
	
	// Check that providers are from session context
	for _, provider := range order {
		found := false
		for _, ctxProvider := range sessionContext {
			if provider == ctxProvider {
				found = true
				break
			}
		}
		if !found && provider != "local-worker" {
			t.Errorf("Provider %s not in session context and not default", provider)
		}
	}
}
