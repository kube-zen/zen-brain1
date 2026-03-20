// Package llm provides LLM Gateway implementation for zen-brain.
// This package implements the Provider, Router, and ProviderFactory interfaces
// to provide a complete LLM gateway with local worker and planner escalation lanes.
// Uses zen-sdk/pkg/retry for exponential backoff with jitter.
package llm

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/llm/routing"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
	"github.com/kube-zen/zen-brain1/pkg/llm"
	zenretry "github.com/kube-zen/zen-sdk/pkg/retry"
)

// GatewayConfig holds configuration for the LLM Gateway.
type GatewayConfig struct {
	// Provider configurations
	LocalWorkerModel string `yaml:"local_worker_model" json:"local_worker_model"`
	PlannerModel     string `yaml:"planner_model" json:"planner_model"`
	FallbackModel    string `yaml:"fallback_model" json:"fallback_model"`

	// Cost thresholds (USD)
	LocalWorkerMaxCost float64 `yaml:"local_worker_max_cost" json:"local_worker_max_cost"`
	PlannerMinCost     float64 `yaml:"planner_min_cost" json:"planner_min_cost"`

	// Timeouts (seconds)
	LocalWorkerTimeout int `yaml:"local_worker_timeout" json:"local_worker_timeout"`
	PlannerTimeout     int `yaml:"planner_timeout" json:"planner_timeout"`
	RequestTimeout     int `yaml:"request_timeout" json:"request_timeout"`

	// Ollama keep_alive (e.g. "30m", "-1"); sent on preload, verify, and real chat so model stays resident
	LocalWorkerKeepAlive string `yaml:"local_worker_keep_alive" json:"local_worker_keep_alive"`

	// Model capabilities
	LocalWorkerSupportsTools bool `yaml:"local_worker_supports_tools" json:"local_worker_supports_tools"`
	PlannerSupportsTools     bool `yaml:"planner_supports_tools" json:"planner_supports_tools"`

	// Routing policy
	AutoEscalateComplexTasks bool   `yaml:"auto_escalate_complex_tasks" json:"auto_escalate_complex_tasks"`
	RoutingPolicy            string `yaml:"routing_policy" json:"routing_policy"` // "simple", "cost_aware", "performance"

	// Fallback chain configuration
	EnableFallbackChain  bool `yaml:"enable_fallback_chain" json:"enable_fallback_chain"`
	StrictPreferred      bool `yaml:"strict_preferred" json:"strict_preferred"`             // Only use preferred provider if true
	LocalWorkerMaxTokens int  `yaml:"local_worker_max_tokens" json:"local_worker_max_tokens"` // Skip local worker when estimated tokens exceed this (0 = no limit)
}

// DefaultGatewayConfig returns the default gateway configuration.
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		LocalWorkerModel:         "qwen3.5:0.8b", // Small local model
		PlannerModel:             "glm-4.7",      // Cloud model for complex tasks
		FallbackModel:            "glm-4.7",      // Fallback to cloud
		LocalWorkerMaxCost:       0.01,           // $0.01 max for local worker
		PlannerMinCost:           0.10,           // $0.10 min for planner (cloud costs)
		LocalWorkerTimeout:       30,             // 30 seconds
		PlannerTimeout:           60,             // 60 seconds
		RequestTimeout:           120,            // 120 seconds overall
		LocalWorkerKeepAlive:     "30m",         // keep model resident
		LocalWorkerSupportsTools: true,           // Local models support tools
		PlannerSupportsTools:     true,           // Cloud models support tools
		AutoEscalateComplexTasks: true,           // Auto-escalate complex tasks
		RoutingPolicy:            "simple",       // Simple routing policy
		EnableFallbackChain:      true,           // Enable intelligent provider fallback
		StrictPreferred:          false,          // Allow fallback to other providers
	}
}

// Gateway implements the LLM Provider, Router, and ProviderFactory interfaces.
// It provides a unified gateway with local worker and planner escalation lanes.
type Gateway struct {
	config *GatewayConfig
	mu     sync.RWMutex

	// Provider registry
	providers map[string]llm.Provider

	// Factory for creating providers
	factory llm.ProviderFactory

	// Router for selecting providers
	router llm.Router

	// Fallback chain for intelligent provider selection
	fallbackChain routing.FallbackChain

	// Optional: record token usage to ZenLedger (Block 5 Intelligence)
	tokenRecorder ledger.TokenRecorder

	// Statistics
	stats *GatewayStats
}

// GatewayStats tracks gateway usage statistics.
type GatewayStats struct {
	mu sync.RWMutex

	// Counters
	TotalRequests       int64 `json:"total_requests"`
	LocalWorkerRequests int64 `json:"local_worker_requests"`
	PlannerRequests     int64 `json:"planner_requests"`
	FallbackRequests    int64 `json:"fallback_requests"`
	TimeoutErrors       int64 `json:"timeout_errors"`
	RoutingErrors       int64 `json:"routing_errors"`

	// Latencies (ms)
	TotalLatencyMs       int64 `json:"total_latency_ms"`
	LocalWorkerLatencyMs int64 `json:"local_worker_latency_ms"`
	PlannerLatencyMs     int64 `json:"planner_latency_ms"`

	// Success rates
	LocalWorkerSuccess int64 `json:"local_worker_success"`
	PlannerSuccess     int64 `json:"planner_success"`
	FallbackSuccess    int64 `json:"fallback_success"`
}

// retryWithRetryable wraps a provider call with zen-sdk retry logic.
// It provides exponential backoff, jitter, and configurable retry attempts.
func (g *Gateway) retryWithRetryable(ctx context.Context, providerName string, fn func() (*llm.ChatResponse, error)) (*llm.ChatResponse, error) {
	var lastResponse *llm.ChatResponse
	var lastErr error

	// Configure retry for LLM provider calls
	retryConfig := zenretry.Config{
		MaxAttempts:   3,                      // Retry up to 3 times
		InitialDelay:  200 * time.Millisecond, // Start with 200ms
		MaxDelay:      5 * time.Second,        // Max 5s between retries
		Multiplier:    2.0,                    // Exponential backoff (2x)
		Jitter:        true,                   // Add jitter to prevent thundering herd
		JitterPercent: 0.1,                    // 10% jitter
		RetryableErrors: func(err error) bool {
			// Retry on transient errors: timeouts, rate limits, server errors
			if err == nil {
				return false
			}
			// Check for timeout errors
			if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
				return false
			}
			// Retry on network-like errors (simplified check)
			errStr := err.Error()
			return strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "deadline") ||
				strings.Contains(errStr, "rate limit") ||
				strings.Contains(errStr, "server error") ||
				strings.Contains(errStr, "connection refused")
		},
	}

	// Execute with retry logic
	_ = zenretry.Do(ctx, retryConfig, func() error {
		resp, err := fn()
		if err != nil {
			lastErr = err
			return err
		}
		// Handle case where provider returns nil response without error
		if resp == nil {
			lastErr = fmt.Errorf("provider returned nil response without error")
			return lastErr
		}
		lastResponse = resp
		return nil
	})

	// Check if we succeeded
	if lastResponse != nil {
		// Update success stats
		g.stats.mu.Lock()
		g.stats.TotalLatencyMs += lastResponse.LatencyMs
		switch providerName {
		case "local-worker":
			g.stats.LocalWorkerSuccess++
			g.stats.LocalWorkerLatencyMs += lastResponse.LatencyMs
		case "planner":
			g.stats.PlannerSuccess++
			g.stats.PlannerLatencyMs += lastResponse.LatencyMs
		case "fallback":
			g.stats.FallbackSuccess++
		}
		g.stats.mu.Unlock()

		return lastResponse, nil
	}

	// All retries failed, return last error
	g.stats.mu.Lock()
	g.stats.TimeoutErrors++
	g.stats.mu.Unlock()

	return nil, lastErr
}

// NewGateway creates a new LLM Gateway.
func NewGateway(config *GatewayConfig) (*Gateway, error) {
	if config == nil {
		config = DefaultGatewayConfig()
	}

	g := &Gateway{
		config:    config,
		providers: make(map[string]llm.Provider),
		stats:     &GatewayStats{},
	}

	// Create factory and router
	g.factory = g
	g.router = g

	// Register built-in providers
	if err := g.registerBuiltinProviders(); err != nil {
		return nil, fmt.Errorf("failed to register built-in providers: %w", err)
	}

	// Initialize fallback chain if enabled
	if config.EnableFallbackChain {
		g.initializeFallbackChain()
	}

	log.Printf("[LLM Gateway] Initialized with config: local_worker=%s planner=%s timeout=%ds fallback=%v",
		config.LocalWorkerModel, config.PlannerModel, config.RequestTimeout, config.EnableFallbackChain)

	return g, nil
}

// registerBuiltinProviders registers the built-in providers.
// When OLLAMA_BASE_URL is set, the local-worker lane uses a real Ollama provider; otherwise the simulated LocalWorkerProvider is used.
// When ZEN_GLM_API_KEY is set, the zen-glm provider is registered (Z.AI GLM-5).
//
// ZB-023: Local CPU Inference Policy - Startup Validation
// - Only qwen3.5:0.8b is certified for local CPU inference
// - Host Docker Ollama (http://host.k3d.internal:11434) is ONLY supported path
// - In-cluster Ollama is FORBIDDEN for active local path
func (g *Gateway) registerBuiltinProviders() error {
	var localWorker llm.Provider
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		keepAlive := g.config.LocalWorkerKeepAlive
		if keepAlive == "" {
			keepAlive = DefaultKeepAlive
		}
		localWorker = NewOllamaProvider(baseURL, g.config.LocalWorkerModel, g.config.LocalWorkerTimeout, keepAlive)

		// ZB-023: Log certified local path clearly
		if g.config.LocalWorkerModel == "qwen3.5:0.8b" {
			log.Printf("[LLM Gateway] ZB-023: Local worker lane - Ollama at %s (model=%s, CERTIFIED local CPU path)", baseURL, g.config.LocalWorkerModel)
		} else {
			log.Printf("[LLM Gateway] ZB-023 WARNING: Local worker lane - Ollama at %s (model=%s, NOT CERTIFIED - only qwen3.5:0.8b is certified)", baseURL, g.config.LocalWorkerModel)
		}
	} else {
		localWorker = NewLocalWorkerProvider(g.config.LocalWorkerModel, g.config.LocalWorkerTimeout)
		log.Printf("[LLM Gateway] local-worker lane: simulated (set OLLAMA_BASE_URL for real Ollama)")
	}
	if err := g.RegisterProvider("local-worker", localWorker); err != nil {
		return fmt.Errorf("failed to register local worker provider: %w", err)
	}

	// zen-glm: OpenAI-compatible (Z.AI) when API key is set (from secret/env only)
	if apiKey := os.Getenv("ZEN_GLM_API_KEY"); apiKey != "" {
		baseURL := os.Getenv("ZEN_GLM_BASE_URL")
		model := os.Getenv("ZEN_GLM_MODEL")
		if model == "" {
			model = "GLM-5"
		}
		zenGLM := NewOpenAICompatibleProvider("zen-glm", baseURL, model, apiKey)
		if err := g.RegisterProvider("zen-glm", zenGLM); err != nil {
			return fmt.Errorf("failed to register zen-glm provider: %w", err)
		}
		log.Printf("[LLM Gateway] Registered provider: zen-glm (model=%s)", model)
	}

	// Register planner provider
	planner := NewPlannerProvider(g.config.PlannerModel, g.config.PlannerTimeout)
	if err := g.RegisterProvider("planner", planner); err != nil {
		return fmt.Errorf("failed to register planner provider: %w", err)
	}

	// Register fallback provider (same as planner for now)
	fallback := NewPlannerProvider(g.config.FallbackModel, g.config.PlannerTimeout)
	if err := g.RegisterProvider("fallback", fallback); err != nil {
		return fmt.Errorf("failed to register fallback provider: %w", err)
	}

	return nil
}

// initializeFallbackChain initializes the fallback chain for intelligent provider selection.
func (g *Gateway) initializeFallbackChain() {
	// Create fallback chain configuration with provider capabilities
	fallbackConfig := &routing.FallbackConfig{
		DefaultProvider: "local-worker",
		FallbackOrder:   []string{"planner", "fallback"},
		ProviderCapabilities: map[string]routing.ProviderCapability{
			"local-worker": {
				MaxContextTokens: 4000,
				CostPerToken:     0.000001, // Very cheap (local)
				SupportsTools:    g.config.LocalWorkerSupportsTools,
			},
			"zen-glm": {
				MaxContextTokens: 128000,
				CostPerToken:     0.00002,
				SupportsTools:    true,
			},
			"planner": {
				MaxContextTokens: 128000,
				CostPerToken:     0.00002, // Cloud pricing
				SupportsTools:    g.config.PlannerSupportsTools,
			},
			"fallback": {
				MaxContextTokens: 128000,
				CostPerToken:     0.00002,
				SupportsTools:    g.config.PlannerSupportsTools,
			},
		},
		EnableSmartRouting:   true,
		LocalWorkerMaxTokens: g.config.LocalWorkerMaxTokens,
	}

	// Create fallback chain with provider checker
	g.fallbackChain = routing.NewDefaultFallbackChain(fallbackConfig, func(name string) bool {
		_, exists := g.GetProvider(name)
		return exists
	})

	log.Printf("[LLM Gateway] Fallback chain initialized with smart routing")
}

// RegisterProvider registers a provider with the gateway.
func (g *Gateway) RegisterProvider(name string, provider llm.Provider) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	g.providers[name] = provider
	log.Printf("[LLM Gateway] Registered provider: %s", name)

	return nil
}

// GetProvider returns a provider by name.
func (g *Gateway) GetProvider(name string) (llm.Provider, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	provider, exists := g.providers[name]
	return provider, exists
}

// ListProviders returns all registered provider names.
func (g *Gateway) ListProviders() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.providers))
	for name := range g.providers {
		names = append(names, name)
	}

	return names
}

// Name returns the gateway name.
func (g *Gateway) Name() string {
	return "gateway"
}

// SupportsTools returns true if the gateway supports tools.
// The gateway supports tools if at least one registered provider supports tools.
func (g *Gateway) SupportsTools() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, provider := range g.providers {
		if provider.SupportsTools() {
			return true
		}
	}

	return false
}

// Chat sends a chat request through the gateway.
// It routes the request to the appropriate provider based on routing policy.
func (g *Gateway) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return g.chatWithPreferred(ctx, req, "", false)
}

// ChatWithPreferred sends a chat request using only the preferred provider when preferred is non-empty (no fallback).
// Use for proof/testing to force the local-worker (Ollama) path. When preferred is "", behaves like Chat.
func (g *Gateway) ChatWithPreferred(ctx context.Context, req llm.ChatRequest, preferred string) (*llm.ChatResponse, error) {
	return g.chatWithPreferred(ctx, req, preferred, preferred != "")
}

func (g *Gateway) chatWithPreferred(ctx context.Context, req llm.ChatRequest, preferred string, strictPreferred bool) (*llm.ChatResponse, error) {
	startTime := time.Now()
	g.stats.mu.Lock()
	g.stats.TotalRequests++
	g.stats.mu.Unlock()

	// Apply request timeout
	timeout := time.Duration(g.config.RequestTimeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var resp *llm.ChatResponse
	var err error
	var providerName string
	var reason string

	// Use fallback chain for intelligent provider routing if enabled
	if g.config.EnableFallbackChain && g.fallbackChain != nil {
		// Use ExecuteWithFallback for automatic provider fallback (or single provider when strictPreferred)
		resp, err = routing.ExecuteWithFallback(
			ctx,
			g.fallbackChain,
			g.providers,
			req,
			preferred,
			nil, // No session context
			strictPreferred,
		)

		// Determine which provider was used from the response or error
		if err != nil {
			providerName = "unknown"
			reason = fmt.Sprintf("all providers failed: %v", err)
		} else if resp != nil && resp.Model != "" {
			providerName = g.extractProviderName(resp.Model)
			reason = fmt.Sprintf("fallback chain selected provider: %s", providerName)
		} else {
			providerName = "unknown"
			reason = "fallback chain completed"
		}

		// Track which lane was used
		g.stats.mu.Lock()
		switch providerName {
		case "local-worker":
			g.stats.LocalWorkerRequests++
		case "planner":
			g.stats.PlannerRequests++
		case "fallback":
			g.stats.FallbackRequests++
		}
		g.stats.mu.Unlock()
	} else {
		// Use legacy routing (simple policy)
		provider, routeReason, routeErr := g.Route(ctx, req)
		if routeErr != nil {
			g.stats.mu.Lock()
			g.stats.RoutingErrors++
			g.stats.mu.Unlock()
			return nil, fmt.Errorf("routing failed: %w", routeErr)
		}

		providerName = provider.Name()
		reason = routeReason

		// Track which lane was used
		g.stats.mu.Lock()
		switch providerName {
		case "local-worker":
			g.stats.LocalWorkerRequests++
		case "planner":
			g.stats.PlannerRequests++
		case "fallback":
			g.stats.FallbackRequests++
		}
		g.stats.mu.Unlock()

		// Execute the chat request with retry logic
		resp, err = g.retryWithRetryable(ctx, providerName, func() (*llm.ChatResponse, error) {
			return provider.Chat(ctx, req)
		})
	}

	if err != nil {
		// Update stats for errors
		g.stats.mu.Lock()
		g.stats.RoutingErrors++
		g.stats.mu.Unlock()
		return nil, fmt.Errorf("chat request failed (provider=%s): %w", providerName, err)
	}

	// Defensive check: provider returned nil response without error
	if resp == nil {
		g.stats.mu.Lock()
		g.stats.RoutingErrors++
		g.stats.mu.Unlock()
		return nil, fmt.Errorf("provider %s returned nil response without error", providerName)
	}

	// Log routing information
	latency := time.Since(startTime).Milliseconds()
	log.Printf("[LLM Gateway] Request completed: provider=%s, reason=%s, latency=%dms",
		providerName, reason, latency)

	resp.LatencyMs = latency

	// Record token usage to ZenLedger when recorder is set (Block 5)
	g.recordTokenUsage(ctx, req, resp, providerName, latency)

	// Accumulate latency into stats
	g.stats.mu.Lock()
	g.stats.TotalLatencyMs += latency
	switch providerName {
	case "local-worker":
		g.stats.LocalWorkerSuccess++
		g.stats.LocalWorkerLatencyMs += latency
	case "planner":
		g.stats.PlannerSuccess++
		g.stats.PlannerLatencyMs += latency
	case "fallback":
		g.stats.FallbackSuccess++
	}
	g.stats.mu.Unlock()

	return resp, nil
}

// SetTokenRecorder sets the optional ZenLedger token recorder for usage tracking (Block 5).
func (g *Gateway) SetTokenRecorder(recorder ledger.TokenRecorder) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tokenRecorder = recorder
}

// recordTokenUsage sends a TokenRecord to the ledger when recorder and usage are present.
func (g *Gateway) recordTokenUsage(ctx context.Context, req llm.ChatRequest, resp *llm.ChatResponse, providerName string, latencyMs int64) {
	g.mu.RLock()
	rec := g.tokenRecorder
	g.mu.RUnlock()
	if rec == nil || resp == nil || resp.Usage == nil {
		return
	}
	src := ledger.SourceAPI
	if providerName == "local-worker" {
		src = ledger.SourceLocal
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "unknown"
	}
	taskID := req.TaskID
	if taskID == "" {
		taskID = "unknown"
	}
	record := ledger.TokenRecord{
		SessionID:      sessionID,
		TaskID:         taskID,
		AgentRole:      "worker",
		ModelID:        resp.Model,
		InferenceType:  ledger.InferenceChat,
		Source:         src,
		TokensInput:    resp.Usage.InputTokens,
		TokensOutput:   resp.Usage.OutputTokens,
		TokensCached:   resp.Usage.CachedTokens,
		CostUSD:        0, // Filled by ledger or pricing layer if needed
		LatencyMs:      latencyMs,
		Outcome:        ledger.OutcomeCompleted,
		EvidenceClass:  ledger.EvidenceSummary,
		SREDEligible:   true,
		Timestamp:      time.Now(),
		ClusterID:      req.ClusterID,
		ProjectID:     req.ProjectID,
	}
	if err := rec.Record(ctx, record); err != nil {
		log.Printf("[LLM Gateway] failed to record token usage: %v", err)
	}
}

// ChatStream sends a streaming chat request.
// Falls back to non-streaming Chat if provider doesn't support streaming.
func (g *Gateway) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	// Route to appropriate provider
	provider, reason, err := g.Route(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("routing failed: %w", err)
	}

	log.Printf("[LLM Gateway] Streaming request to %s: %s", provider.Name(), reason)

	// Try streaming, fall back to regular chat if not supported
	resp, err := provider.ChatStream(ctx, req, callback)
	if err != nil {
		// Fall back to non-streaming
		return provider.Chat(ctx, req)
	}

	return resp, nil
}

// Embed generates an embedding.
// Routes to the first provider that supports embeddings.
func (g *Gateway) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Find first provider that supports embeddings
	for _, provider := range g.providers {
		_, err := provider.Embed(ctx, req)
		if err != llm.ErrEmbeddingNotSupported {
			return provider.Embed(ctx, req)
		}
	}

	return nil, llm.ErrEmbeddingNotSupported
}

// Route implements the Router interface.
// It selects the best provider based on the routing policy.
func (g *Gateway) Route(ctx context.Context, req llm.ChatRequest) (llm.Provider, string, error) {
	// Simple routing policy (MVP)
	// 1. Use local worker for simple, tool-based tasks
	// 2. Use planner for complex tasks, planning, or when local worker fails
	// 3. Fallback to planner if local worker unavailable

	// Check if this is a complex task (based on task metadata)
	isComplex := g.isComplexTask(req)

	// Get providers
	localWorker, localWorkerExists := g.GetProvider("local-worker")
	planner, plannerExists := g.GetProvider("planner")
	fallback, fallbackExists := g.GetProvider("fallback")

	// Decision logic
	switch g.config.RoutingPolicy {
	case "simple":
		// Simple policy: use local worker unless complex task or requires tools not supported
		if localWorkerExists && !isComplex {
			if !g.config.AutoEscalateComplexTasks || g.supportsRequiredTools(localWorker, req) {
				return localWorker, "simple routing: local worker for non-complex task", nil
			}
		}

		// Use planner for complex tasks or when local worker doesn't support required tools
		if plannerExists {
			return planner, "simple routing: planner for complex task or unsupported tools", nil
		}

	case "cost_aware":
		// Cost-aware policy: estimate cost and choose accordingly
		// For MVP, we'll implement simple version
		if localWorkerExists && g.estimateCost(req) < g.config.LocalWorkerMaxCost && !isComplex {
			return localWorker, "cost-aware routing: estimated cost below threshold", nil
		}

		if plannerExists && g.estimateCost(req) >= g.config.PlannerMinCost {
			return planner, "cost-aware routing: estimated cost above planner threshold", nil
		}

	default:
		// Default to simple policy
		if localWorkerExists && !isComplex {
			return localWorker, "default routing: local worker", nil
		}
		if plannerExists {
			return planner, "default routing: planner", nil
		}
	}

	// Fallback
	if fallbackExists {
		return fallback, "fallback: no suitable provider found", nil
	}

	return nil, "", fmt.Errorf("no providers available for routing")
}

// RouteForEmbedding implements the Router interface for embeddings.
func (g *Gateway) RouteForEmbedding(ctx context.Context, req llm.EmbeddingRequest) (llm.Provider, string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Find first provider that supports embeddings
	for name, provider := range g.providers {
		_, err := provider.Embed(ctx, req)
		if err != llm.ErrEmbeddingNotSupported {
			return provider, fmt.Sprintf("provider %s supports embeddings", name), nil
		}
	}

	return nil, "", llm.ErrEmbeddingNotSupported
}

// CreateProvider implements the ProviderFactory interface.
func (g *Gateway) CreateProvider(name string) (llm.Provider, error) {
	provider, exists := g.GetProvider(name)
	if !exists {
		return nil, llm.ErrProviderNotFound
	}

	return provider, nil
}

// CreateProviderWithModel implements the ProviderFactory interface.
func (g *Gateway) CreateProviderWithModel(name, model string) (llm.Provider, error) {
	// For MVP, we ignore model override and return the base provider
	return g.CreateProvider(name)
}

// isComplexTask determines if a task is complex based on request metadata.
func (g *Gateway) isComplexTask(req llm.ChatRequest) bool {
	// Check for complex task indicators:
	// 1. High token count estimate
	// 2. Planning-related keywords in messages
	// 3. Session/task metadata indicating complexity

	// Simple heuristic for MVP
	messageCount := len(req.Messages)
	totalLength := 0
	for _, msg := range req.Messages {
		totalLength += len(msg.Content)
	}

	// If message is long or many messages, consider complex
	if totalLength > 1000 || messageCount > 5 {
		return true
	}

	// Check for planning keywords
	planningKeywords := []string{"plan", "design", "architecture", "strategy", "analyze", "review"}
	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = req.Messages[len(req.Messages)-1].Content
	}

	for _, keyword := range planningKeywords {
		if containsCaseInsensitive(lastMessage, keyword) {
			return true
		}
	}

	// Check task metadata
	if req.TaskID != "" {
		// In production, we'd look up task complexity from ledger
		// For MVP, assume all tasks with TaskID are work tasks (could be complex)
		return true
	}

	return false
}

// supportsRequiredTools checks if provider supports all tools required by the request.
func (g *Gateway) supportsRequiredTools(provider llm.Provider, req llm.ChatRequest) bool {
	if len(req.Tools) == 0 {
		return true // No tools required
	}

	return provider.SupportsTools()
}

// estimateCost estimates the cost of a request (very simple MVP).
func (g *Gateway) estimateCost(req llm.ChatRequest) float64 {
	// Very simple cost estimation for MVP
	// In production, this would use actual pricing data
	totalTokens := 0
	for _, msg := range req.Messages {
		totalTokens += len(msg.Content) / 4 // Rough estimate: 4 chars per token
	}

	// Add estimated output tokens
	totalTokens += 1000 // Assume 1000 output tokens

	// Simple pricing: $0.001 per 1000 tokens for local, $0.01 for cloud
	return 0.001 * float64(totalTokens) / 1000.0
}

// GetStats returns gateway statistics.
func (g *Gateway) GetStats() *GatewayStats {
	g.stats.mu.RLock()
	defer g.stats.mu.RUnlock()

	// Return a copy
	stats := *g.stats
	return &stats
}

// MarkLocalWorkerWarmed marks the local-worker provider's model as warmed.
// Call this after external warmup (e.g., warmup coordinator) to prevent duplicate warmup probes.
func (g *Gateway) MarkLocalWorkerWarmed() {
	g.mu.RLock()
	provider, exists := g.providers["local-worker"]
	g.mu.RUnlock()

	if !exists {
		return
	}

	if ollamaProvider, ok := provider.(*OllamaProvider); ok {
		ollamaProvider.MarkWarmed(g.config.LocalWorkerModel)
	}
}

// Helper function
func containsCaseInsensitive(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// extractProviderName extracts the provider name from the model name.
// This is a simple heuristic for tracking purposes.
func (g *Gateway) extractProviderName(modelName string) string {
	// Check for known provider model names
	modelName = strings.ToLower(modelName)

	if strings.Contains(modelName, "qwen") || strings.Contains(modelName, "local") {
		return "local-worker"
	}
	if strings.Contains(modelName, "glm") || strings.Contains(modelName, "planner") {
		return "planner"
	}

	// Default: check which provider has this model registered
	g.mu.RLock()
	defer g.mu.RUnlock()

	for name, provider := range g.providers {
		// Check if this provider would match the model
		// This is a simplified check for MVP
		if strings.Contains(strings.ToLower(provider.Name()), strings.ToLower(modelName)) {
			return name
		}
	}

	// Fallback to "fallback"
	return "fallback"
}
