// Package llm provides the LLM Gateway interface for zen-brain.
package llm

import (
	"context"
	"errors"
)

// ErrEmbeddingNotSupported is returned when a provider doesn't support embeddings.
var ErrEmbeddingNotSupported = errors.New("embedding not supported by this provider")

// ErrProviderNotFound is returned when a provider doesn't exist.
var ErrProviderNotFound = errors.New("provider not found")

// Provider is the interface that all LLM providers must implement.
// This is the core abstraction that allows zen-brain to work with
// multiple LLM backends (OpenAI, Anthropic, local models, etc.)
// without depending on any specific implementation.
type Provider interface {
	// Name returns the provider name (e.g., "openai", "anthropic", "ollama")
	Name() string

	// SupportsTools returns true if this provider supports function calling
	SupportsTools() bool

	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatStream sends a chat request and streams tokens to the callback.
	// Providers that don't support streaming should fall back to Chat.
	ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) (*ChatResponse, error)

	// Embed generates an embedding for the input text (optional).
	// Returns ErrEmbeddingNotSupported if the provider doesn't support embeddings.
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

// ProviderFactory creates providers by name.
// This is used to create providers dynamically based on configuration.
type ProviderFactory interface {
	// CreateProvider creates a provider by name.
	// Returns ErrProviderNotFound if the provider doesn't exist.
	CreateProvider(name string) (Provider, error)

	// CreateProviderWithModel creates a provider with a model override.
	CreateProviderWithModel(name, model string) (Provider, error)

	// ListProviders returns available provider names.
	ListProviders() []string
}

// Router selects the best provider for a request.
// This enables intelligent routing based on cost, latency, capabilities, etc.
type Router interface {
	// Route selects the best provider for a request.
	// Returns the provider and the routing decision reason.
	Route(ctx context.Context, req ChatRequest) (Provider, string, error)

	// RouteForEmbedding selects the best provider for embedding.
	RouteForEmbedding(ctx context.Context, req EmbeddingRequest) (Provider, string, error)
}
