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
	ModelID   string  `json:"model_id"`
	Reason    string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// RecommendModel returns the recommended model for the given project and task type.
// Uses ledger efficiency data (success rate, cost); falls back to DefaultModel when no data.
func (r *ModelRouter) RecommendModel(ctx context.Context, projectID, taskType string) (*ModelRecommendation, error) {
	if r.LedgerClient == nil {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Reason:     "No ledger configured",
			Confidence: 0,
		}, nil
	}
	efficiencies, err := r.LedgerClient.GetModelEfficiency(ctx, projectID, taskType)
	if err != nil {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Reason:     fmt.Sprintf("Ledger query failed: %v", err),
			Confidence: 0,
		}, nil
	}
	minSamples := r.MinSamples
	if minSamples < 1 {
		minSamples = 10
	}
	var best ledger.ModelEfficiency
	var bestScore float64
	for _, eff := range efficiencies {
		if eff.SampleSize < minSamples {
			continue
		}
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
	if bestScore == 0 {
		return &ModelRecommendation{
			ModelID:    r.DefaultModel,
			Reason:     "No suitable model in efficiency data",
			Confidence: 0.5,
		}, nil
	}
	return &ModelRecommendation{
		ModelID:    best.ModelID,
		Reason:     fmt.Sprintf("Best efficiency: %.1f%% success, $%.3f avg cost", best.SuccessRate*100, best.AvgCostPerTask),
		Confidence: best.SuccessRate,
	}, nil
}
