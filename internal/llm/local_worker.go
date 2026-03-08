// Package llm provides LLM provider implementations for zen-brain.
package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// LocalWorkerProvider implements the Provider interface for local worker lane.
// This provider handles simple, tool-based tasks using local models.
type LocalWorkerProvider struct {
	model   string
	timeout int // seconds
}

// NewLocalWorkerProvider creates a new LocalWorkerProvider.
func NewLocalWorkerProvider(model string, timeout int) *LocalWorkerProvider {
	return &LocalWorkerProvider{
		model:   model,
		timeout: timeout,
	}
}

// Name returns the provider name.
func (p *LocalWorkerProvider) Name() string {
	return "local-worker"
}

// SupportsTools returns true if local worker supports tools.
func (p *LocalWorkerProvider) SupportsTools() bool {
	return true // Local worker lane supports tools
}

// Chat sends a chat request to the local worker.
func (p *LocalWorkerProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	startTime := time.Now()
	
	// Apply provider timeout
	timeout := time.Duration(p.timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("[LocalWorker] Processing chat request: model=%s, messages=%d, tools=%d",
		p.model, len(req.Messages), len(req.Tools))

	// Simulate local model processing
	// In production, this would call Ollama, vLLM, or similar
	time.Sleep(50 * time.Millisecond) // Simulate processing delay

	// Generate a simple response
	content := generateLocalWorkerResponse(req)
	
	// Check if tools were requested
	var toolCalls []llm.ToolCall
	if len(req.Tools) > 0 && shouldCallTools(req) {
		toolCalls = generateToolCalls(req)
	}

	latency := time.Since(startTime).Milliseconds()

	resp := &llm.ChatResponse{
		Content:      content,
		FinishReason: "stop",
		Model:        p.model,
		ToolCalls:    toolCalls,
		Usage: &llm.TokenUsage{
			InputTokens:  estimateTokens(req.Messages),
			OutputTokens: estimateTokensFromContent(content),
			TotalTokens:  estimateTokens(req.Messages) + estimateTokensFromContent(content),
		},
		LatencyMs: latency,
	}

	log.Printf("[LocalWorker] Response generated: latency=%dms, tokens=%d",
		latency, resp.Usage.TotalTokens)

	return resp, nil
}

// ChatStream sends a streaming chat request.
// For MVP, falls back to non-streaming Chat.
func (p *LocalWorkerProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	// Local worker doesn't support streaming in MVP
	// Fall back to regular chat
	return p.Chat(ctx, req)
}

// Embed generates an embedding.
// Local worker doesn't support embeddings in MVP.
func (p *LocalWorkerProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}

// generateLocalWorkerResponse generates a response for the local worker lane.
func generateLocalWorkerResponse(req llm.ChatRequest) string {
	if len(req.Messages) == 0 {
		return "I need more information to help you."
	}

	lastMessage := req.Messages[len(req.Messages)-1].Content
	
	// Simple response generation based on input
	// In production, this would be replaced with actual model inference
	if strings.Contains(strings.ToLower(lastMessage), "hello") || strings.Contains(strings.ToLower(lastMessage), "hi") {
		return "Hello! I'm the local worker. How can I help you today?"
	}

	if strings.Contains(strings.ToLower(lastMessage), "help") {
		return "I can help you with simple tasks. What would you like me to do?"
	}

	if strings.Contains(strings.ToLower(lastMessage), "tool") || strings.Contains(strings.ToLower(lastMessage), "function") {
		return "I support tool calling. Let me know what tools you'd like me to use."
	}

	// Default response
	return fmt.Sprintf("I've processed your request about: %s...\n\nAs a local worker, I can handle this task efficiently.", 
		truncateString(lastMessage, 100))
}

// shouldCallTools determines if tools should be called.
func shouldCallTools(req llm.ChatRequest) bool {
	if len(req.Tools) == 0 {
		return false
	}

	// Check if last message suggests tool usage
	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = strings.ToLower(req.Messages[len(req.Messages)-1].Content)
	}

	toolKeywords := []string{"call", "use", "invoke", "execute", "run", "tool", "function"}
	for _, keyword := range toolKeywords {
		if strings.Contains(lastMessage, keyword) {
			return true
		}
	}

	return false
}

// generateToolCalls generates example tool calls for demonstration.
func generateToolCalls(req llm.ChatRequest) []llm.ToolCall {
	if len(req.Tools) == 0 {
		return nil
	}

	// Use the first tool as an example
	tool := req.Tools[0]
	return []llm.ToolCall{
		{
			ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
			Name: tool.Name,
			Args: map[string]interface{}{
				"action": "process",
				"input":  "example input",
			},
		},
	}
}

// estimateTokens estimates token count from messages.
func estimateTokens(messages []llm.Message) int64 {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
		if msg.ReasoningContent != "" {
			total += len(msg.ReasoningContent)
		}
	}
	// Rough estimate: 4 chars per token
	return int64(total / 4)
}

// estimateTokensFromContent estimates token count from content.
func estimateTokensFromContent(content string) int64 {
	return int64(len(content) / 4)
}

// truncateString truncates a string to max length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}