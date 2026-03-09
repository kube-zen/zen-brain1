// Package routing provides LLM provider routing and fallback logic.
// This pattern is rescued from zen-brain 0.1, adapted for 1.0's clean boundaries.
// Uses zen-sdk/pkg/retry for exponential backoff with jitter.
package routing

import (
	"context"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
	zenretry "github.com/kube-zen/zen-sdk/pkg/retry"
)

// FallbackChain defines provider fallback ordering and error classification.
// This pattern is rescued from zen-brain 0.1, adapted for 1.0's clean boundaries.
type FallbackChain interface {
	// ProviderOrder returns the list of provider names to try (preferred first, then fallbacks).
	ProviderOrder(preferred string) []string
	
	// ProviderOrderForContext returns context-aware chain based on estimated tokens,
	// session context, and strict preferred provider requirements.
	ProviderOrderForContext(preferred string, estimatedTokens int, sessionContext []string, strictPreferred bool) []string
	
	// IsRetryable returns true if the error should trigger fallback to next provider.
	IsRetryable(err error) bool
}

// DefaultFallbackChain implements FallbackChain with sensible defaults.
// This is adapted from zen-brain 0.1's gateway/default_fallback_chain.go.
type DefaultFallbackChain struct {
	// ProviderChecker checks if a provider is available.
	ProviderChecker func(name string) bool
	
	// Config provides provider capabilities and limits.
	Config *FallbackConfig
}

// FallbackConfig holds configuration for fallback chain behavior.
type FallbackConfig struct {
	// DefaultProvider is the default provider to use when none is specified.
	DefaultProvider string
	
	// FallbackOrder is the ordered list of providers to try as fallbacks.
	FallbackOrder []string
	
	// ProviderCapabilities maps provider names to their token limits.
	ProviderCapabilities map[string]ProviderCapability
	
	// EnableSmartRouting enables context-aware routing.
	EnableSmartRouting bool
}

// ProviderCapability defines a provider's capabilities.
type ProviderCapability struct {
	// MaxContextTokens is the maximum context length supported.
	MaxContextTokens int
	
	// CostPerToken is the approximate cost per token (for routing decisions).
	CostPerToken float64
	
	// SupportsTools indicates if the provider supports function calling.
	SupportsTools bool
}

// NewDefaultFallbackChain creates a new fallback chain with the given configuration.
func NewDefaultFallbackChain(config *FallbackConfig, providerChecker func(name string) bool) *DefaultFallbackChain {
	if config == nil {
		config = DefaultFallbackConfig()
	}
	
	return &DefaultFallbackChain{
		ProviderChecker: providerChecker,
		Config:         config,
	}
}

// DefaultFallbackConfig returns a sensible default configuration.
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		DefaultProvider: "local-worker",
		FallbackOrder:   []string{"planner", "fallback"},
		ProviderCapabilities: map[string]ProviderCapability{
			"local-worker": {
				MaxContextTokens: 4000,
				CostPerToken:     0.000001, // Very cheap (local)
				SupportsTools:    true,
			},
			"planner": {
				MaxContextTokens: 128000,
				CostPerToken:     0.00002, // Cloud pricing
				SupportsTools:    true,
			},
			"fallback": {
				MaxContextTokens: 128000,
				CostPerToken:     0.00002,
				SupportsTools:    true,
			},
		},
		EnableSmartRouting: true,
	}
}

// ProviderOrder implements FallbackChain.ProviderOrder.
func (f *DefaultFallbackChain) ProviderOrder(preferred string) []string {
	// If preferred provider is specified and available, use it
	if preferred != "" && f.ProviderChecker(preferred) {
		return []string{preferred}
	}
	
	// Otherwise use default + fallbacks
	chain := []string{}
	if f.Config.DefaultProvider != "" && f.ProviderChecker(f.Config.DefaultProvider) {
		chain = append(chain, f.Config.DefaultProvider)
	}
	
	for _, provider := range f.Config.FallbackOrder {
		if !contains(chain, provider) && f.ProviderChecker(provider) {
			chain = append(chain, provider)
		}
	}
	
	return chain
}

// ProviderOrderForContext implements FallbackChain.ProviderOrderForContext.
func (f *DefaultFallbackChain) ProviderOrderForContext(preferred string, estimatedTokens int, sessionContext []string, strictPreferred bool) []string {
	// Strict preferred mode: use only the preferred provider if available and can handle tokens
	if strictPreferred && preferred != "" && f.ProviderChecker(preferred) {
		if f.canHandleTokens(preferred, estimatedTokens) {
			return []string{preferred}
		}
		// If strict preferred but can't handle tokens, return empty (will fall through)
	}
	
	// Filter session context to available providers
	if len(sessionContext) > 0 {
		availableContext := f.filterToAvailable(sessionContext)
		if len(availableContext) > 0 {
			chain := []string{}
			
			// Include preferred if in context and can handle tokens
			if preferred != "" {
				for _, provider := range availableContext {
					if provider == preferred {
						if f.canHandleTokens(preferred, estimatedTokens) {
							chain = append(chain, preferred)
						}
						break
					}
				}
			}
			
			// Add other context providers that can handle tokens
			for _, provider := range availableContext {
				if !contains(chain, provider) && f.canHandleTokens(provider, estimatedTokens) {
					chain = append(chain, provider)
				}
			}
			
			if len(chain) > 0 {
				return chain
			}
		}
	}
	
	// If preferred provider specified and can handle tokens, start with it
	// (but include fallbacks unless strictPreferred is true)
	chain := []string{}
	if preferred != "" && f.ProviderChecker(preferred) && f.canHandleTokens(preferred, estimatedTokens) {
		chain = append(chain, preferred)
	}
	
	// Use smart routing if enabled
	if f.Config.EnableSmartRouting {
		return f.smartProviderOrder(estimatedTokens, preferred)
	}
	
	// Default to simple provider order (which includes fallbacks)
	return f.ProviderOrder(preferred)
}

// IsRetryable implements FallbackChain.IsRetryable.
// Classifies errors as retryable based on error message patterns.
// This pattern is rescued from zen-brain 0.1's error classification.
func (f *DefaultFallbackChain) IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Retry on transient errors
	retryablePatterns := []string{
		"timeout",
		"deadline",
		"rate limit",
		"rate exceeded",
		"too many requests",
		"server error",
		"internal error",
		"connection refused",
		"connection reset",
		"temporary failure",
		"service unavailable",
		"gateway timeout",
		"bad gateway",
	}
	
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	
	return false
}

// canHandleTokens checks if a provider can handle the estimated token count.
func (f *DefaultFallbackChain) canHandleTokens(provider string, estimatedTokens int) bool {
	if cap, ok := f.Config.ProviderCapabilities[provider]; ok {
		return estimatedTokens <= cap.MaxContextTokens
	}
	return true // Assume yes if capability unknown
}

// filterToAvailable filters provider names to those that are available.
func (f *DefaultFallbackChain) filterToAvailable(providers []string) []string {
	var result []string
	for _, provider := range providers {
		if f.ProviderChecker(provider) {
			result = append(result, provider)
		}
	}
	return result
}

// smartProviderOrder selects providers based on token capacity and cost.
// If preferredProvider is specified and can handle tokens, it will be first.
func (f *DefaultFallbackChain) smartProviderOrder(estimatedTokens int, preferredProvider string) []string {
	var chain []string
	
	// Start with preferred provider if specified and can handle tokens
	if preferredProvider != "" && f.ProviderChecker(preferredProvider) && 
	   f.canHandleTokens(preferredProvider, estimatedTokens) {
		chain = append(chain, preferredProvider)
	}
	
	// Add default provider if not already included and can handle tokens
	if f.Config.DefaultProvider != "" && f.ProviderChecker(f.Config.DefaultProvider) && 
	   f.canHandleTokens(f.Config.DefaultProvider, estimatedTokens) && !contains(chain, f.Config.DefaultProvider) {
		chain = append(chain, f.Config.DefaultProvider)
	}
	
	// Add fallback providers that can handle tokens
	for _, provider := range f.Config.FallbackOrder {
		if f.ProviderChecker(provider) && f.canHandleTokens(provider, estimatedTokens) && !contains(chain, provider) {
			chain = append(chain, provider)
		}
	}
	
	if len(chain) == 0 {
		// No providers can handle the tokens, use all available
		chain = f.ProviderOrder("")
	}
	
	return chain
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RetryConfig provides retry configuration using zen-sdk/pkg/retry.
func RetryConfig() zenretry.Config {
	return zenretry.Config{
		MaxAttempts:    3,
		InitialDelay:   200 * time.Millisecond,
		MaxDelay:       5 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
		JitterPercent:  0.1,
		RetryableErrors: func(err error) bool {
			// Delegate to fallback chain's IsRetryable
			chain := NewDefaultFallbackChain(DefaultFallbackConfig(), func(name string) bool { return true })
			return chain.IsRetryable(err)
		},
	}
}

// ExecuteWithFallback executes an LLM request with fallback chain support.
// This pattern is rescued from zen-brain 0.1's provider fallback logic, adapted for 1.0.
func ExecuteWithFallback(ctx context.Context, chain FallbackChain, providers map[string]llm.Provider, req llm.ChatRequest, preferredProvider string, sessionContext []string, strictPreferred bool) (*llm.ChatResponse, error) {
	providerOrder := chain.ProviderOrderForContext(
		preferredProvider,
		estimateTokens(req.Messages),
		sessionContext,
		strictPreferred,
	)
	
	var lastErr error
	for _, providerName := range providerOrder {
		provider, ok := providers[providerName]
		if !ok {
			continue
		}
		
		resp, err := provider.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		
		lastErr = err
		
		// Check if error is retryable for next provider
		if !chain.IsRetryable(err) {
			return nil, err
		}
	}
	
	return nil, lastErr
}

// estimateTokens provides a rough token estimate for messages.
func estimateTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4 // Rough estimate: 4 chars per token
		if msg.ReasoningContent != "" {
			total += len(msg.ReasoningContent) / 4
		}
	}
	return total
}
