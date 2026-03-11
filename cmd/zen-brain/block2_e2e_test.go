package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestBlock2_AnalyzeWithHistory tests A001 and A002: rich output + history persistence
func TestBlock2_AnalyzeWithHistory(t *testing.T) {
	// Create temporary history store
	tmpDir := t.TempDir()
	historyStore, err := analyzer.NewFileAnalysisStore(tmpDir)
	if err != nil {
		t.Fatalf("Create history store: %v", err)
	}

	// Create simple analyzer with history
	anal := &testAnalyzer{
		historyStore: historyStore,
	}

	// Create work item
	workItem := &contracts.WorkItem{
		ID:       "TEST-123",
		Title:    "Implement feature X",
		Body:     "Description of feature X",
		WorkType: contracts.WorkTypeImplementation,
		Source: contracts.SourceMetadata{
			System:   "jira",
			IssueKey: "PROJ-456",
		},
	}

	ctx := context.Background()

	// Analyze
	result, err := anal.Analyze(ctx, workItem)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Verify basic result
	if result.WorkItem.ID != workItem.ID {
		t.Errorf("Expected work item ID %s, got %s", workItem.ID, result.WorkItem.ID)
	}

	// Verify audit enrichment (A003)
	if result.AnalyzedAt.IsZero() {
		t.Error("Expected AnalyzedAt to be set")
	}
	if result.AnalyzerVersion == "" {
		t.Error("Expected AnalyzerVersion to be set")
	}
	if result.WorkItemSnapshot == nil {
		t.Fatal("Expected WorkItemSnapshot to be set")
	}
	if result.WorkItemSnapshot.SourceKey != "PROJ-456" {
		t.Errorf("Expected SourceKey 'PROJ-456', got %s", result.WorkItemSnapshot.SourceKey)
	}

	// Generate rich output (A001)
	richResult := analyzer.EnrichForRichAnalysis(result, workItem)

	// Verify rich output fields
	if richResult.ExecutiveSummary == "" {
		t.Error("Expected ExecutiveSummary to be generated")
	}
	if richResult.TechnicalSummary == "" {
		t.Error("Expected TechnicalSummary to be generated")
	}
	if richResult.ReplayID == "" {
		t.Error("Expected ReplayID to be set")
	}
	if richResult.CorrelationID == "" {
		t.Error("Expected CorrelationID to be set")
	}
	if richResult.AuditTrail == nil {
		t.Fatal("Expected AuditTrail to be set")
	}

	// Verify Jira linkage (A003)
	if richResult.AuditTrail.JiraKey != "PROJ-456" {
		t.Errorf("Expected JiraKey 'PROJ-456', got %s", richResult.AuditTrail.JiraKey)
	}
	if richResult.AuditTrail.WorkItemSource != "jira" {
		t.Errorf("Expected WorkItemSource 'jira', got %s", richResult.AuditTrail.WorkItemSource)
	}
	if richResult.AuditTrail.AnalysisID == "" {
		t.Error("Expected AnalysisID to be set")
	}

	// Verify history persistence (A002)
	history, err := historyStore.GetHistory(ctx, workItem.ID)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}
	if history[0].WorkItem.ID != workItem.ID {
		t.Errorf("Expected history work item ID %s, got %s", workItem.ID, history[0].WorkItem.ID)
	}

	// Verify risk assessment
	if richResult.RiskAssessment == nil {
		t.Fatal("Expected RiskAssessment to be generated")
	}
	if richResult.RiskAssessment.OverallRisk == "" {
		t.Error("Expected OverallRisk to be set")
	}

	// Verify action items
	if len(richResult.ActionItems) == 0 {
		t.Error("Expected ActionItems to be generated")
	}
}

// TestBlock2_HistoryReplay tests A002: history retrieval and replayability
func TestBlock2_HistoryReplay(t *testing.T) {
	// Create temporary history store
	tmpDir := t.TempDir()
	historyStore, err := analyzer.NewFileAnalysisStore(tmpDir)
	if err != nil {
		t.Fatalf("Create history store: %v", err)
	}

	anal := &testAnalyzer{
		historyStore: historyStore,
	}

	workItem := &contracts.WorkItem{
		ID:       "TEST-456",
		Title:    "Fix bug Y",
		Body:     "Description of bug Y",
		WorkType: contracts.WorkTypeDebug,
		Source: contracts.SourceMetadata{
			System:   "jira",
			IssueKey: "PROJ-789",
		},
	}

	ctx := context.Background()

	// Analyze multiple times
	result1, err := anal.Analyze(ctx, workItem)
	if err != nil {
		t.Fatalf("Analyze 1: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Ensure different timestamp

	result2, err := anal.Analyze(ctx, workItem)
	if err != nil {
		t.Fatalf("Analyze 2: %v", err)
	}

	// Retrieve history
	history, err := historyStore.GetHistory(ctx, workItem.ID)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("Expected 2 history entries, got %d", len(history))
	}

	// Verify order (oldest first)
	if !history[0].AnalyzedAt.Before(history[1].AnalyzedAt) {
		t.Error("Expected history in chronological order")
	}

	// Verify different analysis IDs (may be same if analyzed in same second)
	rich1 := analyzer.EnrichForRichAnalysis(result1, workItem)
	rich2 := analyzer.EnrichForRichAnalysis(result2, workItem)

	// AnalysisID should include timestamp, so if analyses were at different times, IDs should differ
	// Note: In fast tests, they may be the same second, so we just verify the field is set
	if rich1.AuditTrail.AnalysisID == "" {
		t.Error("Expected AnalysisID to be set")
	}
	if rich2.AuditTrail.AnalysisID == "" {
		t.Error("Expected AnalysisID to be set")
	}

	// Test latest retrieval
	latest := history[len(history)-1]
	if latest.AnalyzedAt != result2.AnalyzedAt {
		t.Error("Expected latest to be result2")
	}
}

// TestBlock2_AuditTrail tests A003: complete audit trail with Jira linkage
func TestBlock2_AuditTrail(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:       "TEST-789",
		Title:    "Refactor component Z",
		Body:     "Description of refactor Z",
		WorkType: contracts.WorkTypeRefactor,
		Source: contracts.SourceMetadata{
			System:   "jira",
			IssueKey: "PROJ-999",
			Project:  "PROJECT",
			Reporter: "john.doe",
			Assignee: "jane.smith",
		},
	}

	// Create base result
	result := &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{
				ID:          "TASK-001",
				Title:       "Task 1",
				WorkItemID:  workItem.ID,
				SourceKey:   "PROJ-999",
				WorkType:    workItem.WorkType,
				Priority:    contracts.PriorityHigh,
				Objective:   "Refactor component",
				CreatedAt:   time.Now(),
			},
		},
		Confidence:       0.85,
		EstimatedTotalCostUSD: 5.50,
	}

	// Enrich for audit
	analyzer.EnrichForAudit(result, workItem, "zen-brain", "1.0")

	// Generate rich output with audit trail
	richResult := analyzer.EnrichForRichAnalysis(result, workItem)

	// Verify audit trail
	audit := richResult.AuditTrail
	if audit == nil {
		t.Fatal("Expected AuditTrail to be set")
	}

	// Check audit fields
	if audit.WorkItemID != workItem.ID {
		t.Errorf("Expected WorkItemID %s, got %s", workItem.ID, audit.WorkItemID)
	}
	if audit.JiraKey != "PROJ-999" {
		t.Errorf("Expected JiraKey 'PROJ-999', got %s", audit.JiraKey)
	}
	if audit.WorkItemSource != "jira" {
		t.Errorf("Expected WorkItemSource 'jira', got %s", audit.WorkItemSource)
	}
	if audit.AnalysisID == "" {
		t.Error("Expected AnalysisID to be set")
	}

	// Verify chain of trust
	if len(audit.ChainOfTrust) == 0 {
		t.Error("Expected ChainOfTrust to have entries")
	}
	hasZenBrain := false
	for _, actor := range audit.ChainOfTrust {
		if actor == "zen-brain" {
			hasZenBrain = true
			break
		}
	}
	if !hasZenBrain {
		t.Error("Expected 'zen-brain' in ChainOfTrust")
	}

	// Verify analysis chain
	if len(audit.AnalysisChain) == 0 {
		t.Error("Expected AnalysisChain to have steps")
	}

	// Verify task chain
	if len(audit.TaskChain) == 0 {
		t.Error("Expected TaskChain to have task linkages")
	}

	// Verify Jira linkage
	if len(audit.JiraLinkage) == 0 {
		t.Error("Expected JiraLinkage to have correlations")
	}

	// Verify custody timestamps
	if audit.CustodyStart.IsZero() {
		t.Error("Expected CustodyStart to be set")
	}
	if audit.CustodyEnd.IsZero() {
		t.Error("Expected CustodyEnd to be set")
	}
	if audit.CustodyEnd.Before(audit.CustodyStart) {
		t.Error("Expected CustodyEnd >= CustodyStart")
	}
}

// TestBlock2_OperatorOutput tests A004: operator-facing summaries
func TestBlock2_OperatorOutput(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:       "TEST-OPERATOR",
		Title:    "Implement login feature",
		Body:     "Add OAuth2 login with Google and GitHub",
		WorkType: contracts.WorkTypeImplementation,
		Priority: contracts.PriorityHigh,
		Source: contracts.SourceMetadata{
			System:   "jira",
			IssueKey: "PROJ-OPERATOR",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{ID: "TASK-001", Title: "Setup OAuth2 provider", Priority: contracts.PriorityHigh},
			{ID: "TASK-002", Title: "Implement Google login", Priority: contracts.PriorityMedium},
			{ID: "TASK-003", Title: "Implement GitHub login", Priority: contracts.PriorityMedium},
			{ID: "TASK-004", Title: "Add tests", Priority: contracts.PriorityMedium},
		},
		Confidence:            0.92,
		EstimatedTotalCostUSD: 12.50,
	}

	analyzer.EnrichForAudit(result, workItem, "zen-brain", "1.0")
	richResult := analyzer.EnrichForRichAnalysis(result, workItem)

	// Verify executive summary
	if richResult.ExecutiveSummary == "" {
		t.Error("Expected ExecutiveSummary to be generated")
	}
	// Should mention key metrics
	if !containsSubstring(richResult.ExecutiveSummary, "task") {
		t.Error("Expected ExecutiveSummary to mention tasks")
	}

	// Verify technical summary
	if richResult.TechnicalSummary == "" {
		t.Error("Expected TechnicalSummary to be generated")
	}

	// Verify risk assessment
	if richResult.RiskAssessment == nil {
		t.Fatal("Expected RiskAssessment")
	}
	validRisks := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	if !validRisks[richResult.RiskAssessment.OverallRisk] {
		t.Errorf("Expected valid OverallRisk, got %s", richResult.RiskAssessment.OverallRisk)
	}

	// Verify action items
	if len(richResult.ActionItems) == 0 {
		t.Error("Expected ActionItems to be generated")
	}
	for i, item := range richResult.ActionItems {
		if item.ID == "" {
			t.Errorf("ActionItem %d: Expected ID to be set", i)
		}
		if item.Title == "" {
			t.Errorf("ActionItem %d: Expected Title to be set", i)
		}
		if item.Priority == "" {
			t.Errorf("ActionItem %d: Expected Priority to be set", i)
		}
	}

	// Verify task breakdown summary
	if len(richResult.BrainTaskSpecs) != 4 {
		t.Errorf("Expected 4 tasks, got %d", len(richResult.BrainTaskSpecs))
	}

	// Verify confidence is surfaced
	if richResult.Confidence < 0.9 || richResult.Confidence > 1.0 {
		t.Errorf("Expected confidence ~0.92, got %.2f", richResult.Confidence)
	}

	// Verify cost estimate is surfaced
	if richResult.EstimatedTotalCostUSD < 10.0 || richResult.EstimatedTotalCostUSD > 15.0 {
		t.Errorf("Expected cost ~$12.50, got $%.2f", richResult.EstimatedTotalCostUSD)
	}
}

// TestBlock2_HistoryDegradation tests behavior when HistoryStore is absent
func TestBlock2_HistoryDegradation(t *testing.T) {
	// Analyzer without history store
	anal := &testAnalyzer{
		historyStore: nil,
	}

	workItem := &contracts.WorkItem{
		ID:       "TEST-NO-HISTORY",
		Title:    "Test without history",
		WorkType: contracts.WorkTypeImplementation,
	}

	ctx := context.Background()

	// Should still analyze successfully
	result, err := anal.Analyze(ctx, workItem)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Verify basic result
	if result.WorkItem.ID != workItem.ID {
		t.Errorf("Expected work item ID %s, got %s", workItem.ID, result.WorkItem.ID)
	}

	// Verify audit enrichment still works
	if result.AnalyzedAt.IsZero() {
		t.Error("Expected AnalyzedAt to be set even without history")
	}

	// GetAnalysisHistory should return error
	_, err = anal.GetAnalysisHistory(ctx, workItem.ID)
	if err == nil {
		t.Error("Expected error when HistoryStore is not configured")
	}
}

// TestBlock2_JSONSerialization tests that rich output can be serialized/deserialized
func TestBlock2_JSONSerialization(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:       "TEST-JSON",
		Title:    "JSON serialization test",
		WorkType: contracts.WorkTypeImplementation,
		Source: contracts.SourceMetadata{
			System:   "jira",
			IssueKey: "PROJ-JSON",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:       workItem,
		Confidence:     0.88,
		AnalyzedAt:     time.Now(),
		AnalyzerVersion: "1.0",
	}

	analyzer.EnrichForAudit(result, workItem, "zen-brain", "1.0")
	richResult := analyzer.EnrichForRichAnalysis(result, workItem)

	// Serialize
	data, err := json.Marshal(richResult)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify it's valid JSON
	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON output")
	}

	// Deserialize
	var decoded analyzer.RichAnalysisResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Verify key fields survived round-trip
	if decoded.ExecutiveSummary == "" {
		t.Error("Expected ExecutiveSummary to survive round-trip")
	}
	if decoded.ReplayID == "" {
		t.Error("Expected ReplayID to survive round-trip")
	}
	if decoded.AuditTrail == nil {
		t.Fatal("Expected AuditTrail to survive round-trip")
	}
	if decoded.AuditTrail.JiraKey != "PROJ-JSON" {
		t.Errorf("Expected JiraKey to survive round-trip, got %s", decoded.AuditTrail.JiraKey)
	}
}

// TestBlock2_FilePersistence tests that history is correctly persisted to files
func TestBlock2_FilePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := analyzer.NewFileAnalysisStore(tmpDir)
	if err != nil {
		t.Fatalf("Create store: %v", err)
	}

	workItemID := "PERSIST-123"
	result := &contracts.AnalysisResult{
		WorkItem: &contracts.WorkItem{
			ID:       workItemID,
			Title:    "Persistence test",
			WorkType: contracts.WorkTypeImplementation,
		},
		Confidence: 0.95,
	}

	analyzer.EnrichForAudit(result, result.WorkItem, "zen-brain", "1.0")

	ctx := context.Background()

	// Store
	if err := store.Store(ctx, workItemID, result); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Verify file exists
	filename := filepath.Join(tmpDir, "PERSIST-123.jsonl")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("Expected file %s to exist", filename)
	}

	// Retrieve
	history, err := store.GetHistory(ctx, workItemID)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(history))
	}

	// Verify content
	if history[0].WorkItem.ID != workItemID {
		t.Errorf("Expected work item ID %s, got %s", workItemID, history[0].WorkItem.ID)
	}
	if history[0].Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %.2f", history[0].Confidence)
	}
	if history[0].AnalyzerVersion != "1.0" {
		t.Errorf("Expected analyzer version '1.0', got %s", history[0].AnalyzerVersion)
	}
}

// Simple analyzer for testing
type testAnalyzer struct {
	historyStore analyzer.AnalysisHistoryStore
}

func (a *testAnalyzer) Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error) {
	result := &contracts.AnalysisResult{
		WorkItem:       workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{},
		Confidence:     0.85,
	}

	analyzer.EnrichForAudit(result, workItem, "zen-brain", "1.0")

	if a.historyStore != nil {
		_ = a.historyStore.Store(ctx, workItem.ID, result)
	}

	return result, nil
}

func (a *testAnalyzer) GetAnalysisHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error) {
	if a.historyStore == nil {
		return nil, fmt.Errorf("history not available")
	}
	return a.historyStore.GetHistory(ctx, workItemID)
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s[1:], substr) || s[:len(substr)] == substr)
}
