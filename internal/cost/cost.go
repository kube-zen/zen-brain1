// Package cost implements token cost tracking for AI providers.
// Adapted from zen-brain 0.1 internal/cost.
//
// Pricing uses integer cents*100 for precision (no floating point errors).
// e.g., $0.14/M = 14 cents = 1400 (cents * 100)
package cost

import (
	"fmt"
	"sync"
)

// Pricing represents cost per million tokens (in USD cents * 100 for precision)
type Pricing struct {
	InputPerMillion  int // cents * 100 per million input tokens
	OutputPerMillion int // cents * 100 per million output tokens
}

// Provider costs as of Mar 2026. Prices in cents*100 per million tokens.
// Example: $0.14/M = 14 cents = 1400 (cents*100)
var providerCosts = map[string]Pricing{
	// Local worker — free
	"local-worker:qwen3.5:0.8b": {InputPerMillion: 0, OutputPerMillion: 0},

	// DeepSeek - very cheap: $0.14/M input, $0.28/M output
	"deepseek:deepseek-chat": {InputPerMillion: 1400, OutputPerMillion: 2800},

	// Qwen - cheap
	"qwen-portal:qwen-flash-us":                  {InputPerMillion: 500, OutputPerMillion: 1500},
	"qwen-portal:qwen-plus-us":                   {InputPerMillion: 5000, OutputPerMillion: 15000},
	"qwen-portal:qwen3-coder-30b-a3b-instruct":   {InputPerMillion: 700, OutputPerMillion: 4500},
	"qwen-portal:qwen3-235b-a22b-instruct-2507":  {InputPerMillion: 2000, OutputPerMillion: 8000},
	"qwen-portal:qwen3-coder-480b-a35b-instruct": {InputPerMillion: 5000, OutputPerMillion: 20000},

	// MiniMax: $0.30/M
	"minimax:MiniMax-M2.1": {InputPerMillion: 3000, OutputPerMillion: 3000},

	// Kimi
	"kimi:moonshot-v1-8k":   {InputPerMillion: 3000, OutputPerMillion: 6000},
	"kimi:moonshot-v1-32k":  {InputPerMillion: 6000, OutputPerMillion: 12000},
	"kimi:moonshot-v1-128k": {InputPerMillion: 12000, OutputPerMillion: 24000},

	// GLM (Z.AI)
	"glm:GLM-4.7":             {InputPerMillion: 5000, OutputPerMillion: 5000},
	"glm:GLM-5":               {InputPerMillion: 7500, OutputPerMillion: 7500},
	"glm:GLM-5-Turbo":         {InputPerMillion: 2500, OutputPerMillion: 2500},
	"glm:GLM-5.1":             {InputPerMillion: 10000, OutputPerMillion: 10000},
	"zen-glm:GLM-5":           {InputPerMillion: 7500, OutputPerMillion: 7500},
	"zen-glm:GLM-5-Turbo":     {InputPerMillion: 2500, OutputPerMillion: 2500},
	"zen-glm:GLM-5.1":         {InputPerMillion: 10000, OutputPerMillion: 10000},
	"zenmesh-glm:GLM-5":       {InputPerMillion: 7500, OutputPerMillion: 7500},
	"zenmesh-glm:GLM-5-Turbo": {InputPerMillion: 2500, OutputPerMillion: 2500},
	"zenmesh-glm:GLM-5.1":     {InputPerMillion: 10000, OutputPerMillion: 10000},

	// OpenAI
	"openai:gpt-4o-mini": {InputPerMillion: 1500, OutputPerMillion: 6000},
	"openai:gpt-4o":      {InputPerMillion: 25000, OutputPerMillion: 100000},
	"openai:gpt-4-turbo": {InputPerMillion: 100000, OutputPerMillion: 300000},
	"openai:o1-mini":     {InputPerMillion: 30000, OutputPerMillion: 120000},
	"openai:o1":          {InputPerMillion: 150000, OutputPerMillion: 600000},

	// Anthropic
	"anthropic:claude-3-5-haiku-20241022":  {InputPerMillion: 1000, OutputPerMillion: 5000},
	"anthropic:claude-3-5-sonnet-20241022": {InputPerMillion: 30000, OutputPerMillion: 150000},
	"anthropic:claude-3-opus-20240229":     {InputPerMillion: 150000, OutputPerMillion: 750000},

	// Ollama (local) - free
	"ollama:qwen3.5:0.8b":        {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:llama3.2:3b":         {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:mistral:7b":          {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:deepseek-coder:6.7b": {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:phi:2.7b":            {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:qwen2.5:1.5b":        {InputPerMillion: 0, OutputPerMillion: 0},
	"ollama:qwen3.5:2b":          {InputPerMillion: 0, OutputPerMillion: 0},
}

// Usage tracks token usage for a session or time window.
type Usage struct {
	mu sync.RWMutex

	InputTokens  int
	OutputTokens int
	TotalCost    int // cents * 100

	// Per-provider breakdown
	ByProvider map[string]*ProviderUsage
}

// ProviderUsage tracks usage per provider:model.
type ProviderUsage struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	Calls        int
	Cost         int // cents * 100
}

// NewUsage creates a new usage tracker.
func NewUsage() *Usage {
	return &Usage{
		ByProvider: make(map[string]*ProviderUsage),
	}
}

// Record records token usage for a call.
func (u *Usage) Record(provider, model string, inputTokens, outputTokens int) {
	u.mu.Lock()
	defer u.mu.Unlock()

	cost := Calculate(provider, model, inputTokens, outputTokens)

	u.InputTokens += inputTokens
	u.OutputTokens += outputTokens
	u.TotalCost += cost

	key := provider + ":" + model
	if u.ByProvider[key] == nil {
		u.ByProvider[key] = &ProviderUsage{
			Provider: provider,
			Model:    model,
		}
	}

	pu := u.ByProvider[key]
	pu.InputTokens += inputTokens
	pu.OutputTokens += outputTokens
	pu.Calls++
	pu.Cost += cost
}

// Calculate returns cost in cents * 100 for given tokens.
func Calculate(provider, model string, inputTokens, outputTokens int) int {
	key := provider + ":" + model
	pricing, ok := providerCosts[key]
	if !ok {
		// Unknown model - use conservative estimate
		pricing = Pricing{InputPerMillion: 100, OutputPerMillion: 300}
	}

	// Cost = (tokens / 1M) * price_per_M
	costIn := (inputTokens * pricing.InputPerMillion) / 1_000_000
	costOut := (outputTokens * pricing.OutputPerMillion) / 1_000_000

	// Minimum 1 unit if any tokens used (only for non-free providers)
	if pricing.InputPerMillion > 0 {
		if costIn == 0 && inputTokens > 0 {
			costIn = 1
		}
	}
	if pricing.OutputPerMillion > 0 {
		if costOut == 0 && outputTokens > 0 {
			costOut = 1
		}
	}

	return costIn + costOut
}

// FormatCost formats cost in cents * 100 to human readable USD.
func FormatCost(cost int) string {
	dollars := float64(cost) / 10000.0 // cents*100 → dollars
	if dollars < 0.01 {
		return fmt.Sprintf("$0.%04d", cost)
	}
	return fmt.Sprintf("$%.4f", dollars)
}

// Summary returns a formatted summary of usage.
func (u *Usage) Summary() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return fmt.Sprintf(
		"Tokens: %d in / %d out | Cost: %s",
		u.InputTokens,
		u.OutputTokens,
		FormatCost(u.TotalCost),
	)
}

// Totals returns input tokens, output tokens, and cost (cents*100).
func (u *Usage) Totals() (inputTokens, outputTokens int, costCents100 int) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.InputTokens, u.OutputTokens, u.TotalCost
}

// Reset clears all usage data.
func (u *Usage) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.InputTokens = 0
	u.OutputTokens = 0
	u.TotalCost = 0
	u.ByProvider = make(map[string]*ProviderUsage)
}

// GetProviderCosts returns the cost table for display.
func GetProviderCosts() map[string]Pricing {
	return providerCosts
}
