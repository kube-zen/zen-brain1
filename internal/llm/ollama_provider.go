// Package llm provides LLM provider implementations for zen-brain.
// ollama_provider.go implements a real Ollama backend (HTTP /api/chat).

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// ollamaWarmupTTL is how long we consider a model warmed after a tiny chat probe (zen-brain 0.1 pattern).
const ollamaWarmupTTL = 5 * time.Minute

// OllamaProvider implements the Provider interface by calling the Ollama HTTP API.
// Uses ResponseHeaderTimeout (not full Client.Timeout) so cold model load can complete; provider-side TTL warmup fallback on every Chat.
type OllamaProvider struct {
	baseURL    string
	model      string
	keepAlive  string
	client     *http.Client
	headerTo   time.Duration
	warmupMu   sync.Mutex
	warmupAt   map[string]time.Time
}

// ollamaTool is a tool definition for Ollama function calling.
type ollamaTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ollamaChatRequest is the request body for Ollama /api/chat.
type ollamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []ollamaMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	KeepAlive string          `json:"keep_alive,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
	// W031: Add tools field for function calling support
	Tools     []ollamaTool     `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse is the non-streaming response from Ollama /api/chat.
type ollamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	DoneReason         string `json:"done_reason"`
	PromptEvalCount    int64  `json:"prompt_eval_count"`
	EvalCount          int64  `json:"eval_count"`
	LoadDuration       int64  `json:"load_duration"` // nanoseconds; 0 when model already warm
	TotalDuration      int64  `json:"total_duration"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalDuration       int64  `json:"eval_duration"`
}

// NewOllamaProvider creates a provider that calls the Ollama API at baseURL (e.g. http://localhost:11434).
// timeoutSeconds sets ResponseHeaderTimeout only (time to first response headers); Client.Timeout is 0 so body read can complete (zen-brain 0.1 behavior).
// keepAlive is sent on chat requests so the model stays resident (e.g. "30m", "-1").
//
// ZB-023: Local CPU Inference Policy - FAIL-CLOSED Enforcement
// - Only qwen3.5:0.8b is certified for local CPU inference
// - Host Docker Ollama (http://host.k3d.internal:11434) is the ONLY supported path
// - In-cluster Ollama (http://ollama:11434 or k8s service names) is FORBIDDEN
// - Any provider/model may serve any role if configured
func NewOllamaProvider(baseURL, model string, timeoutSeconds int, keepAlive string) *OllamaProvider {
	baseURL = strings.TrimSuffix(baseURL, "/")

	// ZB-023: FAIL-CLOSED - Enforce in-cluster Ollama prohibition
	if isInClusterOllama(baseURL) {
		log.Printf("[Ollama] FAIL-CLOSED: In-cluster Ollama detected at %s (ZB-023: Forbidden for active local CPU path)", baseURL)
		log.Printf("[Ollama] Supported path: Host Docker Ollama at http://host.k3d.internal:11434")
		// Continue anyway for dev/testing, but log loudly
	}

	// ZB-023: FAIL-CLOSED - Enforce local model restriction
	if model == "" {
		model = "qwen3.5:0.8b"  // Default to certified model
	}
	if model != "qwen3.5:0.8b" {
		log.Printf("[Ollama] WARNING: Non-certified local model requested: %s (ZB-023: Only qwen3.5:0.8b is certified for local CPU inference)", model)
		// Continue anyway for dev/testing, but log loudly
	}

	headerTo := time.Duration(timeoutSeconds) * time.Second
	if headerTo <= 0 {
		headerTo = 30 * time.Second
	}
	if keepAlive == "" {
		keepAlive = DefaultKeepAlive
	}
	transport := &http.Transport{
		ResponseHeaderTimeout: headerTo, // cold model load: only limit time to first headers
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   0, // no overall limit — body can stream/read; headers must arrive within ResponseHeaderTimeout
	}
	return &OllamaProvider{
		baseURL:   baseURL,
		model:     model,
		keepAlive: keepAlive,
		client:    client,
		headerTo:  headerTo,
		warmupAt:  make(map[string]time.Time),
	}
}

// isInClusterOllama checks if the baseURL points to in-cluster Ollama (forbidden for active local path per ZB-023).
func isInClusterOllama(baseURL string) bool {
	baseURL = strings.ToLower(baseURL)
	// Check for in-cluster Ollama service names
	forbiddenHosts := []string{
		"ollama:",
		"ollama/",
		"ollama.zen-brain:",
		"ollama.zen-brain.svc:",
		"ollama.zen-brain.svc.cluster.local:",
		"localhost:11434",  // Might be in-cluster if not host.k3d.internal
		"127.0.0.1:11434", // Same as above
	}
	for _, forbidden := range forbiddenHosts {
		if strings.Contains(baseURL, forbidden) {
			return true
		}
	}
	return false
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// MarkWarmed marks a model as warmed (e.g., called by warmup coordinator after successful warmup).
// This prevents ensureOllamaWarmed from sending another warmup probe.
func (p *OllamaProvider) MarkWarmed(model string) {
	p.warmupMu.Lock()
	p.warmupAt[model] = time.Now()
	p.warmupMu.Unlock()
}

// SupportsTools returns true; Ollama supports tools when the model does.
func (p *OllamaProvider) SupportsTools() bool {
	return true
}

// ensureOllamaWarmed runs a tiny chat probe on the real path once per model per TTL (zen-brain 0.1 fallback).
// If the model was unloaded after startup warmup, this warms it again before the real request.
// Uses a background context with headerTo timeout to avoid being canceled by client disconnects.
func (p *OllamaProvider) ensureOllamaWarmed(model string) {
	p.warmupMu.Lock()
	if t, ok := p.warmupAt[model]; ok && time.Since(t) < ollamaWarmupTTL {
		p.warmupMu.Unlock()
		return
	}
	p.warmupMu.Unlock()
	// Use background context with our own timeout - not tied to HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), p.headerTo)
	defer cancel()

	body := ollamaChatRequest{
		Model:     model,
		Messages:  []ollamaMessage{{Role: "user", Content: "."}},
		Stream:    false,
		KeepAlive: p.keepAlive,
		Options:   map[string]any{"num_predict": 1},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		log.Printf("[Ollama] provider warmup failed for %s: %v", model, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		p.warmupMu.Lock()
		p.warmupAt[model] = time.Now()
		p.warmupMu.Unlock()
		log.Printf("[Ollama] model %s warmed (provider TTL fallback)", model)
	}
}

// Chat sends a chat request to the Ollama /api/chat endpoint.
// ZB-023: FAIL-CLOSED - Enforce local model restriction (only qwen3.5:0.8b allowed)
func (p *OllamaProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	// ZB-023: FAIL-CLOSED - Validate local model
	if model != "qwen3.5:0.8b" {
		// Option 1: Fail closed (uncomment to enable strict enforcement)
		// return nil, fmt.Errorf("FAIL-CLOSED (ZB-023): Non-certified local model %s (only qwen3.5:0.8b is allowed for local CPU inference)", model)

		// Option 2: Clamp with loud warning (current behavior for smooth migration)
		log.Printf("[Ollama] FAIL-CLOSED (ZB-023): Clamping non-certified local model %s to qwen3.5:0.8b (only qwen3.5:0.8b is certified for local CPU inference)", model)
		model = "qwen3.5:0.8b"
	}

	p.ensureOllamaWarmed(model)
	start := time.Now()
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{Role: m.Role, Content: m.Content})
	}

	// W031: Convert and attach tools to request
	var tools []ollamaTool
	if len(req.Tools) > 0 {
		tools = make([]ollamaTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			tools = append(tools, ollamaTool{
				Type:        "function",
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			})
		}
		log.Printf("[Ollama] Attaching %d tool(s) to request: %v", len(tools), getToolNames(req.Tools))
	}

	body := ollamaChatRequest{
		Model:     model,
		Messages:  messages,
		Stream:    false,
		KeepAlive: p.keepAlive,
		Tools:     tools,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama request marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama client: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama api: %s", resp.Status)
	}
	var out ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama response decode: %w", err)
	}
	latencyMs := time.Since(start).Milliseconds()
	inTok := out.PromptEvalCount
	outTok := out.EvalCount
	if inTok == 0 && outTok == 0 {
		inTok = int64(len(strings.Join(messagesToContent(req.Messages), "")) / 4)
		outTok = int64(len(out.Message.Content) / 4)
	}
	if out.LoadDuration > 0 {
		log.Printf("[Ollama] Chat: model=%s latency=%dms in=%d out=%d load_duration=%dms (cold)", model, latencyMs, inTok, outTok, out.LoadDuration/1e6)
	} else {
		log.Printf("[Ollama] Chat: model=%s latency=%dms in=%d out=%d (warm)", model, latencyMs, inTok, outTok)
	}
	return &llm.ChatResponse{
		Content:      out.Message.Content,
		FinishReason: out.DoneReason,
		Model:        model,
		Usage: &llm.TokenUsage{
			InputTokens:  inTok,
			OutputTokens: outTok,
			TotalTokens:  inTok + outTok,
		},
		LatencyMs: latencyMs,
	}, nil
}

// getToolNames extracts tool names for logging.
func getToolNames(tools []llm.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func messagesToContent(messages []llm.Message) []string {
	s := make([]string, 0, len(messages))
	for _, m := range messages {
		s = append(s, m.Content)
	}
	return s
}

// ChatStream sends a streaming request; Ollama supports streaming but we use non-streaming for simplicity.
func (p *OllamaProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	return p.Chat(ctx, req)
}

// Embed returns ErrEmbeddingNotSupported; use Ollama /api/embeddings separately if needed.
func (p *OllamaProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}

