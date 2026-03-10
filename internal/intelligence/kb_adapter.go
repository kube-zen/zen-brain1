// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// KBPatternAdapter stores learned patterns in the knowledge base.
// It converts pattern summaries to KB documents for human consumption.
type KBPatternAdapter struct {
	kbStore kb.Store
}

// NewKBPatternAdapter creates a new KB pattern adapter.
func NewKBPatternAdapter(kbStore kb.Store) *KBPatternAdapter {
	return &KBPatternAdapter{
		kbStore: kbStore,
	}
}

// StorePatternSummary stores a mining result summary as a KB document.
func (a *KBPatternAdapter) StorePatternSummary(ctx context.Context, result *MiningResult) error {
	if a.kbStore == nil {
		log.Printf("[KBPatternAdapter] KB store is nil, skipping pattern storage")
		return nil
	}

	// Create pattern summary document
	doc := a.createPatternDocument(result)

	// Store as KB document (in a real implementation, this would use a document store)
	// For now, we'll log the document content since the stub KB doesn't support document creation
	docJSON, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pattern document: %w", err)
	}

	log.Printf("[KBPatternAdapter] Pattern summary (would be stored in KB):\n%s", string(docJSON))

	return nil
}

// createPatternDocument creates a KB document from a mining result.
func (a *KBPatternAdapter) createPatternDocument(result *MiningResult) *PatternDocument {
	now := time.Now()

	doc := &PatternDocument{
		ID:        fmt.Sprintf("pattern-summary-%d", now.Unix()),
		Title:     fmt.Sprintf("Execution Pattern Summary - %s", now.Format("2006-01-02")),
		Path:      "patterns/execution-patterns",
		Domain:    "intelligence",
		Source:    "internal:intelligence:miner",
		CreatedAt: result.StartTime,
		UpdatedAt: result.EndTime,
		Content: PatternContent{
			AnalysisPeriod: AnalysisPeriod{
				StartTime: result.StartTime,
				EndTime:   result.EndTime,
				Duration:  result.Duration,
			},
			Summary: PatternSummary{
				ArtifactsFound:    result.ArtifactsFound,
				ArtifactsMined:    result.ArtifactsMined,
				PatternsExtracted: result.PatternsExtracted,
				TotalWorkTypes:    len(result.WorkTypeStatistics),
				TotalTemplates:    len(result.TemplateStatistics),
			},
			WorkTypes:       result.WorkTypeStatistics,
			Templates:       result.TemplateStatistics,
			Durations:       result.DurationStatistics,
			Recommendations: a.generateRecommendations(result),
		},
	}

	return doc
}

// generateRecommendations generates human-readable recommendations from patterns.
func (a *KBPatternAdapter) generateRecommendations(result *MiningResult) []string {
	recommendations := []string{}

	// Analyze work types
	for _, wt := range result.WorkTypeStatistics {
		if wt.TotalRuns < 3 {
			continue // Skip with insufficient data
		}

		// High failure rate
		if wt.SuccessRate < 0.7 && wt.TotalRuns >= 5 {
			recommendations = append(recommendations,
				fmt.Sprintf("Work type '%s/%s' has low success rate (%.1f%%). Consider improving template or increasing retries.",
					wt.WorkType, wt.WorkDomain, wt.SuccessRate*100))
		}

		// Slow execution
		if wt.AverageDuration > 10*time.Minute {
			recommendations = append(recommendations,
				fmt.Sprintf("Work type '%s/%s' has long average duration (%s). Consider optimizing execution steps.",
					wt.WorkType, wt.WorkDomain, wt.AverageDuration))
		}
	}

	// Template recommendations
	for _, t := range result.TemplateStatistics {
		if t.TotalRuns < 3 {
			continue
		}

		if t.SuccessRate < 0.7 {
			recommendations = append(recommendations,
				fmt.Sprintf("Template '%s' has low success rate (%.1f%%). Review and update.",
					t.TemplateName, t.SuccessRate*100))
		}

		if t.SuccessRate > 0.95 && t.TotalRuns >= 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("Template '%s' performs excellently (%.1f%% success). Consider as default for related work types.",
					t.TemplateName, t.SuccessRate*100))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No specific recommendations. Continue monitoring patterns.")
	}

	return recommendations
}

// PatternDocument represents a pattern summary stored as a KB document.
type PatternDocument struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Path      string         `json:"path"`
	Domain    string         `json:"domain"`
	Tags      []string       `json:"tags,omitempty"`
	Source    string         `json:"source"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Content   PatternContent `json:"content"`
}

// PatternContent contains the pattern analysis.
type PatternContent struct {
	AnalysisPeriod  AnalysisPeriod       `json:"analysis_period"`
	Summary         PatternSummary       `json:"summary"`
	WorkTypes       []WorkTypeStatistics `json:"work_types"`
	Templates       []TemplateStatistics `json:"templates"`
	Durations       []DurationStatistics `json:"durations"`
	Recommendations []string             `json:"recommendations"`
}

// AnalysisPeriod describes the time range of the analysis.
type AnalysisPeriod struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// PatternSummary provides high-level statistics.
type PatternSummary struct {
	ArtifactsFound    int `json:"artifacts_found"`
	ArtifactsMined    int `json:"artifacts_mined"`
	PatternsExtracted int `json:"patterns_extracted"`
	TotalWorkTypes    int `json:"total_work_types"`
	TotalTemplates    int `json:"total_templates"`
}

// FormatDocument formats the pattern document for human reading.
func (d *PatternDocument) FormatDocument() string {
	output := fmt.Sprintf("# %s\n\n", d.Title)
	output += fmt.Sprintf("**Analysis Period:** %s to %s (%s)\n\n",
		d.Content.AnalysisPeriod.StartTime.Format("2006-01-02 15:04"),
		d.Content.AnalysisPeriod.EndTime.Format("2006-01-02 15:04"),
		d.Content.AnalysisPeriod.Duration.Round(time.Second))

	output += "## Summary\n\n"
	output += fmt.Sprintf("- Artifacts Found: %d\n", d.Content.Summary.ArtifactsFound)
	output += fmt.Sprintf("- Artifacts Mined: %d\n", d.Content.Summary.ArtifactsMined)
	output += fmt.Sprintf("- Patterns Extracted: %d\n", d.Content.Summary.PatternsExtracted)
	output += fmt.Sprintf("- Total Work Types: %d\n", d.Content.Summary.TotalWorkTypes)
	output += fmt.Sprintf("- Total Templates: %d\n\n", d.Content.Summary.TotalTemplates)

	if len(d.Content.WorkTypes) > 0 {
		output += "## Top Work Types\n\n"
		output += "| Work Type | Domain | Runs | Success | Avg Duration |\n"
		output += "|-----------|--------|------|---------|--------------|\n"
		for _, wt := range d.Content.WorkTypes[:min(len(d.Content.WorkTypes), 10)] {
			output += fmt.Sprintf("| %s | %s | %d | %.1f%% | %s |\n",
				wt.WorkType, wt.WorkDomain, wt.TotalRuns,
				wt.SuccessRate*100, wt.AverageDuration.Round(time.Second))
		}
		output += "\n"
	}

	if len(d.Content.Templates) > 0 {
		output += "## Top Templates\n\n"
		output += "| Template | Runs | Success | Avg Duration |\n"
		output += "|----------|------|---------|--------------|\n"
		for _, t := range d.Content.Templates[:min(len(d.Content.Templates), 10)] {
			output += fmt.Sprintf("| %s | %d | %.1f%% | %s |\n",
				t.TemplateName, t.TotalRuns, t.SuccessRate*100,
				t.AverageDuration.Round(time.Second))
		}
		output += "\n"
	}

	if len(d.Content.Recommendations) > 0 {
		output += "## Recommendations\n\n"
		for i, rec := range d.Content.Recommendations {
			output += fmt.Sprintf("%d. %s\n", i+1, rec)
		}
		output += "\n"
	}

	return output
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
