// Package analyzer provides advanced Block 2 features (confidence calibration, multi-model comparison, streaming, feedback loop).
package analyzer

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ============================================================================
// CONFIDENCE CALIBRATION
// ============================================================================

// ConfidenceCalibrator maintains historical confidence data to improve predictions.
type ConfidenceCalibrator struct {
	history   map[string][]ConfidenceRecord // work_type → records
	mu         sync.RWMutex
	calibrated  bool
	lastUpdate  time.Time
}

// ConfidenceRecord tracks past confidence vs actual outcomes.
type ConfidenceRecord struct {
	WorkType       string    `json:"work_type"`
	PredictedConf float64   `json:"predicted_conf"`
	ActualSuccess  bool       `json:"actual_success"`
	Timestamp      time.Time  `json:"timestamp"`
	ModelVersion   string     `json:"model_version"`
	Notes          string     `json:"notes,omitempty"`
}

// CalibrationStats provides calibration metrics.
type CalibrationStats struct {
	WorkType         string  `json:"work_type"`
	TotalAnalyses   int     `json:"total_analyses"`
	SuccessRate      float64 `json:"success_rate"`
	AvgConfidence    float64 `json:"avg_confidence"`
	CalibrationError float64 `json:"calibration_error"` // |predicted - actual|
	IsOverconfident bool    `json:"is_overconfident"` // Predicted > Actual
	IsUnderconfident bool    `json:"is_underconfident"` // Predicted < Actual
}

// NewConfidenceCalibrator creates a new calibrator.
func NewConfidenceCalibrator() *ConfidenceCalibrator {
	return &ConfidenceCalibrator{
		history:  make(map[string][]ConfidenceRecord),
		lastUpdate: time.Now(),
	}
}

// RecordResult records a confidence prediction and actual outcome.
func (c *ConfidenceCalibrator) RecordResult(workType string, predictedConf float64, actualSuccess bool, modelVersion string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record := ConfidenceRecord{
		WorkType:      workType,
		PredictedConf: predictedConf,
		ActualSuccess: actualSuccess,
		Timestamp:     time.Now(),
		ModelVersion:  modelVersion,
	}

	c.history[workType] = append(c.history[workType], record)
	c.calibrated = true
	c.lastUpdate = time.Now()

	// Keep only last 100 records per work type to avoid unbounded growth
	if len(c.history[workType]) > 100 {
		c.history[workType] = c.history[workType][len(c.history[workType])-100:]
	}
}

// GetCalibrationStats returns calibration statistics for a work type.
func (c *ConfidenceCalibrator) GetCalibrationStats(workType string) (*CalibrationStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	records, exists := c.history[workType]
	if !exists || len(records) == 0 {
		return nil, fmt.Errorf("no calibration data for work type: %s", workType)
	}

	stats := &CalibrationStats{
		WorkType: workType,
	}

	var totalConf float64
	var successCount int
	var errorSum float64

	for _, record := range records {
		totalConf += record.PredictedConf
		if record.ActualSuccess {
			successCount++
		}

		// Calibration error: difference between predicted confidence and actual outcome (1.0 or 0.0)
		actual := 0.0
		if record.ActualSuccess {
			actual = 1.0
		}
		errorSum += math.Abs(record.PredictedConf - actual)
	}

	stats.TotalAnalyses = len(records)
	stats.SuccessRate = float64(successCount) / float64(len(records))
	stats.AvgConfidence = totalConf / float64(len(records))
	stats.CalibrationError = errorSum / float64(len(records))

	// Determine if over/under confident
	if stats.CalibrationError > 0.2 {
		stats.IsOverconfident = stats.AvgConfidence > stats.SuccessRate
		stats.IsUnderconfident = stats.AvgConfidence < stats.SuccessRate
	}

	return stats, nil
}

// CalibrateConfidence adjusts a predicted confidence based on historical data.
// Returns adjusted confidence and whether calibration was applied.
func (c *ConfidenceCalibrator) CalibrateConfidence(workType string, predictedConf float64) (float64, bool) {
	stats, err := c.GetCalibrationStats(workType)
	if err != nil {
		// No calibration data, return as-is
		return predictedConf, false
	}

	// If overconfident, reduce predicted confidence
	// If underconfident, increase predicted confidence
	adjustment := 0.0

	if stats.IsOverconfident && stats.CalibrationError > 0.1 {
		// Reduce confidence by calibration error amount (capped)
		adjustment = -math.Min(stats.CalibrationError, 0.3)
	} else if stats.IsUnderconfident && stats.CalibrationError > 0.1 {
		// Increase confidence by calibration error amount (capped)
		adjustment = math.Min(stats.CalibrationError, 0.3)
	}

	adjusted := predictedConf + adjustment

	// Clamp to [0.0, 1.0]
	adjusted = math.Max(0.0, math.Min(1.0, adjusted))

	return adjusted, math.Abs(adjustment) > 0.01 // Only return true if meaningful adjustment
}

// ============================================================================
// MULTI-MODEL COMPARISON
// ============================================================================

// ModelComparison compares analysis results from multiple models.
type ModelComparison struct {
	WorkItemID     string                         `json:"work_item_id"`
	Comparisons    []*ModelAnalysisComparison     `json:"comparisons"`
	Summary        *ComparisonSummary            `json:"summary"`
	CreatedAt      time.Time                     `json:"created_at"`
	RecommendedModel string                       `json:"recommended_model,omitempty"`
}

// ModelAnalysisComparison represents one model's analysis.
type ModelAnalysisComparison struct {
	Model          string                       `json:"model"`
	AnalysisResult *contracts.AnalysisResult   `json:"analysis_result"`
	Confidence     float64                      `json:"confidence"`
	TaskCount      int                          `json:"task_count"`
	CostUSD        float64                      `json:"cost_usd"`
	TokenUsage     *TokenUsage                  `json:"token_usage,omitempty"`
	ExecutionTime  time.Duration                 `json:"execution_time"`
	QualityScore   float64                      `json:"quality_score"` // Composite quality score
}

// TokenUsage tracks LLM token consumption.
type TokenUsage struct {
	RequestTokens  int `json:"request_tokens"`
	ResponseTokens int `json:"response_tokens"`
	TotalTokens    int `json:"total_tokens"`
}

// ComparisonSummary provides meta-comparison across models.
type ComparisonSummary struct {
	ConsensusRate   float64             `json:"consensus_rate"`   // % agreement on work type
	AvgConfidence   float64             `json:"avg_confidence"`
	AvgTaskCount    float64             `json:"avg_task_count"`
	AvgCostUSD      float64             `json:"avg_cost_usd"`
	BestValueModel  string              `json:"best_value_model"` // Best cost/benefit ratio
	FastestModel    string              `json:"fastest_model"`
	MostConfident   string              `json:"most_confident"`
	Disagreements   []*FieldDisagreement `json:"disagreements"` // Where models disagree
}

// FieldDisagreement represents a field where models disagree.
type FieldDisagreement struct {
	Field      string                `json:"field"`       // e.g., "work_type", "priority"
	Values     []ModelFieldValue    `json:"values"`      // Different values from each model
	Consensus  string                `json:"consensus"`   // Most common value
	Confidence float64               `json:"confidence"`  // Confidence in consensus (0-1)
}

// ModelFieldValue represents a model's value for a field.
type ModelFieldValue struct {
	Model     string `json:"model"`
	Value     string `json:"value"`
	Confidence float64 `json:"confidence"`
}

// CompareModels compares analysis results from multiple models.
func CompareModels(workItemID string, results []*ModelAnalysisComparison) *ModelComparison {
	if len(results) == 0 {
		return &ModelComparison{WorkItemID: workItemID}
	}

	// Calculate quality scores for each model
	for _, comp := range results {
		comp.QualityScore = calculateQualityScore(comp)
	}

	summary := calculateComparisonSummary(results)

	// Recommend model based on best quality score
	recommendedModel := ""
	maxQuality := -1.0
	for _, comp := range results {
		if comp.QualityScore > maxQuality {
			maxQuality = comp.QualityScore
			recommendedModel = comp.Model
		}
	}

	return &ModelComparison{
		WorkItemID:      workItemID,
		Comparisons:     results,
		Summary:         summary,
		CreatedAt:        time.Now(),
		RecommendedModel: recommendedModel,
	}
}

// calculateQualityScore computes a composite quality score for a model's analysis.
func calculateQualityScore(comp *ModelAnalysisComparison) float64 {
	if comp.AnalysisResult == nil {
		return 0.0
	}

	score := 0.0

	// Confidence contribution (0-40 points)
	score += comp.Confidence * 40.0

	// Reasonable task count (0-20 points)
	// Too few tasks = incomplete, too many = over-engineering
	taskScore := 20.0
	if comp.TaskCount < 1 {
		taskScore = 5.0 // No tasks generated
	} else if comp.TaskCount > 10 {
		taskScore = 10.0 // Too many tasks
	}
	score += float64(taskScore)

	// Cost efficiency (0-20 points)
	// Lower cost is better (normalized against $10 max)
	costScore := 20.0
	if comp.CostUSD < 5.0 {
		costScore = 20.0 // Excellent
	} else if comp.CostUSD < 10.0 {
		costScore = 15.0 // Good
	} else if comp.CostUSD < 20.0 {
		costScore = 10.0 // Acceptable
	} else {
		costScore = 5.0 // Expensive
	}
	score += float64(costScore)

	// Execution time (0-20 points)
	// Faster is better (normalized against 60s max)
	timeScore := 20.0
	timeSecs := comp.ExecutionTime.Seconds()
	if timeSecs < 10.0 {
		timeScore = 20.0 // Excellent
	} else if timeSecs < 30.0 {
		timeScore = 15.0 // Good
	} else if timeSecs < 60.0 {
		timeScore = 10.0 // Acceptable
	} else {
		timeScore = 5.0 // Slow
	}
	score += float64(timeScore)

	return score // Max 100 points
}

// calculateComparisonSummary computes meta-comparison statistics.
func calculateComparisonSummary(comparisons []*ModelAnalysisComparison) *ComparisonSummary {
	if len(comparisons) == 0 {
		return &ComparisonSummary{}
	}

	summary := &ComparisonSummary{}

	var totalConf, totalTasks, totalCost float64
	bestValueModel, fastestModel, mostConfident := "", "", ""
	maxQuality, minTime, maxConf := -1.0, 24*time.Hour, -1.0

	// Track field values for disagreement analysis
	fieldValues := make(map[string][]ModelFieldValue)

	for _, comp := range comparisons {
		totalConf += comp.Confidence
		totalTasks += float64(comp.TaskCount)
		totalCost += comp.CostUSD

		if comp.QualityScore > maxQuality {
			maxQuality = comp.QualityScore
			bestValueModel = comp.Model
		}

		if comp.ExecutionTime < minTime {
			minTime = comp.ExecutionTime
			fastestModel = comp.Model
		}

		if comp.Confidence > maxConf {
			maxConf = comp.Confidence
			mostConfident = comp.Model
		}

		// Collect field values for disagreement analysis
		if comp.AnalysisResult != nil {
			fieldValues["work_type"] = append(fieldValues["work_type"], ModelFieldValue{
				Model:     comp.Model,
				Value:     string(comp.AnalysisResult.WorkItem.WorkType),
				Confidence: comp.Confidence,
			})
			fieldValues["priority"] = append(fieldValues["priority"], ModelFieldValue{
				Model:     comp.Model,
				Value:     string(comp.AnalysisResult.WorkItem.Priority),
				Confidence: comp.Confidence,
			})
		}
	}

	// Calculate averages
	n := float64(len(comparisons))
	summary.AvgConfidence = totalConf / n
	summary.AvgTaskCount = totalTasks / n
	summary.AvgCostUSD = totalCost / n

	summary.BestValueModel = bestValueModel
	summary.FastestModel = fastestModel
	summary.MostConfident = mostConfident

	// Calculate consensus rate and disagreements
	summary.ConsensusRate = calculateConsensusRate(fieldValues)
	summary.Disagreements = findFieldDisagreements(fieldValues)

	return summary
}

// calculateConsensusRate computes overall agreement rate across all fields.
func calculateConsensusRate(fieldValues map[string][]ModelFieldValue) float64 {
	if len(fieldValues) == 0 {
		return 0.0
	}

	totalAgreements := 0
	totalFields := len(fieldValues)

	for _, values := range fieldValues {
		if len(values) == 0 {
			continue
		}

		// Count how many models agree with the most common value
		valueCounts := make(map[string]int)
		for _, v := range values {
			valueCounts[v.Value]++
		}

		maxCount := 0
		for _, count := range valueCounts {
			if count > maxCount {
				maxCount = count
			}
		}

		totalAgreements += maxCount
	}

	// Average agreement across all fields
	return float64(totalAgreements) / float64(totalFields)
}

// findFieldDisagreement identifies fields where models disagree.
func findFieldDisagreements(fieldValues map[string][]ModelFieldValue) []*FieldDisagreement {
	disagreements := []*FieldDisagreement{}

	for field, values := range fieldValues {
		if len(values) < 2 {
			continue // No disagreement if only one model
		}

		// Count value frequencies
		valueCounts := make(map[string]int)
		totalConf := 0.0
		for _, v := range values {
			valueCounts[v.Value]++
			totalConf += v.Confidence
		}

		// Find consensus (most common value)
		consensus := ""
		maxCount := 0
		for value, count := range valueCounts {
			if count > maxCount {
				maxCount = count
				consensus = value
			}
		}

		// If not all models agree, it's a disagreement
		if maxCount < len(values) {
			confidence := float64(maxCount) / float64(len(values))
			disagreements = append(disagreements, &FieldDisagreement{
				Field:      field,
				Values:     values,
				Consensus:  consensus,
				Confidence: confidence,
			})
		}
	}

	return disagreements
}

// ============================================================================
// REAL-TIME STREAMING
// ============================================================================

// AnalysisStream represents a streaming analysis result.
type AnalysisStream struct {
	WorkItemID     string              `json:"work_item_id"`
	StreamID        string              `json:"stream_id"`
	StartedAt       time.Time           `json:"started_at"`
	CurrentStage    string              `json:"current_stage,omitempty"`
	StageProgress   *StageProgress      `json:"stage_progress,omitempty"`
	CompletedStages []string            `json:"completed_stages"`
	Errors          []string            `json:"errors,omitempty"`
	Status          string              `json:"status"` // streaming/complete/failed/cancelled
	mu              sync.Mutex
	cancelCh        chan struct{}
}

// StageProgress tracks progress within a single stage.
type StageProgress struct {
	Stage         Stage     `json:"stage"`
	Percent       float64   `json:"percent"`        // 0.0-1.0
	Message       string    `json:"message"`        // Human-readable status
	TokensSoFar  int       `json:"tokens_so_far"` // For streaming LLM output
	EstimatedTimeRemaining time.Duration `json:"estimated_time_remaining,omitempty"`
}

// StreamEvent represents an event in the analysis stream.
type StreamEvent struct {
	EventType   StreamEventType `json:"event_type"`
	Timestamp   time.Time     `json:"timestamp"`
	StreamID    string        `json:"stream_id"`
	Data        interface{}   `json:"data,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// StreamEventType types of events that can occur during streaming.
type StreamEventType string

const (
	StreamEventStageStart    StreamEventType = "stage_start"
	StreamEventStageProgress StreamEventType = "stage_progress"
	StreamEventStageComplete StreamEventType = "stage_complete"
	StreamEventTokenEmit    StreamEventType = "token_emit"
	StreamEventComplete      StreamEventType = "complete"
	StreamEventError        StreamEventType = "error"
	StreamEventCancelled     StreamEventType = "cancelled"
)

// StreamCallback is called for each event during streaming analysis.
type StreamCallback func(event *StreamEvent) error

// NewAnalysisStream creates a new analysis stream.
func NewAnalysisStream(workItemID string) *AnalysisStream {
	streamID := fmt.Sprintf("stream-%d-%s", time.Now().UnixNano(), workItemID)
	return &AnalysisStream{
		WorkItemID:      workItemID,
		StreamID:       streamID,
		StartedAt:      time.Now(),
		CompletedStages: []string{},
		Status:          "streaming",
		cancelCh:       make(chan struct{}),
	}
}

// StreamAnalysis performs analysis with real-time streaming callbacks.
// This is a simplified version that works with IntentAnalyzer interface.
// For full streaming, integrate with specific analyzer implementations.
func StreamAnalysis(
	ctx context.Context,
	analyzer IntentAnalyzer,
	workItem *contracts.WorkItem,
	callback StreamCallback,
) (*contracts.AnalysisResult, error) {
	stream := NewAnalysisStream(workItem.ID)
	stream.mu.Lock()
	stream.Status = "streaming"
	stream.mu.Unlock()

	// Send start event
	if err := callback(&StreamEvent{
		EventType: "stream_start",
		Timestamp: time.Now(),
		StreamID:  stream.StreamID,
		Data:       stream,
	}); err != nil {
		return nil, fmt.Errorf("callback failed on start: %w", err)
	}

	// Execute analysis
	result, err := analyzer.Analyze(ctx, workItem)

	if err != nil {
		stream.mu.Lock()
		stream.Errors = append(stream.Errors, err.Error())
		stream.Status = "failed"
		stream.mu.Unlock()

		_ = callback(&StreamEvent{
			EventType: StreamEventError,
			Timestamp: time.Now(),
			StreamID:  stream.StreamID,
			Error:      err.Error(),
		})
		return nil, err
	}

	// Send completion event
	stream.mu.Lock()
	stream.Status = "complete"
	stream.CurrentStage = ""
	stream.mu.Unlock()

	_ = callback(&StreamEvent{
		EventType: StreamEventComplete,
		Timestamp: time.Now(),
		StreamID:  stream.StreamID,
	})

	return result, nil
}

// Cancel cancels the streaming analysis.
func (s *AnalysisStream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status != "streaming" {
		return
	}

	s.Status = "cancelled"
	close(s.cancelCh)
}

// GetProgress returns current progress.
func (s *AnalysisStream) GetProgress() *AnalysisStream {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return copy to avoid race conditions
	return &AnalysisStream{
		WorkItemID:      s.WorkItemID,
		StreamID:       s.StreamID,
		StartedAt:       s.StartedAt,
		CurrentStage:    s.CurrentStage,
		StageProgress:    s.StageProgress,
		CompletedStages: s.CompletedStages,
		Errors:          s.Errors,
		Status:          s.Status,
	}
}

// ============================================================================
// FEEDBACK LOOP
// ============================================================================

// AnalysisFeedback tracks feedback on analysis results.
type AnalysisFeedback struct {
	AnalysisID     string    `json:"analysis_id"`
	WorkItemID     string    `json:"work_item_id"`
	FeedbackType    FeedbackType `json:"feedback_type"`
	Rating          int       `json:"rating"`          // 1-5 stars
	Comments        string    `json:"comments"`        // Free-form feedback
	CorrectedFields []string  `json:"corrected_fields"` // Fields that were corrected
	SubmittedAt     time.Time `json:"submitted_at"`
	SubmittedBy     string    `json:"submitted_by"`    // user/system
}

// FeedbackType type of feedback.
type FeedbackType string

const (
	FeedbackTypeTaskBreakdown FeedbackType = "task_breakdown"
	FeedbackTypeWorkType    FeedbackType = "work_type"
	FeedbackTypeConfidence  FeedbackType = "confidence"
	FeedbackTypeRequirements FeedbackType = "requirements"
	FeedbackTypeGeneral     FeedbackType = "general"
)

// FeedbackStore persists and retrieves analysis feedback.
type FeedbackStore interface {
	Store(ctx context.Context, feedback *AnalysisFeedback) error
	GetFeedback(ctx context.Context, analysisID string) ([]*AnalysisFeedback, error)
	GetAverageRating(ctx context.Context, workType string) (float64, error)
}

// MemoryFeedbackStore implements FeedbackStore in memory.
type MemoryFeedbackStore struct {
	feedback map[string][]*AnalysisFeedback // analysis_id -> feedback
	mu        sync.RWMutex
}

// NewMemoryFeedbackStore creates a new in-memory feedback store.
func NewMemoryFeedbackStore() *MemoryFeedbackStore {
	return &MemoryFeedbackStore{
		feedback: make(map[string][]*AnalysisFeedback),
	}
}

// Store stores feedback for an analysis.
func (s *MemoryFeedbackStore) Store(ctx context.Context, feedback *AnalysisFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.feedback[feedback.AnalysisID] = append(s.feedback[feedback.AnalysisID], feedback)
	return nil
}

// GetFeedback retrieves all feedback for an analysis.
func (s *MemoryFeedbackStore) GetFeedback(ctx context.Context, analysisID string) ([]*AnalysisFeedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fb, exists := s.feedback[analysisID]
	if !exists {
		return []*AnalysisFeedback{}, nil
	}

	// Return copy
	result := make([]*AnalysisFeedback, len(fb))
	copy(result, fb)
	return result, nil
}

// GetAverageRating calculates average rating for a work type.
func (s *MemoryFeedbackStore) GetAverageRating(ctx context.Context, workType string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// This would need analysis ID to work type mapping
	// For now, return average of all feedback
	var total, count float64

	for _, fbList := range s.feedback {
		for _, fb := range fbList {
			if fb.Rating > 0 {
				total += float64(fb.Rating)
				count++
			}
		}
	}

	if count == 0 {
		return 0.0, nil
	}

	return total / count, nil
}

// ApplyFeedback learns from feedback to improve future analyses.
func ApplyFeedback(feedback *AnalysisFeedback, calibrator *ConfidenceCalibrator) {
	// If feedback is about confidence, record outcome
	if feedback.FeedbackType == FeedbackTypeConfidence && feedback.Rating > 0 {
		// Rating > 3 means confidence was good (actual success)
		// Rating <= 3 means confidence was too high/low (actual failure)
		actualSuccess := feedback.Rating > 3

		// This would need to know the original predicted confidence
		// For now, we store it and calibrator can use it
		_ = actualSuccess
	}

	// Could also adjust prompt templates based on corrected fields
	// e.g., if work_type was frequently corrected, adjust classification prompt
}

// ============================================================================
// CACHING
// ============================================================================

// AnalysisCache caches analysis results to avoid redundant work.
type AnalysisCache struct {
	cache    map[string]*CachedAnalysis
	mu        sync.RWMutex
	ttl       time.Duration // Time-to-live for cache entries
}

// CachedAnalysis represents a cached analysis result.
type CachedAnalysis struct {
	AnalysisResult *contracts.AnalysisResult
	CachedAt       time.Time
	AccessCount    int
	LastAccessed   time.Time
	HitScore       float64 // For cache eviction (lower = more hits)
}

// NewAnalysisCache creates a new analysis cache.
func NewAnalysisCache(ttl time.Duration) *AnalysisCache {
	return &AnalysisCache{
		cache: make(map[string]*CachedAnalysis),
		ttl:    ttl,
	}
}

// GenerateCacheKey generates a cache key for a work item.
func GenerateCacheKey(workItem *contracts.WorkItem) string {
	// Use work item fields for cache key
	return fmt.Sprintf("%s:%s:%s:%s",
		workItem.WorkType,
		workItem.WorkDomain,
		workItem.Priority,
		computeContentHash(workItem.Body),
	)
}

// computeContentHash creates a simple hash of content for cache key.
func computeContentHash(content string) string {
	// Simple hash: first 100 chars + length
	if len(content) > 100 {
		content = content[:100]
	}
	return fmt.Sprintf("%s-%d", content, len(content))
}

// Get retrieves cached analysis if available and not expired.
func (c *AnalysisCache) Get(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, bool) {
	key := GenerateCacheKey(workItem)

	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(cached.CachedAt) > c.ttl {
		return nil, false
	}

	// Update access stats
	c.mu.RUnlock()
	c.mu.Lock()
	cached.AccessCount++
	cached.LastAccessed = time.Now()
	c.cache[key] = cached
	c.mu.RLock()

	return cached.AnalysisResult, true
}

// Put stores analysis result in cache.
func (c *AnalysisCache) Put(ctx context.Context, workItem *contracts.WorkItem, result *contracts.AnalysisResult) {
	key := GenerateCacheKey(workItem)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &CachedAnalysis{
		AnalysisResult: result,
		CachedAt:      time.Now(),
		AccessCount:    0,
		LastAccessed:   time.Now(),
		HitScore:       1.0, // New entry
	}
}

// EvictExpired removes expired entries from cache.
func (c *AnalysisCache) EvictExpired(ctx context.Context) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	evicted := 0
	now := time.Now()

	for key, cached := range c.cache {
		if now.Sub(cached.CachedAt) > c.ttl {
			delete(c.cache, key)
			evicted++
		}
	}

	return evicted
}

// GetStats returns cache statistics.
func (c *AnalysisCache) GetStats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalEntries := len(c.cache)
	totalAccesses := 0
	expired := 0
	now := time.Now()

	for _, cached := range c.cache {
		totalAccesses += cached.AccessCount
		if now.Sub(cached.CachedAt) > c.ttl {
			expired++
		}
	}

	return &CacheStats{
		TotalEntries:  totalEntries,
		TotalAccesses:  totalAccesses,
		ExpiredCount:   expired,
		ActiveEntries:  totalEntries - expired,
	}
}

// CacheStats provides cache statistics.
type CacheStats struct {
	TotalEntries  int `json:"total_entries"`
	TotalAccesses int `json:"total_accesses"`
	ExpiredCount  int `json:"expired_count"`
	ActiveEntries int `json:"active_entries"`
}

// ============================================================================
// RISK PREDICTION
// ============================================================================

// RiskPredictor predicts risks for work items based on historical data.
type RiskPredictor struct {
	historicalRisks map[string][]string // work_type → historical risks
	mu              sync.RWMutex
}

// NewRiskPredictor creates a new risk predictor.
func NewRiskPredictor() *RiskPredictor {
	return &RiskPredictor{
		historicalRisks: make(map[string][]string),
	}
}

// RecordRiskOutcome records a risk that materialized during execution.
func (r *RiskPredictor) RecordRiskOutcome(workType string, risk string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.historicalRisks[workType] = append(r.historicalRisks[workType], risk)

	// Keep only last 50 risks per work type
	if len(r.historicalRisks[workType]) > 50 {
		r.historicalRisks[workType] = r.historicalRisks[workType][len(r.historicalRisks[workType])-50:]
	}
}

// PredictRisks predicts likely risks for a work item.
func (r *RiskPredictor) PredictRisks(workItem *contracts.WorkItem) []*RiskPrediction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	predictions := []*RiskPrediction{}

	// Check historical risks for this work type
	risks, exists := r.historicalRisks[string(workItem.WorkType)]
	if exists {
		// Count frequency of each risk
		riskCounts := make(map[string]int)
		for _, risk := range risks {
			riskCounts[risk]++
		}

		// Convert to predictions
		for risk, count := range riskCounts {
			probability := float64(count) / float64(len(risks))

			if probability >= 0.3 { // Only predict if 30%+ historical occurrence
				predictions = append(predictions, &RiskPrediction{
					Risk:          risk,
					Probability:    probability,
					Category:       inferRiskCategory(risk),
					Mitigation:     getStandardMitigation(risk),
				})
			}
		}
	}

	return predictions
}

// RiskPrediction represents a predicted risk with mitigation.
type RiskPrediction struct {
	Risk       string  `json:"risk"`
	Probability float64 `json:"probability"` // 0.0-1.0
	Category    string  `json:"category"`
	Mitigation  string  `json:"mitigation"`
}

func inferRiskCategory(risk string) string {
	riskLower := strings.ToLower(risk)

	if strings.Contains(riskLower, "complexity") || strings.Contains(riskLower, "refactor") {
		return "technical"
	}
	if strings.Contains(riskLower, "time") || strings.Contains(riskLower, "schedule") {
		return "operational"
	}
	if strings.Contains(riskLower, "api") || strings.Contains(riskLower, "dependency") {
		return "dependency"
	}

	return "unknown"
}

func getStandardMitigation(risk string) string {
	riskLower := strings.ToLower(risk)

	if strings.Contains(riskLower, "complexity") {
		return "Create detailed design document and review before implementation"
	}
	if strings.Contains(riskLower, "time") || strings.Contains(riskLower, "schedule") {
		return "Break into smaller tasks, add buffer time, monitor progress closely"
	}
	if strings.Contains(riskLower, "api") {
		return "Test API contract, implement retries and fallback, monitor SLA"
	}
	if strings.Contains(riskLower, "dependency") {
		return "Validate dependency availability, implement circuit breakers, plan fallbacks"
	}

	return "Monitor and address if risk materializes"
}
