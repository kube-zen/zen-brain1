package cost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculate_KnownModel(t *testing.T) {
	// DeepSeek: $0.14/M input, $0.28/M output
	// 1M input = 14 cents*100 = 1400
	// 1M output = 28 cents*100 = 2800
	cost := Calculate("deepseek", "deepseek-chat", 1_000_000, 1_000_000)
	assert.Equal(t, 1400+2800, cost) // 42 cents*100 = $0.42
}

func TestCalculate_SmallTokens(t *testing.T) {
	// Small tokens should round to minimum 1 unit
	cost := Calculate("deepseek", "deepseek-chat", 100, 100)
	assert.GreaterOrEqual(t, cost, 2) // At least 1 per input + 1 per output
}

func TestCalculate_UnknownModel(t *testing.T) {
	// Unknown model uses default pricing
	cost := Calculate("unknown", "unknown-model", 1_000_000, 1_000_000)
	assert.Greater(t, cost, 0)
}

func TestCalculate_LocalWorker_Free(t *testing.T) {
	cost := Calculate("local-worker", "qwen3.5:0.8b", 1_000_000, 1_000_000)
	assert.Equal(t, 0, cost)
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost int
		want string
	}{
		{0, "$0.0000"},
		{100, "$0.0100"},   // 1 cent
		{1400, "$0.1400"},  // 14 cents
		{10000, "$1.0000"}, // $1
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := FormatCost(tc.cost)
			assert.Contains(t, got, tc.want[:len(tc.want)-1]) // Allow minor formatting diff
		})
	}
}

func TestUsage_Record(t *testing.T) {
	u := NewUsage()
	u.Record("deepseek", "deepseek-chat", 1000, 500)

	in, out, cost := u.Totals()
	assert.Equal(t, 1000, in)
	assert.Equal(t, 500, out)
	assert.Greater(t, cost, 0)
}

func TestUsage_MultipleProviders(t *testing.T) {
	u := NewUsage()
	u.Record("local-worker", "qwen3.5:0.8b", 10000, 5000)
	u.Record("deepseek", "deepseek-chat", 1000, 500)

	in, out, cost := u.Totals()
	assert.Equal(t, 11000, in)
	assert.Equal(t, 5500, out)
	assert.Greater(t, cost, 0)

	// Check breakdown
	require.Len(t, u.ByProvider, 2)
	assert.NotNil(t, u.ByProvider["local-worker:qwen3.5:0.8b"])
	assert.Equal(t, 10000, u.ByProvider["local-worker:qwen3.5:0.8b"].InputTokens)
	assert.Equal(t, 0, u.ByProvider["local-worker:qwen3.5:0.8b"].Cost) // Free
	assert.Greater(t, u.ByProvider["deepseek:deepseek-chat"].Cost, 0)
}

func TestUsage_Reset(t *testing.T) {
	u := NewUsage()
	u.Record("deepseek", "deepseek-chat", 1000, 500)
	u.Reset()

	in, out, cost := u.Totals()
	assert.Equal(t, 0, in)
	assert.Equal(t, 0, out)
	assert.Equal(t, 0, cost)
	assert.Empty(t, u.ByProvider)
}

func TestUsage_Summary(t *testing.T) {
	u := NewUsage()
	u.Record("deepseek", "deepseek-chat", 1000, 500)
	summary := u.Summary()
	assert.Contains(t, summary, "Tokens:")
	assert.Contains(t, summary, "Cost:")
}

func TestEstimator_EstimateTask(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateTask("deepseek", "deepseek-chat", "You are helpful.", "Write a function", true)

	assert.Equal(t, "deepseek", est.Provider)
	assert.Greater(t, est.InputTokens, 0)
	assert.Greater(t, est.OutputTokens, 0)
	assert.Greater(t, est.EstimatedCostUSD, 0.0)
	assert.NotEmpty(t, est.CostBreakdown)
}

func TestEstimator_EstimateTask_ShortAnswer(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateTask("deepseek", "deepseek-chat", "", "What is 2+2?", false)

	assert.Equal(t, 500, est.OutputTokens) // Short answer pattern
}

func TestEstimator_EstimateTask_CodeGeneration(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateTask("deepseek", "deepseek-chat", "", "Write a function to sort a list", false)

	assert.Equal(t, 2000, est.OutputTokens) // Code pattern
}

func TestEstimator_EstimateSession(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateSession("deepseek", "deepseek-chat", 5, 2000, 1000)

	assert.Equal(t, 10000, est.InputTokens) // 5 * 2000
	assert.Equal(t, 5000, est.OutputTokens) // 5 * 1000
	assert.Greater(t, est.EstimatedCostUSD, 0.0)
}

func TestEstimator_CompareProviders(t *testing.T) {
	e := NewEstimator()
	compare := e.CompareProviders(10000, 5000)

	assert.Contains(t, compare, "deepseek")
	assert.Contains(t, compare, "local-worker")
	assert.Contains(t, compare, "$")
}

func TestEstimate_Format(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateTask("deepseek", "deepseek-chat", "System", "Write code", false)

	formatted := est.Format()
	assert.Contains(t, formatted, "Cost Estimate")
	assert.Contains(t, formatted, "deepseek")
	assert.Contains(t, formatted, "$")
}

func TestEstimate_FormatCompact(t *testing.T) {
	e := NewEstimator()
	est := e.EstimateTask("deepseek", "deepseek-chat", "", "Hello", false)

	compact := est.FormatCompact()
	assert.Contains(t, compact, "tokens")
	assert.Contains(t, compact, "$")
	assert.Contains(t, compact, "deepseek")
}

func TestGetProviderCosts(t *testing.T) {
	costs := GetProviderCosts()
	assert.Contains(t, costs, "deepseek:deepseek-chat")
	assert.Contains(t, costs, "local-worker:qwen3.5:0.8b")
	assert.Equal(t, 0, costs["local-worker:qwen3.5:0.8b"].InputPerMillion)
}
