// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ProofOfWorkSummary represents a minimal proof-of-work summary for mining.
// This is a subset of factory.ProofOfWorkSummary to avoid circular dependencies.
type ProofOfWorkSummary struct {
	TaskID             string        `json:"task_id"`
	SessionID          string        `json:"session_id"`
	WorkItemID         string        `json:"work_item_id"`
	WorkType           string        `json:"work_type"`
	WorkDomain         string        `json:"work_domain"`
	Title              string        `json:"title"`
	Objective          string        `json:"objective"`
	Result             string        `json:"result"`
	StartedAt          time.Time     `json:"started_at"`
	CompletedAt        time.Time     `json:"completed_at"`
	Duration           time.Duration `json:"duration"`
	ModelUsed           string  `json:"model_used"`
	TemplateUsed        string  `json:"template_used,omitempty"`        // Actual template; prefer over ModelUsed when present
	SelectionSource     string  `json:"selection_source,omitempty"`     // e.g. "recommended", "static"
	SelectionConfidence float64 `json:"selection_confidence,omitempty"`
	SelectionReasoning  string  `json:"selection_reasoning,omitempty"`
	FilesChanged        []string `json:"files_changed,omitempty"`
	TestsFailed        []string      `json:"tests_failed,omitempty"`
	ErrorLog           string        `json:"error_log,omitempty"`
	OutputLog          string        `json:"output_log,omitempty"`
	RecommendedAction  string        `json:"recommended_action,omitempty"`
}

// Miner extracts patterns from proof-of-work artifacts.
type Miner struct {
	runtimeDir   string
	patternStore PatternStore
	kbAdapter    *KBPatternAdapter // Optional KB integration for human-readable summaries
}

// NewMiner creates a new proof-of-work miner.
func NewMiner(runtimeDir string, patternStore PatternStore) *Miner {
	return &Miner{
		runtimeDir:   runtimeDir,
		patternStore: patternStore,
		kbAdapter:    nil,
	}
}

// SetKBAdapter sets the KB pattern adapter for storing human-readable summaries.
func (m *Miner) SetKBAdapter(adapter *KBPatternAdapter) {
	m.kbAdapter = adapter
}

// MineProofOfWorks scans proof-of-work directories and extracts patterns.
func (m *Miner) MineProofOfWorks(ctx context.Context) (*MiningResult, error) {
	log.Printf("[Miner] Starting proof-of-work mining")

	proofDir := filepath.Join(m.runtimeDir, "proof-of-work")
	entries, err := os.ReadDir(proofDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof-of-work directory: %w", err)
	}

	result := &MiningResult{
		StartTime:         time.Now(),
		ArtifactsFound:    0,
		ArtifactsMined:    0,
		PatternsExtracted: 0,
		Errors:            []string{},
	}

	workTypeStats := make(map[string]*WorkTypeStatistics)
	templateStats := make(map[string]*TemplateStatistics)
	durationStats := make(map[string]*DurationStatistics)
	failureStats := make(map[string]*FailureStatistics)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		artifactDir := filepath.Join(proofDir, entry.Name())
		jsonPath := filepath.Join(artifactDir, "proof-of-work.json")

		// Check if JSON exists
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			continue
		}

		result.ArtifactsFound++

		// Read proof-of-work JSON
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to read %s: %v", jsonPath, err))
			continue
		}

		var summary ProofOfWorkSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to unmarshal %s: %v", jsonPath, err))
			continue
		}

		// Extract patterns from this artifact
		m.extractPatterns(&summary, workTypeStats, templateStats, durationStats)

		// Extract failure patterns
		m.extractFailurePatterns(&summary, failureStats)

		result.ArtifactsMined++
	}

	// Aggregate statistics
	result.WorkTypeStatistics = AggregateWorkTypeStats(workTypeStats)
	result.TemplateStatistics = AggregateTemplateStats(templateStats)
	result.DurationStatistics = aggregateDurationStats(durationStats)
	result.FailureStatistics = aggregateFailureStats(failureStats)
	result.PatternsExtracted = len(result.WorkTypeStatistics) + len(result.TemplateStatistics)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Store patterns in the pattern store
	if err := m.patternStore.StorePatterns(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to store patterns: %w", err)
	}

	// Store human-readable summary in KB (optional)
	if m.kbAdapter != nil {
		if err := m.kbAdapter.StorePatternSummary(ctx, result); err != nil {
			log.Printf("[Miner] Warning: failed to store pattern summary in KB: %v", err)
		}
	}

	log.Printf("[Miner] Mining completed: found=%d, mined=%d, patterns=%d, duration=%s",
		result.ArtifactsFound, result.ArtifactsMined, result.PatternsExtracted, result.Duration)

	return result, nil
}

// extractPatterns extracts patterns from a single proof-of-work summary.
func (m *Miner) extractPatterns(
	summary *ProofOfWorkSummary,
	workTypeStats map[string]*WorkTypeStatistics,
	templateStats map[string]*TemplateStatistics,
	durationStats map[string]*DurationStatistics,
) {
	// Determine completion timestamp for recency tracking
	completedAt := summary.CompletedAt
	if completedAt.IsZero() {
		completedAt = summary.StartedAt
	}
	isRecentArtifact := isRecent(completedAt)

	// Work type statistics
	workTypeKey := fmt.Sprintf("%s:%s", summary.WorkType, summary.WorkDomain)
	if _, exists := workTypeStats[workTypeKey]; !exists {
		workTypeStats[workTypeKey] = &WorkTypeStatistics{
			WorkType:           summary.WorkType,
			WorkDomain:         summary.WorkDomain,
			TotalRuns:          0,
			SuccessRate:        0.0,
			AverageDuration:    0,
			FilesChangedPerRun: 0,
			FirstSeenAt:        completedAt,
			LastSeenAt:         completedAt,
		}
	}

	stats := workTypeStats[workTypeKey]
	stats.TotalRuns++
	if summary.Result == "completed" {
		stats.SuccessfulRuns++
	}
	stats.TotalDuration += summary.Duration

	// Update recency fields
	if completedAt.Before(stats.FirstSeenAt) || stats.FirstSeenAt.IsZero() {
		stats.FirstSeenAt = completedAt
	}
	if completedAt.After(stats.LastSeenAt) || stats.LastSeenAt.IsZero() {
		stats.LastSeenAt = completedAt
	}
	if isRecentArtifact {
		stats.RecentRuns++
		if summary.Result == "completed" {
			stats.RecentSuccessfulRuns++
		}
	}

	// Files changed
	stats.TotalFilesChanged += len(summary.FilesChanged)

	// Duration statistics
	durationKey := workTypeKey
	if _, exists := durationStats[durationKey]; !exists {
		durationStats[durationKey] = &DurationStatistics{
			WorkType:   summary.WorkType,
			WorkDomain: summary.WorkDomain,
			Samples:    []time.Duration{},
		}
	}
	durationStats[durationKey].Samples = append(durationStats[durationKey].Samples, summary.Duration)

	// Template statistics: use actual template when present; ModelUsed fallback is backward-compat only for older artifacts
	templateKey := summary.TemplateUsed
	if templateKey == "" {
		templateKey = summary.ModelUsed
	}
	if _, exists := templateStats[templateKey]; !exists {
		templateStats[templateKey] = &TemplateStatistics{
			TemplateName:    templateKey,
			TotalRuns:       0,
			SuccessRate:     0.0,
			AverageDuration: 0,
			FirstSeenAt:     completedAt,
			LastSeenAt:      completedAt,
		}
	}

	tStats := templateStats[templateKey]
	tStats.TotalRuns++
	if summary.Result == "completed" {
		tStats.SuccessfulRuns++
	}
	tStats.TotalDuration += summary.Duration

	// Update template recency fields
	if completedAt.Before(tStats.FirstSeenAt) || tStats.FirstSeenAt.IsZero() {
		tStats.FirstSeenAt = completedAt
	}
	if completedAt.After(tStats.LastSeenAt) || tStats.LastSeenAt.IsZero() {
		tStats.LastSeenAt = completedAt
	}
	if isRecentArtifact {
		tStats.RecentRuns++
		if summary.Result == "completed" {
			tStats.RecentSuccessfulRuns++
		}
	}
}

// AggregateWorkTypeStats aggregates and finalizes work type statistics.
// Exported for testing.
func AggregateWorkTypeStats(stats map[string]*WorkTypeStatistics) []WorkTypeStatistics {
	result := make([]WorkTypeStatistics, 0, len(stats))

	for _, s := range stats {
		copied := *s
		if copied.TotalRuns > 0 {
			copied.SuccessRate = float64(copied.SuccessfulRuns) / float64(copied.TotalRuns)
			copied.AverageDuration = copied.TotalDuration / time.Duration(copied.TotalRuns)
			copied.FilesChangedPerRun = float64(copied.TotalFilesChanged) / float64(copied.TotalRuns)
		}
		// Calculate recent success rate
		if copied.RecentRuns > 0 {
			copied.RecentSuccessRate = float64(copied.RecentSuccessfulRuns) / float64(copied.RecentRuns)
		} else {
			copied.RecentSuccessRate = 0.0
		}
		result = append(result, copied)
	}

	return result
}

// AggregateTemplateStats aggregates and finalizes template statistics.
// Exported for testing.
func AggregateTemplateStats(stats map[string]*TemplateStatistics) []TemplateStatistics {
	result := make([]TemplateStatistics, 0, len(stats))

	for _, s := range stats {
		copied := *s
		if copied.TotalRuns > 0 {
			copied.SuccessRate = float64(copied.SuccessfulRuns) / float64(copied.TotalRuns)
			copied.AverageDuration = copied.TotalDuration / time.Duration(copied.TotalRuns)
		}
		// Calculate recent success rate
		if copied.RecentRuns > 0 {
			copied.RecentSuccessRate = float64(copied.RecentSuccessfulRuns) / float64(copied.RecentRuns)
		} else {
			copied.RecentSuccessRate = 0.0
		}
		result = append(result, copied)
	}

	return result
}

// aggregateDurationStats aggregates and finalizes duration statistics.
func aggregateDurationStats(stats map[string]*DurationStatistics) []DurationStatistics {
	result := make([]DurationStatistics, 0, len(stats))

	for _, s := range stats {
		if len(s.Samples) > 0 {
			// Calculate percentiles
			s.MinDuration = minDuration(s.Samples)
			s.MaxDuration = maxDuration(s.Samples)
			s.MedianDuration = medianDuration(s.Samples)
			s.P95Duration = percentileDuration(s.Samples, 0.95)
			s.P99Duration = percentileDuration(s.Samples, 0.99)
		}
		result = append(result, *s)
	}

	return result
}

// MiningResult represents the output of a mining operation.
type MiningResult struct {
	StartTime           time.Time
	EndTime             time.Time
	Duration            time.Duration
	ArtifactsFound      int
	ArtifactsMined      int
	PatternsExtracted   int
	Errors              []string
	WorkTypeStatistics  []WorkTypeStatistics
	TemplateStatistics  []TemplateStatistics
	DurationStatistics  []DurationStatistics
	FailureStatistics   []FailureStatistics `json:"failure_statistics,omitempty"`
}

// WorkTypeStatistics tracks performance metrics by work type and domain.
type WorkTypeStatistics struct {
	WorkType            string
	WorkDomain          string
	TotalRuns           int
	SuccessfulRuns      int
	SuccessRate         float64
	AverageDuration     time.Duration
	TotalDuration       time.Duration
	TotalFilesChanged   int
	FilesChangedPerRun  float64
	FirstSeenAt         time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt          time.Time `json:"last_seen_at,omitempty"`
	RecentRuns          int       `json:"recent_runs,omitempty"`
	RecentSuccessfulRuns int       `json:"recent_successful_runs,omitempty"`
	RecentSuccessRate   float64   `json:"recent_success_rate,omitempty"`
}

// extractFailurePatterns extracts failure patterns from a proof-of-work summary.
func (m *Miner) extractFailurePatterns(summary *ProofOfWorkSummary, failureStats map[string]*FailureStatistics) {
	// Only track failures
	if summary.Result == "completed" {
		return
	}

	// Classify failure mode
	failureMode := classifyFailure(summary)
	if failureMode == "" {
		return // No failure
	}

	// Determine completion timestamp
	completedAt := summary.CompletedAt
	if completedAt.IsZero() {
		completedAt = summary.StartedAt
	}

	// Create or update failure statistics by work type/domain
	key := fmt.Sprintf("%s:%s", summary.WorkType, summary.WorkDomain)
	if _, exists := failureStats[key]; !exists {
		failureStats[key] = &FailureStatistics{
			WorkType:          summary.WorkType,
			WorkDomain:        summary.WorkDomain,
			TotalFailures:     0,
			FailureModes:      make(map[string]int),
			LastFailureAt:     completedAt,
			RecommendedActions: make(map[string]int),
		}
	}

	stats := failureStats[key]
	stats.TotalFailures++

	// Track failure mode
	if failureMode != "" {
		stats.FailureModes[string(failureMode)]++
	}

	// Track recommended action
	if summary.RecommendedAction != "" {
		stats.RecommendedActions[summary.RecommendedAction]++
	}

	// Update last failure timestamp
	if completedAt.After(stats.LastFailureAt) || stats.LastFailureAt.IsZero() {
		stats.LastFailureAt = completedAt
	}

	// Also track per-template failures if template is known
	if summary.TemplateUsed != "" || summary.ModelUsed != "" {
		templateKey := summary.TemplateUsed
		if templateKey == "" {
			templateKey = summary.ModelUsed
		}
		templateFullKey := fmt.Sprintf("%s:%s:%s", summary.WorkType, summary.WorkDomain, templateKey)
		if _, exists := failureStats[templateFullKey]; !exists {
			failureStats[templateFullKey] = &FailureStatistics{
				WorkType:          summary.WorkType,
				WorkDomain:        summary.WorkDomain,
				TemplateName:      templateKey,
				TotalFailures:     0,
				FailureModes:      make(map[string]int),
				LastFailureAt:     completedAt,
				RecommendedActions: make(map[string]int),
			}
		}

		tStats := failureStats[templateFullKey]
		tStats.TotalFailures++
		if failureMode != "" {
			tStats.FailureModes[string(failureMode)]++
		}
		if summary.RecommendedAction != "" {
			tStats.RecommendedActions[summary.RecommendedAction]++
		}
		if completedAt.After(tStats.LastFailureAt) || tStats.LastFailureAt.IsZero() {
			tStats.LastFailureAt = completedAt
		}
	}
}

// aggregateFailureStats aggregates failure statistics into a slice.
func aggregateFailureStats(stats map[string]*FailureStatistics) []FailureStatistics {
	result := make([]FailureStatistics, 0, len(stats))

	for _, s := range stats {
		copied := *s
		result = append(result, copied)
	}

	return result
}

// TemplateStatistics tracks performance metrics by template.
type TemplateStatistics struct {
	TemplateName         string
	TotalRuns            int
	SuccessfulRuns       int
	SuccessRate          float64
	AverageDuration      time.Duration
	TotalDuration        time.Duration
	FirstSeenAt          time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt           time.Time `json:"last_seen_at,omitempty"`
	RecentRuns           int       `json:"recent_runs,omitempty"`
	RecentSuccessfulRuns int       `json:"recent_successful_runs,omitempty"`
	RecentSuccessRate    float64   `json:"recent_success_rate,omitempty"`
}

// DurationStatistics tracks duration percentiles.
type DurationStatistics struct {
	WorkType       string
	WorkDomain     string
	Samples        []time.Duration
	MinDuration    time.Duration
	MaxDuration    time.Duration
	MedianDuration time.Duration
	P95Duration    time.Duration
	P99Duration    time.Duration
}

// Helper functions for duration statistics

func minDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

func maxDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

func medianDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted[len(sorted)/2]
}

func percentileDuration(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
