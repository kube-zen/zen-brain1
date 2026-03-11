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

// ollamaChatRequest is the request body for Ollama /api/chat.
type ollamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []ollamaMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	KeepAlive string          `json:"keep_alive,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
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
func NewOllamaProvider(baseURL, model string, timeoutSeconds int, keepAlive string) *OllamaProvider {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if model == "" {
		model = "qwen3.5:0.8b"
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
func (p *OllamaProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}
	p.ensureOllamaWarmed(model)
	start := time.Now()
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{Role: m.Role, Content: m.Content})
	}
	body := ollamaChatRequest{
		Model:     model,
		Messages:  messages,
		Stream:    false,
		KeepAlive: p.keepAlive,
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

