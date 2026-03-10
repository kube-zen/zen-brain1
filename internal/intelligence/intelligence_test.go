// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test in-memory pattern store
func TestInMemoryPatternStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryPatternStore()

	// Create test mining result with aggregated statistics
	rawWorkTypeStats := map[string]*WorkTypeStatistics{
		"implementation-backend": {
			WorkType:           "implementation",
			WorkDomain:         "backend",
			TotalRuns:          10,
			SuccessfulRuns:     8,
			TotalDuration:      10 * time.Minute,
			TotalFilesChanged:  20,
		},
	}
	rawTemplateStats := map[string]*TemplateStatistics{
		"implementation:real": {
			TemplateName:  "implementation:real",
			TotalRuns:     10,
			SuccessfulRuns: 8,
			TotalDuration:  10 * time.Minute,
		},
	}

	result := &MiningResult{
		WorkTypeStatistics: AggregateWorkTypeStats(rawWorkTypeStats),
		TemplateStatistics: AggregateTemplateStats(rawTemplateStats),
		DurationStatistics: []DurationStatistics{
			{
				WorkType: "implementation",
				WorkDomain: "backend",
				Samples: []time.Duration{
					1 * time.Minute,
					2 * time.Minute,
					3 * time.Minute,
				},
			},
		},
	}

	// Store patterns
	err := store.StorePatterns(ctx, result)
	if err != nil {
		t.Fatalf("Failed to store patterns: %v", err)
	}

	// Retrieve work type stats
	stats, err := store.GetWorkTypeStats(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to get work type stats: %v", err)
	}
	if stats.TotalRuns != 10 {
		t.Errorf("Expected TotalRuns=10, got %d", stats.TotalRuns)
	}
	if stats.SuccessRate != 0.8 {
		t.Errorf("Expected SuccessRate=0.8, got %f", stats.SuccessRate)
	}

	// Retrieve all work type stats
	allStats, err := store.GetAllWorkTypeStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get all work type stats: %v", err)
	}
	if len(allStats) != 1 {
		t.Errorf("Expected 1 work type stat, got %d", len(allStats))
	}

	// Retrieve template stats
	tStats, err := store.GetTemplateStats(ctx, "implementation:real")
	if err != nil {
		t.Fatalf("Failed to get template stats: %v", err)
	}
	if tStats.TotalRuns != 10 {
		t.Errorf("Expected TotalRuns=10, got %d", tStats.TotalRuns)
	}

	// Clear patterns
	err = store.ClearPatterns(ctx)
	if err != nil {
		t.Fatalf("Failed to clear patterns: %v", err)
	}

	// Verify patterns are cleared
	_, err = store.GetWorkTypeStats(ctx, "implementation", "backend")
	if err == nil {
		t.Error("Expected error after clearing patterns, got nil")
	}
}

// Test recommender
func TestRecommender(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryPatternStore()
	recommender := NewRecommender(store, 3)

	// Test with no data
	rec, err := recommender.RecommendTemplate(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to recommend template: %v", err)
	}
	if rec.Confidence != 0.0 {
		t.Errorf("Expected Confidence=0.0 with no data, got %f", rec.Confidence)
	}
	if rec.TemplateName != "default" {
		t.Errorf("Expected default template with no data, got %s", rec.TemplateName)
	}

	// Add test data with aggregated statistics
	rawWorkTypeStats := map[string]*WorkTypeStatistics{
		"implementation-backend": {
			WorkType:           "implementation",
			WorkDomain:         "backend",
			TotalRuns:          10,
			SuccessfulRuns:     9,
			TotalDuration:      15 * time.Minute,
			TotalFilesChanged:  30,
		},
	}
	rawTemplateStats := map[string]*TemplateStatistics{
		"implementation:real": {
			TemplateName:  "implementation:real",
			TotalRuns:     10,
			SuccessfulRuns: 9,
			TotalDuration:  15 * time.Minute,
		},
	}

	result := &MiningResult{
		WorkTypeStatistics: AggregateWorkTypeStats(rawWorkTypeStats),
		TemplateStatistics: AggregateTemplateStats(rawTemplateStats),
		DurationStatistics: []DurationStatistics{
			{
				WorkType: "implementation",
				WorkDomain: "backend",
				Samples: []time.Duration{
					1 * time.Minute,
					2 * time.Minute,
					3 * time.Minute,
					4 * time.Minute,
					5 * time.Minute,
				},
			},
		},
	}

	err = store.StorePatterns(ctx, result)
	if err != nil {
		t.Fatalf("Failed to store patterns: %v", err)
	}

	// Test recommendation with data
	rec, err = recommender.RecommendTemplate(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to recommend template: %v", err)
	}
	if rec.Confidence <= 0.0 {
		t.Errorf("Expected positive Confidence with data, got %f", rec.Confidence)
	}
	if rec.TemplateName != "implementation:real" {
		t.Errorf("Expected implementation:real template, got %s", rec.TemplateName)
	}
	if rec.SuccessRate != 0.9 {
		t.Errorf("Expected SuccessRate=0.9, got %f", rec.SuccessRate)
	}

	// Test configuration recommendation
	configRec, err := recommender.RecommendConfiguration(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to recommend configuration: %v", err)
	}
	if configRec.TimeoutSeconds <= 0 {
		t.Errorf("Expected positive TimeoutSeconds, got %d", configRec.TimeoutSeconds)
	}
	if configRec.MaxRetries <= 0 {
		t.Errorf("Expected positive MaxRetries, got %d", configRec.MaxRetries)
	}

	// Test work type summary
	summary, err := recommender.GetWorkTypeSummary(ctx)
	if err != nil {
		t.Fatalf("Failed to get work type summary: %v", err)
	}
	if summary.TotalWorkTypes != 1 {
		t.Errorf("Expected 1 work type, got %d", summary.TotalWorkTypes)
	}
	if summary.OverallSuccessRate != 0.9 {
		t.Errorf("Expected OverallSuccessRate=0.9, got %f", summary.OverallSuccessRate)
	}

	// Test pattern analysis
	analysis, err := recommender.PatternAnalysis(ctx)
	if err != nil {
		t.Fatalf("Failed to get pattern analysis: %v", err)
	}
	if analysis.WorkTypeCount != 1 {
		t.Errorf("Expected 1 work type, got %d", analysis.WorkTypeCount)
	}
	if analysis.TemplateCount != 1 {
		t.Errorf("Expected 1 template, got %d", analysis.TemplateCount)
	}
	if analysis.TotalExecutions != 10 {
		t.Errorf("Expected 10 executions, got %d", analysis.TotalExecutions)
	}
}

// Test confidence calculation
func TestConfidenceCalculation(t *testing.T) {
	store := NewInMemoryPatternStore()
	recommender := NewRecommender(store, 3)

	tests := []struct {
		samples    int
		minConf    float64
		maxConf    float64
	}{
		{0, 0.0, 0.0},
		{1, 0.1, 0.2},
		{2, 0.2, 0.4},
		{3, 0.5, 0.9},
		{5, 0.6, 1.0},
		{10, 0.7, 1.0},
		{100, 0.9, 1.0},
	}

	for _, tt := range tests {
		conf := recommender.calculateConfidence(tt.samples)
		if conf < tt.minConf || conf > tt.maxConf {
			t.Errorf("Confidence for %d samples out of range [%.1f, %.1f]: %.2f",
				tt.samples, tt.minConf, tt.maxConf, conf)
		}
	}
}

// Test duration statistics
func TestDurationStatistics(t *testing.T) {
	samples := []time.Duration{
		10 * time.Second,
		20 * time.Second,
		30 * time.Second,
		40 * time.Second,
		50 * time.Second,
		60 * time.Second,
		70 * time.Second,
		80 * time.Second,
		90 * time.Second,
		100 * time.Second,
	}

	min := minDuration(samples)
	max := maxDuration(samples)
	median := medianDuration(samples)
	p95 := percentileDuration(samples, 0.95)
	p99 := percentileDuration(samples, 0.99)

	if min != 10*time.Second {
		t.Errorf("Expected min=10s, got %v", min)
	}
	if max != 100*time.Second {
		t.Errorf("Expected max=100s, got %v", max)
	}
	if median < 45*time.Second || median > 65*time.Second {
		t.Errorf("Median %v out of expected range [45s, 65s]", median)
	}
	if p95 != 100*time.Second {
		t.Errorf("Expected p95=100s, got %v", p95)
	}
	if p99 != 100*time.Second {
		t.Errorf("Expected p99=100s, got %v", p99)
	}
}

// Test JSON pattern store
func TestJSONPatternStore(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "pattern-store-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := NewJSONPatternStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create pattern store: %v", err)
	}

	// Create test mining result
	result := &MiningResult{
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
		Duration:  1 * time.Second,
		WorkTypeStatistics: []WorkTypeStatistics{
			{
				WorkType:           "implementation",
				WorkDomain:         "backend",
				TotalRuns:          5,
				SuccessfulRuns:     4,
				TotalDuration:      5 * time.Minute,
				TotalFilesChanged:  10,
			},
		},
		TemplateStatistics: []TemplateStatistics{
			{
				TemplateName:  "implementation:real",
				TotalRuns:     5,
				SuccessfulRuns: 4,
				TotalDuration:  5 * time.Minute,
			},
		},
		DurationStatistics: []DurationStatistics{
			{
				WorkType:  "implementation",
				WorkDomain: "backend",
				Samples: []time.Duration{
					30 * time.Second,
					60 * time.Second,
					90 * time.Second,
				},
			},
		},
	}

	// Store patterns
	err = store.StorePatterns(ctx, result)
	if err != nil {
		t.Fatalf("Failed to store patterns: %v", err)
	}

	// Verify files were created
	latestResultPath := filepath.Join(tmpDir, "latest-mining-result.json")
	if _, err := os.Stat(latestResultPath); os.IsNotExist(err) {
		t.Errorf("Latest result file not created: %s", latestResultPath)
	}

	workTypePath := filepath.Join(tmpDir, "worktypes", "implementation-backend.json")
	if _, err := os.Stat(workTypePath); os.IsNotExist(err) {
		t.Errorf("Work type stats file not created: %s", workTypePath)
	}

	templatePath := filepath.Join(tmpDir, "templates", "implementation:real.json")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Errorf("Template stats file not created: %s", templatePath)
	}

	durationPath := filepath.Join(tmpDir, "durations", "implementation-backend.json")
	if _, err := os.Stat(durationPath); os.IsNotExist(err) {
		t.Errorf("Duration stats file not created: %s", durationPath)
	}

	// Retrieve patterns
	stats, err := store.GetWorkTypeStats(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to get work type stats: %v", err)
	}
	if stats.TotalRuns != 5 {
		t.Errorf("Expected TotalRuns=5, got %d", stats.TotalRuns)
	}

	// Clear patterns
	err = store.ClearPatterns(ctx)
	if err != nil {
		t.Fatalf("Failed to clear patterns: %v", err)
	}

	// Verify patterns are cleared
	_, err = store.GetWorkTypeStats(ctx, "implementation", "backend")
	if err == nil {
		t.Error("Expected error after clearing patterns, got nil")
	}
}

// Test miner with real proof-of-work files
func TestMinerWithRealProofOfWorks(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "miner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create proof-of-work directory
	proofDir := filepath.Join(tmpDir, "proof-of-work")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatalf("Failed to create proof-of-work directory: %v", err)
	}

	// Create sample proof-of-work artifacts
	artifact1Dir := filepath.Join(proofDir, "20260309-120000")
	if err := os.MkdirAll(artifact1Dir, 0755); err != nil {
		t.Fatalf("Failed to create artifact directory: %v", err)
	}

	proofOfWork1 := `{
		"version": "1.0.0",
		"task_id": "task-1",
		"session_id": "session-1",
		"work_item_id": "ITEM-1",
		"title": "Test Task 1",
		"objective": "Test objective",
		"result": "completed",
		"started_at": "2026-03-09T12:00:00Z",
		"completed_at": "2026-03-09T12:01:00Z",
		"duration": 60000000000,
		"model_used": "implementation:real",
		"agent_role": "factory",
		"files_changed": ["file1.go", "file2.go"],
		"work_type": "implementation",
		"work_domain": "backend"
	}`

	if err := os.WriteFile(filepath.Join(artifact1Dir, "proof-of-work.json"), []byte(proofOfWork1), 0644); err != nil {
		t.Fatalf("Failed to write proof-of-work file: %v", err)
	}

	// Create miner
	store := NewInMemoryPatternStore()
	miner := NewMiner(tmpDir, store)

	// Mine proof-of-works
	result, err := miner.MineProofOfWorks(ctx)
	if err != nil {
		t.Fatalf("Failed to mine proof-of-works: %v", err)
	}

	if result.ArtifactsFound != 1 {
		t.Errorf("Expected 1 artifact found, got %d", result.ArtifactsFound)
	}
	if result.ArtifactsMined != 1 {
		t.Errorf("Expected 1 artifact mined, got %d", result.ArtifactsMined)
	}
	if result.PatternsExtracted < 1 {
		t.Errorf("Expected at least 1 pattern extracted, got %d", result.PatternsExtracted)
	}

	// Verify patterns were stored
	stats, err := store.GetWorkTypeStats(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to get work type stats: %v", err)
	}
	if stats.TotalRuns != 1 {
		t.Errorf("Expected 1 run, got %d", stats.TotalRuns)
	}
}

// Test full workflow
func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "workflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create proof-of-work directory
	proofDir := filepath.Join(tmpDir, "proof-of-work")
	if err := os.MkdirAll(proofDir, 0755); err != nil {
		t.Fatalf("Failed to create proof-of-work directory: %v", err)
	}

	// Create multiple proof-of-work artifacts
	for i := 0; i < 5; i++ {
		artifactDir := filepath.Join(proofDir, fmt.Sprintf("20260309-%02d0000", 12+i))
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			t.Fatalf("Failed to create artifact directory: %v", err)
		}

		success := i < 4 // First 4 succeed, last 1 fails
		result := "completed"
		if !success {
			result = "failed"
		}

		proofOfWork := fmt.Sprintf(`{
			"version": "1.0.0",
			"task_id": "task-%d",
			"session_id": "session-1",
			"work_item_id": "ITEM-%d",
			"title": "Test Task %d",
			"objective": "Test objective",
			"result": "%s",
			"started_at": "2026-03-09T%02d:00:00Z",
			"completed_at": "2026-03-09T%02d:01:00Z",
			"duration": 60000000000,
			"model_used": "implementation:real",
			"agent_role": "factory",
			"files_changed": ["file1.go", "file2.go"],
			"work_type": "implementation",
			"work_domain": "backend"
		}`, i, i, i, result, 12+i, 12+i)

		if err := os.WriteFile(filepath.Join(artifactDir, "proof-of-work.json"), []byte(proofOfWork), 0644); err != nil {
			t.Fatalf("Failed to write proof-of-work file: %v", err)
		}
	}

	// Create full pipeline
	patternStoreDir := filepath.Join(tmpDir, "patterns")
	patternStore, err := NewJSONPatternStore(patternStoreDir)
	if err != nil {
		t.Fatalf("Failed to create pattern store: %v", err)
	}

	miner := NewMiner(tmpDir, patternStore)
	recommender := NewRecommender(patternStore, 3)

	// Step 1: Mine proof-of-works
	miningResult, err := miner.MineProofOfWorks(ctx)
	if err != nil {
		t.Fatalf("Failed to mine proof-of-works: %v", err)
	}

	t.Logf("Mining result: found=%d, mined=%d, patterns=%d",
		miningResult.ArtifactsFound, miningResult.ArtifactsMined, miningResult.PatternsExtracted)

	// Step 2: Get recommendations
	templateRec, configRec, err := recommender.RecommendAll(ctx, "implementation", "backend")
	if err != nil {
		t.Fatalf("Failed to get recommendations: %v", err)
	}

	t.Logf("Template recommendation: %s (confidence: %.2f)", templateRec.TemplateName, templateRec.Confidence)
	t.Logf("Configuration recommendation: timeout=%ds, retries=%d (confidence: %.2f)",
		configRec.TimeoutSeconds, configRec.MaxRetries, configRec.Confidence)

	// Step 3: Get pattern analysis
	analysis, err := recommender.PatternAnalysis(ctx)
	if err != nil {
		t.Fatalf("Failed to get pattern analysis: %v", err)
	}

	t.Logf("\n%s", analysis.FormatAnalysis())

	// Verify results
	if templateRec.TemplateName != "implementation:real" {
		t.Errorf("Expected implementation:real template, got %s", templateRec.TemplateName)
	}
	if templateRec.SuccessRate != 0.8 {
		t.Errorf("Expected SuccessRate=0.8, got %f", templateRec.SuccessRate)
	}
	if analysis.TotalExecutions != 5 {
		t.Errorf("Expected 5 executions, got %d", analysis.TotalExecutions)
	}
}
