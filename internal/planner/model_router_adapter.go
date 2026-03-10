// Package planner provides an adapter so internal/intelligence.ModelRouter satisfies ModelRecommender.
package planner

import (
	"context"

	"github.com/kube-zen/zen-brain1/internal/intelligence"
)

// modelRouterAdapter adapts *intelligence.ModelRouter to ModelRecommender.
type modelRouterAdapter struct {
	router *intelligence.ModelRouter
}

// RecommendModel implements ModelRecommender.
func (a *modelRouterAdapter) RecommendModel(ctx context.Context, projectID, taskType string) (modelID, reason string, confidence float64, err error) {
	if a.router == nil {
		return "", "", 0, nil
	}
	rec, err := a.router.RecommendModel(ctx, projectID, taskType)
	if err != nil {
		return "", "", 0, err
	}
	if rec == nil {
		return "", "", 0, nil
	}
	return rec.ModelID, rec.Reason, rec.Confidence, nil
}

// NewModelRouterRecommender returns a ModelRecommender that uses the given ModelRouter (Block 5 routing).
func NewModelRouterRecommender(router *intelligence.ModelRouter) ModelRecommender {
	if router == nil {
		return nil
	}
	return &modelRouterAdapter{router: router}
}
