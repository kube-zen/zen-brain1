// Package intelligence provides enhanced failure analysis capabilities.
package intelligence

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// RootCauseAnalysis represents a root cause analysis result.
type RootCauseAnalysis struct {
	WorkType           string    `json:"work_type"`
	WorkDomain         string    `json:"work_domain"`
	FailureMode        string    `json:"failure_mode"`
	RootCause          string    `json:"root_cause"`
	Evidence           []string  `json:"evidence,omitempty"`
	Confidence         float64   `json:"confidence"` // 0.0 to 1.0
	Occurrences        int       `json:"occurrences"`
	LastOccurrence     time.Time `json:"last_occurrence"`
	MitigationStrategy string    `json:"mitigation_strategy,omitempty"`
}

// FailureCorrelation represents a correlation between failures and system state.
type FailureCorrelation struct {
	FailureMode      string  `json:"failure_mode"`
	CorrelatedFactor string  `json:"correlated_factor"`
	CorrelationType  string  `json:"correlation_type"` // "temporal", "causal", "environmental"
	Strength         float64 `json:"strength"` // -1.0 to 1.0
	SampleSize       int     `json:"sample_size"`
	PValue           float64 `json:"p_value,omitempty"`
}

// PredictiveModel represents a predictive failure model.
type PredictiveModel struct {
	WorkType          string    `json:"work_type"`
	WorkDomain        string    `json:"work_domain"`
	RiskFactors       []string  `json:"risk_factors"`
	PredictedFailureMode string `json:"predicted_failure_mode"`
	Probability       float64   `json:"probability"`
	Confidence        float64   `json:"confidence"`
	LastUpdated       time.Time `json:"last_updated"`
}

// FailureAnalyzer provides enhanced failure analysis capabilities.
type FailureAnalyzer struct {
	store FailureStore
}

// NewFailureAnalyzer creates a new failure analyzer.
func NewFailureAnalyzer(store FailureStore) *FailureAnalyzer {
	return &FailureAnalyzer{
		store: store,
	}
}

// AnalyzeRootCauses performs root cause analysis on failure statistics.
func (a *FailureAnalyzer) AnalyzeRootCauses(ctx context.Context) ([]RootCauseAnalysis, error) {
	// Get all failure statistics
	allStats, err := a.store.GetAllFailureStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure stats: %w", err)
	}

	analyses := []RootCauseAnalysis{}

	for _, stats := range allStats {
		// Analyze each failure mode
		for mode, count := range stats.FailureModes {
			if count == 0 {
				continue
			}

			// Determine root cause based on failure mode and patterns
			rootCause := determineRootCause(FailureMode(mode), stats)

			// Gather evidence
			evidence := gatherEvidence(FailureMode(mode), stats)

			// Calculate confidence based on sample size and consistency
			confidence := calculateConfidence(count, len(evidence))

			// Generate mitigation strategy
			mitigation := generateMitigation(FailureMode(mode), rootCause)

			analysis := RootCauseAnalysis{
				WorkType:           stats.WorkType,
				WorkDomain:         stats.WorkDomain,
				FailureMode:        mode,
				RootCause:          rootCause,
				Evidence:           evidence,
				Confidence:         confidence,
				Occurrences:        count,
				LastOccurrence:     stats.LastFailureAt,
				MitigationStrategy: mitigation,
			}

			analyses = append(analyses, analysis)
		}
	}

	// Sort by occurrences (most frequent first)
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].Occurrences > analyses[j].Occurrences
	})

	return analyses, nil
}

// AnalyzeCorrelations finds correlations between failures and system state.
func (a *FailureAnalyzer) AnalyzeCorrelations(ctx context.Context) ([]FailureCorrelation, error) {
	// Get all failure statistics
	allStats, err := a.store.GetAllFailureStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure stats: %w", err)
	}

	correlations := []FailureCorrelation{}

	for _, stats := range allStats {
		for mode := range stats.FailureModes {
			// Temporal correlations
			correlations = append(correlations, analyzeTemporalCorrelations(mode, stats)...)

			// Environmental correlations
			correlations = append(correlations, analyzeEnvironmentalCorrelations(mode, stats)...)

			// Causal correlations
			correlations = append(correlations, analyzeCausalCorrelations(mode, stats)...)
		}
	}

	// Filter weak correlations
	filtered := []FailureCorrelation{}
	for _, c := range correlations {
		if abs(c.Strength) >= 0.5 && c.SampleSize >= 3 {
			filtered = append(filtered, c)
		}
	}

	// Sort by strength
	sort.Slice(filtered, func(i, j int) bool {
		return abs(filtered[i].Strength) > abs(filtered[j].Strength)
	})

	return filtered, nil
}

// BuildPredictiveModel builds a predictive failure model.
func (a *FailureAnalyzer) BuildPredictiveModel(ctx context.Context, workType, workDomain string) (*PredictiveModel, error) {
	// Get failure statistics
	stats, err := a.store.GetFailureStats(ctx, workType, workDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure stats: %w", err)
	}

	// Identify risk factors
	riskFactors := identifyRiskFactors(*stats)

	// Predict most likely failure mode
	predictedMode := ""
	maxProbability := 0.0
	for mode, count := range stats.FailureModes {
		prob := float64(count) / float64(stats.TotalFailures)
		if prob > maxProbability {
			maxProbability = prob
			predictedMode = mode
		}
	}

	// Calculate confidence
	confidence := calculateModelConfidence(*stats)

	model := &PredictiveModel{
		WorkType:              workType,
		WorkDomain:            workDomain,
		RiskFactors:           riskFactors,
		PredictedFailureMode:  predictedMode,
		Probability:           maxProbability,
		Confidence:            confidence,
		LastUpdated:           time.Now(),
	}

	return model, nil
}

// Helper functions

func determineRootCause(mode FailureMode, stats FailureStatistics) string {
	switch mode {
	case FailureTest:
		return "Test suite instability or incomplete test coverage"
	case FailureTimeout:
		return "Resource constraints or inefficient execution paths"
	case FailureValidation:
		return "Input validation gaps or schema mismatches"
	case FailureRuntime:
		return "Unhandled edge cases or resource exhaustion"
	case FailureWorkspace:
		return "Git state conflicts or workspace contamination"
	case FailurePolicy:
		return "Insufficient permissions or policy violations"
	case FailureInfra:
		return "Network issues or service unavailability"
	default:
		return "Unknown root cause - requires manual investigation"
	}
}

func gatherEvidence(mode FailureMode, stats FailureStatistics) []string {
	evidence := []string{}

	// Add failure mode as evidence
	evidence = append(evidence, fmt.Sprintf("Failure mode: %s", mode))

	// Add recommended actions
	for action, count := range stats.RecommendedActions {
		evidence = append(evidence, fmt.Sprintf("Recommended action: %s (%d times)", action, count))
	}

	// Add temporal evidence
	if !stats.LastFailureAt.IsZero() {
		evidence = append(evidence, fmt.Sprintf("Last failure: %s", stats.LastFailureAt.Format(time.RFC3339)))
	}

	// Add frequency evidence
	evidence = append(evidence, fmt.Sprintf("Total failures: %d", stats.TotalFailures))

	// Add template-specific evidence if available
	if stats.TemplateName != "" {
		evidence = append(evidence, fmt.Sprintf("Template: %s", stats.TemplateName))
	}

	return evidence
}

func calculateConfidence(occurrences int, evidenceCount int) float64 {
	// Base confidence from sample size
	confidence := 0.0
	if occurrences >= 10 {
		confidence = 0.9
	} else if occurrences >= 5 {
		confidence = 0.7
	} else if occurrences >= 3 {
		confidence = 0.5
	} else {
		confidence = 0.3
	}

	// Adjust for evidence quality
	evidenceBonus := float64(evidenceCount) * 0.05
	if evidenceBonus > 0.1 {
		evidenceBonus = 0.1 // Cap at 10% bonus
	}

	confidence += evidenceBonus
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func generateMitigation(mode FailureMode, rootCause string) string {
	switch mode {
	case FailureTest:
		return "Stabilize test suite, add retry logic, improve test isolation"
	case FailureTimeout:
		return "Increase timeout thresholds, optimize execution paths, add resource limits"
	case FailureValidation:
		return "Strengthen input validation, add schema checks, improve error messages"
	case FailureRuntime:
		return "Add error handling, implement circuit breakers, improve logging"
	case FailureWorkspace:
		return "Clean workspace state, reset git, isolate execution environments"
	case FailurePolicy:
		return "Review permissions, update policies, add approval workflows"
	case FailureInfra:
		return "Add retry logic, implement fallbacks, improve monitoring"
	default:
		return "Manual investigation required - insufficient data for automated mitigation"
	}
}

func analyzeTemporalCorrelations(mode string, stats FailureStatistics) []FailureCorrelation {
	correlations := []FailureCorrelation{}

	// Check for time-based patterns
	if stats.TotalFailures >= 3 {
		// High failure rate correlation
		correlations = append(correlations, FailureCorrelation{
			FailureMode:      mode,
			CorrelatedFactor: "high_failure_rate",
			CorrelationType:  "temporal",
			Strength:         0.7,
			SampleSize:       stats.TotalFailures,
		})

		// Recent failures correlation
		correlations = append(correlations, FailureCorrelation{
			FailureMode:      mode,
			CorrelatedFactor: "recent_failures",
			CorrelationType:  "temporal",
			Strength:         0.6,
			SampleSize:       stats.TotalFailures,
		})
	}

	return correlations
}

func analyzeEnvironmentalCorrelations(mode string, stats FailureStatistics) []FailureCorrelation {
	correlations := []FailureCorrelation{}

	// Template correlation
	if stats.TemplateName != "" {
		correlations = append(correlations, FailureCorrelation{
			FailureMode:      mode,
			CorrelatedFactor: fmt.Sprintf("template:%s", stats.TemplateName),
			CorrelationType:  "environmental",
			Strength:         0.8,
			SampleSize:       stats.TotalFailures,
		})
	}

	// Work type correlation
	correlations = append(correlations, FailureCorrelation{
		FailureMode:      mode,
		CorrelatedFactor: fmt.Sprintf("work_type:%s", stats.WorkType),
		CorrelationType:  "environmental",
		Strength:         0.6,
		SampleSize:       stats.TotalFailures,
	})

	return correlations
}

func analyzeCausalCorrelations(mode string, stats FailureStatistics) []FailureCorrelation {
	correlations := []FailureCorrelation{}

	// Recommended action correlation
	for action := range stats.RecommendedActions {
		correlations = append(correlations, FailureCorrelation{
			FailureMode:      mode,
			CorrelatedFactor: fmt.Sprintf("recommended_action:%s", action),
			CorrelationType:  "causal",
			Strength:         0.7,
			SampleSize:       stats.TotalFailures,
		})
	}

	return correlations
}

func identifyRiskFactors(stats FailureStatistics) []string {
	factors := []string{}

	// High failure rate
	if stats.TotalFailures > 5 {
		factors = append(factors, "high_historical_failure_rate")
	}

	// Recent failures
	if time.Since(stats.LastFailureAt) < 7*24*time.Hour {
		factors = append(factors, "recent_failures")
	}

	// Multiple failure modes
	if len(stats.FailureModes) > 2 {
		factors = append(factors, "diverse_failure_modes")
	}

	// Template-specific issues
	if stats.TemplateName != "" {
		factors = append(factors, fmt.Sprintf("template:%s", stats.TemplateName))
	}

	// Recommended actions indicate issues
	for action := range stats.RecommendedActions {
		factors = append(factors, fmt.Sprintf("requires:%s", action))
	}

	return factors
}

func calculateModelConfidence(stats FailureStatistics) float64 {
	// Base confidence from sample size
	if stats.TotalFailures < 3 {
		return 0.3
	} else if stats.TotalFailures < 5 {
		return 0.5
	} else if stats.TotalFailures < 10 {
		return 0.7
	} else {
		return 0.9
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// DiagnoseFailure performs comprehensive failure diagnosis.
func (a *FailureAnalyzer) DiagnoseFailure(ctx context.Context, workType, workDomain string) (*FailureDiagnosis, error) {
	// Get root causes
	allRootCauses, err := a.AnalyzeRootCauses(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for this work type/domain
	relevantRootCauses := []RootCauseAnalysis{}
	for _, rc := range allRootCauses {
		if rc.WorkType == workType && rc.WorkDomain == workDomain {
			relevantRootCauses = append(relevantRootCauses, rc)
		}
	}

	// Get correlations
	correlations, err := a.AnalyzeCorrelations(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for this work type/domain
	relevantCorrelations := []FailureCorrelation{}
	for _, c := range correlations {
		// Check if correlation involves this work type/domain
		if strings.Contains(c.CorrelatedFactor, workType) || strings.Contains(c.CorrelatedFactor, workDomain) {
			relevantCorrelations = append(relevantCorrelations, c)
		}
	}

	// Build predictive model
	model, err := a.BuildPredictiveModel(ctx, workType, workDomain)
	if err != nil {
		// Continue without model if insufficient data
		model = nil
	}

	diagnosis := &FailureDiagnosis{
		WorkType:        workType,
		WorkDomain:      workDomain,
		RootCauses:      relevantRootCauses,
		Correlations:    relevantCorrelations,
		PredictiveModel: model,
		DiagnosedAt:     time.Now(),
	}

	return diagnosis, nil
}

// FailureDiagnosis represents a comprehensive failure diagnosis.
type FailureDiagnosis struct {
	WorkType        string               `json:"work_type"`
	WorkDomain      string               `json:"work_domain"`
	RootCauses      []RootCauseAnalysis  `json:"root_causes,omitempty"`
	Correlations    []FailureCorrelation `json:"correlations,omitempty"`
	PredictiveModel *PredictiveModel     `json:"predictive_model,omitempty"`
	DiagnosedAt     time.Time            `json:"diagnosed_at"`
}
