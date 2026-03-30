package cost

import (
	"fmt"
	"strings"
)

// Estimate represents a cost estimate for a task
type Estimate struct {
	Provider         string
	Model            string
	InputTokens      int
	OutputTokens     int // Estimated
	CachedTokens     int // Tokens that might be cached
	EstimatedCostUSD float64
	CostBreakdown    string
	Warning          string
}

// Estimator estimates costs before running tasks
type Estimator struct {
	prices map[string]Pricing
}

// NewEstimator creates a cost estimator
func NewEstimator() *Estimator {
	return &Estimator{
		prices: providerCosts,
	}
}

// EstimateTask estimates cost for a task
func (e *Estimator) EstimateTask(provider, model, systemPrompt, userPrompt string, hasTools bool) Estimate {
	est := Estimate{
		Provider: provider,
		Model:    model,
	}

	// Estimate input tokens
	systemTokens := estimateTokens(systemPrompt)
	userTokens := estimateTokens(userPrompt)
	toolTokens := 0
	if hasTools {
		toolTokens = 2000 // ~2K tokens for tool definitions
	}

	est.InputTokens = systemTokens + userTokens + toolTokens

	// Estimate output tokens based on task type
	est.OutputTokens = estimateOutputTokens(userPrompt)

	// System prompt is often cacheable
	est.CachedTokens = systemTokens

	// Get pricing (in cents*100)
	pricing, ok := e.prices[provider+":"+model]
	if !ok {
		pricing = Pricing{InputPerMillion: 100, OutputPerMillion: 300}
		est.Warning = fmt.Sprintf("Unknown provider:model %s:%s, using default pricing", provider, model)
	}

	// Convert to USD for estimate
	inputPricePerToken := float64(pricing.InputPerMillion) / 1_000_000 / 100 // cents*100 → dollars
	outputPricePerToken := float64(pricing.OutputPerMillion) / 1_000_000 / 100

	// Calculate cost (assume no caching for estimate)
	est.EstimatedCostUSD = float64(est.InputTokens)*inputPricePerToken + float64(est.OutputTokens)*outputPricePerToken

	// Build breakdown
	est.CostBreakdown = fmt.Sprintf(
		"Input: %d tokens ($%.4f) + Output: ~%d tokens ($%.4f)",
		est.InputTokens, float64(est.InputTokens)*inputPricePerToken,
		est.OutputTokens, float64(est.OutputTokens)*outputPricePerToken,
	)

	return est
}

// EstimateSession estimates cost for a multi-turn session
func (e *Estimator) EstimateSession(provider, model string, turns int, avgInputTokens, avgOutputTokens int) Estimate {
	est := Estimate{
		Provider:     provider,
		Model:        model,
		InputTokens:  turns * avgInputTokens,
		OutputTokens: turns * avgOutputTokens,
	}

	pricing, ok := e.prices[provider+":"+model]
	if !ok {
		pricing = Pricing{InputPerMillion: 100, OutputPerMillion: 300}
	}

	inputPricePerToken := float64(pricing.InputPerMillion) / 1_000_000 / 100
	outputPricePerToken := float64(pricing.OutputPerMillion) / 1_000_000 / 100

	est.EstimatedCostUSD = float64(est.InputTokens)*inputPricePerToken + float64(est.OutputTokens)*outputPricePerToken

	est.CostBreakdown = fmt.Sprintf(
		"%d turns × (%d in + %d out) = $%.4f",
		turns, avgInputTokens, avgOutputTokens, est.EstimatedCostUSD,
	)

	return est
}

// Format formats an estimate for display
func (e *Estimate) Format() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cost Estimate (%s/%s)\n", e.Provider, e.Model))
	sb.WriteString(fmt.Sprintf("  Input:  %d tokens\n", e.InputTokens))
	sb.WriteString(fmt.Sprintf("  Output: ~%d tokens (estimated)\n", e.OutputTokens))
	if e.CachedTokens > 0 {
		sb.WriteString(fmt.Sprintf("  Cached: %d tokens (system prompt)\n", e.CachedTokens))
	}
	sb.WriteString(fmt.Sprintf("  Cost:   $%.4f USD\n", e.EstimatedCostUSD))
	sb.WriteString(fmt.Sprintf("  %s\n", e.CostBreakdown))
	if e.Warning != "" {
		sb.WriteString(fmt.Sprintf("  ⚠️  %s\n", e.Warning))
	}
	return sb.String()
}

// FormatCompact formats estimate in one line
func (e *Estimate) FormatCompact() string {
	return fmt.Sprintf("~%d tokens, ~$%.4f (%s)", e.InputTokens+e.OutputTokens, e.EstimatedCostUSD, e.Provider)
}

// estimateTokens estimates token count from text
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Rough estimate: ~4 chars per token
	return len(text) / 4
}

// estimateOutputTokens estimates output based on task type
func estimateOutputTokens(prompt string) int {
	promptLower := strings.ToLower(prompt)

	// Short answers (single-word, factual)
	shortPatterns := []string{
		"what is", "how many", "which", "when", "where", "who",
		"yes or no", "true or false",
		"summarize", "tldr", "briefly",
	}
	for _, p := range shortPatterns {
		if strings.Contains(promptLower, p) {
			return 500
		}
	}

	// Code generation
	codePatterns := []string{
		"write", "create", "implement", "generate", "build",
		"refactor", "add", "modify", "fix", "update",
		"function", "class", "method",
	}
	for _, p := range codePatterns {
		if strings.Contains(promptLower, p) {
			return 2000
		}
	}

	// Complex analysis
	analysisPatterns := []string{
		"analyze", "review", "explain", "compare", "evaluate",
		"architecture", "design", "security",
	}
	for _, p := range analysisPatterns {
		if strings.Contains(promptLower, p) {
			return 3000
		}
	}

	// Default
	return 1000
}

// CompareProviders shows cost comparison across providers
func (e *Estimator) CompareProviders(inputTokens, outputTokens int) string {
	var sb strings.Builder
	sb.WriteString("Provider Cost Comparison:\n")
	sb.WriteString(fmt.Sprintf("  (for %d input + %d output tokens)\n\n", inputTokens, outputTokens))

	providers := []struct {
		key   string
		name  string
		model string
	}{
		{"local-worker:qwen3.5:0.8b", "local-worker", "qwen3.5:0.8b"},
		{"deepseek:deepseek-chat", "deepseek", "deepseek-chat"},
		{"qwen-portal:qwen-flash-us", "qwen-portal", "qwen-flash-us"},
		{"qwen-portal:qwen-plus-us", "qwen-portal", "qwen-plus-us"},
		{"glm:GLM-5-Turbo", "glm", "GLM-5-Turbo"},
		{"openai:gpt-4o-mini", "openai", "gpt-4o-mini"},
		{"anthropic:claude-3-5-sonnet-20241022", "anthropic", "claude-3.5-sonnet"},
	}

	for _, p := range providers {
		pricing, ok := e.prices[p.key]
		if !ok {
			continue
		}
		costCents100 := (inputTokens*pricing.InputPerMillion)/1_000_000 + (outputTokens*pricing.OutputPerMillion)/1_000_000
		dollars := float64(costCents100) / 10000.0
		sb.WriteString(fmt.Sprintf("  %-12s/%-20s: $%.4f\n", p.name, p.model, dollars))
	}

	return sb.String()
}
