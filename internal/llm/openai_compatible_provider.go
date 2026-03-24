// Package llm: OpenAI-compatible HTTP provider (e.g. zen-glm / Z.AI GLM-5).
// POST baseURL/chat/completions with Authorization: Bearer <apiKey>.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

const defaultZenGLMBaseURL = "https://api.z.ai/api/coding/paas/v4"

// OpenAICompatibleProvider calls an OpenAI-compatible chat/completions API (e.g. zen-glm).
type OpenAICompatibleProvider struct {
	name    string
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

// NewOpenAICompatibleProvider creates a provider for baseURL (e.g. Z.AI), model (e.g. GLM-5), apiKey.
// baseURL is trimmed and must not include /chat/completions.
func NewOpenAICompatibleProvider(name, baseURL, model, apiKey string) *OpenAICompatibleProvider {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultZenGLMBaseURL
	}
	if model == "" {
		model = "GLM-5"
	}
	return NewOpenAICompatibleProviderWithTimeout(name, baseURL, model, apiKey, 120*time.Second)
}

// NewOpenAICompatibleProviderWithTimeout creates a provider with custom timeout (for llama.cpp long-running requests).
func NewOpenAICompatibleProviderWithTimeout(name, baseURL, model, apiKey string, timeout time.Duration) *OpenAICompatibleProvider {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultZenGLMBaseURL
	}
	if model == "" {
		model = "GLM-5"
	}
	client := &http.Client{
		Timeout: timeout,
	}
	return &OpenAICompatibleProvider{
		name:    name,
		baseURL: baseURL,
		model:   model,
		apiKey:  strings.TrimSpace(apiKey),
		client:  client,
	}
}

// Name returns the provider name.
func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

// SupportsTools returns true.
func (p *OpenAICompatibleProvider) SupportsTools() bool {
	return true
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRequest struct {
	Model     string       `json:"model"`
	Messages  []oaiMessage  `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Stream    bool          `json:"stream"`
}

type oaiChoice struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends the request to the OpenAI-compatible endpoint.
func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}
	messages := req.Messages
	if len(messages) == 0 {
		return nil, fmt.Errorf("%s: messages required", p.name)
	}
	oaiMsgs := make([]oaiMessage, 0, len(messages))
	for _, m := range messages {
		content := m.Content
		if content == "" && m.Role == "assistant" {
			content = "."
		}
		oaiMsgs = append(oaiMsgs, oaiMessage{Role: m.Role, Content: content})
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := oaiRequest{
		Model:     model,
		Messages:  oaiMsgs,
		MaxTokens: maxTokens,
		Stream:    false,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s request marshal: %w", p.name, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", p.name, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read response: %w", p.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(respBody)
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return nil, fmt.Errorf("%s API error: status %d %s", p.name, resp.StatusCode, msg)
	}
	var oaiResp oaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("%s parse response: %w", p.name, err)
	}
	if oaiResp.Error != nil && oaiResp.Error.Message != "" {
		return nil, fmt.Errorf("%s: %s", p.name, oaiResp.Error.Message)
	}
	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("%s returned no choices", p.name)
	}
	choice := oaiResp.Choices[0]
	content := choice.Message.Content
	latencyMs := time.Since(start).Milliseconds()
	log.Printf("[%s] request completed in %dms", p.name, latencyMs)
	return &llm.ChatResponse{
		Content:      content,
		FinishReason: choice.FinishReason,
		Model:        model,
		Usage:        &llm.TokenUsage{TotalTokens: 0},
		LatencyMs:    latencyMs,
	}, nil
}

// ChatStream falls back to non-streaming Chat.
func (p *OpenAICompatibleProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	return p.Chat(ctx, req)
}

// Embed is not supported.
func (p *OpenAICompatibleProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}
