// Package intelligence provides model routing, recommendations, and pattern learning (Block 5).
//
// ModelRouter provides cost-aware model recommendation using ZenLedger efficiency data.
package intelligence

import (
	"context"
	"fmt"

	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// ModelRouter recommends a model for a task type using ZenLedger efficiency data (Block 5).
type ModelRouter struct {
	LedgerClient ledger.ZenLedgerClient
	DefaultModel string
	MinSamples   int // Minimum sample size to prefer a model over default (default 10)
}

// NewModelRouter returns a model router that uses the given ledger for efficiency data.
func NewModelRouter(lc ledger.ZenLedgerClient, defaultModel string) *ModelRouter {
	if defaultModel == "" {
		defaultModel = "default"
	}
	return &ModelRouter{
		LedgerClient: lc,
		DefaultModel: defaultModel,
		MinSamples:   10,
	}
}

// ModelRecommendation is the result of recommending a model.
type ModelRecommendation struct {
	ModelID      string   `json:"model_id"`
	Source       string   `json:"source"`                 // "ledger", "fallback", "default"
	Reason       string   `json:"reason"`
	Confidence   float64  `json:"confidence"`
	SampleSize   int      `json:"sample_size,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// RecommendModel returns the recommended model for the given project and task type.
// Uses ledger efficiency data (success rate, cost); falls back to DefaultModel when no data.
func (r *ModelRouter) RecommendModel(ctx context.Context, projectID, taskType string) (*ModelRecommendation, error) {
	if r.LedgerClient == nil {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Source:     "default",
			Reason:     "No ledger configured; using default",
			Confidence: 0.0,
			SampleSize: 0,
		}, nil
	}

	efficiencies, err := r.LedgerClient.GetModelEfficiency(ctx, projectID, taskType)
	if err != nil {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Source:     "fallback",
			Reason:     fmt.Sprintf("Ledger query failed: %v; using default", err),
			Confidence: 0.0,
			SampleSize: 0,
		}, nil
	}

	minSamples := r.MinSamples
	if minSamples < 1 {
		minSamples = 10
	}

	var best ledger.ModelEfficiency
	var bestScore float64
	candidates := make([]ledger.ModelEfficiency, 0)

	for _, eff := range efficiencies {
		if eff.SampleSize >= minSamples {
			candidates = append(candidates, eff)
			costScore := 1.0
			if eff.AvgCostPerTask > 0 {
				costScore = 1.0 / eff.AvgCostPerTask
			}
			score := eff.SuccessRate * costScore
			if score > bestScore || bestScore == 0 {
				bestScore = score
				best = eff
			}
		}
	}

	if bestScore == 0 || len(candidates) == 0 {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Source:     "fallback",
			Reason:     "No suitable model with sufficient samples in efficiency data; using default",
			Confidence: 0.5,
			SampleSize: 0,
		}, nil
	}

	// Collect alternatives (other models with sufficient samples)
	alternatives := make([]string, 0)
	for _, cand := range candidates {
		if cand.ModelID != best.ModelID {
			alternatives = append(alternatives, cand.ModelID)
		}
	}

	return &ModelRecommendation{
		ModelID:      best.ModelID,
		Source:       "ledger",
		Reason:       fmt.Sprintf("Best efficiency: %.1f%% success, $%.3f avg cost, %d samples", best.SuccessRate*100, best.AvgCostPerTask, best.SampleSize),
		Confidence:   best.SuccessRate,
		SampleSize:   best.SampleSize,
		Alternatives: alternatives,
	}, nil
}
