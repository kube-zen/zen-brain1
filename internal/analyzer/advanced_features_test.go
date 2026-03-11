package analyzer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ============================================================================
// CONFIDENCE CALIBRATION TESTS
// ============================================================================

func TestConfidenceCalibrator_RecordResult(t *testing.T) {
	calibrator := NewConfidenceCalibrator()

	// Record some results
	calibrator.RecordResult("implementation", 0.8, true, "v1.0")
	calibrator.RecordResult("implementation", 0.7, false, "v1.0")
	calibrator.RecordResult("implementation", 0.9, true, "v1.0")

	stats, err := calibrator.GetCalibrationStats("implementation")
	if err != nil {
		t.Fatalf("Failed to get calibration stats: %v", err)
	}

	if stats.TotalAnalyses != 3 {
		t.Errorf("Expected 3 total analyses, got %d", stats.TotalAnalyses)
	}

	// Success rate should be 2/3 = 0.667
	expectedSuccessRate := 2.0 / 3.0
	if stats.SuccessRate < expectedSuccessRate-0.01 || stats.SuccessRate > expectedSuccessRate+0.01 {
		t.Errorf("Expected success rate ~%.2f, got %.2f", expectedSuccessRate, stats.SuccessRate)
	}

	// Average confidence should be (0.8 + 0.7 + 0.9) / 3 = 0.8
	if stats.AvgConfidence < 0.79 || stats.AvgConfidence > 0.81 {
		t.Errorf("Expected avg confidence ~0.8, got %.2f", stats.AvgConfidence)
	}
}

func TestConfidenceCalibrator_CalibrateConfidence(t *testing.T) {
	calibrator := NewConfidenceCalibrator()

	// No calibration data yet - should return as-is
	conf, adjusted := calibrator.CalibrateConfidence("implementation", 0.8)
	if adjusted {
		t.Error("Expected no adjustment with no data")
	}
	if conf != 0.8 {
		t.Errorf("Expected confidence 0.8, got %.2f", conf)
	}

	// Record overconfident results (predicted high, actual low success)
	for i := 0; i < 20; i++ {
		calibrator.RecordResult("implementation", 0.9, false, "v1.0") // Overconfident
	}

	// Now calibrate
	conf, adjusted = calibrator.CalibrateConfidence("implementation", 0.9)
	if !adjusted {
		t.Error("Expected adjustment with calibration data")
	}

	// Should reduce confidence since we're overconfident
	if conf >= 0.9 {
		t.Errorf("Expected reduced confidence < 0.9, got %.2f", conf)
	}
}

func TestConfidenceCalibrator_CalibrationStats(t *testing.T) {
	calibrator := NewConfidenceCalibrator()

	// Test with no data
	_, err := calibrator.GetCalibrationStats("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent work type")
	}

	// Add some data
	calibrator.RecordResult("debug", 0.5, true, "v1.0")
	calibrator.RecordResult("debug", 0.6, true, "v1.0")
	calibrator.RecordResult("debug", 0.7, false, "v1.0")

	stats, err := calibrator.GetCalibrationStats("debug")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalAnalyses != 3 {
		t.Errorf("Expected 3 analyses, got %d", stats.TotalAnalyses)
	}

	if stats.SuccessRate != 2.0/3.0 {
		t.Errorf("Expected success rate 0.667, got %.2f", stats.SuccessRate)
	}

	// Check over/under confidence flags
	if stats.AvgConfidence > stats.SuccessRate+0.2 {
		if !stats.IsOverconfident {
			t.Error("Expected IsOverconfident to be true")
		}
	}
}

// ============================================================================
// MULTI-MODEL COMPARISON TESTS
// ============================================================================

func TestCompareModels(t *testing.T) {
	workItemID := "TEST-123"

	comparisons := []*ModelAnalysisComparison{
		{
			Model: "model-a",
			AnalysisResult: &contracts.AnalysisResult{
				WorkItem: &contracts.WorkItem{
					WorkType: contracts.WorkTypeImplementation,
					Priority: contracts.PriorityHigh,
				},
			},
			Confidence:    0.9,
			TaskCount:     3,
			CostUSD:       2.0,
			ExecutionTime: 10 * time.Second,
		},
		{
			Model: "model-b",
			AnalysisResult: &contracts.AnalysisResult{
				WorkItem: &contracts.WorkItem{
					WorkType: contracts.WorkTypeImplementation,
					Priority: contracts.PriorityMedium,
				},
			},
			Confidence:    0.8,
			TaskCount:     2,
			CostUSD:       1.0,
			ExecutionTime: 5 * time.Second,
		},
	}

	comparison := CompareModels(workItemID, comparisons)

	if comparison.WorkItemID != workItemID {
		t.Errorf("Expected work item ID %s, got %s", workItemID, comparison.WorkItemID)
	}

	if len(comparison.Comparisons) != 2 {
		t.Errorf("Expected 2 comparisons, got %d", len(comparison.Comparisons))
	}

	if comparison.Summary == nil {
		t.Fatal("Expected summary to be non-nil")
	}

	if comparison.Summary.AvgConfidence < 0.84 || comparison.Summary.AvgConfidence > 0.86 {
		t.Errorf("Expected avg confidence ~0.85, got %.2f", comparison.Summary.AvgConfidence)
	}

	// Check that recommended model is set
	if comparison.RecommendedModel == "" {
		t.Error("Expected recommended model to be set")
	}
}

func TestCalculateQualityScore(t *testing.T) {
	tests := []struct {
		name     string
		comp     *ModelAnalysisComparison
		expected string // "high", "medium", "low"
	}{
		{
			name: "high quality",
			comp: &ModelAnalysisComparison{
				AnalysisResult: &contracts.AnalysisResult{},
				Confidence:      0.9,
				TaskCount:       3,
				CostUSD:         2.0,
				ExecutionTime:   10 * time.Second,
			},
			expected: "high",
		},
		{
			name: "medium quality",
			comp: &ModelAnalysisComparison{
				AnalysisResult: &contracts.AnalysisResult{},
				Confidence:      0.7,
				TaskCount:       5,
				CostUSD:         5.0,
				ExecutionTime:   30 * time.Second,
			},
			expected: "medium",
		},
		{
			name: "low quality",
			comp: &ModelAnalysisComparison{
				AnalysisResult: nil,
				Confidence:     0.5,
				TaskCount:      15,
				CostUSD:        20.0,
				ExecutionTime:  90 * time.Second,
			},
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateQualityScore(tt.comp)

			switch tt.expected {
			case "high":
				if score < 70 {
					t.Errorf("Expected high quality score (>=70), got %.2f", score)
				}
			case "medium":
				if score < 40 || score >= 70 {
					t.Errorf("Expected medium quality score (40-70), got %.2f", score)
				}
			case "low":
				if score >= 40 {
					t.Errorf("Expected low quality score (<40), got %.2f", score)
				}
			}
		})
	}
}

func TestCalculateConsensusRate(t *testing.T) {
	fieldValues := map[string][]ModelFieldValue{
		"work_type": {
			{Model: "model-a", Value: "implementation"},
			{Model: "model-b", Value: "implementation"},
			{Model: "model-c", Value: "debug"},
		},
		"priority": {
			{Model: "model-a", Value: "high"},
			{Model: "model-b", Value: "high"},
			{Model: "model-c", Value: "high"},
		},
	}

	consensusRate := calculateConsensusRate(fieldValues)

	// 2/3 agree on work_type (0.667) + 3/3 agree on priority (1.0)
	// Average = 0.833
	expectedRate := (2.0/3.0 + 1.0) / 2.0
	if consensusRate < expectedRate-0.01 || consensusRate > expectedRate+0.01 {
		t.Errorf("Expected consensus rate ~%.2f, got %.2f", expectedRate, consensusRate)
	}
}

func TestFindFieldDisagreements(t *testing.T) {
	fieldValues := map[string][]ModelFieldValue{
		"work_type": {
			{Model: "model-a", Value: "implementation"},
			{Model: "model-b", Value: "implementation"},
			{Model: "model-c", Value: "debug"},
		},
		"priority": {
			{Model: "model-a", Value: "high"},
			{Model: "model-b", Value: "high"},
			{Model: "model-c", Value: "high"},
		},
	}

	disagreements := findFieldDisagreements(fieldValues)

	// Should find 1 disagreement (work_type)
	if len(disagreements) != 1 {
		t.Errorf("Expected 1 disagreement, got %d", len(disagreements))
	}

	if len(disagreements) > 0 {
		if disagreements[0].Field != "work_type" {
			t.Errorf("Expected disagreement on work_type, got %s", disagreements[0].Field)
		}

		if disagreements[0].Consensus != "implementation" {
			t.Errorf("Expected consensus 'implementation', got %s", disagreements[0].Consensus)
		}
	}
}

// ============================================================================
// STREAMING TESTS
// ============================================================================

func TestNewAnalysisStream(t *testing.T) {
	workItemID := "TEST-456"
	stream := NewAnalysisStream(workItemID)

	if stream.WorkItemID != workItemID {
		t.Errorf("Expected work item ID %s, got %s", workItemID, stream.WorkItemID)
	}

	if stream.Status != "streaming" {
		t.Errorf("Expected status 'streaming', got %s", stream.Status)
	}

	if stream.StreamID == "" {
		t.Error("Expected stream ID to be set")
	}
}

func TestAnalysisStream_Cancel(t *testing.T) {
	stream := NewAnalysisStream("TEST-789")

	if stream.Status != "streaming" {
		t.Errorf("Expected status 'streaming', got %s", stream.Status)
	}

	stream.Cancel()

	if stream.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %s", stream.Status)
	}

	// Cancel again should be no-op
	stream.Cancel()

	if stream.Status != "cancelled" {
		t.Errorf("Expected status to remain 'cancelled', got %s", stream.Status)
	}
}

func TestAnalysisStream_GetProgress(t *testing.T) {
	stream := NewAnalysisStream("TEST-999")
	stream.CurrentStage = "classification"
	stream.CompletedStages = []string{"requirements"}
	stream.Errors = []string{"test error"}

	progress := stream.GetProgress()

	if progress.CurrentStage != "classification" {
		t.Errorf("Expected current stage 'classification', got %s", progress.CurrentStage)
	}

	if len(progress.CompletedStages) != 1 {
		t.Errorf("Expected 1 completed stage, got %d", len(progress.CompletedStages))
	}

	if len(progress.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(progress.Errors))
	}
}

// ============================================================================
// FEEDBACK LOOP TESTS
// ============================================================================

func TestMemoryFeedbackStore_StoreAndGet(t *testing.T) {
	store := NewMemoryFeedbackStore()
	ctx := context.Background()

	feedback := &AnalysisFeedback{
		AnalysisID:  "analysis-123",
		WorkItemID:  "TEST-123",
		FeedbackType: FeedbackTypeGeneral,
		Rating:      4,
		Comments:    "Good analysis",
		SubmittedAt: time.Now(),
		SubmittedBy: "test-user",
	}

	err := store.Store(ctx, feedback)
	if err != nil {
		t.Fatalf("Failed to store feedback: %v", err)
	}

	retrieved, err := store.GetFeedback(ctx, "analysis-123")
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 feedback, got %d", len(retrieved))
	}

	if retrieved[0].Rating != 4 {
		t.Errorf("Expected rating 4, got %d", retrieved[0].Rating)
	}
}

func TestMemoryFeedbackStore_GetAverageRating(t *testing.T) {
	store := NewMemoryFeedbackStore()
	ctx := context.Background()

	// Store multiple feedbacks
	for i := 0; i < 5; i++ {
		feedback := &AnalysisFeedback{
			AnalysisID:  fmt.Sprintf("analysis-%d", i),
			WorkItemID:  "TEST-123",
			FeedbackType: FeedbackTypeGeneral,
			Rating:      i + 1, // 1, 2, 3, 4, 5
			SubmittedAt: time.Now(),
		}
		_ = store.Store(ctx, feedback)
	}

	avgRating, err := store.GetAverageRating(ctx, "implementation")
	if err != nil {
		t.Fatalf("Failed to get average rating: %v", err)
	}

	// Average of 1,2,3,4,5 = 3.0
	expectedAvg := 3.0
	if avgRating < expectedAvg-0.01 || avgRating > expectedAvg+0.01 {
		t.Errorf("Expected average rating ~%.2f, got %.2f", expectedAvg, avgRating)
	}
}

// ============================================================================
// CACHING TESTS
// ============================================================================

func TestAnalysisCache_GetPut(t *testing.T) {
	cache := NewAnalysisCache(1 * time.Hour)
	ctx := context.Background()

	workItem := &contracts.WorkItem{
		ID:         "TEST-123",
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityHigh,
		Body:       "Test work item",
	}

	// Get non-existent entry
	result, found := cache.Get(ctx, workItem)
	if found {
		t.Error("Expected cache miss, got hit")
	}

	// Put entry
	analysisResult := &contracts.AnalysisResult{
		WorkItem: workItem,
	}
	cache.Put(ctx, workItem, analysisResult)

	// Get existing entry
	result, found = cache.Get(ctx, workItem)
	if !found {
		t.Error("Expected cache hit, got miss")
	}

	if result.WorkItem.ID != workItem.ID {
		t.Errorf("Expected work item ID %s, got %s", workItem.ID, result.WorkItem.ID)
	}
}

func TestAnalysisCache_EvictExpired(t *testing.T) {
	// Very short TTL for testing
	cache := NewAnalysisCache(100 * time.Millisecond)
	ctx := context.Background()

	workItem := &contracts.WorkItem{
		ID:       "TEST-123",
		WorkType: contracts.WorkTypeImplementation,
	}

	cache.Put(ctx, workItem, &contracts.AnalysisResult{WorkItem: workItem})

	// Should be cached immediately
	_, found := cache.Get(ctx, workItem)
	if !found {
		t.Error("Expected cache hit immediately after put")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	evicted := cache.EvictExpired(ctx)
	if evicted != 1 {
		t.Errorf("Expected 1 entry evicted, got %d", evicted)
	}

	// Should be cache miss
	_, found = cache.Get(ctx, workItem)
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestAnalysisCache_GetStats(t *testing.T) {
	cache := NewAnalysisCache(1 * time.Hour)
	ctx := context.Background()

	// Put some entries
	for i := 0; i < 5; i++ {
		workItem := &contracts.WorkItem{
			ID:       fmt.Sprintf("TEST-%d", i),
			WorkType: contracts.WorkTypeImplementation,
		}
		cache.Put(ctx, workItem, &contracts.AnalysisResult{WorkItem: workItem})
	}

	stats := cache.GetStats()

	if stats.TotalEntries != 5 {
		t.Errorf("Expected 5 total entries, got %d", stats.TotalEntries)
	}

	if stats.ActiveEntries != 5 {
		t.Errorf("Expected 5 active entries, got %d", stats.ActiveEntries)
	}

	if stats.ExpiredCount != 0 {
		t.Errorf("Expected 0 expired entries, got %d", stats.ExpiredCount)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	workItem1 := &contracts.WorkItem{
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityHigh,
		Body:       "Test body",
	}

	workItem2 := &contracts.WorkItem{
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityHigh,
		Body:       "Test body",
	}

	key1 := GenerateCacheKey(workItem1)
	key2 := GenerateCacheKey(workItem2)

	// Same content should produce same key
	if key1 != key2 {
		t.Error("Expected same keys for same content")
	}

	// Different content should produce different key
	workItem3 := &contracts.WorkItem{
		WorkType:   contracts.WorkTypeDebug,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityHigh,
		Body:       "Test body",
	}

	key3 := GenerateCacheKey(workItem3)
	if key1 == key3 {
		t.Error("Expected different keys for different work types")
	}
}

// ============================================================================
// RISK PREDICTION TESTS
// ============================================================================

func TestRiskPredictor_RecordAndPredict(t *testing.T) {
	predictor := NewRiskPredictor()

	// Record some historical risks
	predictor.RecordRiskOutcome("implementation", "API compatibility issues")
	predictor.RecordRiskOutcome("implementation", "API compatibility issues")
	predictor.RecordRiskOutcome("implementation", "API compatibility issues")
	predictor.RecordRiskOutcome("implementation", "Performance degradation")

	workItem := &contracts.WorkItem{
		WorkType: contracts.WorkTypeImplementation,
	}

	predictions := predictor.PredictRisks(workItem)

	// Should predict risks with 30%+ historical occurrence
	// "API compatibility issues" appears 3/4 times (75%)
	if len(predictions) == 0 {
		t.Error("Expected risk predictions, got none")
	}

	for _, pred := range predictions {
		if pred.Probability < 0.3 {
			t.Errorf("Expected probability >= 0.3, got %.2f", pred.Probability)
		}

		if pred.Category == "" {
			t.Error("Expected risk category to be set")
		}

		if pred.Mitigation == "" {
			t.Error("Expected mitigation to be set")
		}
	}
}

func TestRiskPredictor_NoHistory(t *testing.T) {
	predictor := NewRiskPredictor()

	workItem := &contracts.WorkItem{
		WorkType: contracts.WorkTypeRefactor,
	}

	predictions := predictor.PredictRisks(workItem)

	// No historical data, should return empty predictions
	if len(predictions) != 0 {
		t.Errorf("Expected 0 predictions with no history, got %d", len(predictions))
	}
}

func TestInferRiskCategory(t *testing.T) {
	tests := []struct {
		risk     string
		expected string
	}{
		{"Complexity issues", "technical"},
		{"Refactor regression", "technical"},
		{"Time overrun", "operational"},
		{"Schedule delay", "operational"},
		{"API compatibility", "dependency"},
		{"Dependency update", "dependency"},
		{"Unknown issue", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.risk, func(t *testing.T) {
			category := inferRiskCategory(tt.risk)
			if category != tt.expected {
				t.Errorf("Expected category '%s', got '%s'", tt.expected, category)
			}
		})
	}
}

func TestGetStandardMitigation(t *testing.T) {
	tests := []struct {
		risk     string
		expected string
	}{
		{"Complexity issues", "Create detailed design document"},
		{"Time overrun", "Break into smaller tasks"},
		{"API compatibility", "Test API contract"},
		{"Dependency update", "Validate dependency availability"},
		{"Unknown issue", "Monitor and address"},
	}

	for _, tt := range tests {
		t.Run(tt.risk, func(t *testing.T) {
			mitigation := getStandardMitigation(tt.risk)
			if !contains(mitigation, tt.expected) {
				t.Errorf("Expected mitigation to contain '%s', got '%s'", tt.expected, mitigation)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
