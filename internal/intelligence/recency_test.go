// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecencyHelpers(t *testing.T) {
	now := time.Now()

	t.Run("isRecent", func(t *testing.T) {
		// Within 30 days (recentWindow is strictly less than 30 days)
		recentTime := now.Add(-15 * 24 * time.Hour)
		assert.True(t, isRecent(recentTime))

		// Exactly 30 days: implementation uses < so boundary is not recent
		boundaryTime := now.Add(-30 * 24 * time.Hour)
		assert.False(t, isRecent(boundaryTime))

		// Just over 30 days
		staleTime := now.Add(-31 * 24 * time.Hour)
		assert.False(t, isRecent(staleTime))

		// Zero time
		assert.False(t, isRecent(time.Time{}))
	})

	t.Run("calculateFreshnessFactor", func(t *testing.T) {
		// Use times clearly inside each bucket to avoid boundary flakiness (time.Since uses current time)
		// <= 7 days => 1.0
		assert.Equal(t, 1.0, calculateFreshnessFactor(now.Add(-3*24*time.Hour)))
		// <= 30 days => 0.85
		assert.Equal(t, 0.85, calculateFreshnessFactor(now.Add(-14*24*time.Hour)))
		// <= 90 days => 0.60
		assert.Equal(t, 0.60, calculateFreshnessFactor(now.Add(-60*24*time.Hour)))
		// older => 0.35
		assert.Equal(t, 0.35, calculateFreshnessFactor(now.Add(-100*24*time.Hour)))

		// Zero time
		assert.Equal(t, 0.35, calculateFreshnessFactor(time.Time{}))
	})

	t.Run("getDaysSince", func(t *testing.T) {
		// 5 days ago
		assert.Equal(t, 5, getDaysSince(now.Add(-5*24*time.Hour)))

		// Zero time
		assert.Equal(t, -1, getDaysSince(time.Time{}))
	})
}

func TestRecencyAwareRecommendation(t *testing.T) {
	store := NewInMemoryPatternStore()
	recommender := NewRecommender(store, 3)
	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-60 * 24 * time.Hour) // 60 days ago

	// Create a work type with recent runs
	recentStats := &WorkTypeStatistics{
		WorkType:            string(contracts.WorkTypeImplementation),
		WorkDomain:          string(contracts.DomainFactory),
		TotalRuns:           20,
		SuccessfulRuns:      18,
		SuccessRate:         0.9,
		RecentRuns:          10,
		RecentSuccessfulRuns: 9,
		RecentSuccessRate:   0.9,
		FirstSeenAt:         now.Add(-10 * 24 * time.Hour),
		LastSeenAt:          now.Add(-1 * 24 * time.Hour),
	}

	// Create a work type with only old runs
	oldStats := &WorkTypeStatistics{
		WorkType:            string(contracts.WorkTypeDocumentation),
		WorkDomain:          "",
		TotalRuns:           100,
		SuccessfulRuns:      99,
		SuccessRate:         0.99,
		RecentRuns:          0,
		RecentSuccessfulRuns: 0,
		RecentSuccessRate:   0.0,
		FirstSeenAt:         oldTime.Add(-100 * 24 * time.Hour),
		LastSeenAt:          oldTime,
	}

	// Store stats
	recentKey := "implementation-factory"
	store.workTypeStats[recentKey] = recentStats
	oldKey := "documentation-"
	store.workTypeStats[oldKey] = oldStats

	t.Run("PrefersRecentExactMatch", func(t *testing.T) {
		rec, err := recommender.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)

		// Should recommend based on exact-match history with high confidence
		assert.Contains(t, rec.Reasoning, "Exact-match history")
		assert.Greater(t, rec.Confidence, 0.5)
		assert.Equal(t, rec.SuccessRate, 0.9)
	})

	t.Run("AppliesFreshnessPenaltyToOldData", func(t *testing.T) {
		rec, err := recommender.RecommendTemplate(ctx, contracts.WorkTypeDocumentation, "")
		require.NoError(t, err)

		// Should mention freshness penalty and age (e.g. "last seen" or "days ago")
		assert.Contains(t, rec.Reasoning, "freshness penalty")
		assert.True(t, strings.Contains(rec.Reasoning, "last seen") || strings.Contains(rec.Reasoning, "days ago") || strings.Contains(rec.Reasoning, "Exact-match"),
			"reasoning should mention recency: %s", rec.Reasoning)
		assert.Less(t, rec.Confidence, 0.99) // Should be reduced from 0.99
	})
}

func TestFailureClassification(t *testing.T) {
	t.Run("TestFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:      "failed",
			TestsFailed: []string{"TestSomething"},
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureTest, mode)
	})

	t.Run("TimeoutFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "context deadline exceeded after 30s",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureTimeout, mode)
	})

	t.Run("ValidationFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "validation error: invalid field",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureValidation, mode)
	})

	t.Run("WorkspaceFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "git clone failed",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureWorkspace, mode)
	})

	t.Run("InfraFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "connection refused: redis:6379",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureInfra, mode)
	})

	t.Run("RuntimeFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "panic: runtime error",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureRuntime, mode)
	})

	t.Run("NoFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result: "completed",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureMode(""), mode) // Empty for success
	})

	t.Run("UnknownFailure", func(t *testing.T) {
		summary := &ProofOfWorkSummary{
			Result:    "failed",
			ErrorLog:  "",
		}
		mode := classifyFailure(summary)
		assert.Equal(t, FailureUnknown, mode)
	})
}

func TestFailureStatistics(t *testing.T) {
	ctx := context.Background()

	t.Run("StoreAndRetrieveFailureStats", func(t *testing.T) {
		store := NewInMemoryFailureStore()

		stats := &FailureStatistics{
			WorkType:      "implementation",
			WorkDomain:    "backend",
			TotalFailures: 5,
			FailureModes: map[string]int{
				"test":      3,
				"timeout":   2,
			},
			LastFailureAt: time.Now(),
		}

		err := store.StoreFailureStats(ctx, stats)
		require.NoError(t, err)

		retrieved, err := store.GetFailureStats(ctx, "implementation", "backend")
		require.NoError(t, err)

		assert.Equal(t, 5, retrieved.TotalFailures)
		assert.Equal(t, 3, retrieved.FailureModes["test"])
		assert.Equal(t, 2, retrieved.FailureModes["timeout"])
	})

	t.Run("GetAllFailureStats", func(t *testing.T) {
		store := NewInMemoryFailureStore()

		stats1 := &FailureStatistics{
			WorkType:      "implementation",
			WorkDomain:    "backend",
			TotalFailures: 3,
		}
		stats2 := &FailureStatistics{
			WorkType:      "docs",
			WorkDomain:    "generic",
			TotalFailures: 2,
		}

		store.StoreFailureStats(ctx, stats1)
		store.StoreFailureStats(ctx, stats2)

		all, err := store.GetAllFailureStats(ctx)
		require.NoError(t, err)

		assert.Equal(t, 2, len(all))
	})
}

func TestFailureAwareDowngrade(t *testing.T) {
	store := NewInMemoryPatternStore()
	recommender := NewRecommender(store, 3)
	ctx := context.Background()

	now := time.Now()

	// Create work type stats with high success
	stats := &WorkTypeStatistics{
		WorkType:            string(contracts.WorkTypeImplementation),
		WorkDomain:          string(contracts.DomainFactory),
		TotalRuns:           20,
		SuccessfulRuns:      18,
		SuccessRate:         0.9,
		RecentRuns:          10,
		RecentSuccessfulRuns: 9,
		RecentSuccessRate:   0.9,
		FirstSeenAt:         now.Add(-10 * 24 * time.Hour),
		LastSeenAt:          now.Add(-1 * 24 * time.Hour),
	}

	store.workTypeStats["implementation-factory"] = stats

	t.Run("NoDowngradeWithoutFailureStats", func(t *testing.T) {
		rec, err := recommender.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)

		// Should have high confidence
		assert.Greater(t, rec.Confidence, 0.5)
		assert.NotContains(t, rec.Reasoning, "failure")
	})

	t.Run("DowngradeWithRecentFailures", func(t *testing.T) {
		// Add failure stats with recent failures
		failureStats := &FailureStatistics{
			WorkType:      string(contracts.WorkTypeImplementation),
			WorkDomain:    string(contracts.DomainFactory),
			TotalFailures: 5,
			FailureModes: map[string]int{
				"timeout": 4,
				"test":    1,
			},
			LastFailureAt: now,
		}
		store.failureStats["implementation-factory"] = failureStats

		rec, err := recommender.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)

		// Should have reduced confidence and mention failures
		assert.Contains(t, rec.Reasoning, "failure")
		assert.Contains(t, rec.Reasoning, "timeout")
		assert.Less(t, rec.Confidence, 0.9) // Reduced from high confidence
	})

	t.Run("NoDowngradeWithInsufficientFailures", func(t *testing.T) {
		// Clear previous stats
		store.failureStats = make(map[string]*FailureStatistics)

		// Add failure stats with only 1 failure
		failureStats := &FailureStatistics{
			WorkType:      string(contracts.WorkTypeImplementation),
			WorkDomain:    string(contracts.DomainFactory),
			TotalFailures: 1,
			FailureModes: map[string]int{
				"test": 1,
			},
			LastFailureAt: now,
		}
		store.failureStats["implementation-factory"] = failureStats

		rec, err := recommender.RecommendTemplate(ctx, contracts.WorkTypeImplementation, contracts.DomainFactory)
		require.NoError(t, err)

		// Should NOT downgrade with only 1 failure
		assert.NotContains(t, rec.Reasoning, "failure")
	})
}
