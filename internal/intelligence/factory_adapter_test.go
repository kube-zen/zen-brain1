// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactoryRecommenderAdapter(t *testing.T) {
	store := NewInMemoryPatternStore()
	recommender := NewRecommender(store, 3)
	adapter := NewFactoryRecommenderAdapter(recommender)

	t.Run("RecommendTemplate", func(t *testing.T) {
		ctx := context.Background()

		// Without historical data, should return default
		templateName, err := adapter.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)
		assert.Equal(t, "default", templateName)

		// Add some historical data
		store.StorePatterns(ctx, &MiningResult{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(time.Hour),
			Duration:  time.Hour,
			WorkTypeStatistics: []WorkTypeStatistics{
				{
					WorkType:       string(contracts.WorkTypeImplementation),
					WorkDomain:     string(contracts.DomainFactory),
					TotalRuns:      10,
					SuccessfulRuns: 9,
					AverageDuration: 5 * time.Minute,
				},
			},
			TemplateStatistics: []TemplateStatistics{
				{
					TemplateName:    "implementation:real",
					TotalRuns:       10,
					SuccessfulRuns:  9,
					AverageDuration: 5 * time.Minute,
				},
			},
		})

		// Now should recommend based on data
		templateName, err = adapter.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)
		assert.Equal(t, "implementation:real", templateName)
	})

	t.Run("RecommendConfiguration", func(t *testing.T) {
		ctx := context.Background()

		// Without historical data, should return conservative defaults
		timeout, retries, err := adapter.RecommendConfiguration(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)
		assert.Equal(t, int64(300), timeout) // 5 minutes default
		assert.Equal(t, 3, retries)

		// Add duration data
		now := time.Now()
		store.StorePatterns(ctx, &MiningResult{
			StartTime: now,
			EndTime:   now.Add(time.Hour),
			Duration:  time.Hour,
			DurationStatistics: []DurationStatistics{
				{
					WorkType:      string(contracts.WorkTypeImplementation),
					WorkDomain:    string(contracts.DomainFactory),
					Samples:       []time.Duration{4*time.Minute, 5*time.Minute, 6*time.Minute, 8*time.Minute, 10*time.Minute},
					P95Duration:   10 * time.Minute,
					P99Duration:   10 * time.Minute,
				},
			},
			WorkTypeStatistics: []WorkTypeStatistics{
				{
					WorkType:   string(contracts.WorkTypeImplementation),
					WorkDomain: string(contracts.DomainFactory),
					TotalRuns:  10,
					SuccessRate: 0.9,
				},
			},
		})

		// Should recommend based on P95 duration
		timeout, retries, err = adapter.RecommendConfiguration(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)
		assert.Equal(t, int64(1200), timeout) // 2 * P95 (10 min)
		assert.Equal(t, 3, retries)           // High success rate, default retries
	})
}

func TestMiningIntegration(t *testing.T) {
	store := NewInMemoryPatternStore()
	kbAdapter := NewKBPatternAdapter(nil) // Nil KB store is ok for testing

	t.Run("CreateMiningIntegration", func(t *testing.T) {
		integration := NewMiningIntegration("/tmp/test-runtime", store, kbAdapter)

		assert.NotNil(t, integration.GetMiner())
		assert.NotNil(t, integration.GetRecommender())
		assert.NotNil(t, integration.GetFactoryRecommender())
	})

	t.Run("GetFactoryRecommenderImplementsInterface", func(t *testing.T) {
		integration := NewMiningIntegration("/tmp/test-runtime", store, kbAdapter)

		// Verify the adapter implements the FactoryRecommenderInterface
		var _ FactoryRecommenderInterface = integration.GetFactoryRecommender()
	})

	t.Run("GetPatternAnalysis", func(t *testing.T) {
		ctx := context.Background()
		integration := NewMiningIntegration("/tmp/test-runtime", store, kbAdapter)

		// Add some data
		now := time.Now()
		store.StorePatterns(ctx, &MiningResult{
			StartTime: now,
			EndTime:   now.Add(time.Hour),
			Duration:  time.Hour,
			WorkTypeStatistics: []WorkTypeStatistics{
				{
					WorkType:       string(contracts.WorkTypeImplementation),
					WorkDomain:     string(contracts.DomainFactory),
					TotalRuns:      10,
					SuccessfulRuns: 9,
				},
			},
			TemplateStatistics: []TemplateStatistics{
				{
					TemplateName:   "implementation:real",
					TotalRuns:      10,
					SuccessfulRuns: 9,
				},
			},
		})

		analysis, err := integration.GetPatternAnalysis(ctx)
		require.NoError(t, err)
		assert.NotNil(t, analysis)
		assert.Equal(t, 1, analysis.WorkTypeCount)
		assert.Equal(t, 1, analysis.TemplateCount)
		assert.Equal(t, 10, analysis.TotalExecutions)
		assert.Equal(t, 0.9, analysis.AverageSuccessRate)
	})
}
