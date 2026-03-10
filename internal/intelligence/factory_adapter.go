// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// FactoryRecommenderInterface is the interface that the Factory expects for intelligence recommendations.
// This interface is defined in the intelligence package to avoid circular dependencies.
type FactoryRecommenderInterface interface {
	// RecommendTemplate suggests a template based on work type and domain.
	// Returns template name or "default" if no recommendation available.
	RecommendTemplate(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (string, error)

	// RecommendTemplateWithMetadata returns template name plus source, confidence, and reasoning for persistence.
	RecommendTemplateWithMetadata(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (templateName, source string, confidence float64, reasoning string, err error)

	// RecommendConfiguration suggests execution configuration (timeout, retries).
	RecommendConfiguration(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (timeoutSeconds int64, maxRetries int, err error)
}

// FactoryRecommenderAdapter adapts the intelligence Recommender to the FactoryRecommenderInterface.
type FactoryRecommenderAdapter struct {
	recommender *Recommender
}

// NewFactoryRecommenderAdapter creates a new factory recommender adapter.
func NewFactoryRecommenderAdapter(recommender *Recommender) *FactoryRecommenderAdapter {
	return &FactoryRecommenderAdapter{
		recommender: recommender,
	}
}

// RecommendTemplate suggests a template based on work type and domain.
func (a *FactoryRecommenderAdapter) RecommendTemplate(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (string, error) {
	rec, err := a.recommender.RecommendTemplate(ctx, workType, workDomain)
	if err != nil {
		return "", err
	}
	return rec.TemplateName, nil
}

// RecommendTemplateWithMetadata returns template name plus source, confidence, and reasoning.
func (a *FactoryRecommenderAdapter) RecommendTemplateWithMetadata(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (templateName, source string, confidence float64, reasoning string, err error) {
	rec, err := a.recommender.RecommendTemplate(ctx, workType, workDomain)
	if err != nil {
		return "default", "static", 0, "", err
	}
	if rec.SampleCount == 0 || rec.TemplateName == "default" {
		return rec.TemplateName, "static", rec.Confidence, rec.Reasoning, nil
	}
	return rec.TemplateName, "recommended", rec.Confidence, rec.Reasoning, nil
}

// RecommendConfiguration suggests execution configuration (timeout, retries).
func (a *FactoryRecommenderAdapter) RecommendConfiguration(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (timeoutSeconds int64, maxRetries int, err error) {
	config, err := a.recommender.RecommendConfiguration(ctx, workType, workDomain)
	if err != nil {
		return 0, 0, err
	}
	return config.TimeoutSeconds, config.MaxRetries, nil
}

// Ensure FactoryRecommenderAdapter implements FactoryRecommenderInterface
var _ FactoryRecommenderInterface = (*FactoryRecommenderAdapter)(nil)

// MiningIntegration provides a high-level interface for integrating mining into the Factory.
type MiningIntegration struct {
	miner       *Miner
	recommender *Recommender
	adapter     *FactoryRecommenderAdapter
}

// NewMiningIntegration creates a new mining integration instance.
func NewMiningIntegration(runtimeDir string, patternStore PatternStore, kbAdapter *KBPatternAdapter) *MiningIntegration {
	miner := NewMiner(runtimeDir, patternStore)
	miner.SetKBAdapter(kbAdapter)

	recommender := NewRecommender(patternStore, 3)
	adapter := NewFactoryRecommenderAdapter(recommender)

	return &MiningIntegration{
		miner:       miner,
		recommender: recommender,
		adapter:     adapter,
	}
}

// MineProofOfWorks executes a mining operation on all proof-of-work artifacts.
func (m *MiningIntegration) MineProofOfWorks(ctx context.Context) (*MiningResult, error) {
	return m.miner.MineProofOfWorks(ctx)
}

// GetFactoryRecommender returns the factory recommender adapter for use by the Factory.
func (m *MiningIntegration) GetFactoryRecommender() FactoryRecommenderInterface {
	return m.adapter
}

// GetRecommender returns the raw intelligence recommender.
func (m *MiningIntegration) GetRecommender() *Recommender {
	return m.recommender
}

// GetMiner returns the raw miner.
func (m *MiningIntegration) GetMiner() *Miner {
	return m.miner
}

// GetPatternAnalysis returns a pattern analysis.
func (m *MiningIntegration) GetPatternAnalysis(ctx context.Context) (*PatternAnalysis, error) {
	return m.recommender.PatternAnalysis(ctx)
}
