package intelligence

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFailureAnalyzer_AnalyzeRootCauses(t *testing.T) {
	store := NewInMemoryFailureStore()
	analyzer := NewFailureAnalyzer(store)
	ctx := context.Background()

	// Store test failure statistics
	stats1 := &FailureStatistics{
		WorkType:       "implementation",
		WorkDomain:     "core",
		TotalFailures:  5,
		FailureModes:   map[string]int{"test": 3, "timeout": 2},
		LastFailureAt:  time.Now(),
		RecommendedActions: map[string]int{"review_logs": 3, "increase_timeout": 2},
	}

	stats2 := &FailureStatistics{
		WorkType:       "bug-fix",
		WorkDomain:     "api",
		TemplateName:   "bug-fix:real",
		TotalFailures:  10,
		FailureModes:   map[string]int{"validation": 7, "test": 3},
		LastFailureAt:  time.Now(),
		RecommendedActions: map[string]int{"fix_schema": 7},
	}

	if err := store.StoreFailureStats(ctx, stats1); err != nil {
		t.Fatalf("Failed to store stats1: %v", err)
	}

	if err := store.StoreFailureStats(ctx, stats2); err != nil {
		t.Fatalf("Failed to store stats2: %v", err)
	}

	// Analyze root causes
	analyses, err := analyzer.AnalyzeRootCauses(ctx)
	if err != nil {
		t.Fatalf("AnalyzeRootCauses failed: %v", err)
	}

	if len(analyses) == 0 {
		t.Fatal("Expected at least one root cause analysis")
	}

	// Verify analyses
	for _, analysis := range analyses {
		t.Logf("Root cause: %s/%s - %s (confidence: %.2f, occurrences: %d)",
			analysis.WorkType, analysis.WorkDomain, analysis.RootCause, analysis.Confidence, analysis.Occurrences)

		if analysis.RootCause == "" {
			t.Error("Root cause should not be empty")
		}

		if analysis.Confidence < 0.0 || analysis.Confidence > 1.0 {
			t.Errorf("Confidence should be between 0.0 and 1.0, got: %.2f", analysis.Confidence)
		}

		if analysis.Occurrences <= 0 {
			t.Error("Occurrences should be positive")
		}

		if analysis.MitigationStrategy == "" {
			t.Error("Mitigation strategy should not be empty")
		}
	}

	t.Logf("✅ Analyzed %d root causes", len(analyses))
}

func TestFailureAnalyzer_AnalyzeCorrelations(t *testing.T) {
	store := NewInMemoryFailureStore()
	analyzer := NewFailureAnalyzer(store)
	ctx := context.Background()

	// Store test failure statistics
	stats := &FailureStatistics{
		WorkType:           "implementation",
		WorkDomain:         "core",
		TemplateName:       "implementation:real",
		TotalFailures:      10,
		FailureModes:       map[string]int{"test": 6, "timeout": 4},
		LastFailureAt:      time.Now(),
		RecommendedActions: map[string]int{"fix_tests": 6, "increase_timeout": 4},
	}

	if err := store.StoreFailureStats(ctx, stats); err != nil {
		t.Fatalf("Failed to store stats: %v", err)
	}

	// Analyze correlations
	correlations, err := analyzer.AnalyzeCorrelations(ctx)
	if err != nil {
		t.Fatalf("AnalyzeCorrelations failed: %v", err)
	}

	if len(correlations) == 0 {
		t.Fatal("Expected at least one correlation")
	}

	// Verify correlations
	for _, c := range correlations {
		t.Logf("Correlation: %s <-> %s (type: %s, strength: %.2f, samples: %d)",
			c.FailureMode, c.CorrelatedFactor, c.CorrelationType, c.Strength, c.SampleSize)

		if c.FailureMode == "" {
			t.Error("Failure mode should not be empty")
		}

		if c.CorrelatedFactor == "" {
			t.Error("Correlated factor should not be empty")
		}

		if c.Strength < -1.0 || c.Strength > 1.0 {
			t.Errorf("Strength should be between -1.0 and 1.0, got: %.2f", c.Strength)
		}

		if c.SampleSize < 3 {
			t.Errorf("Sample size should be >= 3 (filtered), got: %d", c.SampleSize)
		}
	}

	t.Logf("✅ Found %d significant correlations", len(correlations))
}

func TestFailureAnalyzer_BuildPredictiveModel(t *testing.T) {
	store := NewInMemoryFailureStore()
	analyzer := NewFailureAnalyzer(store)
	ctx := context.Background()

	// Store test failure statistics (without template for basic lookup)
	stats := &FailureStatistics{
		WorkType:           "implementation",
		WorkDomain:         "core",
		TotalFailures:      15,
		FailureModes:       map[string]int{"test": 10, "timeout": 3, "validation": 2},
		LastFailureAt:      time.Now(),
		RecommendedActions: map[string]int{"fix_tests": 10},
	}

	if err := store.StoreFailureStats(ctx, stats); err != nil {
		t.Fatalf("Failed to store stats: %v", err)
	}

	// Build predictive model
	model, err := analyzer.BuildPredictiveModel(ctx, "implementation", "core")
	if err != nil {
		t.Fatalf("BuildPredictiveModel failed: %v", err)
	}

	if model == nil {
		t.Fatal("Model should not be nil")
	}

	t.Logf("Predictive model for implementation/core:")
	t.Logf("  Predicted failure mode: %s", model.PredictedFailureMode)
	t.Logf("  Probability: %.2f", model.Probability)
	t.Logf("  Confidence: %.2f", model.Confidence)
	t.Logf("  Risk factors: %v", model.RiskFactors)

	// Verify model
	if model.WorkType != "implementation" {
		t.Errorf("Expected work type 'implementation', got: %s", model.WorkType)
	}

	if model.WorkDomain != "core" {
		t.Errorf("Expected work domain 'core', got: %s", model.WorkDomain)
	}

	if model.PredictedFailureMode == "" {
		t.Error("Predicted failure mode should not be empty")
	}

	if model.Probability < 0.0 || model.Probability > 1.0 {
		t.Errorf("Probability should be between 0.0 and 1.0, got: %.2f", model.Probability)
	}

	if model.Confidence < 0.0 || model.Confidence > 1.0 {
		t.Errorf("Confidence should be between 0.0 and 1.0, got: %.2f", model.Confidence)
	}

	if len(model.RiskFactors) == 0 {
		t.Error("Expected at least one risk factor")
	}

	// Expected prediction: "test" has highest count (10/15 = 0.67)
	if model.PredictedFailureMode != "test" {
		t.Errorf("Expected predicted failure mode 'test', got: %s", model.PredictedFailureMode)
	}

	if model.Probability < 0.6 {
		t.Errorf("Expected probability >= 0.6, got: %.2f", model.Probability)
	}

	t.Logf("✅ Predictive model built successfully")
}

func TestFailureAnalyzer_DiagnoseFailure(t *testing.T) {
	store := NewInMemoryFailureStore()
	analyzer := NewFailureAnalyzer(store)
	ctx := context.Background()

	// Store test failure statistics (without template for basic lookup)
	stats := &FailureStatistics{
		WorkType:           "bug-fix",
		WorkDomain:         "api",
		TotalFailures:      12,
		FailureModes:       map[string]int{"validation": 8, "test": 4},
		LastFailureAt:      time.Now(),
		RecommendedActions: map[string]int{"fix_schema": 8, "fix_tests": 4},
	}

	if err := store.StoreFailureStats(ctx, stats); err != nil {
		t.Fatalf("Failed to store stats: %v", err)
	}

	// Diagnose failure
	diagnosis, err := analyzer.DiagnoseFailure(ctx, "bug-fix", "api")
	if err != nil {
		t.Fatalf("DiagnoseFailure failed: %v", err)
	}

	if diagnosis == nil {
		t.Fatal("Diagnosis should not be nil")
	}

	t.Logf("Failure diagnosis for bug-fix/api:")
	t.Logf("  Root causes: %d", len(diagnosis.RootCauses))
	t.Logf("  Correlations: %d", len(diagnosis.Correlations))
	t.Logf("  Predictive model: %v", diagnosis.PredictiveModel != nil)

	// Verify diagnosis
	if diagnosis.WorkType != "bug-fix" {
		t.Errorf("Expected work type 'bug-fix', got: %s", diagnosis.WorkType)
	}

	if diagnosis.WorkDomain != "api" {
		t.Errorf("Expected work domain 'api', got: %s", diagnosis.WorkDomain)
	}

	if len(diagnosis.RootCauses) == 0 {
		t.Error("Expected at least one root cause")
	}

	if diagnosis.PredictiveModel == nil {
		t.Error("Expected predictive model to be built")
	}

	// Verify root causes contain expected modes
	foundValidation := false
	foundTest := false
	for _, rc := range diagnosis.RootCauses {
		if rc.FailureMode == "validation" {
			foundValidation = true
			t.Logf("  ✓ Validation root cause: %s", rc.RootCause)
		}
		if rc.FailureMode == "test" {
			foundTest = true
			t.Logf("  ✓ Test root cause: %s", rc.RootCause)
		}
	}

	if !foundValidation {
		t.Error("Expected validation failure mode in root causes")
	}

	if !foundTest {
		t.Error("Expected test failure mode in root causes")
	}

	t.Logf("✅ Failure diagnosis completed successfully")
}

func TestDetermineRootCause(t *testing.T) {
	tests := []struct {
		mode     FailureMode
		contains string
	}{
		{FailureTest, "Test"},
		{FailureTimeout, "Resource"},
		{FailureValidation, "Input validation"},
		{FailureRuntime, "edge case"},
		{FailureWorkspace, "Git"},
		{FailurePolicy, "Insufficient permissions"},
		{FailureInfra, "Network"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			stats := FailureStatistics{}
			rootCause := determineRootCause(tt.mode, stats)

			if rootCause == "" {
				t.Error("Root cause should not be empty")
			}

			if !strings.Contains(rootCause, tt.contains) {
				t.Errorf("Expected root cause to contain '%s', got: %s", tt.contains, rootCause)
			}

			t.Logf("  %s -> %s", tt.mode, rootCause)
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		occurrences   int
		evidenceCount int
		minConfidence float64
		maxConfidence float64
	}{
		{1, 0, 0.2, 0.4},   // Low occurrences, low evidence
		{3, 2, 0.5, 0.7},   // Medium occurrences, medium evidence
		{5, 4, 0.7, 0.85},  // Good occurrences, good evidence
		{10, 5, 0.9, 1.0},  // High occurrences, high evidence
		{20, 10, 0.95, 1.0}, // Very high occurrences, very high evidence
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			confidence := calculateConfidence(tt.occurrences, tt.evidenceCount)

			if confidence < tt.minConfidence || confidence > tt.maxConfidence {
				t.Errorf("Expected confidence between %.2f and %.2f, got: %.2f",
					tt.minConfidence, tt.maxConfidence, confidence)
			}

			t.Logf("  occurrences=%d, evidence=%d -> confidence=%.2f",
				tt.occurrences, tt.evidenceCount, confidence)
		})
	}
}
