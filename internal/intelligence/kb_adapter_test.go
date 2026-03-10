// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKBPatternAdapter_StorePatternSummary(t *testing.T) {
	t.Run("WithNilKBStore", func(t *testing.T) {
		adapter := NewKBPatternAdapter(nil)

		result := &MiningResult{
			StartTime:         time.Now(),
			EndTime:           time.Now().Add(time.Hour),
			Duration:          time.Hour,
			ArtifactsFound:    5,
			ArtifactsMined:    4,
			PatternsExtracted: 10,
		}

		// Should not error with nil KB store
		err := adapter.StorePatternSummary(context.Background(), result)
		require.NoError(t, err)
	})

	t.Run("CreatePatternDocument", func(t *testing.T) {
		adapter := NewKBPatternAdapter(nil)

		result := &MiningResult{
			StartTime:         time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
			EndTime:           time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC),
			Duration:          time.Hour,
			ArtifactsFound:    10,
			ArtifactsMined:    8,
			PatternsExtracted: 15,
			WorkTypeStatistics: []WorkTypeStatistics{
				{
					WorkType:        "implementation",
					WorkDomain:      "factory",
					TotalRuns:       10,
					SuccessfulRuns:  9,
					AverageDuration: 5 * time.Minute,
				},
				{
					WorkType:        "bugfix",
					WorkDomain:      "core",
					TotalRuns:       5,
					SuccessfulRuns:  2,
					AverageDuration: 8 * time.Minute,
				},
			},
			TemplateStatistics: []TemplateStatistics{
				{
					TemplateName:    "implementation:real",
					TotalRuns:       10,
					SuccessfulRuns:  9,
					AverageDuration: 5 * time.Minute,
				},
				{
					TemplateName:    "bugfix:real",
					TotalRuns:       5,
					SuccessfulRuns:  2,
					AverageDuration: 8 * time.Minute,
				},
			},
			DurationStatistics: []DurationStatistics{
				{
					WorkType:    "implementation",
					WorkDomain:  "factory",
					Samples:     []time.Duration{4 * time.Minute, 5 * time.Minute, 6 * time.Minute},
					P95Duration: 6 * time.Minute,
					P99Duration: 6 * time.Minute,
				},
			},
		}

		doc := adapter.createPatternDocument(result)

		require.NotNil(t, doc)
		assert.NotEmpty(t, doc.ID)
		assert.Contains(t, doc.Title, "Execution Pattern Summary")
		assert.Contains(t, doc.Title, time.Now().Format("2006-01-02"))
		assert.Equal(t, "patterns/execution-patterns", doc.Path)
		assert.Equal(t, "intelligence", doc.Domain)
		assert.Equal(t, "internal:intelligence:miner", doc.Source)

		// Verify content
		assert.Equal(t, 10, doc.Content.Summary.ArtifactsFound)
		assert.Equal(t, 8, doc.Content.Summary.ArtifactsMined)
		assert.Equal(t, 15, doc.Content.Summary.PatternsExtracted)
		assert.Equal(t, 2, doc.Content.Summary.TotalWorkTypes)
		assert.Equal(t, 2, doc.Content.Summary.TotalTemplates)
		assert.Len(t, doc.Content.WorkTypes, 2)
		assert.Len(t, doc.Content.Templates, 2)
		assert.Len(t, doc.Content.Durations, 1)
	})

	t.Run("GenerateRecommendations", func(t *testing.T) {
		adapter := NewKBPatternAdapter(nil)

		now := time.Now()

		t.Run("LowSuccessRateRecommendation", func(t *testing.T) {
			result := &MiningResult{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				WorkTypeStatistics: []WorkTypeStatistics{
					{
						WorkType:       "bugfix",
						WorkDomain:     "core",
						TotalRuns:      10,
						SuccessfulRuns: 5,
						SuccessRate:    0.5, // 50% so that recommendation text contains 50.0%
					},
				},
			}

			recommendations := adapter.generateRecommendations(result)
			assert.NotEmpty(t, recommendations)
			// Should recommend improving the low-success template
			found := false
			for _, rec := range recommendations {
				if contains(rec, "low success rate") && (contains(rec, "50.0%") || contains(rec, "50%")) {
					found = true
					break
				}
			}
			assert.True(t, found, "Should recommend improving low success rate template")
		})

		t.Run("SlowExecutionRecommendation", func(t *testing.T) {
			result := &MiningResult{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				Duration:  time.Hour,
				WorkTypeStatistics: []WorkTypeStatistics{
					{
						WorkType:        "implementation",
						WorkDomain:      "factory",
						TotalRuns:       5,
						SuccessfulRuns:  5,
						AverageDuration: 15 * time.Minute, // Slow
					},
				},
			}

			recommendations := adapter.generateRecommendations(result)
			assert.NotEmpty(t, recommendations)
			// Should recommend optimizing execution steps
			found := false
			for _, rec := range recommendations {
				if contains(rec, "long average duration") || contains(rec, "optimizing") {
					found = true
					break
				}
			}
			assert.True(t, found, "Should recommend optimizing slow execution")
		})

		t.Run("ExcellentTemplateRecommendation", func(t *testing.T) {
			result := &MiningResult{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				Duration:  time.Hour,
				TemplateStatistics: []TemplateStatistics{
					{
						TemplateName:    "implementation:real",
						TotalRuns:       15,
						SuccessfulRuns:  15,
						SuccessRate:     1.0, // 100% so excellent recommendation is generated
						AverageDuration: 3 * time.Minute,
					},
				},
			}

			recommendations := adapter.generateRecommendations(result)
			assert.NotEmpty(t, recommendations)
			// Should recommend using as default
			found := false
			for _, rec := range recommendations {
				if contains(rec, "excellent") && (contains(rec, "default") || contains(rec, "100.0%")) {
					found = true
					break
				}
			}
			assert.True(t, found, "Should recommend excellent template as default")
		})

		t.Run("InsufficientData", func(t *testing.T) {
			result := &MiningResult{
				StartTime: now,
				EndTime:   now.Add(time.Hour),
				Duration:  time.Hour,
				WorkTypeStatistics: []WorkTypeStatistics{
					{
						WorkType:       "implementation",
						WorkDomain:     "factory",
						TotalRuns:      2, // Too few samples
						SuccessfulRuns: 2,
					},
				},
			}

			recommendations := adapter.generateRecommendations(result)
			// Should have the default recommendation
			assert.Len(t, recommendations, 1)
			assert.Contains(t, recommendations[0], "No specific recommendations")
		})
	})
}

func TestPatternDocument_FormatDocument(t *testing.T) {
	adapter := NewKBPatternAdapter(nil)

	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	result := &MiningResult{
		StartTime:         now,
		EndTime:           now.Add(time.Hour),
		Duration:          time.Hour,
		ArtifactsFound:    10,
		ArtifactsMined:    8,
		PatternsExtracted: 15,
		WorkTypeStatistics: []WorkTypeStatistics{
			{
				WorkType:        "implementation",
				WorkDomain:      "factory",
				TotalRuns:       10,
				SuccessfulRuns:  9,
				SuccessRate:     0.9, // Explicitly set for test
				AverageDuration: 5 * time.Minute,
			},
		},
		TemplateStatistics: []TemplateStatistics{
			{
				TemplateName:    "implementation:real",
				TotalRuns:       10,
				SuccessfulRuns:  9,
				SuccessRate:     0.9, // Explicitly set for test
				AverageDuration: 5 * time.Minute,
			},
		},
		DurationStatistics: []DurationStatistics{},
	}

	doc := adapter.createPatternDocument(result)
	formatted := doc.FormatDocument()

	assert.Contains(t, formatted, "# Execution Pattern Summary")
	assert.Contains(t, formatted, "2026-03-09")
	assert.Contains(t, formatted, "Artifacts Found: 10")
	assert.Contains(t, formatted, "Artifacts Mined: 8")
	assert.Contains(t, formatted, "Patterns Extracted: 15")
	assert.Contains(t, formatted, "Top Work Types")
	assert.Contains(t, formatted, "implementation")
	assert.Contains(t, formatted, "90.0%")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
