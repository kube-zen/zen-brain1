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
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// OllamaProvider implements the Provider interface by calling the Ollama HTTP API.
// Use when OLLAMA_BASE_URL is set; otherwise the gateway uses the simulated LocalWorkerProvider.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
	timeout time.Duration
}

// ollamaChatRequest is the request body for Ollama /api/chat.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
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
	TotalDuration      int64  `json:"total_duration"` // nanoseconds
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalDuration       int64  `json:"eval_duration"`
}

// NewOllamaProvider creates a provider that calls the Ollama API at baseURL (e.g. http://localhost:11434).
func NewOllamaProvider(baseURL, model string, timeoutSeconds int) *OllamaProvider {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if model == "" {
		model = "qwen3.5:0.8b"
	}
	t := time.Duration(timeoutSeconds) * time.Second
	if t <= 0 {
		t = 30 * time.Second
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		timeout: t,
		client:  &http.Client{Timeout: t},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// SupportsTools returns true; Ollama supports tools when the model does.
func (p *OllamaProvider) SupportsTools() bool {
	return true
}

// Chat sends a chat request to the Ollama /api/chat endpoint.
func (p *OllamaProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	start := time.Now()
	model := p.model
	if req.Model != "" {
		model = req.Model
	}
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{Role: m.Role, Content: m.Content})
	}
	body := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
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
	log.Printf("[Ollama] Chat: model=%s latency=%dms in=%d out=%d", model, latencyMs, inTok, outTok)
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
