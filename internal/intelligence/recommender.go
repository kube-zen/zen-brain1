// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// Recommender uses learned patterns to recommend templates and configurations.
type Recommender struct {
	patternStore PatternStore
	minSamples   int // Minimum samples required for recommendations
}

// NewRecommender creates a new recommender.
func NewRecommender(patternStore PatternStore, minSamples int) *Recommender {
	if minSamples < 1 {
		minSamples = 3 // Default minimum
	}
	return &Recommender{
		patternStore: patternStore,
		minSamples:   minSamples,
	}
}

// Recommendation represents a suggested template or configuration.
type Recommendation struct {
	TemplateName    string
	WorkType        contracts.WorkType
	WorkDomain      contracts.WorkDomain
	Confidence      float64 // 0.0 to 1.0
	SuccessRate     float64
	AverageDuration time.Duration
	Reasoning       string
	SampleCount     int
}

// ConfigurationRecommendation represents suggested execution configuration.
type ConfigurationRecommendation struct {
	TimeoutSeconds int64
	MaxRetries     int
	Reasoning      string
	Confidence     float64
}

// RecommendTemplate recommends a template based on work type and domain.
func (r *Recommender) RecommendTemplate(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (*Recommendation, error) {
	// Try to get work type statistics
	workTypeStats, err := r.patternStore.GetWorkTypeStats(ctx, string(workType), string(workDomain))
	if err != nil {
		// No statistics available, return low-confidence default
		return &Recommendation{
			TemplateName:    "default",
			WorkType:        workType,
			WorkDomain:      workDomain,
			Confidence:      0.0,
			SuccessRate:     0.0,
			AverageDuration: 0,
			Reasoning:       "No historical data available, using default template",
			SampleCount:     0,
		}, nil
	}

	// Calculate confidence based on sample count
	confidence := r.calculateConfidence(workTypeStats.TotalRuns)

	// Determine best template based on success rate
	templateName := r.selectBestTemplate(ctx, workType, workDomain)

	reasoning := fmt.Sprintf("Based on %d historical executions with %.1f%% success rate",
		workTypeStats.TotalRuns, workTypeStats.SuccessRate*100)

	return &Recommendation{
		TemplateName:    templateName,
		WorkType:        workType,
		WorkDomain:      workDomain,
		Confidence:      confidence,
		SuccessRate:     workTypeStats.SuccessRate,
		AverageDuration: workTypeStats.AverageDuration,
		Reasoning:       reasoning,
		SampleCount:     workTypeStats.TotalRuns,
	}, nil
}

// RecommendConfiguration recommends execution configuration based on work type.
func (r *Recommender) RecommendConfiguration(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (*ConfigurationRecommendation, error) {
	// Get duration statistics
	durationStats, err := r.patternStore.GetDurationStats(ctx, string(workType), string(workDomain))
	if err != nil || len(durationStats.Samples) < r.minSamples {
		// No duration statistics available, return conservative defaults
		return &ConfigurationRecommendation{
			TimeoutSeconds: 300, // 5 minutes default
			MaxRetries:     3,
			Reasoning:      "No historical duration data available, using conservative defaults",
			Confidence:     0.0,
		}, nil
	}

	// Calculate timeout based on P95 duration plus buffer
	timeoutSeconds := int64(durationStats.P95Duration.Seconds() * 2) // 2x P95 as buffer
	if timeoutSeconds < 60 {
		timeoutSeconds = 60 // Minimum 1 minute
	}
	if timeoutSeconds > 3600 {
		timeoutSeconds = 3600 // Maximum 1 hour
	}

	// Calculate retries based on success rate
	workTypeStats, _ := r.patternStore.GetWorkTypeStats(ctx, string(workType), string(workDomain))
	maxRetries := 3
	if workTypeStats != nil && workTypeStats.SuccessRate < 0.8 {
		maxRetries = 5 // More retries for less reliable work types
	}

	confidence := r.calculateConfidence(len(durationStats.Samples))
	reasoning := fmt.Sprintf("Based on %d duration samples (P95: %s, P99: %s), success rate: %.1f%%",
		len(durationStats.Samples), durationStats.P95Duration, durationStats.P99Duration,
		workTypeStats.SuccessRate*100)

	return &ConfigurationRecommendation{
		TimeoutSeconds: timeoutSeconds,
		MaxRetries:     maxRetries,
		Reasoning:      reasoning,
		Confidence:     confidence,
	}, nil
}

// RecommendAll returns combined recommendations for template and configuration.
func (r *Recommender) RecommendAll(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) (*Recommendation, *ConfigurationRecommendation, error) {
	templateRec, err := r.RecommendTemplate(ctx, workType, workDomain)
	if err != nil {
		return nil, nil, err
	}

	configRec, err := r.RecommendConfiguration(ctx, workType, workDomain)
	if err != nil {
		return templateRec, nil, err
	}

	return templateRec, configRec, nil
}

// selectBestTemplate selects the best template for a work type.
func (r *Recommender) selectBestTemplate(ctx context.Context, workType contracts.WorkType, workDomain contracts.WorkDomain) string {
	// Get all template statistics
	allTemplateStats, err := r.patternStore.GetAllTemplateStats(ctx)
	if err != nil || len(allTemplateStats) == 0 {
		return "default"
	}

	// Sort templates by success rate and sample count
	sort.Slice(allTemplateStats, func(i, j int) bool {
		// Prefer templates with higher success rate
		if allTemplateStats[i].SuccessRate != allTemplateStats[j].SuccessRate {
			return allTemplateStats[i].SuccessRate > allTemplateStats[j].SuccessRate
		}
		// Then prefer templates with more samples
		return allTemplateStats[i].TotalRuns > allTemplateStats[j].TotalRuns
	})

	// Return the best template if it meets minimum sample threshold
	if len(allTemplateStats) > 0 && allTemplateStats[0].TotalRuns >= r.minSamples {
		return allTemplateStats[0].TemplateName
	}

	return "default"
}

// calculateConfidence calculates confidence based on sample count.
func (r *Recommender) calculateConfidence(sampleCount int) float64 {
	if sampleCount == 0 {
		return 0.0
	}
	if sampleCount < r.minSamples {
		// Linear ramp from 0 to 0.5 for samples below minimum
		return 0.5 * float64(sampleCount) / float64(r.minSamples)
	}
	// Cap at 1.0, ramp from 0.5 to 1.0 for samples above minimum
	confidence := 0.5 + 0.5*(1.0-1.0/float64(sampleCount))
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// GetWorkTypeSummary returns a summary of work type performance.
func (r *Recommender) GetWorkTypeSummary(ctx context.Context) (*WorkTypeSummary, error) {
	allStats, err := r.patternStore.GetAllWorkTypeStats(ctx)
	if err != nil {
		return nil, err
	}

	summary := &WorkTypeSummary{
		TotalWorkTypes: len(allStats),
		WorkTypes:      allStats,
	}

	// Calculate aggregate statistics
	totalRuns := 0
	totalSuccess := 0
	for _, stats := range allStats {
		totalRuns += stats.TotalRuns
		totalSuccess += stats.SuccessfulRuns
	}

	if totalRuns > 0 {
		summary.OverallSuccessRate = float64(totalSuccess) / float64(totalRuns)
	}

	return summary, nil
}

// WorkTypeSummary represents aggregate work type performance.
type WorkTypeSummary struct {
	TotalWorkTypes     int
	OverallSuccessRate float64
	WorkTypes          []WorkTypeStatistics
}

// PatternAnalysis provides detailed analysis of learned patterns.
func (r *Recommender) PatternAnalysis(ctx context.Context) (*PatternAnalysis, error) {
	workTypeStats, _ := r.patternStore.GetAllWorkTypeStats(ctx)
	templateStats, _ := r.patternStore.GetAllTemplateStats(ctx)
	_, _ = r.patternStore.GetAllWorkTypeStats(ctx) // durationStats not used in this version

	analysis := &PatternAnalysis{
		WorkTypeCount:        len(workTypeStats),
		TemplateCount:        len(templateStats),
		TotalExecutions:      0,
		SuccessfulExecutions: 0,
		AverageSuccessRate:   0.0,
		TopWorkTypes:         []WorkTypeStatistics{},
		TopTemplates:         []TemplateStatistics{},
	}

	// Calculate aggregate statistics
	for _, stats := range workTypeStats {
		analysis.TotalExecutions += stats.TotalRuns
		analysis.SuccessfulExecutions += stats.SuccessfulRuns
	}

	if analysis.TotalExecutions > 0 {
		analysis.AverageSuccessRate = float64(analysis.SuccessfulExecutions) / float64(analysis.TotalExecutions)
	}

	// Sort and select top work types (by total runs)
	sort.Slice(workTypeStats, func(i, j int) bool {
		return workTypeStats[i].TotalRuns > workTypeStats[j].TotalRuns
	})
	top := 5
	if len(workTypeStats) < top {
		top = len(workTypeStats)
	}
	for i := 0; i < top; i++ {
		copied := workTypeStats[i]
		analysis.TopWorkTypes = append(analysis.TopWorkTypes, copied)
	}

	// Sort and select top templates (by total runs)
	sort.Slice(templateStats, func(i, j int) bool {
		return templateStats[i].TotalRuns > templateStats[j].TotalRuns
	})
	top = 5
	if len(templateStats) < top {
		top = len(templateStats)
	}
	for i := 0; i < top; i++ {
		copied := templateStats[i]
		analysis.TopTemplates = append(analysis.TopTemplates, copied)
	}

	return analysis, nil
}

// PatternAnalysis provides detailed analysis of learned patterns.
type PatternAnalysis struct {
	WorkTypeCount        int
	TemplateCount        int
	TotalExecutions      int
	SuccessfulExecutions int
	AverageSuccessRate   float64
	TopWorkTypes         []WorkTypeStatistics
	TopTemplates         []TemplateStatistics
}

// FormatAnalysis formats pattern analysis for logging/display.
func (a *PatternAnalysis) FormatAnalysis() string {
	output := "Pattern Analysis:\n"
	output += fmt.Sprintf("  Work Types: %d\n", a.WorkTypeCount)
	output += fmt.Sprintf("  Templates: %d\n", a.TemplateCount)
	output += fmt.Sprintf("  Total Executions: %d\n", a.TotalExecutions)
	output += fmt.Sprintf("  Success Rate: %.1f%%\n", a.AverageSuccessRate*100)
	output += "\nTop Work Types (by volume):\n"
	for i, wt := range a.TopWorkTypes {
		output += fmt.Sprintf("  %d. %s/%s: %d runs, %.1f%% success, avg %s\n",
			i+1, wt.WorkType, wt.WorkDomain, wt.TotalRuns, wt.SuccessRate*100, wt.AverageDuration)
	}
	output += "\nTop Templates (by volume):\n"
	for i, t := range a.TopTemplates {
		output += fmt.Sprintf("  %d. %s: %d runs, %.1f%% success, avg %s\n",
			i+1, t.TemplateName, t.TotalRuns, t.SuccessRate*100, t.AverageDuration)
	}
	return output
}

// GetTopWorkTypes returns the most frequently executed work types.
func (r *Recommender) GetTopWorkTypes(ctx context.Context, limit int) ([]WorkTypeStatistics, error) {
	allStats, err := r.patternStore.GetAllWorkTypeStats(ctx)
	if err != nil {
		return nil, err
	}

	// Sort by total runs
	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].TotalRuns > allStats[j].TotalRuns
	})

	// Limit results
	if limit > len(allStats) {
		limit = len(allStats)
	}

	return allStats[:limit], nil
}

// GetTopTemplates returns the most frequently used templates.
func (r *Recommender) GetTopTemplates(ctx context.Context, limit int) ([]TemplateStatistics, error) {
	allStats, err := r.patternStore.GetAllTemplateStats(ctx)
	if err != nil {
		return nil, err
	}

	// Sort by total runs
	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].TotalRuns > allStats[j].TotalRuns
	})

	// Limit results
	if limit > len(allStats) {
		limit = len(allStats)
	}

	return allStats[:limit], nil
}
