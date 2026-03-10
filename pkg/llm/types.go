// Package llm provides the LLM Gateway interface for zen-brain.
// This package defines provider-agnostic types and interfaces for LLM interactions.
//
// Design based on zen-brain 0.1 internal/ai/interface.go with improvements:
// - Added cluster context for multi-cluster support (V6)
// - Added streaming support with proper error handling
// - Separated concerns: types vs interface
package llm

// Message represents a single message in the conversation.
type Message struct {
	// Role is the message role: "user", "assistant", "system", "tool"
	Role string `json:"role"`

	// Content is the text content of the message
	Content string `json:"content"`

	// ReasoningContent contains chain-of-thought reasoning (for models that support it)
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// ToolCalls contains tool calls made in this message (assistant role only)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID is the ID of the tool call this message responds to (tool role only)
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Metadata contains additional context (e.g., timestamps, sources)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call in a message.
type ToolCall struct {
	// ID is the unique identifier for this tool call
	ID string `json:"id"`

	// Name is the name of the tool to call
	Name string `json:"name"`

	// Args contains the arguments to pass to the tool
	Args map[string]interface{} `json:"args"`
}

// Tool represents a function tool that can be called by the LLM.
type Tool struct {
	// Name is the tool name (must be unique)
	Name string `json:"name"`

	// Description explains what the tool does (shown to LLM)
	Description string `json:"description"`

	// Parameters is the JSON Schema for the tool parameters
	Parameters map[string]interface{} `json:"parameters"`
}

// ThinkingLevel represents the depth of model reasoning.
type ThinkingLevel string

const (
	ThinkingOff    ThinkingLevel = "off"
	ThinkingLow    ThinkingLevel = "low"    // Basic reasoning
	ThinkingMedium ThinkingLevel = "medium" // Moderate reasoning
	ThinkingHigh   ThinkingLevel = "high"   // Deep reasoning
)

// ChatRequest represents a chat request to an LLM provider.
type ChatRequest struct {
	// Messages is the conversation history
	Messages []Message `json:"messages"`

	// Tools available for the LLM to call
	Tools []Tool `json:"tools,omitempty"`

	// Model override (if empty, uses provider default)
	Model string `json:"model,omitempty"`

	// Temperature controls randomness (0.0-2.0)
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens limits the response length
	MaxTokens int `json:"max_tokens,omitempty"`

	// ContextLimit limits the number of messages sent
	ContextLimit int `json:"context_limit,omitempty"`

	// Thinking enables chain-of-thought reasoning (legacy)
	Thinking bool `json:"thinking,omitempty"`

	// ThinkingLevel controls reasoning depth (preferred over Thinking)
	ThinkingLevel ThinkingLevel `json:"thinking_level,omitempty"`

	// Stream enables streaming responses
	Stream bool `json:"stream,omitempty"`

	// ClusterID for multi-cluster routing (V6)
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project-aware routing (V6)
	ProjectID string `json:"project_id,omitempty"`

	// SessionID for session tracking
	SessionID string `json:"session_id,omitempty"`

	// TaskID for task tracking
	TaskID string `json:"task_id,omitempty"`

	// SkipPrePass skips warden/router pre-pass (already run for this turn)
	SkipPrePass bool `json:"skip_pre_pass,omitempty"`
}

// ChatResponse represents a chat response from an LLM provider.
type ChatResponse struct {
	// Content is the text content of the response
	Content string `json:"content"`

	// ReasoningContent contains chain-of-thought reasoning
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// FinishReason indicates why generation stopped
	FinishReason string `json:"finish_reason"`

	// ToolCalls contains tool calls requested by the LLM
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Model is the actual model used
	Model string `json:"model,omitempty"`

	// Usage contains token usage statistics
	Usage *TokenUsage `json:"usage,omitempty"`

	// LatencyMs is the request latency in milliseconds
	LatencyMs int64 `json:"latency_ms,omitempty"`
}

// TokenUsage contains token usage statistics.
type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CachedTokens int64 `json:"cached_tokens,omitempty"`
	TotalTokens  int64 `json:"total_tokens"`
}

// StreamCallback is called for each token during streaming.
type StreamCallback func(token string)

// EmbeddingRequest represents a request to generate embeddings.
type EmbeddingRequest struct {
	// Input is the text to embed
	Input string `json:"input"`

	// Model override (if empty, uses provider default)
	Model string `json:"model,omitempty"`
}

// EmbeddingResponse represents an embedding response.
type EmbeddingResponse struct {
	// Embedding is the vector representation
	Embedding []float32 `json:"embedding"`

	// Model is the actual model used
	Model string `json:"model,omitempty"`

	// Dimension is the embedding dimension
	Dimension int `json:"dimension"`
}
